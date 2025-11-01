package emulator

import "fmt"

// CPU represents the x86 CPU state
type CPU struct {
	// 16-bit general purpose registers
	AX uint16 // Accumulator
	BX uint16 // Base
	CX uint16 // Counter
	DX uint16 // Data

	// Index and pointer registers
	SI uint16 // Source Index
	DI uint16 // Destination Index
	BP uint16 // Base Pointer
	SP uint16 // Stack Pointer

	// Segment registers
	CS uint16 // Code Segment
	DS uint16 // Data Segment
	ES uint16 // Extra Segment
	SS uint16 // Stack Segment

	// Instruction pointer
	IP uint16

	// Flags register
	Flags Flags

	// Memory
	Memory *Memory

	// Halted state
	Halted bool

	// Mode 13h callback (called when graphics mode is activated)
	Mode13hCallback func()

	// Palette callback (called to set palette colors)
	SetPaletteCallback func(index byte, r, g, b byte)

	// VGA DAC state (for palette manipulation)
	vgaDACWriteIndex uint8 // Port 0x3C8 - DAC write index
	vgaDACReadIndex  uint8 // Port 0x3C7 - DAC read index
	vgaDACState      uint8 // 0=R, 1=G, 2=B (which component we're writing)
	vgaDACColorR     uint8 // Temporary storage for R component
	vgaDACColorG     uint8 // Temporary storage for G component

	// Keyboard state (for BIOS INT 16h)
	keyboardScancode uint8 // Last key scancode
	keyboardASCII    uint8 // Last key ASCII code
	keyAvailable     bool  // True if a key is waiting to be read

	// VBlank state (for VGA synchronization via port 0x3DA)
	VBlankActive   bool          // Current VBlank state (bit 3 of port 0x3DA)
	FrameCounter   uint64        // Frame counter for timing
	vblankChan     chan struct{} // Channel to signal VBlank events
	waitingVBlank  bool          // True if CPU is waiting for VBlank

	// Stop channel for external termination signal
	stopChan chan struct{}
}

// Flags represents CPU flags
type Flags struct {
	CF bool // Carry Flag
	ZF bool // Zero Flag
	SF bool // Sign Flag
	OF bool // Overflow Flag
}

// NewCPU creates a new CPU instance
func NewCPU() *CPU {
	return &CPU{
		Memory:     NewMemory(),
		SP:         0xFFFE, // Stack grows downward from top of memory
		CS:         0x0000, // Code segment starts at 0
		DS:         0x0000, // Data segment starts at 0
		ES:         0x0000, // Extra segment starts at 0
		SS:         0x0000, // Stack segment starts at 0
		stopChan:   make(chan struct{}),
		vblankChan: make(chan struct{}, 1), // Buffered to prevent blocking
	}
}

// Reset resets the CPU to initial state
func (c *CPU) Reset() {
	c.AX = 0
	c.BX = 0
	c.CX = 0
	c.DX = 0
	c.SI = 0
	c.DI = 0
	c.BP = 0
	c.SP = 0xFFFE
	c.CS = 0
	c.DS = 0
	c.ES = 0
	c.SS = 0
	c.IP = 0
	c.Flags = Flags{}
	c.Halted = false
	c.Memory.Clear()
}

// GetAL returns the low byte of AX
func (c *CPU) GetAL() uint8 {
	return uint8(c.AX & 0xFF)
}

// SetAL sets the low byte of AX
func (c *CPU) SetAL(val uint8) {
	c.AX = (c.AX & 0xFF00) | uint16(val)
}

// GetAH returns the high byte of AX
func (c *CPU) GetAH() uint8 {
	return uint8((c.AX >> 8) & 0xFF)
}

// SetAH sets the high byte of AX
func (c *CPU) SetAH(val uint8) {
	c.AX = (c.AX & 0x00FF) | (uint16(val) << 8)
}

