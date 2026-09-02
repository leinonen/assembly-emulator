package io

import (
	"encoding/binary"
	"sync"
)

// SampleRate is the audio output rate of the machine in Hz.
const SampleRate = 48000

// Mixer converts CPU cycles into audio samples from the OPL2 and the
// speaker. Samples are produced on the machine goroutine in Advance and
// consumed by an audio front end through Read (signed 16-bit little-endian
// stereo, as Ebiten's audio player expects) or by a Tap for deterministic
// capture. Until Enable is called the mixer does nothing, so headless runs
// pay one branch per instruction.
type Mixer struct {
	OPL *OPL2
	Spk *Speaker

	// Tap, when set, receives every rendered mono sample block on the
	// machine goroutine (used by the WAV writer).
	Tap func(mono []int16)

	cpuHz, rate uint64
	accum       uint64 // cycles*rate not yet turned into samples
	enabled     bool
	scratch     [256]int32
	tapBuf      [256]int16

	mu   sync.Mutex
	ring []int16
	rd   int
	wr   int
	n    int
	last int16

	// Dropped counts samples discarded because the ring was full;
	// Underruns counts frames the reader had to invent.
	Dropped   uint64
	Underruns uint64
}

// NewMixer returns a disabled mixer for the given sources.
func NewMixer(opl *OPL2, spk *Speaker, cpuHz uint64, rate int) *Mixer {
	return &Mixer{OPL: opl, Spk: spk, cpuHz: cpuHz, rate: uint64(rate)}
}

// Enable starts sample generation. ringFrames is the capacity of the buffer
// Read drains (0 keeps only the Tap). Calling it again keeps the larger
// buffer.
func (x *Mixer) Enable(ringFrames int) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.enabled = true
	if ringFrames > len(x.ring) {
		x.ring = make([]int16, ringFrames)
		x.rd, x.wr, x.n = 0, 0, 0
	}
}

// Enabled reports whether samples are being generated.
func (x *Mixer) Enabled() bool { return x.enabled }

// Advance renders the samples covered by the given CPU cycles.
func (x *Mixer) Advance(cycles uint64) {
	if !x.enabled {
		return
	}
	x.accum += cycles * x.rate
	if x.accum < x.cpuHz {
		return
	}
	n := x.accum / x.cpuHz
	x.accum -= n * x.cpuHz
	for n > 0 {
		k := len(x.scratch)
		if n < uint64(k) {
			k = int(n)
		}
		buf := x.scratch[:k]
		clear(buf)
		x.OPL.Render(buf)
		x.Spk.Render(buf)
		x.push(buf)
		n -= uint64(k)
	}
}

func (x *Mixer) push(buf []int32) {
	out := x.tapBuf[:len(buf)]
	for i, v := range buf {
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[i] = int16(v)
	}
	if len(x.ring) > 0 {
		x.mu.Lock()
		for _, s := range out {
			if x.n == len(x.ring) {
				// Overrun: drop the oldest sample so latency stays bounded.
				x.rd++
				if x.rd == len(x.ring) {
					x.rd = 0
				}
				x.n--
				x.Dropped++
			}
			x.ring[x.wr] = s
			x.wr++
			if x.wr == len(x.ring) {
				x.wr = 0
			}
			x.n++
		}
		x.mu.Unlock()
	}
	if x.Tap != nil {
		x.Tap(out)
	}
}

// Buffered returns the number of frames waiting to be read.
func (x *Mixer) Buffered() int {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.n
}

// Read fills p with 16-bit stereo frames. It never blocks and never
// returns EOF: when the ring is empty the last sample decays to silence.
func (x *Mixer) Read(p []byte) (int, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	frames := len(p) / 4
	for i := 0; i < frames; i++ {
		s := x.last
		if x.n > 0 {
			s = x.ring[x.rd]
			x.rd++
			if x.rd == len(x.ring) {
				x.rd = 0
			}
			x.n--
			x.last = s
		} else {
			x.last -= x.last >> 4
			x.Underruns++
		}
		binary.LittleEndian.PutUint16(p[i*4:], uint16(s))
		binary.LittleEndian.PutUint16(p[i*4+2:], uint16(s))
	}
	return frames * 4, nil
}
