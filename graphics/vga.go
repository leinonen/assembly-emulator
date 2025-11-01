package graphics

import (
	"assembly-emulator/emulator"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	ScreenWidth  = 320
	ScreenHeight = 200
	Scale        = 3 // Scale factor for display
)

// VGADisplay represents the VGA Mode 13h display
type VGADisplay struct {
	memory  *emulator.Memory
	pixels  []byte
	palette [256]color.RGBA
}

// NewVGADisplay creates a new VGA display
func NewVGADisplay(memory *emulator.Memory) *VGADisplay {
	vga := &VGADisplay{
		memory: memory,
		pixels: make([]byte, ScreenWidth*ScreenHeight*4), // RGBA
	}

	// Initialize with standard VGA default palette
	// Programs can override this via OUT instructions to ports 0x3C8/0x3C9
	vga.initializeDefaultPalette()
	return vga
}

// initializeDefaultPalette sets up the standard VGA default palette
// This matches the default VGA Mode 13h palette that programs expect
func (v *VGADisplay) initializeDefaultPalette() {
	// Standard VGA 16-color palette (EGA compatible)
	// These are the default colors 0-15
	v.palette[0] = color.RGBA{0, 0, 0, 255}       // Black
	v.palette[1] = color.RGBA{0, 0, 170, 255}     // Blue
	v.palette[2] = color.RGBA{0, 170, 0, 255}     // Green
	v.palette[3] = color.RGBA{0, 170, 170, 255}   // Cyan
	v.palette[4] = color.RGBA{170, 0, 0, 255}     // Red
	v.palette[5] = color.RGBA{170, 0, 170, 255}   // Magenta
	v.palette[6] = color.RGBA{170, 85, 0, 255}    // Brown
	v.palette[7] = color.RGBA{170, 170, 170, 255} // Light Gray
	v.palette[8] = color.RGBA{85, 85, 85, 255}    // Dark Gray
	v.palette[9] = color.RGBA{85, 85, 255, 255}   // Light Blue
	v.palette[10] = color.RGBA{85, 255, 85, 255}  // Light Green
	v.palette[11] = color.RGBA{85, 255, 255, 255} // Light Cyan
	v.palette[12] = color.RGBA{255, 85, 85, 255}  // Light Red
	v.palette[13] = color.RGBA{255, 85, 255, 255} // Light Magenta
	v.palette[14] = color.RGBA{255, 255, 85, 255} // Yellow
	v.palette[15] = color.RGBA{255, 255, 255, 255} // White

	// Colors 16-231: 216-color cube (6x6x6) - standard VGA default
	idx := 16
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				v.palette[idx] = color.RGBA{
					uint8(r * 51),
					uint8(g * 51),
					uint8(b * 51),
					255,
				}
				idx++
			}
		}
	}

	// Colors 232-255: Grayscale ramp
	for i := 0; i < 24; i++ {
		gray := uint8(8 + i*10)
		v.palette[232+i] = color.RGBA{gray, gray, gray, 255}
	}
}

// Update updates the display from VGA memory
func (v *VGADisplay) Update() error {
	vgaMem := v.memory.GetVGAMemory()

	// Convert VGA memory to RGBA pixels
	for i := 0; i < ScreenWidth*ScreenHeight; i++ {
		colorIndex := vgaMem[i]
		c := v.palette[colorIndex]

		pixelOffset := i * 4
		v.pixels[pixelOffset] = c.R
		v.pixels[pixelOffset+1] = c.G
		v.pixels[pixelOffset+2] = c.B
		v.pixels[pixelOffset+3] = c.A
	}

	return nil
}

// Draw draws the VGA display
func (v *VGADisplay) Draw(screen *ebiten.Image) {
	screen.WritePixels(v.pixels)
}

// SetPaletteColor sets a single palette entry
func (v *VGADisplay) SetPaletteColor(index byte, r, g, b byte) {
	v.palette[index] = color.RGBA{r, g, b, 255}
}

// Layout returns the screen dimensions
func (v *VGADisplay) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}

// Game wraps VGADisplay to implement ebiten.Game interface
type Game struct {
	display          *VGADisplay
	keyPressCallback func(scancode, ascii uint8) // Callback to notify CPU of key press
}

// NewGame creates a new game instance
func NewGame(memory *emulator.Memory) *Game {
	return &Game{
		display: NewVGADisplay(memory),
	}
}

// Update updates the game state
func (g *Game) Update() error {
	// Check if window is being closed
	if ebiten.IsWindowBeingClosed() {
		return ebiten.Termination
	}

	// Check for ESC key to close window
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		// Notify CPU about ESC key press
		if g.keyPressCallback != nil {
			g.keyPressCallback(0x01, 0x1B) // Scancode 0x01, ASCII 0x1B (ESC)
		}
		return ebiten.Termination
	}

	// Check for other keys and notify CPU
	if g.keyPressCallback != nil {
		// Map common keys to BIOS scancodes
		keyMap := map[ebiten.Key]struct {
			scancode uint8
			ascii    uint8
		}{
			ebiten.KeyEnter:     {0x1C, 0x0D}, // Enter
			ebiten.KeySpace:     {0x39, 0x20}, // Space
			ebiten.KeyBackspace: {0x0E, 0x08}, // Backspace
			// Add more keys as needed
		}

		for key, codes := range keyMap {
			if inpututil.IsKeyJustPressed(key) {
				g.keyPressCallback(codes.scancode, codes.ascii)
				break
			}
		}

		// Handle letter keys A-Z
		for k := ebiten.KeyA; k <= ebiten.KeyZ; k++ {
			if inpututil.IsKeyJustPressed(k) {
				ascii := uint8('a' + int(k-ebiten.KeyA))
				scancode := uint8(0x1E + int(k-ebiten.KeyA)) // BIOS scancodes for A-Z
				g.keyPressCallback(scancode, ascii)
				break
			}
		}
	}

	return g.display.Update()
}

// Draw draws the game screen
func (g *Game) Draw(screen *ebiten.Image) {
	g.display.Draw(screen)
}

// Layout returns the screen dimensions
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.display.Layout(outsideWidth, outsideHeight)
}

// RunGraphics starts the graphics window (should be called in a goroutine after mode 13h is detected)
func RunGraphics(memory *emulator.Memory) error {
	ebiten.SetWindowSize(ScreenWidth*Scale, ScreenHeight*Scale)
	ebiten.SetWindowTitle("Assembly Emulator - VGA Mode 13h")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	game := NewGame(memory)
	return ebiten.RunGame(game)
}

// RunGraphicsWithDisplay starts the graphics window with a specific VGA display
func RunGraphicsWithDisplay(display *VGADisplay, keyCallback func(scancode, ascii uint8)) error {
	ebiten.SetWindowSize(ScreenWidth*Scale, ScreenHeight*Scale)
	ebiten.SetWindowTitle("Assembly Emulator - VGA Mode 13h")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	game := &Game{
		display:          display,
		keyPressCallback: keyCallback,
	}
	return ebiten.RunGame(game)
}