// GetBL returns the low byte of BX
func (c *CPU) GetBL() uint8 {
	return uint8(c.BX & 0xFF)
}

// SetBL sets the low byte of BX
func (c *CPU) SetBL(val uint8) {
	c.BX = (c.BX & 0xFF00) | uint16(val)
}

// GetBH returns the high byte of BX
func (c *CPU) GetBH() uint8 {
	return uint8((c.BX >> 8) & 0xFF)
}

// SetBH sets the high byte of BX
func (c *CPU) SetBH(val uint8) {
	c.BX = (c.BX & 0x00FF) | (uint16(val) << 8)
}

// GetCL returns the low byte of CX
func (c *CPU) GetCL() uint8 {
	return uint8(c.CX & 0xFF)
}

// SetCL sets the low byte of CX
func (c *CPU) SetCL(val uint8) {
	c.CX = (c.CX & 0xFF00) | uint16(val)
}

// GetCH returns the high byte of CX
func (c *CPU) GetCH() uint8 {
	return uint8((c.CX >> 8) & 0xFF)
}

// SetCH sets the high byte of CX
func (c *CPU) SetCH(val uint8) {
	c.CX = (c.CX & 0x00FF) | (uint16(val) << 8)
}

// GetDL returns the low byte of DX
func (c *CPU) GetDL() uint8 {
	return uint8(c.DX & 0xFF)
}

// SetDL sets the low byte of DX
func (c *CPU) SetDL(val uint8) {
	c.DX = (c.DX & 0xFF00) | uint16(val)
}

// GetDH returns the high byte of DX
func (c *CPU) GetDH() uint8 {
	return uint8((c.DX >> 8) & 0xFF)
}

// SetDH sets the high byte of DX
func (c *CPU) SetDH(val uint8) {
	c.DX = (c.DX & 0x00FF) | (uint16(val) << 8)
}

// Push pushes a 16-bit value onto the stack using SS:SP
func (c *CPU) Push(val uint16) error {
	if c.SP < 2 {
		return fmt.Errorf("stack overflow")
	}
	c.SP -= 2
	addr := CalculateLinearAddress(c.SS, c.SP)
	c.Memory.WriteWordLinear(addr, val)
	return nil
}

// Pop pops a 16-bit value from the stack using SS:SP
func (c *CPU) Pop() (uint16, error) {
	if c.SP > 0xFFFC {
		return 0, fmt.Errorf("stack underflow")
	}
	addr := CalculateLinearAddress(c.SS, c.SP)
	val := c.Memory.ReadWordLinear(addr)
	c.SP += 2
	return val, nil
}

// UpdateZeroFlag sets the zero flag based on the value
func (c *CPU) UpdateZeroFlag(val uint16) {
	c.Flags.ZF = (val == 0)
}

// UpdateSignFlag sets the sign flag based on the value (16-bit)
func (c *CPU) UpdateSignFlag(val uint16) {
	c.Flags.SF = (val&0x8000) != 0
}

// UpdateSignFlag8 sets the sign flag based on the value (8-bit)
func (c *CPU) UpdateSignFlag8(val uint8) {
	c.Flags.SF = (val&0x80) != 0
}

// UpdateFlags updates zero and sign flags based on the result
func (c *CPU) UpdateFlags(val uint16) {
	c.UpdateZeroFlag(val)
	c.UpdateSignFlag(val)
}

// UpdateFlags8 updates zero and sign flags based on 8-bit result
func (c *CPU) UpdateFlags8(val uint8) {
	c.Flags.ZF = (val == 0)
	c.UpdateSignFlag8(val)
}

// String returns a string representation of CPU state
func (c *CPU) String() string {
	return fmt.Sprintf("AX:%04X BX:%04X CX:%04X DX:%04X SI:%04X DI:%04X BP:%04X SP:%04X IP:%04X\n"+
		"CS:%04X DS:%04X ES:%04X SS:%04X [%s%s%s%s]",
		c.AX, c.BX, c.CX, c.DX, c.SI, c.DI, c.BP, c.SP, c.IP,
		c.CS, c.DS, c.ES, c.SS,
		flagStr("C", c.Flags.CF),
		flagStr("Z", c.Flags.ZF),
		flagStr("S", c.Flags.SF),
		flagStr("O", c.Flags.OF),
	)
}

