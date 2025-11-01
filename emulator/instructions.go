package emulator

import (
	"fmt"
)

// Opcode represents an instruction opcode
type Opcode byte

// Instruction opcodes (simplified encoding)
const (
	// Data movement
	OpMOV  Opcode = 0x01
	OpPUSH Opcode = 0x02
	OpPOP  Opcode = 0x03
	OpXCHG Opcode = 0x04

	// Arithmetic
	OpADD  Opcode = 0x10
	OpSUB  Opcode = 0x11
	OpMUL  Opcode = 0x12
	OpDIV  Opcode = 0x13
	OpIMUL Opcode = 0x14
	OpIDIV Opcode = 0x15
	OpINC  Opcode = 0x16
	OpDEC  Opcode = 0x17
	OpNEG  Opcode = 0x18

	// Logical
	OpAND Opcode = 0x20
	OpOR  Opcode = 0x21
	OpXOR Opcode = 0x22
	OpNOT Opcode = 0x23
	OpSHL Opcode = 0x24
	OpSHR Opcode = 0x25
	OpSAL Opcode = 0x26
	OpSAR Opcode = 0x27
	OpROL Opcode = 0x28
	OpROR Opcode = 0x29

	// Comparison
	OpCMP  Opcode = 0x30
	OpTEST Opcode = 0x31

	// Control flow
	OpJMP   Opcode = 0x40
	OpJE    Opcode = 0x41 // JE/JZ
	OpJNE   Opcode = 0x42 // JNE/JNZ
	OpJG    Opcode = 0x43 // JG/JNLE
	OpJGE   Opcode = 0x44 // JGE/JNL
	OpJL    Opcode = 0x45 // JL/JNGE
	OpJLE   Opcode = 0x46 // JLE/JNG
	OpJA    Opcode = 0x47 // JA (unsigned >)
	OpJAE   Opcode = 0x48 // JAE (unsigned >=)
	OpJB    Opcode = 0x49 // JB (unsigned <)
	OpJBE   Opcode = 0x4A // JBE (unsigned <=)
	OpCALL  Opcode = 0x4B
	OpRET   Opcode = 0x4C
	OpLOOP  Opcode = 0x4D
	OpLOOPZ Opcode = 0x4E
	OpLOOPNZ Opcode = 0x4F

	// Special
	OpINT Opcode = 0x50
	OpNOP Opcode = 0x51
	OpHLT Opcode = 0x52

	// I/O
	OpIN  Opcode = 0x60
	OpOUT Opcode = 0x61

	// String operations
	OpMOVSB Opcode = 0x70
	OpMOVSW Opcode = 0x71
	OpSTOSB Opcode = 0x72
	OpSTOSW Opcode = 0x73
)

// Operand types
type OperandType byte

const (
	OpTypeNone     OperandType = 0
	OpTypeReg16    OperandType = 1 // 16-bit register
	OpTypeReg8     OperandType = 2 // 8-bit register
	OpTypeImm16    OperandType = 3 // 16-bit immediate
	OpTypeImm8     OperandType = 4 // 8-bit immediate
	OpTypeMem      OperandType = 5 // Memory address
	OpTypeMemReg   OperandType = 6 // Memory [register]
)

// Instruction represents a decoded instruction
type Instruction struct {
	Opcode     Opcode
	Dest       Operand
	Src        Operand
	Size       int  // Instruction size in bytes
	HasREP     bool // True if REP prefix (0xF3) is present
}

// Operand represents an instruction operand
type Operand struct {
	Type        OperandType
	Reg16       *uint16 // Pointer to 16-bit register
	RegSeg      *uint16 // Pointer to segment register (CS, DS, ES, SS)
	Reg8Get     func() uint8
	Reg8Set     func(uint8)
	Imm16       uint16
	Imm8        uint8
	MemAddr     uint16 // Offset within segment
	MemSegment  uint16 // Segment for memory access (will be set to DS/ES/SS/CS)
	SegOverride bool   // True if segment was explicitly overridden
}

