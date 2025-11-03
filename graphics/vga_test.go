package graphics

import (
	"assembly-emulator/emulator"
	"image/color"
	"testing"
)

// TestNewVGADisplay tests VGA display creation
func TestNewVGADisplay(t *testing.T) {
	memory := emulator.NewMemory()
	vga := NewVGADisplay(memory)

	if vga == nil {
		t.Fatal("NewVGADisplay returned nil")
	}

	if vga.memory != memory {
		t.Error("VGA display memory reference is incorrect")
	}

	if len(vga.pixels) != ScreenWidth*ScreenHeight*4 {
		t.Errorf("Expected pixel buffer size %d, got %d", ScreenWidth*ScreenHeight*4, len(vga.pixels))
	}

	if vga.screenBuffer == nil {
		t.Error("Screen buffer was not initialized")
	}
}

// TestDefaultPalette tests the default VGA palette initialization
func TestDefaultPalette(t *testing.T) {
	memory := emulator.NewMemory()
	vga := NewVGADisplay(memory)

	// Test standard 16 EGA colors
	tests := []struct {
		index    byte
		expected color.RGBA
		name     string
	}{
		{0, color.RGBA{0, 0, 0, 255}, "Black"},
		{1, color.RGBA{0, 0, 170, 255}, "Blue"},
		{2, color.RGBA{0, 170, 0, 255}, "Green"},
		{3, color.RGBA{0, 170, 170, 255}, "Cyan"},
		{4, color.RGBA{170, 0, 0, 255}, "Red"},
		{5, color.RGBA{170, 0, 170, 255}, "Magenta"},
		{6, color.RGBA{170, 85, 0, 255}, "Brown"},
		{7, color.RGBA{170, 170, 170, 255}, "Light Gray"},
		{8, color.RGBA{85, 85, 85, 255}, "Dark Gray"},
		{9, color.RGBA{85, 85, 255, 255}, "Light Blue"},
		{10, color.RGBA{85, 255, 85, 255}, "Light Green"},
		{11, color.RGBA{85, 255, 255, 255}, "Light Cyan"},
		{12, color.RGBA{255, 85, 85, 255}, "Light Red"},
		{13, color.RGBA{255, 85, 255, 255}, "Light Magenta"},
		{14, color.RGBA{255, 255, 85, 255}, "Yellow"},
		{15, color.RGBA{255, 255, 255, 255}, "White"},
	}

	for _, tt := range tests {
		got := vga.palette[tt.index]
		if got != tt.expected {
			t.Errorf("Palette[%d] (%s): expected %v, got %v", tt.index, tt.name, tt.expected, got)
		}
	}
}

// TestSetPaletteColor tests setting custom palette colors
func TestSetPaletteColor(t *testing.T) {
	memory := emulator.NewMemory()
	vga := NewVGADisplay(memory)

	// Set a custom color
	vga.SetPaletteColor(42, 128, 64, 200)

	expected := color.RGBA{128, 64, 200, 255}
	got := vga.palette[42]

	if got != expected {
		t.Errorf("SetPaletteColor(42, 128, 64, 200): expected %v, got %v", expected, got)
	}
}

// TestVGAMemoryUpdate tests updating pixels from VGA memory
func TestVGAMemoryUpdate(t *testing.T) {
	memory := emulator.NewMemory()
	vga := NewVGADisplay(memory)

	// Write some color indices to VGA memory
	memory.WriteByteLinear(0xA0000, 15) // White (first pixel)
	memory.WriteByteLinear(0xA0001, 4)  // Red (second pixel)
	memory.WriteByteLinear(0xA0002, 2)  // Green (third pixel)

	// Update the display
	err := vga.Update()
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	// Check that pixels were correctly converted to RGBA
	// First pixel (white)
	if vga.pixels[0] != 255 || vga.pixels[1] != 255 || vga.pixels[2] != 255 || vga.pixels[3] != 255 {
		t.Errorf("First pixel (white): expected RGBA(255,255,255,255), got RGBA(%d,%d,%d,%d)",
			vga.pixels[0], vga.pixels[1], vga.pixels[2], vga.pixels[3])
	}

	// Second pixel (red)
	if vga.pixels[4] != 170 || vga.pixels[5] != 0 || vga.pixels[6] != 0 || vga.pixels[7] != 255 {
		t.Errorf("Second pixel (red): expected RGBA(170,0,0,255), got RGBA(%d,%d,%d,%d)",
			vga.pixels[4], vga.pixels[5], vga.pixels[6], vga.pixels[7])
	}

	// Third pixel (green)
	if vga.pixels[8] != 0 || vga.pixels[9] != 170 || vga.pixels[10] != 0 || vga.pixels[11] != 255 {
		t.Errorf("Third pixel (green): expected RGBA(0,170,0,255), got RGBA(%d,%d,%d,%d)",
			vga.pixels[8], vga.pixels[9], vga.pixels[10], vga.pixels[11])
	}
}

