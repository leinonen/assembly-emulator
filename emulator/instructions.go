package emulator

import (
	"fmt"
	"math"
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
	OpLODSB Opcode = 0x74
	OpLODSW Opcode = 0x75

	// FPU: no-operand instructions
	OpFINIT  Opcode = 0x80 // Initialize FPU
	OpFLDZ   Opcode = 0x81 // Push +0.0
	OpFLD1   Opcode = 0x82 // Push +1.0
	OpFLDPI  Opcode = 0x83 // Push pi
	OpFCHS   Opcode = 0x84 // Negate ST(0)
	OpFABS   Opcode = 0x85 // Abs ST(0)
	OpFSQRT  Opcode = 0x86 // sqrt(ST(0))
	OpFSIN   Opcode = 0x87 // sin(ST(0))
	OpFCOS   Opcode = 0x88 // cos(ST(0))
	OpFPTAN  Opcode = 0x89 // ST(0)=tan(ST(0)), push 1.0
	OpFPATAN Opcode = 0x8A // ST(1)=atan2(ST(1),ST(0)), pop
	OpFCOMPP Opcode = 0x8B // Compare ST(0) with ST(1), pop twice
	// FPU: 0-operand stack arithmetic (ST(0) op ST(1))
	OpFADD0  Opcode = 0x8C // ST(0) += ST(1)
	OpFSUB0  Opcode = 0x8D // ST(0) -= ST(1)
	OpFMUL0  Opcode = 0x8E // ST(0) *= ST(1)
	OpFDIV0  Opcode = 0x8F // ST(0) /= ST(1)
	OpFSUBR0 Opcode = 0x90 // ST(0) = ST(1) - ST(0)
	OpFDIVR0 Opcode = 0x91 // ST(0) = ST(1) / ST(0)
	// pop variants: ST(1) op= ST(0), pop
	OpFADDP  Opcode = 0x92
	OpFSUBP  Opcode = 0x93
	OpFMULP  Opcode = 0x94
	OpFDIVP  Opcode = 0x95
	OpFSUBRP Opcode = 0x96
	OpFDIVRP Opcode = 0x97

	// FPU: 1 FPU-register operand
	OpFXCH    Opcode = 0x98 // FXCH ST(0) with ST(i) (0 means default ST1)
	OpFLDReg  Opcode = 0x99 // FLD ST(i) - push copy
	OpFSTPReg Opcode = 0x9A // FSTP ST(i) - copy and pop
	OpFSTReg  Opcode = 0x9B // FST ST(i) - copy, no pop
	OpFCOMReg Opcode = 0x9C // FCOM ST(i)
	OpFCOMPReg Opcode = 0x9D // FCOMP ST(i)

	// FPU: 1 memory operand (integer)
	OpFILD    Opcode = 0xA0 // Load int16 from memory, push
	OpFIST    Opcode = 0xA1 // Store ST(0) as int16, no pop
	OpFISTP   Opcode = 0xA2 // Store ST(0) as int16, pop
	OpFILD32  Opcode = 0xA3 // Load int32 from memory, push
	OpFIST32  Opcode = 0xA4 // Store ST(0) as int32, no pop
	OpFISTP32 Opcode = 0xA5 // Store ST(0) as int32, pop

	// FPU: 1 memory operand (float)
	OpFLD32M  Opcode = 0xA6 // Load float32 from memory, push
	OpFST32M  Opcode = 0xA7 // Store ST(0) as float32, no pop
	OpFSTP32M Opcode = 0xA8 // Store ST(0) as float32, pop
	OpFLD64M  Opcode = 0xA9 // Load float64 from memory, push
	OpFST64M  Opcode = 0xAA // Store ST(0) as float64, no pop
	OpFSTP64M Opcode = 0xAB // Store ST(0) as float64, pop

	// FPU: integer arithmetic (1 memory operand, word)
	OpFIADD  Opcode = 0xAC
	OpFISUB  Opcode = 0xAD
	OpFIMUL  Opcode = 0xAE
	OpFIDIV  Opcode = 0xAF

	// FPU: integer arithmetic (1 memory operand, dword)
	OpFIADD32 Opcode = 0xB0
	OpFISUB32 Opcode = 0xB1
	OpFIMUL32 Opcode = 0xB2
	OpFIDIV32 Opcode = 0xB3

	// FPU: float arithmetic (1 memory operand, dword float)
	OpFADDM  Opcode = 0xB4 // ST(0) += mem32
	OpFSUBM  Opcode = 0xB5
	OpFMULM  Opcode = 0xB6
	OpFDIVM  Opcode = 0xB7

	// FPU: register arithmetic (2 FPU-register operands or 1 non-ST0 reg)
	OpFADDReg  Opcode = 0xB8 // ST(i) += ST(0), operand = dest reg idx
	OpFSUBReg  Opcode = 0xB9
	OpFMULReg  Opcode = 0xBA
	OpFDIVReg  Opcode = 0xBB
	OpFSUBRReg Opcode = 0xBC // ST(i) = ST(0) - ST(i)
	OpFDIVRReg Opcode = 0xBD // ST(i) = ST(0) / ST(i)
	// ST(0) op= ST(i) forms (dest is ST0)
	OpFADDST0  Opcode = 0xBE // ST(0) += ST(i)
	OpFSUBST0  Opcode = 0xBF
	OpFMULST0  Opcode = 0xC0
	OpFDIVST0  Opcode = 0xC1
	OpFSUBRST0 Opcode = 0xC2
	OpFDIVRST0 Opcode = 0xC3
)

