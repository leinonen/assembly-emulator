package io

import "testing"

func TestSpeakerTone(t *testing.T) {
	pic := NewPIC()
	pit := NewPIT(pic, testCPUHz)
	spk := NewSpeaker(pit, SampleRate)

	pit.Out8(0x43, 0xB6) // channel 2, lsb+msb, mode 3
	pit.Out8(0x42, 0xA9) // 1193 -> ~1000 Hz
	pit.Out8(0x42, 0x04)
	pit.Out8(0x61, 0x03) // gate + data

	buf := make([]int32, SampleRate)
	spk.Render(buf)
	crossings := 0
	for i := 1; i < len(buf); i++ {
		if (buf[i-1] < 0) != (buf[i] < 0) {
			crossings++
		}
	}
	if crossings < 1960 || crossings > 2040 {
		t.Fatalf("%d zero crossings, want ~2000", crossings)
	}
	if p := peak(buf); p < speakerAmp/2 {
		t.Fatalf("peak %d too quiet", p)
	}

	pit.Out8(0x61, 0x00)
	clear(buf)
	spk.Render(buf[:SampleRate/10])
	tail := buf[SampleRate/20 : SampleRate/10]
	if p := peak(tail); p != 0 {
		t.Fatalf("speaker still sounding after gate off: peak %d", p)
	}
}

func TestMixerSampleCount(t *testing.T) {
	run := func(chunk uint64) (int, []int16) {
		pic := NewPIC()
		pit := NewPIT(pic, testCPUHz)
		opl := NewOPL2(testCPUHz, SampleRate)
		programCarrier(opl)
		mix := NewMixer(opl, NewSpeaker(pit, SampleRate), testCPUHz, SampleRate)
		var got []int16
		mix.Tap = func(m []int16) { got = append(got, m...) }
		mix.Enable(0)
		for c := uint64(0); c < testCPUHz; c += chunk {
			mix.Advance(chunk)
		}
		return len(got), got
	}
	n1, s1 := run(testCPUHz / 1000) // 1 ms chunks
	n2, s2 := run(3)                // tiny chunks
	if n1 != SampleRate || n2 != SampleRate {
		t.Fatalf("got %d and %d samples, want %d", n1, n2, SampleRate)
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			t.Fatalf("sample %d differs between chunkings: %d vs %d", i, s1[i], s2[i])
		}
	}
}

func TestMixerRead(t *testing.T) {
	pic := NewPIC()
	pit := NewPIT(pic, testCPUHz)
	opl := NewOPL2(testCPUHz, SampleRate)
	mix := NewMixer(opl, NewSpeaker(pit, SampleRate), testCPUHz, SampleRate)

	// Disabled: Advance is a no-op and Read yields silence of full length.
	mix.Advance(testCPUHz)
	p := make([]byte, 400)
	n, err := mix.Read(p)
	if err != nil || n != 400 {
		t.Fatalf("Read = %d, %v", n, err)
	}
	for _, b := range p {
		if b != 0 {
			t.Fatalf("silence expected from an empty ring")
		}
	}

	mix.Enable(100)
	programCarrier(opl)
	mix.Advance(testCPUHz / 100) // 480 samples into a 100 slot ring
	if mix.Buffered() != 100 || mix.Dropped != 380 {
		t.Fatalf("buffered %d dropped %d", mix.Buffered(), mix.Dropped)
	}
	n, _ = mix.Read(p)
	if n != 400 || mix.Buffered() != 0 {
		t.Fatalf("Read drained %d bytes, %d left", n, mix.Buffered())
	}
	// The stereo frames carry the same sample in both channels.
	if p[0] != p[2] || p[1] != p[3] {
		t.Fatalf("channels differ: % x", p[:4])
	}
}
