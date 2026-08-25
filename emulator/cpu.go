package emulator

import "fmt"

// Model selects the few places where 8088 and 386 semantics differ
// (shift-count masking, PUSH SP, divide-fault return address, ...).
type Model int

const (
	Model8088 Model = iota
	Model386
)

// General register indices (Intel encoding order).
const (
	RegAX = iota
	RegCX
	RegDX
	RegBX
	RegSP
	RegBP
	RegSI
	RegDI
)

// Segment register indices (Intel encoding order).
const (
	SegES = iota
	SegCS
	SegSS
	SegDS
	SegFS
	SegGS
	segNone = -1
)

// EFLAGS bits.
const (
	FlagCF = 1 << 0
	FlagPF = 1 << 2
	FlagAF = 1 << 4
	FlagZF = 1 << 6
	FlagSF = 1 << 7
	FlagTF = 1 << 8
	FlagIF = 1 << 9
	FlagDF = 1 << 10
	FlagOF = 1 << 11
	FlagNT = 1 << 14
	FlagRF = 1 << 16

	flagsArith = FlagCF | FlagPF | FlagAF | FlagZF | FlagSF | FlagOF
)

// Bus is the I/O port interface.
type Bus interface {
	In8(port uint16) uint8
	Out8(port uint16, v uint8)
	In16(port uint16) uint16
	Out16(port uint16, v uint16)
	In32(port uint16) uint32
	Out32(port uint16, v uint32)
}

// NullBus returns 0xFF for reads and swallows writes.
type NullBus struct{}

func (NullBus) In8(uint16) uint8     { return 0xFF }
func (NullBus) Out8(uint16, uint8)   {}
func (NullBus) In16(uint16) uint16   { return 0xFFFF }
func (NullBus) Out16(uint16, uint16) {}
func (NullBus) In32(uint16) uint32   { return 0xFFFFFFFF }
func (NullBus) Out32(uint16, uint32) {}

// Fault is returned by Step for conditions the emulator itself cannot
// handle (never for guest-visible exceptions, which are delivered via IVT).
type Fault struct {
	CS, IP uint32
	Msg    string
}

func (f *Fault) Error() string { return fmt.Sprintf("%04X:%04X: %s", f.CS, f.IP, f.Msg) }

// CPU is a 386-class processor running in real mode.
type CPU struct {
	Regs  [8]uint32
	Segs  [6]uint16
	EIP   uint32
	Flags uint32

	Mem   *Memory
	Bus   Bus
	FPU   FPU
	Model Model

	Halted    bool
	Cycles    uint64
	InsnCount uint64

	// LastVector is the interrupt/exception vector taken during the last
	// Step, or -1.
	LastVector int

	// intInhibit is set after MOV SS / POP SS / STI so the following
	// instruction executes before any interrupt is delivered.
	intInhibit bool

	// PendingIRQ peeks at the interrupt controller: vector or -1.
	PendingIRQ func() int
	// AckIRQ tells the controller that the vector is being serviced.
	AckIRQ func(vec int)
	// BIOSCall is invoked for the reserved F1 <svc> trap when CS==0xF000.
	BIOSCall func(c *CPU, svc byte)
	// OnHalt is invoked when HLT executes (used to fast-forward clocks).
	OnHalt func(c *CPU)
	// Clock is advanced after every instruction with the cycle cost.
	Clock func(cycles uint64)

	in insn

	// register snapshot taken at instruction start, restored on faults
	snapRegs  [8]uint32
	snapSegs  [6]uint16
	snapFlags uint32

	stopped bool
}

// cpuException is panicked by raise() and recovered in execute().
type cpuException struct{ vec uint8 }

// raise aborts the current instruction with a fault. Registers are rolled
// back to the instruction start and the exception is delivered via the IVT.
func (c *CPU) raise(vec uint8) {
	panic(cpuException{vec})
}

func (c *CPU) snapshot() {
	c.snapRegs = c.Regs
	c.snapSegs = c.Segs
	c.snapFlags = c.Flags
}

// limitCheck raises #GP (or #SS for the stack segment) when an access of
// n bytes at off crosses the 64K real-mode segment limit.
func (c *CPU) limitCheck(seg int, off uint32, n uint32) {
	if off > 0xFFFF || off+n-1 > 0xFFFF {
		if seg == SegSS {
			c.raise(12)
		}
		c.raise(13)
	}
}