// Operand types
type OperandType byte

const (
	OpTypeNone      OperandType = 0
	OpTypeReg16     OperandType = 1 // 16-bit register
	OpTypeReg8      OperandType = 2 // 8-bit register
	OpTypeImm16     OperandType = 3 // 16-bit immediate
	OpTypeImm8      OperandType = 4 // 8-bit immediate
	OpTypeMem       OperandType = 5 // Memory address
	OpTypeMemReg    OperandType = 6 // Memory [register]
	OpTypeMemSegReg OperandType = 7 // Memory [segment:register+offset]
	OpTypeReg32     OperandType = 8 // 32-bit register (EAX, EBX, ...)
	OpTypeFPUReg    OperandType = 9 // FPU stack register ST(0)..ST(7)
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
	Reg32Get    func() uint32 // 32-bit register getter
	Reg32Set    func(uint32)  // 32-bit register setter
	FPURegIdx   int           // FPU ST register index (0-7) for OpTypeFPUReg
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
	case OpIDIV:
		return c.execIDIV(inst)
	case OpIMUL:
		return c.execIMUL(inst)
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
	case OpROL:
		return c.execROL(inst)
	case OpROR:
		return c.execROR(inst)

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
	case OpLODSB:
		return c.execLODSB(inst)
	case OpLODSW:
		return c.execLODSW(inst)
	case OpSTOSW:
		return c.execSTOSW(inst)

	// FPU no-operand
	case OpFINIT:
		c.FPU.Init()
		return nil
	case OpFLDZ:
		c.FPU.Push(0)
		return nil
	case OpFLD1:
		c.FPU.Push(1)
		return nil
	case OpFLDPI:
		c.FPU.Push(math.Pi)
		return nil
	case OpFCHS:
		c.FPU.SetST0(-c.FPU.ST0())
		return nil
	case OpFABS:
		c.FPU.SetST0(math.Abs(c.FPU.ST0()))
		return nil
	case OpFSQRT:
		c.FPU.SetST0(math.Sqrt(c.FPU.ST0()))
		return nil
	case OpFSIN:
		c.FPU.SetST0(math.Sin(c.FPU.ST0()))
		return nil
	case OpFCOS:
		c.FPU.SetST0(math.Cos(c.FPU.ST0()))
		return nil
	case OpFPTAN:
		c.FPU.SetST0(math.Tan(c.FPU.ST0()))
		c.FPU.Push(1)
		return nil
	case OpFPATAN:
		y := c.FPU.ST0()
		x := c.FPU.STn(1)
		c.FPU.Pop()
		c.FPU.SetST0(math.Atan2(x, y))
		return nil
	case OpFCOMPP:
		c.FPU.Compare(c.FPU.ST0(), c.FPU.STn(1))
		c.FPU.Pop()
		c.FPU.Pop()
		return nil
	case OpFADD0:
		c.FPU.SetST0(c.FPU.ST0() + c.FPU.STn(1))
		return nil
	case OpFSUB0:
		c.FPU.SetST0(c.FPU.ST0() - c.FPU.STn(1))
		return nil
	case OpFMUL0:
		c.FPU.SetST0(c.FPU.ST0() * c.FPU.STn(1))
		return nil
	case OpFDIV0:
		c.FPU.SetST0(c.FPU.ST0() / c.FPU.STn(1))
		return nil
	case OpFSUBR0:
		c.FPU.SetST0(c.FPU.STn(1) - c.FPU.ST0())
		return nil
	case OpFDIVR0:
		c.FPU.SetST0(c.FPU.STn(1) / c.FPU.ST0())
		return nil
	case OpFADDP:
		c.FPU.SetSTn(1, c.FPU.STn(1)+c.FPU.ST0())
		c.FPU.Pop()
		return nil
	case OpFSUBP:
		c.FPU.SetSTn(1, c.FPU.STn(1)-c.FPU.ST0())
		c.FPU.Pop()
		return nil
	case OpFMULP:
		c.FPU.SetSTn(1, c.FPU.STn(1)*c.FPU.ST0())
		c.FPU.Pop()
		return nil
	case OpFDIVP:
		c.FPU.SetSTn(1, c.FPU.STn(1)/c.FPU.ST0())
		c.FPU.Pop()
		return nil
	case OpFSUBRP:
		c.FPU.SetSTn(1, c.FPU.ST0()-c.FPU.STn(1))
		c.FPU.Pop()
		return nil
	case OpFDIVRP:
		c.FPU.SetSTn(1, c.FPU.ST0()/c.FPU.STn(1))
		c.FPU.Pop()
		return nil

	// FPU 1 FPU-register operand
	case OpFXCH:
		i := inst.Dest.FPURegIdx
		tmp := c.FPU.ST0()
		c.FPU.SetST0(c.FPU.STn(i))
		c.FPU.SetSTn(i, tmp)
		return nil
	case OpFLDReg:
		c.FPU.Push(c.FPU.STn(inst.Dest.FPURegIdx))
		return nil
	case OpFSTPReg:
		val := c.FPU.ST0()
		c.FPU.Pop()
		c.FPU.SetSTn(inst.Dest.FPURegIdx, val)
		return nil
	case OpFSTReg:
		c.FPU.SetSTn(inst.Dest.FPURegIdx, c.FPU.ST0())
		return nil
	case OpFCOMReg:
		c.FPU.Compare(c.FPU.ST0(), c.FPU.STn(inst.Dest.FPURegIdx))
		return nil
	case OpFCOMPReg:
		c.FPU.Compare(c.FPU.ST0(), c.FPU.STn(inst.Dest.FPURegIdx))
		c.FPU.Pop()
		return nil

	// FPU integer load/store (word)
	case OpFILD:
		return c.execFILD(inst)
	case OpFIST:
		return c.execFIST(inst, false)
	case OpFISTP:
		return c.execFIST(inst, true)
	// FPU integer load/store (dword)
	case OpFILD32:
		return c.execFILD32(inst)
	case OpFIST32:
		return c.execFIST32(inst, false)
	case OpFISTP32:
		return c.execFIST32(inst, true)

	// FPU float load/store (32-bit)
	case OpFLD32M:
		return c.execFLD32M(inst)
	case OpFST32M:
		return c.execFST32M(inst, false)
	case OpFSTP32M:
		return c.execFST32M(inst, true)
	// FPU float load/store (64-bit)
	case OpFLD64M:
		return c.execFLD64M(inst)
	case OpFST64M:
		return c.execFST64M(inst, false)
	case OpFSTP64M:
		return c.execFST64M(inst, true)

	// FPU integer arithmetic (word)
	case OpFIADD:
		return c.execFIArith(inst, func(a, b float64) float64 { return a + b })
	case OpFISUB:
		return c.execFIArith(inst, func(a, b float64) float64 { return a - b })
	case OpFIMUL:
		return c.execFIArith(inst, func(a, b float64) float64 { return a * b })
	case OpFIDIV:
		return c.execFIArith(inst, func(a, b float64) float64 { return a / b })
	// FPU integer arithmetic (dword)
	case OpFIADD32:
		return c.execFIArith32(inst, func(a, b float64) float64 { return a + b })
	case OpFISUB32:
		return c.execFIArith32(inst, func(a, b float64) float64 { return a - b })
	case OpFIMUL32:
		return c.execFIArith32(inst, func(a, b float64) float64 { return a * b })
	case OpFIDIV32:
		return c.execFIArith32(inst, func(a, b float64) float64 { return a / b })

	// FPU float arithmetic (memory operand)
	case OpFADDM:
		return c.execFArithM(inst, func(a, b float64) float64 { return a + b })
	case OpFSUBM:
		return c.execFArithM(inst, func(a, b float64) float64 { return a - b })
	case OpFMULM:
		return c.execFArithM(inst, func(a, b float64) float64 { return a * b })
	case OpFDIVM:
		return c.execFArithM(inst, func(a, b float64) float64 { return a / b })

	// FPU register arithmetic (ST(i) as dest, ST0 as src)
	case OpFADDReg:
		i := inst.Dest.FPURegIdx
		c.FPU.SetSTn(i, c.FPU.STn(i)+c.FPU.ST0())
		return nil
	case OpFSUBReg:
		i := inst.Dest.FPURegIdx
		c.FPU.SetSTn(i, c.FPU.STn(i)-c.FPU.ST0())
		return nil
	case OpFMULReg:
		i := inst.Dest.FPURegIdx
		c.FPU.SetSTn(i, c.FPU.STn(i)*c.FPU.ST0())
		return nil
	case OpFDIVReg:
		i := inst.Dest.FPURegIdx
		c.FPU.SetSTn(i, c.FPU.STn(i)/c.FPU.ST0())
		return nil
	case OpFSUBRReg:
		i := inst.Dest.FPURegIdx
		c.FPU.SetSTn(i, c.FPU.ST0()-c.FPU.STn(i))
		return nil
	case OpFDIVRReg:
		i := inst.Dest.FPURegIdx
		c.FPU.SetSTn(i, c.FPU.ST0()/c.FPU.STn(i))
		return nil
	// FPU register arithmetic (ST0 as dest, ST(i) as src)
	case OpFADDST0:
		c.FPU.SetST0(c.FPU.ST0() + c.FPU.STn(inst.Dest.FPURegIdx))
		return nil
	case OpFSUBST0:
		c.FPU.SetST0(c.FPU.ST0() - c.FPU.STn(inst.Dest.FPURegIdx))
		return nil
	case OpFMULST0:
		c.FPU.SetST0(c.FPU.ST0() * c.FPU.STn(inst.Dest.FPURegIdx))
		return nil
	case OpFDIVST0:
		c.FPU.SetST0(c.FPU.ST0() / c.FPU.STn(inst.Dest.FPURegIdx))
		return nil
	case OpFSUBRST0:
		c.FPU.SetST0(c.FPU.STn(inst.Dest.FPURegIdx) - c.FPU.ST0())
		return nil
	case OpFDIVRST0:
		c.FPU.SetST0(c.FPU.STn(inst.Dest.FPURegIdx) / c.FPU.ST0())
		return nil

	default:
		return fmt.Errorf("unknown opcode: 0x%02X", inst.Opcode)
	}
}

