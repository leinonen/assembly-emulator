// Command asm-emu runs DOS .COM programs (and, once assembled, NASM-syntax
// sources) on an emulated 386 PC with VGA graphics.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"assembly-emulator/machine"
)

func usage() {
	fmt.Fprintf(os.Stderr, `usage:
  asm-emu run [flags] <file.com|file.asm> [args...]   run a program
  asm-emu asm <file.asm> -o <file.com>                assemble to a .COM

run flags:
`)
	flag.PrintDefaults()
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		os.Exit(runCmd(os.Args[2:]))
	case "asm":
		os.Exit(asmCmd(os.Args[2:]))
	default:
		// Bare file argument: treat as "run".
		os.Exit(runCmd(os.Args[1:]))
	}
}

func runCmd(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	speed := fs.String("speed", "40", "CPU speed in MHz, or 'unlimited'")
	headless := fs.Bool("headless", false, "run without a window")
	maxInsns := fs.Uint64("max-insns", 0, "stop after this many instructions (0 = unlimited)")
	gif := fs.String("gif", "", "record a GIF of the screen to this file")
	gifFrames := fs.Int("gif-frames", 300, "number of frames to record with -gif")
	trace := fs.Bool("stats", false, "print execution statistics on exit")
	shot := fs.String("screenshot", "", "write the final screen to this PNG file")
	wav := fs.String("wav", "", "write the audio output to this WAV file (mono 16-bit 48 kHz)")
	fs.Usage = usage
	fs.Parse(args)
	if fs.NArg() < 1 {
		usage()
		return 2
	}
	path := fs.Arg(0)
	tail := strings.Join(fs.Args()[1:], " ")
	if tail != "" {
		tail = " " + tail
	}

	opts := machine.Options{Root: filepath.Dir(path), Stdout: os.Stdout, MaxInsns: *maxInsns}
	if *speed == "unlimited" {
		opts.Unlimited = true
	} else {
		mhz, err := strconv.ParseFloat(*speed, 64)
		if err != nil || mhz <= 0 {
			fmt.Fprintf(os.Stderr, "invalid -speed %q\n", *speed)
			return 2
		}
		opts.CPUHz = uint64(mhz * 1e6)
	}

	image, err := loadProgram(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	m := machine.New(opts)
	if err := m.LoadCOM(image, tail); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var wavOut *wavSink
	if *wav != "" {
		wavOut = newWAVSink()
		m.Sound.Tap = wavOut.Tap
		m.Sound.Enable(0)
	}

	var runErr error
	switch {
	case *gif != "":
		runErr = recordGIF(m, *gif, *gifFrames)
	case *headless:
		runErr = m.Run()
	default:
		runErr = runWindow(m, filepath.Base(path))
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, "error:", runErr)
	}
	if wavOut != nil {
		if err := wavOut.WriteFile(*wav); err != nil {
			fmt.Fprintln(os.Stderr, "wav:", err)
		}
	}
	if *shot != "" {
		m.ForceFrame()
		if err := writeScreenshot(m, *shot); err != nil {
			fmt.Fprintln(os.Stderr, "screenshot:", err)
		}
	}
	if *trace {
		secs := time.Since(startTime).Seconds()
		fmt.Fprintf(os.Stderr, "\n%d instructions, %d cycles (%.2fs virtual), %.1f MIPS, video mode %02Xh, %d frames, CS:IP=%04X:%04X\n",
			m.CPU.InsnCount, m.CPU.Cycles, float64(m.CPU.Cycles)/float64(m.Opts.CPUHz), float64(m.CPU.InsnCount)/secs/1e6, m.VGA.Mode, m.VGA.Frames, m.CPU.CS(), m.CPU.IP())
	}
	if runErr != nil {
		return 1
	}
	return m.ExitCode()
}

var startTime = time.Now()

// loadProgram returns the .COM image for a path, assembling sources.
func loadProgram(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".asm") {
		return assembleSource(path, data)
	}
	return data, nil
}