// Execute executes a single instruction
func (c *CPU) Execute(inst Instruction) error {
	switch inst.Opcode {
	case OpMOV:
		return c.execMOV(inst)
	case OpPUSH:
		return c.execPUSH(inst)
	case OpPOP:
		return c.execPOP(inst)
	case OpXCHG:
		return c.execXCHG(inst)

	case OpADD:
		return c.execADD(inst)
	case OpSUB:
		return c.execSUB(inst)
	case OpMUL:
		return c.execMUL(inst)
	case OpDIV:
		return c.execDIV(inst)
	case OpINC:
		return c.execINC(inst)
	case OpDEC:
		return c.execDEC(inst)
	case OpNEG:
		return c.execNEG(inst)

	case OpAND:
		return c.execAND(inst)
	case OpOR:
		return c.execOR(inst)
	case OpXOR:
		return c.execXOR(inst)
	case OpNOT:
		return c.execNOT(inst)
	case OpSHL, OpSAL:
		return c.execSHL(inst)
	case OpSHR:
		return c.execSHR(inst)
	case OpSAR:
		return c.execSAR(inst)

	case OpCMP:
		return c.execCMP(inst)
	case OpTEST:
		return c.execTEST(inst)

	case OpJMP:
		return c.execJMP(inst)
	case OpJE:
		return c.execJE(inst)
	case OpJNE:
		return c.execJNE(inst)
	case OpJG:
		return c.execJG(inst)
	case OpJGE:
		return c.execJGE(inst)
	case OpJL:
		return c.execJL(inst)
	case OpJLE:
		return c.execJLE(inst)
	case OpJA:
		return c.execJA(inst)
	case OpJAE:
		return c.execJAE(inst)
	case OpJB:
		return c.execJB(inst)
	case OpJBE:
		return c.execJBE(inst)
	case OpCALL:
		return c.execCALL(inst)
	case OpRET:
		return c.execRET(inst)
	case OpLOOP:
		return c.execLOOP(inst)
	case OpLOOPZ:
		return c.execLOOPZ(inst)
	case OpLOOPNZ:
		return c.execLOOPNZ(inst)

	case OpINT:
		return c.execINT(inst)
	case OpNOP:
		return nil
	case OpHLT:
		c.Halted = true
		return nil

	case OpOUT:
		return c.execOUT(inst)
	case OpIN:
		return c.execIN(inst)

	case OpMOVSB:
		return c.execMOVSB(inst)
	case OpMOVSW:
		return c.execMOVSW(inst)
	case OpSTOSB:
		return c.execSTOSB(inst)
	case OpSTOSW:
		return c.execSTOSW(inst)

	default:
		return fmt.Errorf("unknown opcode: 0x%02X", inst.Opcode)
	}
}

// MOV instruction
func (c *CPU) execMOV(inst Instruction) error {
	// Special handling for 8-bit memory reads (e.g., MOV AL, [SI])
	if (inst.Src.Type == OpTypeMem || inst.Src.Type == OpTypeMemReg) &&
		(inst.Dest.Type == OpTypeReg8) {
		addr := CalculateLinearAddress(inst.Src.MemSegment, inst.Src.MemAddr)
		val := c.Memory.ReadByteLinear(addr)
		c.setOperandValue(inst.Dest, uint16(val))
		return nil
	}

	// Special handling for 8-bit memory writes (e.g., MOV [DI], AL)
	if (inst.Src.Type == OpTypeReg8 || inst.Src.Type == OpTypeImm8) &&
		(inst.Dest.Type == OpTypeMem || inst.Dest.Type == OpTypeMemReg) {
		addr := CalculateLinearAddress(inst.Dest.MemSegment, inst.Dest.MemAddr)
		val := c.getOperandValue(inst.Src)
		c.Memory.WriteByteLinear(addr, uint8(val&0xFF))
		return nil
	}

	// Default: use word operations
	val := c.getOperandValue(inst.Src)
	c.setOperandValue(inst.Dest, val)
	return nil
}

// PUSH instruction
func (c *CPU) execPUSH(inst Instruction) error {
	val := c.getOperandValue(inst.Src)
	return c.Push(val)
}

// POP instruction
func (c *CPU) execPOP(inst Instruction) error {
	val, err := c.Pop()
	if err != nil {
		return err
	}
	c.setOperandValue(inst.Dest, val)
	return nil
}

// XCHG instruction
func (c *CPU) execXCHG(inst Instruction) error {
	val1 := c.getOperandValue(inst.Dest)
	val2 := c.getOperandValue(inst.Src)
	c.setOperandValue(inst.Dest, val2)
	c.setOperandValue(inst.Src, val1)
	return nil
}

// ADD instruction
func (c *CPU) execADD(inst Instruction) error {
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest + src

	// Set flags
	c.Flags.CF = result < dest // Carry occurred
	c.Flags.OF = ((dest^result)&(src^result)&0x8000) != 0 // Overflow
	c.UpdateFlags(result)

	c.setOperandValue(inst.Dest, result)
	return nil
}

