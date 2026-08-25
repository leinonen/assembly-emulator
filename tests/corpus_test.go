// Package tests runs every example and (when available) a corpus of
// real-world DOS programs on the emulator headlessly.
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"assembly-emulator/assembler"
	"assembly-emulator/machine"
)

func fixedClock() time.Time { return time.Date(1995, 6, 1, 12, 0, 0, 0, time.UTC) }

// runProgram runs a .COM image for about a second of virtual time, records
// the video mode and whether the screen has content, then presses ESC
// (make+break) so key-driven demos exit, and runs to completion or the
// instruction limit.
type runResult struct {
	m        *machine.Machine
	mode     uint8
	nonblank bool
}

func runProgram(t *testing.T, image []byte, maxInsns uint64) runResult {
	t.Helper()
	m := machine.New(machine.Options{Unlimited: true, Root: t.TempDir(), Clock: fixedClock, MaxInsns: maxInsns})
	if err := m.LoadCOM(image, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.RunCycles(m.Opts.CPUHz); err != nil { // ~1 s virtual
		t.Fatalf("run: %v", err)
	}
	r := runResult{m: m, mode: m.VGA.Mode}
	for p := 0; p < 4 && !r.nonblank; p++ {
		for _, b := range m.VGA.Planes[p][:16000] {
			if b != 0 {
				r.nonblank = true
				break
			}
		}
	}
	if !m.Exited() {
		m.TypeKey(0x01, false) // ESC
		if err := m.Run(); err != nil {
			t.Fatalf("run: %v", err)
		}
	}
	return r
}

func TestExamples(t *testing.T) {
	files, _ := filepath.Glob("../examples/*.asm")
	if len(files) == 0 {
		t.Skip("no examples")
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			bin, err := assembler.Assemble(src, assembler.Options{Filename: f})
			if err != nil {
				t.Fatalf("assemble: %v", err)
			}
			r := runProgram(t, bin, 20_000_000)
			if r.mode != 0x13 {
				t.Errorf("expected mode 13h, got %02Xh", r.mode)
			}
			if !r.nonblank && !strings.Contains(f, "test-buffer") {
				t.Errorf("screen is blank")
			}
		})
	}
}

// TestCorpus assembles and runs real-world programs from $ASM_EMU_CORPUS_DIR
// (see README): each entry is a directory containing .asm and/or .com files.
func TestCorpus(t *testing.T) {
	dir := os.Getenv("ASM_EMU_CORPUS_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache", "asm-emu", "corpus")
	}
	programs := []struct {
		asm, com string
		checkBin bool // shipped .com must match our assembly
	}{
		{"fire-asm/fire.asm", "fire-asm/fire.com", true},
		{"memories/fx2.asm", "memories/fx2.com", true},
		{"memories/memories-test.asm", "", false},
		{"", "blobz/blobz.com", false},
	}
	found := false
	for _, p := range programs {
		name := p.asm
		if name == "" {
			name = p.com
		}
		t.Run(name, func(t *testing.T) {
			var image []byte
			if p.asm != "" {
				src, err := os.ReadFile(filepath.Join(dir, p.asm))
				if err != nil {
					t.Skipf("corpus file missing: %v", err)
				}
				found = true
				bin, err := assembler.Assemble(src, assembler.Options{Filename: p.asm})
				if err != nil {
					t.Fatalf("assemble: %v", err)
				}
				if p.checkBin {
					want, err := os.ReadFile(filepath.Join(dir, p.com))
					if err == nil && string(want) != string(bin) {
						t.Errorf("assembled output differs from shipped %s", p.com)
					}
				}
				image = bin
			} else {
				b, err := os.ReadFile(filepath.Join(dir, p.com))
				if err != nil {
					t.Skipf("corpus file missing: %v", err)
				}
				found = true
				image = b
			}
			r := runProgram(t, image, 30_000_000)
			if r.mode != 0x13 {
				t.Errorf("mode %02Xh after 1s", r.mode)
			}
			if !r.nonblank {
				t.Errorf("screen is blank")
			}
		})
	}
	if !found {
		t.Skip("corpus not available")
	}
}