// MOV instruction
func (c *CPU) execMOV(inst Instruction) error {
	// 32-bit path
	if is32bit(inst) {
		val := c.getOperandValue32(inst.Src)
		c.setOperandValue32(inst.Dest, val)
		return nil
	}

	isSrcMem := inst.Src.Type == OpTypeMem || inst.Src.Type == OpTypeMemReg || inst.Src.Type == OpTypeMemSegReg
	isDestMem := inst.Dest.Type == OpTypeMem || inst.Dest.Type == OpTypeMemReg || inst.Dest.Type == OpTypeMemSegReg

	// 8-bit memory read (e.g., MOV AL, [SI])
	if isSrcMem && inst.Dest.Type == OpTypeReg8 {
		addr := CalculateLinearAddress(inst.Src.MemSegment, inst.Src.MemAddr)
		val := c.Memory.ReadByteLinear(addr)
		c.setOperandValue(inst.Dest, uint16(val))
		return nil
	}

	// 8-bit memory write (e.g., MOV [DI], AL)
	if (inst.Src.Type == OpTypeReg8 || inst.Src.Type == OpTypeImm8) && isDestMem {
		addr := CalculateLinearAddress(inst.Dest.MemSegment, inst.Dest.MemAddr)
		val := c.getOperandValue(inst.Src)
		c.Memory.WriteByteLinear(addr, uint8(val&0xFF))
		return nil
	}

	val := c.getOperandValue(inst.Src)
	c.setOperandValue(inst.Dest, val)
	return nil
}

