package io

import "testing"

const testCPUHz = 40_000_000

func oplWrite(o *OPL2, reg, v uint8) {
	o.Out8(0x388, reg)
	o.Out8(0x389, v)
}

// programCarrier sets channel 0 up as a pure sine carrier (additive
// algorithm with the modulator silenced) at A-4, full volume, instant attack.
func programCarrier(o *OPL2) {
	oplWrite(o, 0x20, 0x01) // modulator mult 1
	oplWrite(o, 0x40, 0x3F) // modulator silent
	oplWrite(o, 0x23, 0x21) // carrier mult 1, sustaining envelope
	oplWrite(o, 0x43, 0x00) // carrier full volume
	oplWrite(o, 0x63, 0xF0) // carrier AR 15, DR 0
	oplWrite(o, 0x83, 0x0F) // carrier SL 0, RR 15
	oplWrite(o, 0xC0, 0x01) // additive
	oplWrite(o, 0xA0, 0x41) // F-number 0x241 (A-4 at block 4)
	oplWrite(o, 0xB0, 0x32) // key on, block 4, F-number high 2
}

func TestOPL2Detect(t *testing.T) {
	o := NewOPL2(testCPUHz, SampleRate)
	oplWrite(o, 0x04, 0x60) // mask both timers
	oplWrite(o, 0x04, 0x80) // reset IRQ
	if s := o.In8(0x388); s&0xE0 != 0 {
		t.Fatalf("status after reset = %02X, want bits 5-7 clear", s)
	}
	if s := o.In8(0x388); s&0x06 != 0x06 {
		t.Fatalf("status low bits = %02X, want 06", s)
	}
	oplWrite(o, 0x02, 0xFF) // timer 1 preset: overflow after one 80 us tick
	oplWrite(o, 0x04, 0x21) // unmask + start timer 1
	o.Advance(3199)         // 79.975 us at 40 MHz
	if s := o.In8(0x388); s&0xE0 != 0 {
		t.Fatalf("timer flagged early: %02X", s)
	}
	o.Advance(2)
	if s := o.In8(0x388); s&0xE0 != 0xC0 {
		t.Fatalf("status after 80 us = %02X, want C0", s)
	}
	oplWrite(o, 0x04, 0x60)
	oplWrite(o, 0x04, 0x80)
	if s := o.In8(0x388); s&0xE0 != 0 {
		t.Fatalf("status after second reset = %02X", s)
	}
}

func TestOPL2TimerMasked(t *testing.T) {
	o := NewOPL2(testCPUHz, SampleRate)
	oplWrite(o, 0x03, 0x00)
	oplWrite(o, 0x04, 0x22) // start timer 2, masked
	o.Advance(testCPUHz)    // a whole second
	if s := o.In8(0x388); s&0xE0 != 0 {
		t.Fatalf("masked timer set flags: %02X", s)
	}
	oplWrite(o, 0x04, 0x02) // unmask
	o.Advance(uint64(256) * 320 * testCPUHz / 1_000_000)
	if s := o.In8(0x388); s&0xA0 != 0xA0 {
		t.Fatalf("timer 2 not flagged: %02X", s)
	}
}

func TestOPL2SilentAtReset(t *testing.T) {
	o := NewOPL2(testCPUHz, SampleRate)
	buf := make([]int32, SampleRate/10)
	o.Render(buf)
	for i, v := range buf {
		if v != 0 {
			t.Fatalf("sample %d = %d, want silence", i, v)
		}
	}
}

func peak(buf []int32) int32 {
	var m int32
	for _, v := range buf {
		if v < 0 {
			v = -v
		}
		if v > m {
			m = v
		}
	}
	return m
}

func TestOPL2KeyOnProducesSound(t *testing.T) {
	o := NewOPL2(testCPUHz, SampleRate)
	programCarrier(o)
	if o.KeyOns != 1 || o.KeyOnMask() != 1 {
		t.Fatalf("KeyOns=%d mask=%x", o.KeyOns, o.KeyOnMask())
	}
	buf := make([]int32, SampleRate/10)
	o.Render(buf)
	if p := peak(buf); p < 1000 {
		t.Fatalf("peak %d, want a loud carrier", p)
	}
}

func TestOPL2Frequency(t *testing.T) {
	o := NewOPL2(testCPUHz, SampleRate)
	programCarrier(o)
	buf := make([]int32, SampleRate)
	o.Render(buf)
	crossings := 0
	for i := 1; i < len(buf); i++ {
		if (buf[i-1] < 0) != (buf[i] < 0) {
			crossings++
		}
	}
	// F-number 0x241 at block 4 is 440 Hz on the real chip.
	want := 2 * 440
	if crossings < want*98/100 || crossings > want*102/100 {
		t.Fatalf("%d zero crossings in 1 s, want ~%d", crossings, want)
	}
}

func TestOPL2ReleaseToSilence(t *testing.T) {
	o := NewOPL2(testCPUHz, SampleRate)
	programCarrier(o)
	buf := make([]int32, SampleRate/10)
	o.Render(buf)
	oplWrite(o, 0xB0, 0x12) // key off
	if o.KeyOnMask() != 0 {
		t.Fatalf("key still on")
	}
	o.Render(buf[:SampleRate/20]) // 50 ms at RR 15
	tail := make([]int32, 480)
	o.Render(tail)
	if p := peak(tail); p != 0 {
		t.Fatalf("still sounding after release: peak %d", p)
	}
	if o.op[3].state != egOff {
		t.Fatalf("carrier state %d, want off", o.op[3].state)
	}
}

func TestOPL2WaveformsNeedWSE(t *testing.T) {
	render := func(wse bool) []int32 {
		o := NewOPL2(testCPUHz, SampleRate)
		if wse {
			oplWrite(o, 0x01, 0x20)
		}
		oplWrite(o, 0xE3, 0x01) // carrier half sine
		programCarrier(o)
		buf := make([]int32, 2000)
		o.Render(buf)
		return buf
	}
	plain := render(false)
	ref := NewOPL2(testCPUHz, SampleRate)
	programCarrier(ref)
	want := make([]int32, 2000)
	ref.Render(want)
	for i := range plain {
		if plain[i] != want[i] {
			t.Fatalf("waveform applied without WSE at sample %d", i)
		}
	}
	half := render(true)
	neg := 0
	for _, v := range half {
		if v < 0 {
			neg++
		}
	}
	if neg != 0 {
		t.Fatalf("half sine has %d negative samples", neg)
	}
}