// SUB instruction
func (c *CPU) execSUB(inst Instruction) error {
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest - src

	// Set flags
	c.Flags.CF = src > dest // Borrow occurred
	c.Flags.OF = ((dest^src)&(dest^result)&0x8000) != 0 // Overflow
	c.UpdateFlags(result)

	c.setOperandValue(inst.Dest, result)
	return nil
}

// MUL instruction (unsigned)
func (c *CPU) execMUL(inst Instruction) error {
	src := c.getOperandValue(inst.Dest)
	result := uint32(c.AX) * uint32(src)

	c.AX = uint16(result & 0xFFFF)
	c.DX = uint16((result >> 16) & 0xFFFF)

	// CF and OF set if upper half is non-zero
	c.Flags.CF = c.DX != 0
	c.Flags.OF = c.DX != 0

	return nil
}

// DIV instruction (unsigned)
func (c *CPU) execDIV(inst Instruction) error {
	divisor := uint32(c.getOperandValue(inst.Dest))
	if divisor == 0 {
		return fmt.Errorf("division by zero")
	}

	dividend := (uint32(c.DX) << 16) | uint32(c.AX)
	quotient := dividend / divisor
	remainder := dividend % divisor

	if quotient > 0xFFFF {
		return fmt.Errorf("division overflow")
	}

	c.AX = uint16(quotient)
	c.DX = uint16(remainder)

	return nil
}

// INC instruction
func (c *CPU) execINC(inst Instruction) error {
	val := c.getOperandValue(inst.Dest)
	result := val + 1

	// Handle 8-bit vs 16-bit operations correctly
	if inst.Dest.Type == OpTypeReg8 {
		// For 8-bit operations, mask to 8 bits before updating flags
		result = result & 0xFF
		c.Flags.OF = (val == 0x7F) // Overflow from max positive (8-bit)
	} else {
		c.Flags.OF = (val == 0x7FFF) // Overflow from max positive (16-bit)
	}

	c.UpdateFlags(result)
	// Note: INC does not affect CF

	c.setOperandValue(inst.Dest, result)
	return nil
}

// DEC instruction
func (c *CPU) execDEC(inst Instruction) error {
	val := c.getOperandValue(inst.Dest)
	result := val - 1

	// Handle 8-bit vs 16-bit operations correctly
	if inst.Dest.Type == OpTypeReg8 {
		// For 8-bit operations, mask to 8 bits before updating flags
		result = result & 0xFF
		c.Flags.OF = (val == 0x80) // Overflow from min negative (8-bit)
	} else {
		c.Flags.OF = (val == 0x8000) // Overflow from min negative (16-bit)
	}

	c.UpdateFlags(result)
	// Note: DEC does not affect CF

	c.setOperandValue(inst.Dest, result)
	return nil
}

// NEG instruction
func (c *CPU) execNEG(inst Instruction) error {
	val := c.getOperandValue(inst.Dest)
	result := uint16(-int16(val))

	c.Flags.CF = (val != 0)
	c.Flags.OF = (val == 0x8000)
	c.UpdateFlags(result)

	c.setOperandValue(inst.Dest, result)
	return nil
}

// AND instruction
func (c *CPU) execAND(inst Instruction) error {
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest & src

	c.Flags.CF = false
	c.Flags.OF = false
	c.UpdateFlags(result)

	c.setOperandValue(inst.Dest, result)
	return nil
}

// OR instruction
func (c *CPU) execOR(inst Instruction) error {
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest | src

	c.Flags.CF = false
	c.Flags.OF = false
	c.UpdateFlags(result)

	c.setOperandValue(inst.Dest, result)
	return nil
}

// XOR instruction
func (c *CPU) execXOR(inst Instruction) error {
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest ^ src

	c.Flags.CF = false
	c.Flags.OF = false
	c.UpdateFlags(result)

	c.setOperandValue(inst.Dest, result)
	return nil
}

// NOT instruction
func (c *CPU) execNOT(inst Instruction) error {
	val := c.getOperandValue(inst.Dest)
	result := ^val
	c.setOperandValue(inst.Dest, result)
	return nil
}