// PUSH instruction
func (c *CPU) execPUSH(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		return c.Push32(c.getOperandValue32(inst.Dest))
	}
	return c.Push(c.getOperandValue(inst.Dest))
}

// POP instruction
func (c *CPU) execPOP(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val, err := c.Pop32()
		if err != nil {
			return err
		}
		c.setOperandValue32(inst.Dest, val)
		return nil
	}
	val, err := c.Pop()
	if err != nil {
		return err
	}
	c.setOperandValue(inst.Dest, val)
	return nil
}

// XCHG instruction
func (c *CPU) execXCHG(inst Instruction) error {
	if is32bit(inst) {
		v1 := c.getOperandValue32(inst.Dest)
		v2 := c.getOperandValue32(inst.Src)
		c.setOperandValue32(inst.Dest, v2)
		c.setOperandValue32(inst.Src, v1)
		return nil
	}
	val1 := c.getOperandValue(inst.Dest)
	val2 := c.getOperandValue(inst.Src)
	c.setOperandValue(inst.Dest, val2)
	c.setOperandValue(inst.Src, val1)
	return nil
}

// ADD instruction
func (c *CPU) execADD(inst Instruction) error {
	if is32bit(inst) {
		dest := c.getOperandValue32(inst.Dest)
		src := c.getOperandValue32(inst.Src)
		result := dest + src
		c.Flags.CF = result < dest
		c.Flags.OF = ((dest^result)&(src^result)&0x80000000) != 0
		c.UpdateFlags32(result)
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest + src
	c.Flags.CF = result < dest
	c.Flags.OF = ((dest^result)&(src^result)&0x8000) != 0
	c.UpdateFlags(result)
	c.setOperandValue(inst.Dest, result)
	return nil
}

// SUB instruction
func (c *CPU) execSUB(inst Instruction) error {
	if is32bit(inst) {
		dest := c.getOperandValue32(inst.Dest)
		src := c.getOperandValue32(inst.Src)
		result := dest - src
		c.Flags.CF = src > dest
		c.Flags.OF = ((dest^src)&(dest^result)&0x80000000) != 0
		c.UpdateFlags32(result)
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest - src
	c.Flags.CF = src > dest
	c.Flags.OF = ((dest^src)&(dest^result)&0x8000) != 0
	c.UpdateFlags(result)
	c.setOperandValue(inst.Dest, result)
	return nil
}

// MUL instruction (unsigned)
func (c *CPU) execMUL(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		// 32-bit: EDX:EAX = EAX * src
		src := c.getOperandValue32(inst.Dest)
		result := uint64(c.GetEAX()) * uint64(src)
		c.SetEAX(uint32(result))
		c.SetEDX(uint32(result >> 32))
		c.Flags.CF = c.GetEDX() != 0
		c.Flags.OF = c.Flags.CF
		return nil
	}
	src := c.getOperandValue(inst.Dest)
	result := uint32(c.AX) * uint32(src)
	c.AX = uint16(result & 0xFFFF)
	c.DX = uint16((result >> 16) & 0xFFFF)
	c.Flags.CF = c.DX != 0
	c.Flags.OF = c.DX != 0
	return nil
}

// DIV instruction (unsigned)
func (c *CPU) execDIV(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		divisor := uint64(c.getOperandValue32(inst.Dest))
		if divisor == 0 {
			return fmt.Errorf("division by zero")
		}
		dividend := (uint64(c.GetEDX()) << 32) | uint64(c.GetEAX())
		q := dividend / divisor
		r := dividend % divisor
		if q > 0xFFFFFFFF {
			return fmt.Errorf("division overflow")
		}
		c.SetEAX(uint32(q))
		c.SetEDX(uint32(r))
		return nil
	}
	divisor := uint32(c.getOperandValue(inst.Dest))
	if divisor == 0 {
		return fmt.Errorf("division by zero")
	}
	dividend := (uint32(c.DX) << 16) | uint32(c.AX)
	q := dividend / divisor
	r := dividend % divisor
	if q > 0xFFFF {
		return fmt.Errorf("division overflow")
	}
	c.AX = uint16(q)
	c.DX = uint16(r)
	return nil
}

