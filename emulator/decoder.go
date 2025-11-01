package emulator

import "fmt"

// Decode decodes the next instruction at the current IP
func (c *CPU) Decode() (Instruction, error) {
	// Note: len(c.Memory.RAM) is 0x10000, which wraps to 0 when cast to uint16
	// So we need to check against the constant instead
	if int(c.IP) >= len(c.Memory.RAM) {
		return Instruction{}, fmt.Errorf("IP out of bounds: 0x%04X", c.IP)
	}

	opcode := Opcode(c.Memory.ReadByte(c.IP))
	c.IP++

	inst := Instruction{
		Opcode: opcode,
		Size:   1,
	}

	// Decode operands based on instruction
	numOperands := getOperandCount(opcode)

	for i := 0; i < numOperands; i++ {
		op, size, err := c.decodeOperand()
		if err != nil {
			return inst, err
		}

		inst.Size += size

		if i == 0 {
			inst.Dest = op
		} else {
			inst.Src = op
		}
	}

	return inst, nil
}

func (c *CPU) decodeOperand() (Operand, int, error) {
	if int(c.IP) >= len(c.Memory.RAM) {
		return Operand{}, 0, fmt.Errorf("unexpected end of instruction")
	}

	opType := OperandType(c.Memory.ReadByte(c.IP))
	c.IP++
	size := 1

	var op Operand
	op.Type = opType

	switch opType {
	case OpTypeReg16:
		regCode := c.Memory.ReadByte(c.IP)
		c.IP++
		size++

		reg, err := c.decodeRegister16(regCode)
		if err != nil {
			return op, size, err
		}
		op.Reg16 = reg

	case OpTypeReg8:
		regCode := c.Memory.ReadByte(c.IP)
		c.IP++
		size++

		getter, setter, err := c.decodeRegister8(regCode)
		if err != nil {
			return op, size, err
		}
		op.Reg8Get = getter
		op.Reg8Set = setter

	case OpTypeImm8:
		op.Imm8 = c.Memory.ReadByte(c.IP)
		c.IP++
		size++

	case OpTypeImm16:
		op.Imm16 = c.Memory.ReadWord(c.IP)
		c.IP += 2
		size += 2

	case OpTypeMem:
		op.MemAddr = c.Memory.ReadWord(c.IP)
		c.IP += 2
		size += 2

	case OpTypeMemReg:
		regCode := c.Memory.ReadByte(c.IP)
		c.IP++
		size++

		offset := c.Memory.ReadWord(c.IP)
		c.IP += 2
		size += 2

		// Calculate effective address
		addr, err := c.calculateMemRegAddress(regCode, offset)
		if err != nil {
			return op, size, err
		}
		op.MemAddr = addr

	default:
		return op, size, fmt.Errorf("unknown operand type: 0x%02X", opType)
	}

	return op, size, nil
}

func (c *CPU) decodeRegister16(code byte) (*uint16, error) {
	switch code {
	case 0:
		return &c.AX, nil
	case 1:
		return &c.BX, nil
	case 2:
		return &c.CX, nil
	case 3:
		return &c.DX, nil
	case 12:
		return &c.SI, nil
	case 13:
		return &c.DI, nil
	case 14:
		return &c.BP, nil
	case 15:
		return &c.SP, nil
	default:
		return nil, fmt.Errorf("invalid 16-bit register code: %d", code)
	}
}

func (c *CPU) decodeRegister8(code byte) (func() uint8, func(uint8), error) {
	switch code {
	case 4: // AL
		return c.GetAL, c.SetAL, nil
	case 5: // AH
		return c.GetAH, c.SetAH, nil
	case 6: // BL
		return c.GetBL, c.SetBL, nil
	case 7: // BH
		return c.GetBH, c.SetBH, nil
	case 8: // CL
		return c.GetCL, c.SetCL, nil
	case 9: // CH
		return c.GetCH, c.SetCH, nil
	case 10: // DL
		return c.GetDL, c.SetDL, nil
	case 11: // DH
		return c.GetDH, c.SetDH, nil
	default:
		return nil, nil, fmt.Errorf("invalid 8-bit register code: %d", code)
	}
}

func (c *CPU) calculateMemRegAddress(regCode byte, offset uint16) (uint16, error) {
	var base uint16

	switch regCode {
	case 0:
		base = c.AX
	case 1:
		base = c.BX
	case 2:
		base = c.CX
	case 3:
		base = c.DX
	case 12:
		base = c.SI
	case 13:
		base = c.DI
	case 14:
		base = c.BP
	case 15:
		base = c.SP
	default:
		return 0, fmt.Errorf("invalid register code for memory addressing: %d", regCode)
	}

	return base + offset, nil
}

func getOperandCount(opcode Opcode) int {
	switch opcode {
	case OpMOV, OpXCHG, OpADD, OpSUB:
		return 2
	case OpMUL, OpDIV, OpIMUL, OpIDIV:
		return 1
	case OpAND, OpOR, OpXOR, OpSHL, OpSHR, OpSAL, OpSAR, OpROL, OpROR:
		return 2
	case OpCMP, OpTEST:
		return 2
	case OpPUSH, OpPOP, OpINC, OpDEC, OpNEG, OpNOT:
		return 1
	case OpJMP, OpJE, OpJNE, OpJG, OpJGE, OpJL, OpJLE:
		return 1
	case OpJA, OpJAE, OpJB, OpJBE:
		return 1
	case OpCALL, OpLOOP, OpLOOPZ, OpLOOPNZ:
		return 1
	case OpINT:
		return 1
	case OpRET, OpNOP, OpHLT:
		return 0
	case OpOUT, OpIN:
		return 2
	default:
		return 0
	}
}

// Run executes the program until HLT or error
func (c *CPU) Run() error {
	for !c.Halted {
		if err := c.Step(); err != nil {
			return err
		}
	}
	return nil
}

// Step executes a single instruction
func (c *CPU) Step() error {
	if c.Halted {
		return fmt.Errorf("CPU is halted")
	}

	inst, err := c.Decode()
	if err != nil {
		return fmt.Errorf("decode error at IP=0x%04X: %v", c.IP-1, err)
	}

	if err := c.Execute(inst); err != nil {
		return fmt.Errorf("execution error at IP=0x%04X: %v", c.IP-uint16(inst.Size), err)
	}

	return nil
}