func NewCPU(model Model) *CPU {
	c := &CPU{Mem: NewMemory(), Bus: NullBus{}, Model: model}
	c.Mem.A20 = model != Model8088
	c.Reset()
	return c
}

func (c *CPU) Reset() {
	c.Regs = [8]uint32{}
	c.Segs = [6]uint16{}
	c.Segs[SegCS] = 0xF000
	c.EIP = 0xFFF0
	c.Flags = 0x0002
	c.Halted = false
	c.FPU.Init()
}

// ---- register accessors -------------------------------------------------

func (c *CPU) R8(i int) uint8 {
	if i < 4 {
		return uint8(c.Regs[i])
	}
	return uint8(c.Regs[i-4] >> 8)
}

func (c *CPU) SetR8(i int, v uint8) {
	if i < 4 {
		c.Regs[i] = c.Regs[i]&^0xFF | uint32(v)
	} else {
		c.Regs[i-4] = c.Regs[i-4]&^0xFF00 | uint32(v)<<8
	}
}

func (c *CPU) R16(i int) uint16       { return uint16(c.Regs[i]) }
func (c *CPU) SetR16(i int, v uint16) { c.Regs[i] = c.Regs[i]&^0xFFFF | uint32(v) }
func (c *CPU) R32(i int) uint32       { return c.Regs[i] }
func (c *CPU) SetR32(i int, v uint32) { c.Regs[i] = v }

// reg reads a general register at the given width (1, 2 or 4 bytes).
func (c *CPU) reg(w int, i int) uint32 {
	switch w {
	case 1:
		return uint32(c.R8(i))
	case 2:
		return uint32(c.R16(i))
	default:
		return c.Regs[i]
	}
}

func (c *CPU) setReg(w int, i int, v uint32) {
	switch w {
	case 1:
		c.SetR8(i, uint8(v))
	case 2:
		c.SetR16(i, uint16(v))
	default:
		c.Regs[i] = v
	}
}

func (c *CPU) AX() uint16 { return uint16(c.Regs[RegAX]) }
func (c *CPU) BX() uint16 { return uint16(c.Regs[RegBX]) }
func (c *CPU) CX() uint16 { return uint16(c.Regs[RegCX]) }
func (c *CPU) DX() uint16 { return uint16(c.Regs[RegDX]) }
func (c *CPU) SP() uint16 { return uint16(c.Regs[RegSP]) }
func (c *CPU) BP() uint16 { return uint16(c.Regs[RegBP]) }
func (c *CPU) SI() uint16 { return uint16(c.Regs[RegSI]) }
func (c *CPU) DI() uint16 { return uint16(c.Regs[RegDI]) }
func (c *CPU) AL() uint8  { return uint8(c.Regs[RegAX]) }
func (c *CPU) AH() uint8  { return uint8(c.Regs[RegAX] >> 8) }
func (c *CPU) BL() uint8  { return uint8(c.Regs[RegBX]) }
func (c *CPU) BH() uint8  { return uint8(c.Regs[RegBX] >> 8) }
func (c *CPU) CL() uint8  { return uint8(c.Regs[RegCX]) }
func (c *CPU) CH() uint8  { return uint8(c.Regs[RegCX] >> 8) }
func (c *CPU) DL() uint8  { return uint8(c.Regs[RegDX]) }
func (c *CPU) DH() uint8  { return uint8(c.Regs[RegDX] >> 8) }
func (c *CPU) CS() uint16 { return c.Segs[SegCS] }
func (c *CPU) DS() uint16 { return c.Segs[SegDS] }
func (c *CPU) ES() uint16 { return c.Segs[SegES] }
func (c *CPU) SS() uint16 { return c.Segs[SegSS] }
func (c *CPU) IP() uint16 { return uint16(c.EIP) }