// IMUL instruction (signed multiply)
func (c *CPU) execIMUL(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		src := int64(int32(c.getOperandValue32(inst.Dest)))
		result := int64(int32(c.GetEAX())) * src
		c.SetEAX(uint32(result))
		c.SetEDX(uint32(result >> 32))
		signBit := (c.GetEAX() & 0x80000000) != 0
		expected := uint32(0)
		if signBit {
			expected = 0xFFFFFFFF
		}
		c.Flags.CF = c.GetEDX() != expected
		c.Flags.OF = c.Flags.CF
		return nil
	}
	src := int16(c.getOperandValue(inst.Dest))
	result := int32(int16(c.AX)) * int32(src)
	c.AX = uint16(result & 0xFFFF)
	c.DX = uint16((result >> 16) & 0xFFFF)
	signBit := (c.AX & 0x8000) != 0
	expected := uint16(0)
	if signBit {
		expected = 0xFFFF
	}
	c.Flags.CF = c.DX != expected
	c.Flags.OF = c.Flags.CF
	return nil
}

// INC instruction
func (c *CPU) execINC(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val := c.getOperandValue32(inst.Dest)
		result := val + 1
		c.Flags.OF = (val == 0x7FFFFFFF)
		c.UpdateFlags32(result)
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	val := c.getOperandValue(inst.Dest)
	result := val + 1
	if inst.Dest.Type == OpTypeReg8 {
		result = result & 0xFF
		c.Flags.OF = (val == 0x7F)
	} else {
		c.Flags.OF = (val == 0x7FFF)
	}
	c.UpdateFlags(result)
	c.setOperandValue(inst.Dest, result)
	return nil
}

// DEC instruction
func (c *CPU) execDEC(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val := c.getOperandValue32(inst.Dest)
		result := val - 1
		c.Flags.OF = (val == 0x80000000)
		c.UpdateFlags32(result)
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	val := c.getOperandValue(inst.Dest)
	result := val - 1
	if inst.Dest.Type == OpTypeReg8 {
		result = result & 0xFF
		c.Flags.OF = (val == 0x80)
	} else {
		c.Flags.OF = (val == 0x8000)
	}
	c.UpdateFlags(result)
	c.setOperandValue(inst.Dest, result)
	return nil
}

// NEG instruction
func (c *CPU) execNEG(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val := c.getOperandValue32(inst.Dest)
		result := uint32(-int32(val))
		c.Flags.CF = (val != 0)
		c.Flags.OF = (val == 0x80000000)
		c.UpdateFlags32(result)
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
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
	if is32bit(inst) {
		result := c.getOperandValue32(inst.Dest) & c.getOperandValue32(inst.Src)
		c.Flags.CF, c.Flags.OF = false, false
		c.UpdateFlags32(result)
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	result := c.getOperandValue(inst.Dest) & c.getOperandValue(inst.Src)
	c.Flags.CF, c.Flags.OF = false, false
	c.UpdateFlags(result)
	c.setOperandValue(inst.Dest, result)
	return nil
}

// OR instruction
func (c *CPU) execOR(inst Instruction) error {
	if is32bit(inst) {
		result := c.getOperandValue32(inst.Dest) | c.getOperandValue32(inst.Src)
		c.Flags.CF, c.Flags.OF = false, false
		c.UpdateFlags32(result)
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	result := c.getOperandValue(inst.Dest) | c.getOperandValue(inst.Src)
	c.Flags.CF, c.Flags.OF = false, false
	c.UpdateFlags(result)
	c.setOperandValue(inst.Dest, result)
	return nil
}

// XOR instruction
func (c *CPU) execXOR(inst Instruction) error {
	if is32bit(inst) {
		result := c.getOperandValue32(inst.Dest) ^ c.getOperandValue32(inst.Src)
		c.Flags.CF, c.Flags.OF = false, false
		c.UpdateFlags32(result)
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	result := c.getOperandValue(inst.Dest) ^ c.getOperandValue(inst.Src)
	c.Flags.CF, c.Flags.OF = false, false
	c.UpdateFlags(result)
	c.setOperandValue(inst.Dest, result)
	return nil
}

// NOT instruction
func (c *CPU) execNOT(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		c.setOperandValue32(inst.Dest, ^c.getOperandValue32(inst.Dest))
		return nil
	}
	c.setOperandValue(inst.Dest, ^c.getOperandValue(inst.Dest))
	return nil
}

// SHL instruction (shift left)
func (c *CPU) execSHL(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val := c.getOperandValue32(inst.Dest)
		count := c.getOperandValue32(inst.Src) & 31
		if count > 0 {
			c.Flags.CF = ((val >> (32 - count)) & 1) != 0
			result := val << count
			c.UpdateFlags32(result)
			c.setOperandValue32(inst.Dest, result)
		}
		return nil
	}
	val := c.getOperandValue(inst.Dest)
	count := min(c.getOperandValue(inst.Src), 16)
	if count > 0 {
		c.Flags.CF = ((val >> (16 - count)) & 1) != 0
		result := val << count
		c.UpdateFlags(result)
		c.setOperandValue(inst.Dest, result)
	}
	return nil
}

// SHR instruction (shift right logical)
func (c *CPU) execSHR(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val := c.getOperandValue32(inst.Dest)
		count := c.getOperandValue32(inst.Src) & 31
		if count > 0 {
			c.Flags.CF = ((val >> (count - 1)) & 1) != 0
			result := val >> count
			c.UpdateFlags32(result)
			c.setOperandValue32(inst.Dest, result)
		}
		return nil
	}
	val := c.getOperandValue(inst.Dest)
	count := min(c.getOperandValue(inst.Src), 16)
	if count > 0 {
		c.Flags.CF = ((val >> (count - 1)) & 1) != 0
		result := val >> count
		c.UpdateFlags(result)
		c.setOperandValue(inst.Dest, result)
	}
	return nil
}