func flagStr(name string, set bool) string {
	if set {
		return name
	}
	return "-"
}

// OutByte handles OUT instruction - write byte to I/O port
func (c *CPU) OutByte(port uint16, value uint8) {
	switch port {
	case 0x3C8: // DAC Write Index
		c.vgaDACWriteIndex = value
		c.vgaDACState = 0 // Reset to R component
	case 0x3C9: // DAC Data
		switch c.vgaDACState {
		case 0: // Red component
			c.vgaDACColorR = value
			c.vgaDACState = 1
		case 1: // Green component
			c.vgaDACColorG = value
			c.vgaDACState = 2
		case 2: // Blue component
			// We have all three components, update the palette
			// Convert from 6-bit (0-63) to 8-bit (0-255)
			// Use uint16 to avoid overflow, then convert back to uint8
			r := uint8((uint16(c.vgaDACColorR&0x3F) * 255) / 63)
			g := uint8((uint16(c.vgaDACColorG&0x3F) * 255) / 63)
			b := uint8((uint16(value&0x3F) * 255) / 63)

			if c.SetPaletteCallback != nil {
				c.SetPaletteCallback(c.vgaDACWriteIndex, r, g, b)
			}

			// Move to next color index and reset to R component
			c.vgaDACWriteIndex++
			c.vgaDACState = 0
		}
	case 0x3C7: // DAC Read Index
		c.vgaDACReadIndex = value
		c.vgaDACState = 0
	}
}

// InByte handles IN instruction - read byte from I/O port
func (c *CPU) InByte(port uint16) uint8 {
	switch port {
	case 0x3C7: // DAC State
		// Return 0 to indicate DAC is ready
		return 0
	case 0x3C8: // DAC Write Index
		return c.vgaDACWriteIndex
	case 0x3C9: // DAC Data (read)
		// For now, return 0 (proper implementation would read from palette)
		return 0
	case 0x3DA: // Input Status Register 1 (VGA status)
		// Bit 3: Vertical retrace (VBlank) - 1 during VBlank, 0 otherwise
		// Bit 0: Display enable (usually 1)

		// Wait for next VBlank signal from graphics loop
		// This blocks the CPU until the next frame starts
		select {
		case <-c.vblankChan:
			// VBlank occurred - frame sync
			c.VBlankActive = true
		case <-c.stopChan:
			// CPU stopped
		}

		var status uint8 = 0x01 // Display enabled
		if c.VBlankActive {
			status |= 0x08 // Set bit 3
		}
		// Clear VBlank after reading
		c.VBlankActive = false
		return status
	default:
		return 0
	}
}

// SetKeyPress sets the keyboard state when a key is pressed
// scancode is the BIOS scan code, ascii is the ASCII character
func (c *CPU) SetKeyPress(scancode, ascii uint8) {
	c.keyboardScancode = scancode
	c.keyboardASCII = ascii
	c.keyAvailable = true
}

// SetVBlank sets the VBlank state for synchronization with the graphics loop
// This is called by the graphics Update() method at the start of each frame
func (c *CPU) SetVBlank(active bool) {
	if active {
		c.FrameCounter++
		// Signal VBlank to waiting CPU (non-blocking)
		select {
		case c.vblankChan <- struct{}{}:
		default:
			// Channel full, skip (CPU not waiting yet)
		}
	}
}

// Stop signals the CPU to stop execution
// This is used to terminate infinite loops when the graphics window closes
func (c *CPU) Stop() {
	select {
	case <-c.stopChan:
		// Already stopped
	default:
		close(c.stopChan)
	}
}
