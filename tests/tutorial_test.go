package tests

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"assembly-emulator/assembler"
)

// TestTutorial assembles and runs every complete program listed in
// docs/TUTORIAL.md, so the guide cannot drift away from the emulator.
func TestTutorial(t *testing.T) {
	md, err := os.ReadFile("../docs/TUTORIAL.md")
	if err != nil {
		t.Skip("no tutorial")
	}
	blocks := regexp.MustCompile("(?s)```nasm\n(.*?)```").FindAllStringSubmatch(string(md), -1)
	name := regexp.MustCompile(`^; (\S+\.asm)`)
	found := 0
	for _, b := range blocks {
		src := b[1]
		if !strings.Contains(src, "org 100h") {
			continue // an excerpt, not a program
		}
		file := "listing"
		if m := name.FindStringSubmatch(src); m != nil {
			file = m[1]
		}
		found++
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			bin, err := assembler.Assemble([]byte(src), assembler.Options{Filename: file})
			if err != nil {
				t.Fatalf("assemble: %v", err)
			}
			r := runProgram(t, bin, 40_000_000)
			if r.mode == 0x13 && !r.nonblank {
				t.Errorf("mode 13h but the screen is blank")
			}
			if !r.m.Exited() {
				t.Errorf("did not exit after ESC")
			}
		})
	}
	if found < 10 {
		t.Errorf("only %d programs found in the tutorial", found)
	}
}
