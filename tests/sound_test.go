package tests

import (
	"bytes"
	"os"
	"testing"

	"assembly-emulator/assembler"
	"assembly-emulator/machine"
)

// runWithAudio assembles the program, taps the mixer, and runs for about
// a second of virtual time. It returns the machine and the tapped samples.
func runWithAudio(t *testing.T, path string, maxInsns uint64) (*machine.Machine, []int16) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bin, err := assembler.Assemble(src, assembler.Options{Filename: path})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	m := machine.New(machine.Options{Unlimited: true, Root: t.TempDir(), Clock: fixedClock, MaxInsns: maxInsns})
	if err := m.LoadCOM(bin, ""); err != nil {
		t.Fatal(err)
	}
	var samples []int16
	m.Sound.Tap = func(mono []int16) { samples = append(samples, mono...) }
	m.Sound.Enable(0)
	if err := m.RunCycles(m.Opts.CPUHz); err != nil {
		t.Fatalf("run: %v", err)
	}
	return m, samples
}

func TestCracktroMusic(t *testing.T) {
	m, samples := runWithAudio(t, "../examples/cracktro.asm", 20_000_000)
	if m.OPL.Writes < 100 {
		t.Errorf("only %d OPL2 register writes", m.OPL.Writes)
	}
	if m.OPL.KeyOns == 0 {
		t.Errorf("no OPL2 key-on")
	}
	var peak int16
	for _, s := range samples {
		if s < 0 {
			s = -s
		}
		if s > peak {
			peak = s
		}
	}
	if len(samples) < 47000 || len(samples) > 49000 {
		t.Errorf("%d samples for ~1 s", len(samples))
	}
	if peak < 500 {
		t.Errorf("audio peak %d, want music", peak)
	}

	m.TypeKey(0x01, false) // ESC
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if !m.Exited() {
		t.Errorf("program did not exit on ESC")
	}
	if mask := m.OPL.KeyOnMask(); mask != 0 {
		t.Errorf("channels still keyed on at exit: %03x", mask)
	}
}

func TestCracktroAudioDeterministic(t *testing.T) {
	_, a := runWithAudio(t, "../examples/cracktro.asm", 20_000_000)
	_, b := runWithAudio(t, "../examples/cracktro.asm", 20_000_000)
	if len(a) != len(b) {
		t.Fatalf("sample counts differ: %d vs %d", len(a), len(b))
	}
	ab := make([]byte, 0, len(a)*2)
	bb := make([]byte, 0, len(b)*2)
	for i := range a {
		ab = append(ab, byte(a[i]), byte(a[i]>>8))
		bb = append(bb, byte(b[i]), byte(b[i]>>8))
	}
	if !bytes.Equal(ab, bb) {
		t.Fatalf("audio differs between identical runs")
	}
}
