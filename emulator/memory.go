package emulator

// Memory models the physical address space of a real-mode PC: 1 MB of RAM
// plus the 64 KB HMA that becomes reachable when the A20 gate is enabled.
//
// Addresses 0xA0000-0xBFFFF are routed through an optional MMIO handler so
// a VGA card can implement planar memory; everything else is plain RAM.
// 0xF0000-0xFFFFF is treated as ROM (writes ignored) once ROMProtect is set.
type Memory struct {
	RAM        []byte
	A20        bool
	ROMProtect bool

	// MMIO handlers for 0xA0000-0xBFFFF. nil means plain RAM.
	MMIORead  func(addr uint32) byte
	MMIOWrite func(addr uint32, v byte)
}

const (
	MemSize     = 0x110000 // 1 MB + 64 KB HMA
	mmioStart   = 0xA0000
	mmioEnd     = 0xC0000
	romStart    = 0xF0000
	romEnd      = 0x100000
	VGAMemStart = 0xA0000
)

func NewMemory() *Memory {
	return &Memory{RAM: make([]byte, MemSize)}
}

// mask applies the A20 gate: with A20 disabled the address wraps at 1 MB.
func (m *Memory) mask(addr uint32) uint32 {
	if m.A20 {
		return addr & 0x1FFFFF % MemSize
	}
	return addr & 0xFFFFF
}

func (m *Memory) Read8(addr uint32) byte {
	addr = m.mask(addr)
	if addr >= mmioStart && addr < mmioEnd && m.MMIORead != nil {
		return m.MMIORead(addr)
	}
	return m.RAM[addr]
}

func (m *Memory) Write8(addr uint32, v byte) {
	addr = m.mask(addr)
	if addr >= mmioStart && addr < mmioEnd && m.MMIOWrite != nil {
		m.MMIOWrite(addr, v)
		return
	}
	if m.ROMProtect && addr >= romStart && addr < romEnd {
		return
	}
	m.RAM[addr] = v
}

func (m *Memory) Read16(addr uint32) uint16 {
	return uint16(m.Read8(addr)) | uint16(m.Read8(addr+1))<<8
}

func (m *Memory) Write16(addr uint32, v uint16) {
	m.Write8(addr, byte(v))
	m.Write8(addr+1, byte(v>>8))
}

func (m *Memory) Read32(addr uint32) uint32 {
	return uint32(m.Read16(addr)) | uint32(m.Read16(addr+2))<<16
}

func (m *Memory) Write32(addr uint32, v uint32) {
	m.Write16(addr, uint16(v))
	m.Write16(addr+2, uint16(v>>16))
}

// WriteROM bypasses ROM protection and MMIO; used to install BIOS images.
func (m *Memory) WriteROM(addr uint32, data []byte) {
	copy(m.RAM[addr:], data)
}

// Linear computes a real-mode physical address from segment:offset.
func Linear(seg uint16, off uint32) uint32 {
	return uint32(seg)<<4 + off
}
