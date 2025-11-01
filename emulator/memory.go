package emulator

const (
	// Memory size constants for x86 real mode (1MB addressable)
	TotalMemorySize = 0x100000 // 1MB total addressable memory
	VGAMemoryStart  = 0xA0000  // VGA memory starts at 0xA0000 (linear address)
	VGAMemorySize   = 64000    // 320x200 pixels
)

// Memory represents the system memory including VGA video memory
type Memory struct {
	RAM []byte // 1MB RAM (VGA is mapped within this space at 0xA0000)
	VGA []byte // VGA video memory (separate for easy rendering access)
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

// ReadByte reads a byte from memory (legacy flat addressing - deprecated)
// Use ReadByteLinear for segmented addressing
func (m *Memory) ReadByte(addr uint16) uint8 {
	return m.ReadByteLinear(uint32(addr))
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

// WriteByte writes a byte to memory (legacy flat addressing - deprecated)
// Use WriteByteLinear for segmented addressing
func (m *Memory) WriteByte(addr uint16, val uint8) {
	m.WriteByteLinear(uint32(addr), val)
}

// WriteByteLinear writes a byte to linear (physical) address
func (m *Memory) WriteByteLinear(addr uint32, val uint8) {
	// Ensure address is within 1MB
	addr = addr & 0xFFFFF

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