// SHL instruction (shift left)
func (c *CPU) execSHL(inst Instruction) error {
	val := c.getOperandValue(inst.Dest)
	count := c.getOperandValue(inst.Src)
	if count > 16 {
		count = 16
	}

	if count > 0 {
		// Last bit shifted out goes to CF
		c.Flags.CF = ((val >> (16 - count)) & 1) != 0
		result := val << count
		c.UpdateFlags(result)
		c.setOperandValue(inst.Dest, result)
	}

	return nil
}

// SHR instruction (shift right logical)
func (c *CPU) execSHR(inst Instruction) error {
	val := c.getOperandValue(inst.Dest)
	count := c.getOperandValue(inst.Src)
	if count > 16 {
		count = 16
	}

	if count > 0 {
		// Last bit shifted out goes to CF
		c.Flags.CF = ((val >> (count - 1)) & 1) != 0
		result := val >> count
		c.UpdateFlags(result)
		c.setOperandValue(inst.Dest, result)
	}

	return nil
}

// SAR instruction (shift right arithmetic - preserves sign)
func (c *CPU) execSAR(inst Instruction) error {
	val := c.getOperandValue(inst.Dest)
	count := c.getOperandValue(inst.Src)
	if count > 16 {
		count = 16
	}

	if count > 0 {
		signed := int16(val)
		c.Flags.CF = ((val >> (count - 1)) & 1) != 0
		result := uint16(signed >> count)
		c.UpdateFlags(result)
		c.setOperandValue(inst.Dest, result)
	}

	return nil
}

// CMP instruction (compare - SUB without storing result)
func (c *CPU) execCMP(inst Instruction) error {
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest - src

	c.Flags.CF = src > dest
	c.Flags.OF = ((dest^src)&(dest^result)&0x8000) != 0
	c.UpdateFlags(result)

	return nil
}

// TEST instruction (AND without storing result)
func (c *CPU) execTEST(inst Instruction) error {
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest & src

	c.Flags.CF = false
	c.Flags.OF = false
	c.UpdateFlags(result)

	return nil
}

// JMP instruction
func (c *CPU) execJMP(inst Instruction) error {
	c.IP = c.getOperandValue(inst.Dest)
	return nil
}

