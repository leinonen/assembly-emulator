package emulator

const (
	// Memory size constants
	TotalMemorySize = 0x10000 // 64KB total addressable memory
	VGAMemoryStart  = 0xA000  // VGA memory starts at 0xA000
	VGAMemoryEnd    = 0xA4E8  // VGA memory ends at 0xA4E8 (320*200 = 64000 bytes = 0xFA00)
	VGAMemorySize   = 64000   // 320x200 pixels
)

// Memory represents the system memory including VGA video memory
type Memory struct {
	RAM []byte      // Main RAM
	VGA []byte      // VGA video memory (separate for easy access)
}

// NewMemory creates a new memory instance
func NewMemory() *Memory {
	return &Memory{
		RAM: make([]byte, TotalMemorySize),
		VGA: make([]byte, VGAMemorySize),
	}
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

// ReadByte reads a byte from memory
func (m *Memory) ReadByte(addr uint16) uint8 {
	// VGA memory mapping for Mode 13h (64000 bytes total)
	// Map 0xA000-0xFFFF to VGA[0:24576] (first 24576 bytes)
	if addr >= VGAMemoryStart {
		offset := int(addr - VGAMemoryStart)
		if offset >= 0 && offset < len(m.VGA) {
			return m.VGA[offset]
		}
	}

	// Map wrapped addresses (0x0400-0x9FFF) to remaining VGA memory
	// Protect 0x0000-0x03FF (first 1KB) for program code/data
	if addr >= 0x0400 && addr < VGAMemoryStart {
		offset := int(addr - 0x0400 + 24576) // Wrapped addresses map to second part of VGA
		if offset < len(m.VGA) {
			return m.VGA[offset]
		}
	}

	return m.RAM[addr]
}

// WriteByte writes a byte to memory
func (m *Memory) WriteByte(addr uint16, val uint8) {
	// VGA memory mapping for Mode 13h (64000 bytes total)
	// Map 0xA000-0xFFFF to VGA[0:24576] (first 24576 bytes)
	if addr >= VGAMemoryStart {
		offset := int(addr - VGAMemoryStart)
		if offset >= 0 && offset < len(m.VGA) {
			m.VGA[offset] = val
			return
		}
	}

	// Map wrapped addresses (0x0400-0x9FFF) to remaining VGA memory
	// Protect 0x0000-0x03FF (first 1KB) for program code/data
	if addr >= 0x0400 && addr < VGAMemoryStart {
		offset := int(addr - 0x0400 + 24576) // Wrapped addresses map to second part of VGA
		if offset < len(m.VGA) {
			m.VGA[offset] = val
			return
		}
	}

	// Protect low memory from accidental writes
	if addr < 0x0400 {
		// Silently ignore writes to protected area
		return
	}

	m.RAM[addr] = val
}

// ReadWord reads a 16-bit word from memory (little-endian)
func (m *Memory) ReadWord(addr uint16) uint16 {
	low := m.ReadByte(addr)
	high := m.ReadByte(addr + 1)
	return uint16(low) | (uint16(high) << 8)
}

// WriteWord writes a 16-bit word to memory (little-endian)
func (m *Memory) WriteWord(addr uint16, val uint16) {
	m.WriteByte(addr, uint8(val&0xFF))
	m.WriteByte(addr+1, uint8((val>>8)&0xFF))
}

// LoadProgram loads a program into memory at the specified address
func (m *Memory) LoadProgram(addr uint16, program []byte) {
	for i, b := range program {
		if int(addr)+i >= TotalMemorySize {
			break
		}
		// Store program code in both RAM and track its extent
		m.RAM[int(addr)+i] = b
	}
}

// GetVGAMemory returns a reference to the VGA memory for rendering
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
