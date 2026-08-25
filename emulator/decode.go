package emulator

// insn holds the decode state of the instruction currently executing.
type insn struct {
	start    uint32 // IP at start (for faults and REP restart)
	opsize   int    // 2 or 4
	addrsize int    // 2 or 4
	seg      int    // segment override or segNone
	rep      byte   // 0, 0xF2, 0xF3
	lock     bool
	op       byte

	modrm    byte
	mod, reg byte
	rm       byte
	isReg    bool   // r/m selects a register
	ea       uint32 // effective offset when !isReg
	easeg    int    // segment used for ea
	cycles   int
}

// segOr returns the override segment if present, else def.
func (c *CPU) segOr(def int) int {
	if c.in.seg != segNone {
		return c.in.seg
	}
	return def
}

// fetchModRM decodes a ModR/M byte (and SIB/displacement) into c.in.
func (c *CPU) fetchModRM() {
	in := &c.in
	b := c.fetch8()
	in.modrm = b
	in.mod = b >> 6
	in.reg = (b >> 3) & 7
	in.rm = b & 7
	if in.mod == 3 {
		in.isReg = true
		return
	}
	in.isReg = false
	in.cycles += 2
	if in.addrsize == 2 {
		c.ea16()
	} else {
		c.ea32()
	}
}

func (c *CPU) ea16() {
	in := &c.in
	var base uint32
	defSeg := SegDS
	switch in.rm {
	case 0:
		base = uint32(c.R16(RegBX) + c.R16(RegSI))
	case 1:
		base = uint32(c.R16(RegBX) + c.R16(RegDI))
	case 2:
		base = uint32(c.R16(RegBP) + c.R16(RegSI))
		defSeg = SegSS
	case 3:
		base = uint32(c.R16(RegBP) + c.R16(RegDI))
		defSeg = SegSS
	case 4:
		base = uint32(c.R16(RegSI))
	case 5:
		base = uint32(c.R16(RegDI))
	case 6:
		if in.mod == 0 {
			base = 0
		} else {
			base = uint32(c.R16(RegBP))
			defSeg = SegSS
		}
	case 7:
		base = uint32(c.R16(RegBX))
	}
	var disp uint32
	switch in.mod {
	case 0:
		if in.rm == 6 {
			disp = uint32(c.fetch16())
		}
	case 1:
		disp = uint32(int32(int8(c.fetch8())))
	case 2:
		disp = uint32(c.fetch16())
	}
	in.ea = (base + disp) & 0xFFFF
	in.easeg = c.segOr(defSeg)
}

func (c *CPU) ea32() {
	in := &c.in
	var base uint32
	defSeg := SegDS
	rm := in.rm
	if rm == 4 {
		sib := c.fetch8()
		scale := sib >> 6
		idx := (sib >> 3) & 7
		b := sib & 7
		if idx != 4 {
			base += c.Regs[idx] << scale
		}
		if b == 5 && in.mod == 0 {
			base += c.fetch32()
		} else {
			// 386 quirk: with no index register the scale is applied to
			// the base register (undocumented but deterministic).
			if idx == 4 {
				base += c.Regs[b] << scale
			} else {
				base += c.Regs[b]
			}
			if b == 4 || b == 5 {
				defSeg = SegSS
			}
		}
	} else if rm == 5 && in.mod == 0 {
		base = c.fetch32()
	} else {
		base = c.Regs[rm]
		if rm == 5 {
			defSeg = SegSS
		}
	}
	switch in.mod {
	case 1:
		base += uint32(int32(int8(c.fetch8())))
	case 2:
		base += c.fetch32()
	}
	in.ea = base
	in.easeg = c.segOr(defSeg)
}

// rdRM reads the r/m operand at width w.
func (c *CPU) rdRM(w int) uint32 {
	if c.in.isReg {
		return c.reg(w, int(c.in.rm))
	}
	return c.rdw(w, c.in.easeg, c.in.ea)
}

func (c *CPU) wrRM(w int, v uint32) {
	if c.in.isReg {
		c.setReg(w, int(c.in.rm), v)
		return
	}
	c.wrw(w, c.in.easeg, c.in.ea, v)
}

// rdReg / wrReg access the register named by the reg field.
func (c *CPU) rdReg(w int) uint32    { return c.reg(w, int(c.in.reg)) }
func (c *CPU) wrReg(w int, v uint32) { c.setReg(w, int(c.in.reg), v) }

// opw is the current operand width in bytes.
func (c *CPU) opw() int { return c.in.opsize }

