package main

import (
	"bytes"
	"encoding/binary"
	"os"

	emio "assembly-emulator/emulator/io"
)

// wavSink collects the mixer's mono samples and writes them as a RIFF WAV.
type wavSink struct {
	buf bytes.Buffer
}

func newWAVSink() *wavSink { return &wavSink{} }

// Tap is installed as Mixer.Tap; it runs on the machine goroutine.
func (w *wavSink) Tap(mono []int16) {
	for _, s := range mono {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(s))
		w.buf.Write(b[:])
	}
}

// WriteFile writes the collected samples with a 16-bit mono PCM header.
func (w *wavSink) WriteFile(path string) error {
	data := w.buf.Bytes()
	hdr := make([]byte, 44)
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(data)))
	copy(hdr[8:], "WAVE")
	copy(hdr[12:], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:], 1) // mono
	binary.LittleEndian.PutUint32(hdr[24:], emio.SampleRate)
	binary.LittleEndian.PutUint32(hdr[28:], emio.SampleRate*2)
	binary.LittleEndian.PutUint16(hdr[32:], 2)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(data)))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(hdr); err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}
