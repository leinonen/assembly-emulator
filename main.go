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

			// Run graphics in goroutine
			go func() {
				if err := graphics.RunGraphicsWithDisplay(vgaDisplay); err != nil {
					fmt.Fprintf(os.Stderr, "Graphics error: %v\n", err)
				}
				close(graphicsDone)
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
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Program halted.")
	fmt.Printf("Final CPU state: %s\n", cpu.String())

	// If graphics was started, wait for it to close
	graphicsMutex.Lock()
	if graphicsStarted {
		graphicsMutex.Unlock()
		fmt.Println("Graphics window is open. Press ESC or close the window to exit.")
		// Wait for graphics window to close
		<-graphicsDone
	} else {
		graphicsMutex.Unlock()
	}
}