func (c *CPU) SetAX(v uint16) { c.SetR16(RegAX, v) }
func (c *CPU) SetBX(v uint16) { c.SetR16(RegBX, v) }
func (c *CPU) SetCX(v uint16) { c.SetR16(RegCX, v) }
func (c *CPU) SetDX(v uint16) { c.SetR16(RegDX, v) }
func (c *CPU) SetSI(v uint16) { c.SetR16(RegSI, v) }
func (c *CPU) SetDI(v uint16) { c.SetR16(RegDI, v) }
func (c *CPU) SetBP(v uint16) { c.SetR16(RegBP, v) }
func (c *CPU) SetSP(v uint16) { c.SetR16(RegSP, v) }
func (c *CPU) SetAL(v uint8)  { c.SetR8(0, v) }
func (c *CPU) SetAH(v uint8)  { c.SetR8(4, v) }
func (c *CPU) SetBL(v uint8)  { c.SetR8(3, v) }
func (c *CPU) SetBH(v uint8)  { c.SetR8(7, v) }
func (c *CPU) SetCL(v uint8)  { c.SetR8(1, v) }
func (c *CPU) SetCH(v uint8)  { c.SetR8(5, v) }
func (c *CPU) SetDL(v uint8)  { c.SetR8(2, v) }
func (c *CPU) SetDH(v uint8)  { c.SetR8(6, v) }
func (c *CPU) SetIP(v uint16) { c.EIP = uint32(v) }

func (c *CPU) GetFlag(f uint32) bool { return c.Flags&f != 0 }
func (c *CPU) SetFlag(f uint32, on bool) {
	if on {
		c.Flags |= f
	} else {
		c.Flags &^= f
	}
}

// ---- memory access through segments ------------------------------------

func (c *CPU) lin(seg int, off uint32) uint32 {
	return uint32(c.Segs[seg])<<4 + off
}

func (c *CPU) rd8(seg int, off uint32) uint8 {
	if off > 0xFFFF {
		c.limitCheck(seg, off, 1)
	}
	return c.Mem.Read8(c.lin(seg, off))
}

// Multi-byte accesses: the 8088 wraps offsets within the 64K segment;
// the 386 raises #GP/#SS when an operand crosses the segment limit.
func (c *CPU) rd16(seg int, off uint32) uint16 {
	if off > 0xFFFE {
		if c.Model == Model8088 {
			return uint16(c.rd8(seg, off&0xFFFF)) | uint16(c.rd8(seg, (off+1)&0xFFFF))<<8
		}
		c.limitCheck(seg, off, 2)
	}
	return c.Mem.Read16(c.lin(seg, off))
}

func (c *CPU) rd32(seg int, off uint32) uint32 {
	if off > 0xFFFC {
		if c.Model == Model8088 {
			return uint32(c.rd16(seg, off&0xFFFF)) | uint32(c.rd16(seg, (off+2)&0xFFFF))<<16
		}
		c.limitCheck(seg, off, 4)
	}
	return c.Mem.Read32(c.lin(seg, off))
}

func (c *CPU) wr8(seg int, off uint32, v uint8) {
	if off > 0xFFFF {
		c.limitCheck(seg, off, 1)
	}
	c.Mem.Write8(c.lin(seg, off), v)
}

func (c *CPU) wr16(seg int, off uint32, v uint16) {
	if off > 0xFFFE {
		if c.Model == Model8088 {
			c.wr8(seg, off&0xFFFF, uint8(v))
			c.wr8(seg, (off+1)&0xFFFF, uint8(v>>8))
			return
		}
		c.limitCheck(seg, off, 2)
	}
	c.Mem.Write16(c.lin(seg, off), v)
}

func (c *CPU) wr32(seg int, off uint32, v uint32) {
	if off > 0xFFFC {
		if c.Model == Model8088 {
			c.wr16(seg, off&0xFFFF, uint16(v))
			c.wr16(seg, (off+2)&0xFFFF, uint16(v>>16))
			return
		}
		c.limitCheck(seg, off, 4)
	}
	c.Mem.Write32(c.lin(seg, off), v)
}

func (c *CPU) rdw(w int, seg int, off uint32) uint32 {
	switch w {
	case 1:
		return uint32(c.rd8(seg, off))
	case 2:
		return uint32(c.rd16(seg, off))
	default:
		return c.rd32(seg, off)
	}
}

func (c *CPU) wrw(w int, seg int, off uint32, v uint32) {
	switch w {
	case 1:
		c.wr8(seg, off, uint8(v))
	case 2:
		c.wr16(seg, off, uint16(v))
	default:
		c.wr32(seg, off, v)
	}
}

// ---- stack --------------------------------------------------------------

// stackMask: in real mode with a 16-bit stack, SP wraps at 64K.
func (c *CPU) push16(v uint16) {
	sp := uint16(c.Regs[RegSP]) - 2
	c.wr16(SegSS, uint32(sp), v)
	c.SetR16(RegSP, sp)
}

func (c *CPU) pop16() uint16 {
	sp := uint16(c.Regs[RegSP])
	v := c.rd16(SegSS, uint32(sp))
	c.SetR16(RegSP, sp+2)
	return v
}

