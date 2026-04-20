package emulator

import "fmt"

// Decode decodes the next instruction at the current IP using CS:IP
func (c *CPU) Decode() (Instruction, error) {
	// Calculate linear address from CS:IP
	addr := CalculateLinearAddress(c.CS, c.IP)
	if addr >= TotalMemorySize {
		return Instruction{}, fmt.Errorf("IP out of bounds: CS:IP = %04X:%04X (linear: 0x%05X)", c.CS, c.IP, addr)
	}

	// Check for REP prefix (0xF3)
	firstByte := c.Memory.ReadByteLinear(addr)
	hasREP := false
	if firstByte == 0xF3 {
		hasREP = true
		c.IP++
		addr = CalculateLinearAddress(c.CS, c.IP)
		if addr >= TotalMemorySize {
			return Instruction{}, fmt.Errorf("IP out of bounds after REP prefix")
		}
	}

	opcode := Opcode(c.Memory.ReadByteLinear(addr))
	c.IP++

	inst := Instruction{
		Opcode: opcode,
		Size:   1,
		HasREP: hasREP,
	}
	if hasREP {
		inst.Size++ // Account for REP prefix byte
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
	addr := CalculateLinearAddress(c.CS, c.IP)
	if addr >= TotalMemorySize {
		return Operand{}, 0, fmt.Errorf("unexpected end of instruction")
	}

	opType := OperandType(c.Memory.ReadByteLinear(addr))
	c.IP++
	size := 1

	var op Operand
	op.Type = opType

	switch opType {
	case OpTypeReg16:
		addr = CalculateLinearAddress(c.CS, c.IP)
		regCode := c.Memory.ReadByteLinear(addr)
		c.IP++
		size++

		reg, err := c.decodeRegister16(regCode)
		if err != nil {
			return op, size, err
		}
		op.Reg16 = reg

	case OpTypeReg8:
		addr = CalculateLinearAddress(c.CS, c.IP)
		regCode := c.Memory.ReadByteLinear(addr)
		c.IP++
		size++

		getter, setter, err := c.decodeRegister8(regCode)
		if err != nil {
			return op, size, err
		}
		op.Reg8Get = getter
		op.Reg8Set = setter

	case OpTypeImm8:
		addr = CalculateLinearAddress(c.CS, c.IP)
		op.Imm8 = c.Memory.ReadByteLinear(addr)
		c.IP++
		size++

	case OpTypeImm16:
		addr = CalculateLinearAddress(c.CS, c.IP)
		op.Imm16 = c.Memory.ReadWordLinear(addr)
		c.IP += 2
		size += 2

	case OpTypeMem:
		addr = CalculateLinearAddress(c.CS, c.IP)
		op.MemAddr = c.Memory.ReadWordLinear(addr)
		c.IP += 2
		size += 2
		// Default to DS segment for direct memory access
		op.MemSegment = c.DS
		op.SegOverride = false

	case OpTypeMemReg:
		addr = CalculateLinearAddress(c.CS, c.IP)
		regCode := c.Memory.ReadByteLinear(addr)
		c.IP++
		size++

		addr = CalculateLinearAddress(c.CS, c.IP)
		offset := c.Memory.ReadWordLinear(addr)
		c.IP += 2
		size += 2

		// Calculate effective offset (not linear address yet)
		effectiveOffset, seg, err := c.calculateMemRegAddress(regCode, offset)
		if err != nil {
			return op, size, err
		}
		op.MemAddr = effectiveOffset
		op.MemSegment = seg
		op.SegOverride = false

	case OpTypeMemSegReg:
		// Encoding: [segRegCode:1][regCode:1][offset:2]
		addr = CalculateLinearAddress(c.CS, c.IP)
		segRegCode := c.Memory.ReadByteLinear(addr)
		c.IP++
		size++

		addr = CalculateLinearAddress(c.CS, c.IP)
		regCode := c.Memory.ReadByteLinear(addr)
		c.IP++
		size++

		addr = CalculateLinearAddress(c.CS, c.IP)
		offset := c.Memory.ReadWordLinear(addr)
		c.IP += 2
		size += 2

		segPtr, err := c.decodeRegister16(segRegCode)
		if err != nil {
			return op, size, err
		}
		effectiveOffset, _, err := c.calculateMemRegAddress(regCode, offset)
		if err != nil {
			return op, size, err
		}
		op.MemAddr = effectiveOffset
		op.MemSegment = *segPtr
		op.SegOverride = true

	case OpTypeReg32:
		addr = CalculateLinearAddress(c.CS, c.IP)
		regCode := c.Memory.ReadByteLinear(addr)
		c.IP++
		size++

		getter, setter, err := c.decodeRegister32(regCode)
		if err != nil {
			return op, size, err
		}
		op.Reg32Get = getter
		op.Reg32Set = setter

	case OpTypeFPUReg:
		addr = CalculateLinearAddress(c.CS, c.IP)
		op.FPURegIdx = int(c.Memory.ReadByteLinear(addr))
		c.IP++
		size++

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
	// Segment registers
	case 16:
		return &c.CS, nil
	case 17:
		return &c.DS, nil
	case 18:
		return &c.ES, nil
	case 19:
		return &c.SS, nil
	default:
		return nil, fmt.Errorf("invalid 16-bit register code: %d", code)
	}
}

func (c *CPU) decodeRegister32(code byte) (func() uint32, func(uint32), error) {
	switch code {
	case 20:
		return c.GetEAX, c.SetEAX, nil
	case 21:
		return c.GetEBX, c.SetEBX, nil
	case 22:
		return c.GetECX, c.SetECX, nil
	case 23:
		return c.GetEDX, c.SetEDX, nil
	case 24:
		return c.GetESI, c.SetESI, nil
	case 25:
		return c.GetEDI, c.SetEDI, nil
	case 26:
		return c.GetEBP, c.SetEBP, nil
	case 27:
		return c.GetESP, c.SetESP, nil
	default:
		return nil, nil, fmt.Errorf("invalid 32-bit register code: %d", code)
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

func (c *CPU) calculateMemRegAddress(regCode byte, offset uint16) (uint16, uint16, error) {
	var base uint16
	var segment uint16

	switch regCode {
	case 0:
		base = c.AX
		segment = c.DS // Default to DS
	case 1:
		base = c.BX
		segment = c.DS
	case 2:
		base = c.CX
		segment = c.DS
	case 3:
		base = c.DX
		segment = c.DS
	case 12:
		base = c.SI
		segment = c.DS
	case 13:
		base = c.DI
		segment = c.ES // DI typically uses ES for string operations
	case 14:
		base = c.BP
		segment = c.SS // BP typically uses SS (stack frame access)
	case 15:
		base = c.SP
		segment = c.SS // SP uses SS
	default:
		return 0, 0, fmt.Errorf("invalid register code for memory addressing: %d", regCode)
	}

	return base + offset, segment, nil
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

	// FPU no-operand
	case OpFINIT, OpFLDZ, OpFLD1, OpFLDPI,
		OpFCHS, OpFABS, OpFSQRT, OpFSIN, OpFCOS, OpFPTAN, OpFPATAN, OpFCOMPP,
		OpFADD0, OpFSUB0, OpFMUL0, OpFDIV0, OpFSUBR0, OpFDIVR0,
		OpFADDP, OpFSUBP, OpFMULP, OpFDIVP, OpFSUBRP, OpFDIVRP:
		return 0

	// FPU 1 FPU-register operand
	case OpFXCH, OpFLDReg, OpFSTPReg, OpFSTReg, OpFCOMReg, OpFCOMPReg:
		return 1

	// FPU 1 memory operand
	case OpFILD, OpFIST, OpFISTP, OpFILD32, OpFIST32, OpFISTP32,
		OpFLD32M, OpFST32M, OpFSTP32M, OpFLD64M, OpFST64M, OpFSTP64M,
		OpFIADD, OpFISUB, OpFIMUL, OpFIDIV,
		OpFIADD32, OpFISUB32, OpFIMUL32, OpFIDIV32,
		OpFADDM, OpFSUBM, OpFMULM, OpFDIVM:
		return 1

	// FPU 1 FPU-register operand (register arithmetic forms)
	case OpFADDReg, OpFSUBReg, OpFMULReg, OpFDIVReg, OpFSUBRReg, OpFDIVRReg,
		OpFADDST0, OpFSUBST0, OpFMULST0, OpFDIVST0, OpFSUBRST0, OpFDIVRST0:
		return 1

	default:
		return 0
	}
}

// Run executes the program until HLT or error
func (c *CPU) Run() error {
	for !c.Halted {
		// Check if stop signal was received (non-blocking)
		select {
		case <-c.stopChan:
			return fmt.Errorf("CPU stopped by external signal")
		default:
			// Continue execution
		}

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

	// Handle REP prefix for string instructions
	if inst.HasREP {
		// REP repeats the string instruction CX times
		switch inst.Opcode {
		case OpMOVSB, OpMOVSW, OpSTOSB, OpSTOSW, OpLODSB, OpLODSW:
			repCount := c.CX
			for c.CX > 0 {
				if err := c.Execute(inst); err != nil {
					return fmt.Errorf("execution error at IP=0x%04X: %v", c.IP-uint16(inst.Size), err)
				}
				c.CX--
			}
			// Count REP iterations as separate instructions
			c.InstructionCount += uint64(repCount)
			return nil
		default:
			return fmt.Errorf("REP prefix not valid for opcode 0x%02X", inst.Opcode)
		}
	}

	if err := c.Execute(inst); err != nil {
		return fmt.Errorf("execution error at IP=0x%04X: %v", c.IP-uint16(inst.Size), err)
	}

	// Increment instruction counter
	c.InstructionCount++

	return nil
}