// TestVGAMemoryFullScreen tests updating entire screen from VGA memory
func TestVGAMemoryFullScreen(t *testing.T) {
	memory := emulator.NewMemory()
	vga := NewVGADisplay(memory)

	// Fill VGA memory with a pattern
	for i := 0; i < ScreenWidth*ScreenHeight; i++ {
		memory.WriteByteLinear(0xA0000+uint32(i), byte(i%256))
	}

	// Update the display
	err := vga.Update()
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	// Verify a few random pixels
	testPixels := []int{0, 100, 1000, 10000, ScreenWidth*ScreenHeight - 1}
	for _, pixelIdx := range testPixels {
		colorIdx := byte(pixelIdx % 256)
		expectedColor := vga.palette[colorIdx]

		offset := pixelIdx * 4
		gotR := vga.pixels[offset]
		gotG := vga.pixels[offset+1]
		gotB := vga.pixels[offset+2]
		gotA := vga.pixels[offset+3]

		if gotR != expectedColor.R || gotG != expectedColor.G || gotB != expectedColor.B || gotA != expectedColor.A {
			t.Errorf("Pixel %d (color index %d): expected RGBA(%d,%d,%d,%d), got RGBA(%d,%d,%d,%d)",
				pixelIdx, colorIdx, expectedColor.R, expectedColor.G, expectedColor.B, expectedColor.A,
				gotR, gotG, gotB, gotA)
		}
	}
}

// TestLayout tests the Layout method
func TestLayout(t *testing.T) {
	memory := emulator.NewMemory()
	vga := NewVGADisplay(memory)

	w, h := vga.Layout(1000, 1000)
	if w != ScreenWidth || h != ScreenHeight {
		t.Errorf("Layout(): expected (%d, %d), got (%d, %d)", ScreenWidth, ScreenHeight, w, h)
	}
}

// TestColorCubeRange tests the 216-color cube in the palette
func TestColorCubeRange(t *testing.T) {
	memory := emulator.NewMemory()
	vga := NewVGADisplay(memory)

	// Test that color cube starts at index 16
	// First color in cube should be (0, 0, 0) - but it's not black because that's index 0
	// Actually it should be r=0*51, g=0*51, b=0*51
	expected := color.RGBA{0, 0, 0, 255}
	got := vga.palette[16]
	if got != expected {
		t.Errorf("Color cube start (index 16): expected %v, got %v", expected, got)
	}

	// Test a middle value in the color cube
	// Index 16 + some offset into the 6x6x6 cube
	// Let's test r=5, g=5, b=5 (max values)
	// This should be at index 16 + (5*6*6 + 5*6 + 5) = 16 + 215 = 231
	expected = color.RGBA{255, 255, 255, 255}
	got = vga.palette[231]
	if got != expected {
		t.Errorf("Color cube end (index 231): expected %v, got %v", expected, got)
	}
}

// TestGrayscaleRange tests the grayscale ramp in the palette
func TestGrayscaleRange(t *testing.T) {
	memory := emulator.NewMemory()
	vga := NewVGADisplay(memory)

	// Test first grayscale color (index 232)
	// gray = 8 + 0*10 = 8
	expected := color.RGBA{8, 8, 8, 255}
	got := vga.palette[232]
	if got != expected {
		t.Errorf("Grayscale start (index 232): expected %v, got %v", expected, got)
	}

	// Test last grayscale color (index 255)
	// gray = 8 + 23*10 = 238
	expected = color.RGBA{238, 238, 238, 255}
	got = vga.palette[255]
	if got != expected {
		t.Errorf("Grayscale end (index 255): expected %v, got %v", expected, got)
	}
}