// JE/JZ instruction (jump if equal/zero)
func (c *CPU) execJE(inst Instruction) error {
	if c.Flags.ZF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JNE/JNZ instruction (jump if not equal/not zero)
func (c *CPU) execJNE(inst Instruction) error {
	if !c.Flags.ZF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JG/JNLE instruction (jump if greater - signed)
func (c *CPU) execJG(inst Instruction) error {
	if !c.Flags.ZF && (c.Flags.SF == c.Flags.OF) {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JGE/JNL instruction (jump if greater or equal - signed)
func (c *CPU) execJGE(inst Instruction) error {
	if c.Flags.SF == c.Flags.OF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JL/JNGE instruction (jump if less - signed)
func (c *CPU) execJL(inst Instruction) error {
	if c.Flags.SF != c.Flags.OF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JLE/JNG instruction (jump if less or equal - signed)
func (c *CPU) execJLE(inst Instruction) error {
	if c.Flags.ZF || (c.Flags.SF != c.Flags.OF) {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JA instruction (jump if above - unsigned)
func (c *CPU) execJA(inst Instruction) error {
	if !c.Flags.CF && !c.Flags.ZF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JAE instruction (jump if above or equal - unsigned)
func (c *CPU) execJAE(inst Instruction) error {
	if !c.Flags.CF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JB instruction (jump if below - unsigned)
func (c *CPU) execJB(inst Instruction) error {
	if c.Flags.CF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// JBE instruction (jump if below or equal - unsigned)
func (c *CPU) execJBE(inst Instruction) error {
	if c.Flags.CF || c.Flags.ZF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// CALL instruction
func (c *CPU) execCALL(inst Instruction) error {
	// Push return address (next instruction)
	if err := c.Push(c.IP); err != nil {
		return err
	}
	c.IP = c.getOperandValue(inst.Dest)
	return nil
}

// RET instruction
func (c *CPU) execRET(inst Instruction) error {
	addr, err := c.Pop()
	if err != nil {
		return err
	}
	c.IP = addr
	return nil
}

// LOOP instruction
func (c *CPU) execLOOP(inst Instruction) error {
	c.CX--
	if c.CX != 0 {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// LOOPZ instruction (loop while zero)
func (c *CPU) execLOOPZ(inst Instruction) error {
	c.CX--
	if c.CX != 0 && c.Flags.ZF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// LOOPNZ instruction (loop while not zero)
func (c *CPU) execLOOPNZ(inst Instruction) error {
	c.CX--
	if c.CX != 0 && !c.Flags.ZF {
		c.IP = c.getOperandValue(inst.Dest)
	}
	return nil
}

// INT instruction (interrupt)
func (c *CPU) execINT(inst Instruction) error {
	intNum := uint8(c.getOperandValue(inst.Dest))

	switch intNum {
	case 0x10: // Video services
		return c.handleInt10()
	case 0x16: // Keyboard services
		return c.handleInt16()
	case 0x21: // DOS services
		return c.handleInt21()
	default:
		// Ignore unknown interrupts for now
		return nil
	}
}

// INT 10h - Video services
func (c *CPU) handleInt10() error {
	ah := c.GetAH()

	switch ah {
	case 0x00: // Set video mode
		al := c.GetAL()
		if al == 0x13 {
			// Mode 13h - 320x200 256-color graphics
			// Notify that graphics mode has been activated
			if c.Mode13hCallback != nil {
				c.Mode13hCallback()
			}
		}
		return nil

	case 0x10: // Set palette register
		al := c.GetAL()
		if al == 0x00 {
			// Set single palette register
			// BL = color register to set
			// BH = color value
			if c.SetPaletteCallback != nil {
				index := c.GetBL()
				colorValue := c.GetBH()
				// Convert 6-bit VGA color value to 8-bit RGB
				// VGA uses 6 bits per channel (0-63), we scale to 0-255
				r := byte((colorValue & 0x3F) * 4)
				g := r // For now, use same value for simple greyscale
				b := r
				c.SetPaletteCallback(index, r, g, b)
			}
		} else if al == 0x10 {
			// Set individual DAC register
			// BX = register number
			// DH = green, CH = blue, CL = red (each 0-63)
			if c.SetPaletteCallback != nil {
				index := byte(c.BX & 0xFF)
				r := byte((c.CX & 0x3F) * 4)      // CL * 4
				g := byte(((c.DX >> 8) & 0x3F) * 4) // DH * 4
				b := byte(((c.CX >> 8) & 0x3F) * 4) // CH * 4
				c.SetPaletteCallback(index, r, g, b)
			}
		}
		return nil

	default:
		return nil
	}
}

// INT 16h - Keyboard services
func (c *CPU) handleInt16() error {
	ah := c.GetAH()

	switch ah {
	case 0x00: // Read keystroke (wait for key and return it)
		// Return the key if available, otherwise return 0 (non-blocking in emulator)
		if c.keyAvailable {
			c.SetAH(c.keyboardScancode) // Scan code in AH
			c.SetAL(c.keyboardASCII)    // ASCII code in AL
			// Consume the key
			c.keyAvailable = false
		} else {
			// No key available - return 0
			c.SetAH(0)
			c.SetAL(0)
		}
		return nil

	case 0x01: // Check for keystroke (non-destructive)
		// ZF = 0 if key available, ZF = 1 if no key
		if c.keyAvailable {
			c.Flags.ZF = false
			// Also set AX to the key that would be read (but don't consume it)
			c.SetAH(c.keyboardScancode)
			c.SetAL(c.keyboardASCII)
		} else {
			c.Flags.ZF = true
		}
		return nil

	default:
		return nil
	}
}

// INT 21h - DOS services
func (c *CPU) handleInt21() error {
	ah := c.GetAH()

	switch ah {
	case 0x4C: // Exit program
		c.Halted = true
		return nil
	default:
		return nil
	}
}

// Helper: Get operand value
func (c *CPU) getOperandValue(op Operand) uint16 {
	switch op.Type {
	case OpTypeReg16:
		if op.Reg16 != nil {
			return *op.Reg16
		}
	case OpTypeReg8:
		if op.Reg8Get != nil {
			return uint16(op.Reg8Get())
		}
	case OpTypeImm16:
		return op.Imm16
	case OpTypeImm8:
		return uint16(op.Imm8)
	case OpTypeMem, OpTypeMemReg:
		// Use segmented addressing
		addr := CalculateLinearAddress(op.MemSegment, op.MemAddr)
		return c.Memory.ReadWordLinear(addr)
	}
	return 0
}

// Helper: Set operand value
func (c *CPU) setOperandValue(op Operand, val uint16) {
	switch op.Type {
	case OpTypeReg16:
		if op.Reg16 != nil {
			*op.Reg16 = val
		}
	case OpTypeReg8:
		if op.Reg8Set != nil {
			op.Reg8Set(uint8(val & 0xFF))
		}
	case OpTypeMem, OpTypeMemReg:
		// Use segmented addressing
		addr := CalculateLinearAddress(op.MemSegment, op.MemAddr)
		c.Memory.WriteWordLinear(addr, val)
	}
}

// OUT instruction - write to I/O port
// OUT port, value
// Typically: OUT DX, AL (port in DX, value in AL)
func (c *CPU) execOUT(inst Instruction) error {
	// Get port number (typically from DX register or immediate)
	port := uint16(0)
	if inst.Dest.Type == OpTypeReg16 && inst.Dest.Reg16 == &c.DX {
		port = c.DX
	} else if inst.Dest.Type == OpTypeImm16 {
		port = inst.Dest.Imm16
	} else if inst.Dest.Type == OpTypeImm8 {
		port = uint16(inst.Dest.Imm8)
	} else {
		return fmt.Errorf("OUT: invalid port operand")
	}

	// Get value (typically from AL register)
	value := uint8(0)
	if inst.Src.Type == OpTypeReg8 {
		if inst.Src.Reg8Get != nil {
			value = inst.Src.Reg8Get()
		}
	} else if inst.Src.Type == OpTypeImm8 {
		value = inst.Src.Imm8
	} else {
		return fmt.Errorf("OUT: invalid value operand")
	}

	c.OutByte(port, value)
	return nil
}

// IN instruction - read from I/O port
// IN value, port
// Typically: IN AL, DX (port in DX, result to AL)
func (c *CPU) execIN(inst Instruction) error {
	// Get port number (typically from DX register or immediate)
	port := uint16(0)
	if inst.Src.Type == OpTypeReg16 && inst.Src.Reg16 == &c.DX {
		port = c.DX
	} else if inst.Src.Type == OpTypeImm16 {
		port = inst.Src.Imm16
	} else if inst.Src.Type == OpTypeImm8 {
		port = uint16(inst.Src.Imm8)
	} else {
		return fmt.Errorf("IN: invalid port operand")
	}

	// Read value from port
	value := c.InByte(port)

	// Store in destination (typically AL register)
	if inst.Dest.Type == OpTypeReg8 {
		if inst.Dest.Reg8Set != nil {
			inst.Dest.Reg8Set(value)
		}
	} else {
		return fmt.Errorf("IN: invalid destination operand")
	}

	return nil
}

// MOVSB - Move byte from DS:SI to ES:DI
func (c *CPU) execMOVSB(inst Instruction) error {
	// Read byte from DS:SI
	srcAddr := CalculateLinearAddress(c.DS, c.SI)
	value := c.Memory.ReadByteLinear(srcAddr)

	// Write byte to ES:DI
	destAddr := CalculateLinearAddress(c.ES, c.DI)
	c.Memory.WriteByteLinear(destAddr, value)

	// Update SI and DI (assume DF=0, increment)
	c.SI++
	c.DI++

	return nil
}

// MOVSW - Move word from DS:SI to ES:DI
func (c *CPU) execMOVSW(inst Instruction) error {
	// Read word from DS:SI
	srcAddr := CalculateLinearAddress(c.DS, c.SI)
	value := c.Memory.ReadWordLinear(srcAddr)

	// Write word to ES:DI
	destAddr := CalculateLinearAddress(c.ES, c.DI)
	c.Memory.WriteWordLinear(destAddr, value)

	// Update SI and DI by 2 (assume DF=0, increment)
	c.SI += 2
	c.DI += 2

	return nil
}

// STOSB - Store AL to ES:DI
func (c *CPU) execSTOSB(inst Instruction) error {
	// Get value from AL
	value := c.GetAL()

	// Write byte to ES:DI
	destAddr := CalculateLinearAddress(c.ES, c.DI)
	c.Memory.WriteByteLinear(destAddr, value)

	// Update DI (assume DF=0, increment)
	c.DI++

	return nil
}

// STOSW - Store AX to ES:DI
func (c *CPU) execSTOSW(inst Instruction) error {
	// Get value from AX
	value := c.AX

	// Write word to ES:DI
	destAddr := CalculateLinearAddress(c.ES, c.DI)
	c.Memory.WriteWordLinear(destAddr, value)

	// Update DI by 2 (assume DF=0, increment)
	c.DI += 2

	return nil
}
