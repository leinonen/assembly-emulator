package main

import (
	"assembly-emulator/assembler"
	"assembly-emulator/emulator"
	"assembly-emulator/graphics"
	"fmt"
	"os"
	"sync"
)

func main() {
	// Check for command-line arguments
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <assembly-file.asm>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s examples/noise.asm\n", os.Args[0])
		os.Exit(1)
	}

	asmFile := os.Args[1]

	// Read assembly file
	source, err := os.ReadFile(asmFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", asmFile, err)
		os.Exit(1)
	}

	fmt.Printf("Assembling %s...\n", asmFile)

	// Assemble the code
	lexer := assembler.NewLexer(string(source))
	tokens, err := lexer.Tokenize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Lexer error: %v\n", err)
		os.Exit(1)
	}

	parser := assembler.NewParser(tokens)
	bytecode, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parser error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Assembly successful! Generated %d bytes of code.\n", len(bytecode))

	// Create CPU and load program
	cpu := emulator.NewCPU()
	cpu.Memory.LoadProgram(0, bytecode)

	// Setup graphics initialization callback
	var graphicsStarted bool
	var graphicsMutex sync.Mutex
	graphicsDone := make(chan struct{})
	var vgaDisplay *graphics.VGADisplay

	cpu.Mode13hCallback = func() {
		graphicsMutex.Lock()
		defer graphicsMutex.Unlock()
		if !graphicsStarted {
			graphicsStarted = true
			fmt.Println("Mode 13h detected - initializing graphics...")

			// Create VGA display immediately (before releasing mutex)
			vgaDisplay = graphics.NewVGADisplay(cpu.Memory)

			// Create keyboard callback
			keyCallback := func(scancode, ascii uint8) {
				cpu.SetKeyPress(scancode, ascii)
			}

			// Run graphics in goroutine
			go func() {
				if err := graphics.RunGraphicsWithDisplay(vgaDisplay, keyCallback); err != nil {
					fmt.Fprintf(os.Stderr, "Graphics error: %v\n", err)
				}
				close(graphicsDone)
				// Signal CPU to stop when graphics window closes
				cpu.Stop()
			}()
		}
	}

	// Setup palette manipulation callback
	cpu.SetPaletteCallback = func(index byte, r, g, b byte) {
		graphicsMutex.Lock()
		defer graphicsMutex.Unlock()
		if vgaDisplay != nil {
			vgaDisplay.SetPaletteColor(index, r, g, b)
		}
	}

	fmt.Println("Running program...")

	// Run the program
	err = cpu.Run()
	if err != nil {
		// Check if it's a stop signal (not a real error)
		if err.Error() != "CPU stopped by external signal" {
			fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
			os.Exit(1)
		}
		// If stopped by external signal, this is normal (window closed)
		fmt.Println("Program stopped (window closed).")
	} else {
		fmt.Println("Program halted.")
		fmt.Printf("Final CPU state: %s\n", cpu.String())
	}

	// If graphics was started, wait for it to close
	graphicsMutex.Lock()
	if graphicsStarted {
		graphicsMutex.Unlock()
		// Only wait if we haven't already been stopped
		if err == nil {
			fmt.Println("Graphics window is open. Press ESC or close the window to exit.")
			// Wait for graphics window to close
			<-graphicsDone
		}
	} else {
		graphicsMutex.Unlock()
	}
}
