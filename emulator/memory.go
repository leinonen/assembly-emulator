package emulator

import (
	"assembly-emulator/font"
	"sync"
)

const (
	// Memory size constants for x86 real mode (1MB addressable)
	TotalMemorySize = 0x100000 // 1MB total addressable memory
	VGAMemoryStart  = 0xA0000  // VGA memory starts at 0xA0000 (linear address)
	VGAMemorySize   = 64000    // 320x200 pixels

	// BIOS ROM constants
	ROMStart      = 0xF0000  // BIOS ROM starts at 0xF0000 (960KB)
	ROMSize       = 0x10000  // 64KB ROM space
	BIOSFontAddr  = 0xFA000  // CP437 font location (F000:A000, adjusted to fit in 1MB)
	BIOSFontSize  = 4096     // 256 characters * 16 bytes
)

// Memory represents the system memory including VGA video memory
type Memory struct {
	RAM     []byte     // 1MB RAM (VGA is mapped within this space at 0xA0000)
	VGA     []byte     // VGA video memory (separate for easy rendering access)
	vgaMux  sync.Mutex // Mutex to protect VGA memory from race conditions
}

// NewMemory creates a new memory instance
func NewMemory() *Memory {
	return &Memory{
		RAM: make([]byte, TotalMemorySize),
		VGA: make([]byte, VGAMemorySize),
	}
}

// CalculateLinearAddress converts segment:offset to 20-bit linear address
// In real mode: linear = (segment << 4) + offset
func CalculateLinearAddress(segment, offset uint16) uint32 {
	return (uint32(segment) << 4) + uint32(offset)
}

// Clear clears all memory
func (m *Memory) Clear() {
	for i := range m.RAM {
		m.RAM[i] = 0
	}
	for i := range m.VGA {
		m.VGA[i] = 0
	}
}

// ReadByteLinear reads a byte from linear (physical) address
func (m *Memory) ReadByteLinear(addr uint32) uint8 {
	// Ensure address is within 1MB
	addr = addr & 0xFFFFF

	// VGA memory mapping at 0xA0000-0xAFA00 (64000 bytes)
	if addr >= VGAMemoryStart && addr < VGAMemoryStart+uint32(VGAMemorySize) {
		offset := addr - VGAMemoryStart
		return m.VGA[offset]
	}

	return m.RAM[addr]
}

// WriteByteLinear writes a byte to linear (physical) address
func (m *Memory) WriteByteLinear(addr uint32, val uint8) {
	// Ensure address is within 1MB
	addr = addr & 0xFFFFF

	// ROM area is read-only - ignore writes to 0xF0000-0xFFFFF
	if addr >= ROMStart && addr < ROMStart+ROMSize {
		// Silently ignore writes to ROM
		return
	}

	// VGA memory mapping at 0xA0000-0xAFA00 (64000 bytes)
	if addr >= VGAMemoryStart && addr < VGAMemoryStart+uint32(VGAMemorySize) {
		offset := addr - VGAMemoryStart
		m.VGA[offset] = val
		// Also update RAM for consistency
		m.RAM[addr] = val
		return
	}

	m.RAM[addr] = val
}

// ReadWord reads a 16-bit word from memory (little-endian, legacy)
func (m *Memory) ReadWord(addr uint16) uint16 {
	return m.ReadWordLinear(uint32(addr))
}

// WriteWord writes a 16-bit word to memory (little-endian, legacy)
func (m *Memory) WriteWord(addr uint16, val uint16) {
	m.WriteWordLinear(uint32(addr), val)
}

// ReadWordLinear reads a 16-bit word from linear address (little-endian)
func (m *Memory) ReadWordLinear(addr uint32) uint16 {
	low := m.ReadByteLinear(addr)
	high := m.ReadByteLinear(addr + 1)
	return uint16(low) | (uint16(high) << 8)
}

// WriteWordLinear writes a 16-bit word to linear address (little-endian)
func (m *Memory) WriteWordLinear(addr uint32, val uint16) {
	m.WriteByteLinear(addr, uint8(val&0xFF))
	m.WriteByteLinear(addr+1, uint8((val>>8)&0xFF))
}

// LoadProgram loads a program into memory at the specified linear address
func (m *Memory) LoadProgram(addr uint32, program []byte) {
	for i, b := range program {
		if addr+uint32(i) >= TotalMemorySize {
			break
		}
		m.RAM[addr+uint32(i)] = b
	}
}

// LockVGA locks the VGA memory mutex (call before reading VGA memory for rendering)
func (m *Memory) LockVGA() {
	m.vgaMux.Lock()
}

// UnlockVGA unlocks the VGA memory mutex (call after reading VGA memory for rendering)
func (m *Memory) UnlockVGA() {
	m.vgaMux.Unlock()
}

// GetVGAMemory returns a reference to the VGA memory for rendering
// IMPORTANT: Caller must call LockVGA() before and UnlockVGA() after using this
func (m *Memory) GetVGAMemory() []byte {
	return m.VGA
}

// GetVGAPixel gets a pixel color at x, y coordinates (Mode 13h: 320x200)
func (m *Memory) GetVGAPixel(x, y int) uint8 {
	if x < 0 || x >= 320 || y < 0 || y >= 200 {
		return 0
	}
	offset := y*320 + x
	if offset < len(m.VGA) {
		return m.VGA[offset]
	}
	return 0
}

// SetVGAPixel sets a pixel color at x, y coordinates (Mode 13h: 320x200)
func (m *Memory) SetVGAPixel(x, y int, color uint8) {
	if x < 0 || x >= 320 || y < 0 || y >= 200 {
		return
	}
	offset := y*320 + x
	if offset < len(m.VGA) {
		m.VGA[offset] = color
	}
}

// InitializeBIOSROM initializes the BIOS ROM area with CP437 font data
// This should be called once during CPU initialization
func (m *Memory) InitializeBIOSROM() {
	// Copy CP437 font data to ROM at standard BIOS font location
	// Font is 256 characters * 16 bytes = 4096 bytes
	for char := 0; char < 256; char++ {
		for row := 0; row < 16; row++ {
			addr := BIOSFontAddr + uint32(char*16+row)
			// Directly write to RAM (bypass WriteByteLinear's ROM protection)
			m.RAM[addr] = font.CP437Font[char][row]
		}
	}
}
