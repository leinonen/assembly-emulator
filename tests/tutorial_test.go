package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"assembly-emulator/assembler"
)

// TestTutorial assembles and runs every complete program listed in
// docs/tutorial, so the guide cannot drift away from the emulator.
func TestTutorial(t *testing.T) {
	pages, _ := filepath.Glob("../docs/tutorial/*.md")
	if len(pages) == 0 {
		t.Skip("no tutorial")
	}
	fence := regexp.MustCompile("(?s)```nasm\n(.*?)```")
	name := regexp.MustCompile(`^; (\S+\.asm)`)
	var blocks [][]string
	for _, p := range pages {
		md, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, fence.FindAllStringSubmatch(string(md), -1)...)
	}
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