// SAR instruction (shift right arithmetic)
func (c *CPU) execSAR(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val := c.getOperandValue32(inst.Dest)
		count := c.getOperandValue32(inst.Src) & 31
		if count > 0 {
			c.Flags.CF = ((val >> (count - 1)) & 1) != 0
			result := uint32(int32(val) >> count)
			c.UpdateFlags32(result)
			c.setOperandValue32(inst.Dest, result)
		}
		return nil
	}
	val := c.getOperandValue(inst.Dest)
	count := min(c.getOperandValue(inst.Src), 16)
	if count > 0 {
		c.Flags.CF = ((val >> (count - 1)) & 1) != 0
		result := uint16(int16(val) >> count)
		c.UpdateFlags(result)
		c.setOperandValue(inst.Dest, result)
	}
	return nil
}

// ROL instruction (rotate left)
func (c *CPU) execROL(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val := c.getOperandValue32(inst.Dest)
		count := c.getOperandValue32(inst.Src) & 31
		if count == 0 {
			return nil
		}
		result := (val << count) | (val >> (32 - count))
		c.Flags.CF = (result & 1) != 0
		if count == 1 {
			c.Flags.OF = c.Flags.CF != ((result & 0x80000000) != 0)
		}
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	val := c.getOperandValue(inst.Dest)
	count := c.getOperandValue(inst.Src) % 16
	if count == 0 {
		return nil
	}
	result := (val << count) | (val >> (16 - count))
	c.Flags.CF = (result & 1) != 0
	if count == 1 {
		c.Flags.OF = c.Flags.CF != ((result & 0x8000) != 0)
	}
	c.setOperandValue(inst.Dest, result)
	return nil
}

// ROR instruction (rotate right)
func (c *CPU) execROR(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		val := c.getOperandValue32(inst.Dest)
		count := c.getOperandValue32(inst.Src) & 31
		if count == 0 {
			return nil
		}
		result := (val >> count) | (val << (32 - count))
		c.Flags.CF = (result & 0x80000000) != 0
		if count == 1 {
			c.Flags.OF = (result&0x80000000 != 0) != (result&0x40000000 != 0)
		}
		c.setOperandValue32(inst.Dest, result)
		return nil
	}
	val := c.getOperandValue(inst.Dest)
	count := c.getOperandValue(inst.Src) % 16
	if count == 0 {
		return nil
	}
	result := (val >> count) | (val << (16 - count))
	c.Flags.CF = (result & 0x8000) != 0
	if count == 1 {
		c.Flags.OF = (result&0x8000 != 0) != (result&0x4000 != 0)
	}
	c.setOperandValue(inst.Dest, result)
	return nil
}

// IDIV instruction (signed divide)
func (c *CPU) execIDIV(inst Instruction) error {
	if inst.Dest.Type == OpTypeReg32 {
		divisor := int64(int32(c.getOperandValue32(inst.Dest)))
		if divisor == 0 {
			return fmt.Errorf("division by zero")
		}
		dividend := (int64(int32(c.GetEDX())) << 32) | int64(c.GetEAX())
		q := dividend / divisor
		r := dividend % divisor
		if q > 0x7FFFFFFF || q < -0x80000000 {
			return fmt.Errorf("division overflow")
		}
		c.SetEAX(uint32(int32(q)))
		c.SetEDX(uint32(int32(r)))
		return nil
	}
	divisor := int32(int16(c.getOperandValue(inst.Dest)))
	if divisor == 0 {
		return fmt.Errorf("division by zero")
	}
	dividend := (int32(int16(c.DX)) << 16) | int32(c.AX)
	q := dividend / divisor
	r := dividend % divisor
	if q > 0x7FFF || q < -0x8000 {
		return fmt.Errorf("division overflow")
	}
	c.AX = uint16(int16(q))
	c.DX = uint16(int16(r))
	return nil
}

// CMP instruction
func (c *CPU) execCMP(inst Instruction) error {
	if is32bit(inst) {
		dest := c.getOperandValue32(inst.Dest)
		src := c.getOperandValue32(inst.Src)
		result := dest - src
		c.Flags.CF = src > dest
		c.Flags.OF = ((dest^src)&(dest^result)&0x80000000) != 0
		c.UpdateFlags32(result)
		return nil
	}
	dest := c.getOperandValue(inst.Dest)
	src := c.getOperandValue(inst.Src)
	result := dest - src
	c.Flags.CF = src > dest
	c.Flags.OF = ((dest^src)&(dest^result)&0x8000) != 0
	c.UpdateFlags(result)
	return nil
}