func (c *CPU) push32(v uint32) {
	sp := uint16(c.Regs[RegSP]) - 4
	c.wr32(SegSS, uint32(sp), v)
	c.SetR16(RegSP, sp)
}

func (c *CPU) pop32() uint32 {
	sp := uint16(c.Regs[RegSP])
	v := c.rd32(SegSS, uint32(sp))
	c.SetR16(RegSP, sp+4)
	return v
}

// push pushes at the current operand size.
func (c *CPU) push(w int, v uint32) {
	if w == 4 {
		c.push32(v)
	} else {
		c.push16(uint16(v))
	}
}

func (c *CPU) pop(w int) uint32 {
	if w == 4 {
		return c.pop32()
	}
	return uint32(c.pop16())
}

// Public helpers for the BIOS/DOS layers.
func (c *CPU) Push16(v uint16) { c.push16(v) }
func (c *CPU) Pop16() uint16   { return c.pop16() }

// ---- instruction fetch --------------------------------------------------

func (c *CPU) fetch8() uint8 {
	if c.EIP > 0xFFFF {
		if c.Model == Model8088 {
			c.EIP &= 0xFFFF
		} else {
			c.raise(13)
		}
	}
	v := c.Mem.Read8(uint32(c.Segs[SegCS])<<4 + c.EIP)
	c.EIP++
	return v
}

func (c *CPU) fetch16() uint16 {
	lo := uint16(c.fetch8())
	return lo | uint16(c.fetch8())<<8
}

func (c *CPU) fetch32() uint32 {
	lo := uint32(c.fetch16())
	return lo | uint32(c.fetch16())<<16
}

func (c *CPU) fetchImm(w int) uint32 {
	switch w {
	case 1:
		return uint32(c.fetch8())
	case 2:
		return uint32(c.fetch16())
	default:
		return c.fetch32()
	}
}

// ---- execution ----------------------------------------------------------

// Stop requests the run loop to exit.
func (c *CPU) Stop()         { c.stopped = true }
func (c *CPU) Stopped() bool { return c.stopped }
func (c *CPU) ClearStop()    { c.stopped = false }

// Step executes one instruction (a REP string instruction counts as one,
// unless interrupted by a pending IRQ) and then delivers a pending
// hardware interrupt if IF permits.
func (c *CPU) Step() error {
	if c.Halted {
		// Halted: only an interrupt can resume us.
		if c.Flags&FlagIF != 0 && c.PendingIRQ != nil {
			if v := c.PendingIRQ(); v >= 0 {
				c.Halted = false
				if c.AckIRQ != nil {
					c.AckIRQ(v)
				}
				c.Interrupt(uint8(v))
				return nil
			}
		}
		if c.Clock != nil {
			c.Clock(4)
		}
		c.Cycles += 4
		return nil
	}

	inhibit := c.intInhibit
	c.intInhibit = false

	if c.Flags&FlagTF != 0 && !inhibit {
		// Single-step trap after the previous instruction.
		c.Interrupt(1)
	}

	c.LastVector = -1
	start := c.EIP
	if err := c.execute(); err != nil {
		return err
	}
	c.InsnCount++
	cyc := uint64(c.in.cycles)
	if cyc == 0 {
		cyc = 2
	}
	c.Cycles += cyc
	if c.Clock != nil {
		c.Clock(cyc)
	}
	_ = start

	if !inhibit && !c.intInhibit && c.Flags&FlagIF != 0 && c.PendingIRQ != nil && !c.Halted {
		if v := c.PendingIRQ(); v >= 0 {
			if c.AckIRQ != nil {
				c.AckIRQ(v)
			}
			c.Interrupt(uint8(v))
		}
	}
	return nil
}

// Run executes until halted, stopped, or an emulator fault occurs.
func (c *CPU) Run() error {
	for !c.stopped {
		if err := c.Step(); err != nil {
			return err
		}
	}
	return nil
}

// RunCycles executes until at least n more cycles have elapsed.
func (c *CPU) RunCycles(n uint64) error {
	target := c.Cycles + n
	for c.Cycles < target && !c.stopped {
		if err := c.Step(); err != nil {
			return err
		}
	}
	return nil
}

func (c *CPU) fault(format string, args ...any) error {
	return &Fault{CS: uint32(c.Segs[SegCS]), IP: c.in.start, Msg: fmt.Sprintf(format, args...)}
}
