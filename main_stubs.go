package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"assembly-emulator/assembler"
	"assembly-emulator/graphics"
	"assembly-emulator/machine"
)

func assembleSource(path string, src []byte) ([]byte, error) {
	return assembler.Assemble(src, assembler.Options{Filename: path, IncludeDirs: []string{filepath.Dir(path)}})
}

func asmCmd(args []string) int {
	var in, out, lst string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-o" && i+1 < len(args):
			out = args[i+1]
			i++
		case args[i] == "-l" && i+1 < len(args):
			lst = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-o="):
			out = args[i][3:]
		case in == "":
			in = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: asm-emu asm <file.asm> [-o out.com]")
			return 2
		}
	}
	if in == "" {
		fmt.Fprintln(os.Stderr, "usage: asm-emu asm <file.asm> [-o out.com]")
		return 2
	}
	src, err := os.ReadFile(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	opts := assembler.Options{Filename: in, IncludeDirs: []string{filepath.Dir(in)}}
	var lstFile *os.File
	if lst != "" {
		f, err := os.Create(lst)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer f.Close()
		lstFile = f
		lines := map[string][]string{}
		opts.Listing = func(addr int64, code []byte, file string, line int) {
			src, ok := lines[file]
			if !ok {
				data, _ := os.ReadFile(file)
				src = strings.Split(string(data), "\n")
				lines[file] = src
			}
			text := ""
			if line-1 < len(src) && line > 0 {
				text = src[line-1]
			}
			hexs := fmt.Sprintf("% X", code)
			if len(hexs) > 24 {
				hexs = hexs[:24] + "..."
			}
			fmt.Fprintf(f, "%05X  %-27s %s\n", addr, hexs, text)
		}
	}
	_ = lstFile
	bin, err := assembler.Assemble(src, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dst := out
	if dst == "" {
		dst = in[:len(in)-len(filepath.Ext(in))] + ".com"
	}
	if err := os.WriteFile(dst, bin, 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runWindow(m *machine.Machine, title string) error {
	return graphics.Run(m, title)
}

func recordGIF(m *machine.Machine, path string, frames int) error {
	return graphics.RecordGIF(m, path, frames)
}