// TEST instruction
func (c *CPU) execTEST(inst Instruction) error {
	if is32bit(inst) {
		result := c.getOperandValue32(inst.Dest) & c.getOperandValue32(inst.Src)
		c.Flags.CF, c.Flags.OF = false, false
		c.UpdateFlags32(result)
		return nil
	}
	result := c.getOperandValue(inst.Dest) & c.getOperandValue(inst.Src)
	c.Flags.CF, c.Flags.OF = false, false
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
func (c *CPU) execRET(_ Instruction) error {
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

	case 0x0E: // Teletype output
		// AL = character to write
		// BL = foreground color (in graphics modes)
		// BH = page number (ignored - we always use page 0)
		char := c.GetAL()
		color := c.GetBL()

		// Update text color
		c.textColor = color

		// Handle special characters
		switch char {
		case 0x0D: // Carriage return
			c.cursorX = 0
		case 0x0A: // Line feed
			c.cursorY++
			// Check if we need to scroll (cursor beyond bottom of screen)
			maxRows := uint8(200 / 16) // 12 rows for 16-pixel tall chars
			if c.cursorY >= maxRows {
				c.cursorY = maxRows - 1
				// TODO: Implement scrolling if needed
			}
		case 0x08: // Backspace
			if c.cursorX > 0 {
				c.cursorX--
			}
		case 0x07: // Bell
			// Ignore bell character (no audio support yet)
		default:
			// Draw character at cursor position
			pixelX := int(c.cursorX) * 8 * int(c.textScale)
			pixelY := int(c.cursorY) * 16 * int(c.textScale)

			// Draw character directly to VGA memory
			c.drawCharToVGA(char, pixelX, pixelY, c.textColor, int(c.textScale))

			// Advance cursor
			c.cursorX++
			maxCols := uint8(320 / (8 * int(c.textScale))) // 40 cols for 8-pixel wide chars at 1x scale
			if c.cursorX >= maxCols {
				c.cursorX = 0
				c.cursorY++
				maxRows := uint8(200 / (16 * int(c.textScale))) // 12 rows for 16-pixel tall chars at 1x scale
				if c.cursorY >= maxRows {
					c.cursorY = maxRows - 1
					// TODO: Implement scrolling if needed
				}
			}
		}
		return nil

	case 0x10: // Set palette register
		al := c.GetAL()
		switch al {
		case 0x00:
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
		case 0x10:
			// Set individual DAC register
			// BX = register number
			// DH = green, CH = blue, CL = red (each 0-63)
			if c.SetPaletteCallback != nil {
				index := byte(c.BX & 0xFF)
				r := byte((c.CX & 0x3F) * 4)        // CL * 4
				g := byte(((c.DX >> 8) & 0x3F) * 4) // DH * 4
				b := byte(((c.CX >> 8) & 0x3F) * 4) // CH * 4
				c.SetPaletteCallback(index, r, g, b)
			}
		}
		return nil

	case 0x11: // Character generator routines
		al := c.GetAL()
		switch al {
		case 0x30: // Get font information
			// Returns:
			// ES:BP = pointer to font data
			// CX = bytes per character (16 for 8x16 font)
			// DL = rows on screen - 1 (11 for 200/16 - 1)

			// Calculate segment:offset for BIOS font address
			// BIOS font is at 0xFA000 (F000:A000 in segment:offset)
			// Use F000:A000 representation (more traditional BIOS ROM segment)
			fontSeg := uint16(0xF000)  // Segment: F000
			fontOff := uint16(0xA000)  // Offset: A000

			c.ES = fontSeg
			c.BP = fontOff
			c.CX = 16  // 16 bytes per character (8x16 font)
			c.SetDL(11) // 12 rows - 1 (200/16 = 12.5, use 12)
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
	case OpTypeMem, OpTypeMemReg, OpTypeMemSegReg:
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
	case OpTypeMem, OpTypeMemReg, OpTypeMemSegReg:
		addr := CalculateLinearAddress(op.MemSegment, op.MemAddr)
		c.Memory.WriteWordLinear(addr, val)
	}
}

// Helper: Get 32-bit operand value
func (c *CPU) getOperandValue32(op Operand) uint32 {
	switch op.Type {
	case OpTypeReg32:
		if op.Reg32Get != nil {
			return op.Reg32Get()
		}
	case OpTypeReg16:
		if op.Reg16 != nil {
			return uint32(*op.Reg16)
		}
	case OpTypeReg8:
		if op.Reg8Get != nil {
			return uint32(op.Reg8Get())
		}
	case OpTypeImm16:
		return uint32(op.Imm16)
	case OpTypeImm8:
		return uint32(op.Imm8)
	case OpTypeMem, OpTypeMemReg, OpTypeMemSegReg:
		addr := CalculateLinearAddress(op.MemSegment, op.MemAddr)
		return c.Memory.ReadDWordLinear(addr)
	}
	return 0
}

// Helper: Set 32-bit operand value
func (c *CPU) setOperandValue32(op Operand, val uint32) {
	switch op.Type {
	case OpTypeReg32:
		if op.Reg32Set != nil {
			op.Reg32Set(val)
		}
	case OpTypeReg16:
		if op.Reg16 != nil {
			*op.Reg16 = uint16(val)
		}
	case OpTypeMem, OpTypeMemReg, OpTypeMemSegReg:
		addr := CalculateLinearAddress(op.MemSegment, op.MemAddr)
		c.Memory.WriteDWordLinear(addr, val)
	}
}

// is32bit returns true when either operand is a 32-bit register
func is32bit(inst Instruction) bool {
	return inst.Dest.Type == OpTypeReg32 || inst.Src.Type == OpTypeReg32
}

// memAddr returns the linear address for a memory operand
func memAddr(op Operand) uint32 {
	return CalculateLinearAddress(op.MemSegment, op.MemAddr)
}

// FPU memory helpers

func (c *CPU) execFILD(inst Instruction) error {
	addr := memAddr(inst.Dest)
	val := int16(c.Memory.ReadWordLinear(addr))
	c.FPU.Push(float64(val))
	return nil
}

func (c *CPU) execFIST(inst Instruction, pop bool) error {
	val := math.Round(c.FPU.ST0())
	if val > 32767 {
		val = 32767
	} else if val < -32768 {
		val = -32768
	}
	addr := memAddr(inst.Dest)
	c.Memory.WriteWordLinear(addr, uint16(int16(val)))
	if pop {
		c.FPU.Pop()
	}
	return nil
}

func (c *CPU) execFILD32(inst Instruction) error {
	addr := memAddr(inst.Dest)
	val := int32(c.Memory.ReadDWordLinear(addr))
	c.FPU.Push(float64(val))
	return nil
}

func (c *CPU) execFIST32(inst Instruction, pop bool) error {
	val := math.Round(c.FPU.ST0())
	if val > 2147483647 {
		val = 2147483647
	} else if val < -2147483648 {
		val = -2147483648
	}
	addr := memAddr(inst.Dest)
	c.Memory.WriteDWordLinear(addr, uint32(int32(val)))
	if pop {
		c.FPU.Pop()
	}
	return nil
}

func (c *CPU) execFLD32M(inst Instruction) error {
	addr := memAddr(inst.Dest)
	bits := c.Memory.ReadDWordLinear(addr)
	c.FPU.Push(float64(math.Float32frombits(bits)))
	return nil
}

func (c *CPU) execFST32M(inst Instruction, pop bool) error {
	bits := math.Float32bits(float32(c.FPU.ST0()))
	addr := memAddr(inst.Dest)
	c.Memory.WriteDWordLinear(addr, bits)
	if pop {
		c.FPU.Pop()
	}
	return nil
}

func (c *CPU) execFLD64M(inst Instruction) error {
	addr := memAddr(inst.Dest)
	bits := c.Memory.ReadQWordLinear(addr)
	c.FPU.Push(math.Float64frombits(bits))
	return nil
}

func (c *CPU) execFST64M(inst Instruction, pop bool) error {
	bits := math.Float64bits(c.FPU.ST0())
	addr := memAddr(inst.Dest)
	c.Memory.WriteQWordLinear(addr, bits)
	if pop {
		c.FPU.Pop()
	}
	return nil
}

func (c *CPU) execFIArith(inst Instruction, op func(float64, float64) float64) error {
	addr := memAddr(inst.Dest)
	val := float64(int16(c.Memory.ReadWordLinear(addr)))
	c.FPU.SetST0(op(c.FPU.ST0(), val))
	return nil
}

func (c *CPU) execFIArith32(inst Instruction, op func(float64, float64) float64) error {
	addr := memAddr(inst.Dest)
	val := float64(int32(c.Memory.ReadDWordLinear(addr)))
	c.FPU.SetST0(op(c.FPU.ST0(), val))
	return nil
}

func (c *CPU) execFArithM(inst Instruction, op func(float64, float64) float64) error {
	addr := memAddr(inst.Dest)
	bits := c.Memory.ReadDWordLinear(addr)
	val := float64(math.Float32frombits(bits))
	c.FPU.SetST0(op(c.FPU.ST0(), val))
	return nil
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
	switch inst.Src.Type {
	case OpTypeReg8:
		if inst.Src.Reg8Get != nil {
			value = inst.Src.Reg8Get()
		}
	case OpTypeImm8:
		value = inst.Src.Imm8
	default:
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
func (c *CPU) execMOVSB(_ Instruction) error {
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
func (c *CPU) execMOVSW(_ Instruction) error {
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
func (c *CPU) execSTOSB(_ Instruction) error {
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
func (c *CPU) execSTOSW(_ Instruction) error {
	// Get value from AX
	value := c.AX

	// Write word to ES:DI
	destAddr := CalculateLinearAddress(c.ES, c.DI)
	c.Memory.WriteWordLinear(destAddr, value)

	// Update DI by 2 (assume DF=0, increment)
	c.DI += 2

	return nil
}

// LODSB - Load byte from DS:SI into AL
func (c *CPU) execLODSB(_ Instruction) error {
	// Read byte from DS:SI
	srcAddr := CalculateLinearAddress(c.DS, c.SI)
	value := c.Memory.ReadByteLinear(srcAddr)

	// Store in AL
	c.SetAL(value)

	// Update SI (assume DF=0, increment)
	c.SI++

	return nil
}

// LODSW - Load word from DS:SI into AX
func (c *CPU) execLODSW(_ Instruction) error {
	// Read word from DS:SI
	srcAddr := CalculateLinearAddress(c.DS, c.SI)
	value := c.Memory.ReadWordLinear(srcAddr)

	// Store in AX
	c.AX = value

	// Update SI by 2 (assume DF=0, increment)
	c.SI += 2

	return nil
}