// execute decodes prefixes and dispatches one instruction.
func (c *CPU) execute() (err error) {
	in := &c.in
	*in = insn{start: c.EIP, opsize: 2, addrsize: 2, seg: segNone}
	c.snapshot()
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(cpuException)
			if !ok {
				panic(r)
			}
			c.Regs = c.snapRegs
			c.Segs = c.snapSegs
			c.Flags = c.snapFlags
			if c.Model != Model8088 {
				c.EIP = in.start
			}
			// A fault while delivering the exception is a double fault; a
			// real CPU would shut down. Report it as an emulator fault.
			defer func() {
				if r2 := recover(); r2 != nil {
					if _, ok := r2.(cpuException); ok {
						err = c.fault("double fault delivering exception %d", e.vec)
						return
					}
					panic(r2)
				}
			}()
			c.Interrupt(e.vec)
			err = nil
		}
	}()

	for {
		b := c.fetch8()
		switch b {
		case 0x26:
			in.seg = SegES
		case 0x2E:
			in.seg = SegCS
		case 0x36:
			in.seg = SegSS
		case 0x3E:
			in.seg = SegDS
		case 0x64:
			if c.Model == Model8088 {
				in.op = b
				return c.exec1(b)
			}
			in.seg = SegFS
		case 0x65:
			if c.Model == Model8088 {
				in.op = b
				return c.exec1(b)
			}
			in.seg = SegGS
		case 0x66:
			if c.Model == Model8088 {
				in.op = b
				return c.exec1(b)
			}
			in.opsize = 4
		case 0x67:
			if c.Model == Model8088 {
				in.op = b
				return c.exec1(b)
			}
			in.addrsize = 4
		case 0xF0:
			in.lock = true
		case 0xF1:
			if c.Model == Model8088 {
				// F1 is an undocumented LOCK alias on the 8086/8088.
				in.lock = true
				continue
			}
			in.op = b
			return c.exec1(b)
		case 0xF2, 0xF3:
			in.rep = b
		default:
			in.op = b
			return c.exec1(b)
		}
	}
}

// Interrupt performs a software/hardware interrupt through the IVT.
// The return address is the current EIP.
func (c *CPU) Interrupt(vec uint8) {
	c.LastVector = int(vec)
	// The vector is fetched before the frame is pushed: with SS=0 the
	// pushes can overwrite the IVT entry itself.
	addr := uint32(vec) * 4
	ip := c.Mem.Read16(addr)
	cs := c.Mem.Read16(addr + 2)
	c.push16(uint16(c.Flags))
	c.push16(c.Segs[SegCS])
	c.push16(uint16(c.EIP))
	c.Flags &^= FlagIF | FlagTF
	c.EIP = uint32(ip)
	c.Segs[SegCS] = cs
	c.Halted = false
}

// exception raises a fault whose return address is the start of the
// faulting instruction (386 behaviour); 8088 pushes the next IP.
func (c *CPU) exception(vec uint8) {
	c.raise(vec)
}

// ud raises #UD. It never returns.
func (c *CPU) ud() error {
	c.raise(6)
	return nil
}

// jcc evaluates the condition code (low nibble of Jcc/SETcc/CMOVcc).
func (c *CPU) cond(cc byte) bool {
	f := c.Flags
	var r bool
	switch cc >> 1 {
	case 0:
		r = f&FlagOF != 0
	case 1:
		r = f&FlagCF != 0
	case 2:
		r = f&FlagZF != 0
	case 3:
		r = f&(FlagCF|FlagZF) != 0
	case 4:
		r = f&FlagSF != 0
	case 5:
		r = f&FlagPF != 0
	case 6:
		r = (f&FlagSF != 0) != (f&FlagOF != 0)
	case 7:
		r = f&FlagZF != 0 || (f&FlagSF != 0) != (f&FlagOF != 0)
	}
	if cc&1 != 0 {
		r = !r
	}
	return r
}

// count register at current address size.
func (c *CPU) cxGet() uint32 {
	if c.in.addrsize == 4 {
		return c.Regs[RegCX]
	}
	return uint32(c.R16(RegCX))
}

func (c *CPU) cxSet(v uint32) {
	if c.in.addrsize == 4 {
		c.Regs[RegCX] = v
	} else {
		c.SetR16(RegCX, uint16(v))
	}
}

// jumpRel performs a near relative jump, masking IP to 16 bits when the
// operand size is 16.
func (c *CPU) jumpRel(disp int32) {
	if c.in.opsize == 2 {
		c.EIP = uint32(uint16(c.EIP) + uint16(disp))
	} else {
		c.EIP = uint32(int32(c.EIP) + disp)
	}
}
