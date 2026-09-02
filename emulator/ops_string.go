package emulator

// stringOp implements MOVS/CMPS/STOS/LODS/SCAS/INS/OUTS with REP prefixes.
func (c *CPU) stringOp(op byte) error {
	in := &c.in
	w := in.opsize
	if op&1 == 0 {
		w = 1
	}
	rep := in.rep != 0
	// On the 8088 a REP prefix on INS/OUTS doesn't exist (they are Jcc
	// aliases and never reach here).
	if rep && c.cxGet() == 0 {
		return nil
	}
	srcSeg := c.segOr(SegDS)
	step := uint32(w)
	if c.Flags&FlagDF != 0 {
		step = -step
	}
	a32 := in.addrsize == 4

	getSI := func() uint32 {
		if a32 {
			return c.Regs[RegSI]
		}
		return uint32(c.R16(RegSI))
	}
	getDI := func() uint32 {
		if a32 {
			return c.Regs[RegDI]
		}
		return uint32(c.R16(RegDI))
	}
	addSI := func() {
		if a32 {
			c.Regs[RegSI] += step
		} else {
			c.SetR16(RegSI, c.R16(RegSI)+uint16(step))
		}
	}
	addDI := func() {
		if a32 {
			c.Regs[RegDI] += step
		} else {
			c.SetR16(RegDI, c.R16(RegDI)+uint16(step))
		}
	}

	for {
		c.snapshot()
		switch op {
		case 0xA4, 0xA5: // MOVS
			c.wrw(w, SegES, getDI(), c.rdw(w, srcSeg, getSI()))
			addSI()
			addDI()
		case 0xA6, 0xA7: // CMPS
			a := c.rdw(w, srcSeg, getSI())
			b := c.rdw(w, SegES, getDI())
			c.flagsSub(w, a, b, 0)
			addSI()
			addDI()
		case 0xAA, 0xAB: // STOS
			c.wrw(w, SegES, getDI(), c.reg(w, RegAX))
			addDI()
		case 0xAC, 0xAD: // LODS
			c.setReg(w, RegAX, c.rdw(w, srcSeg, getSI()))
			addSI()
		case 0xAE, 0xAF: // SCAS
			b := c.rdw(w, SegES, getDI())
			c.flagsSub(w, c.reg(w, RegAX), b, 0)
			addDI()
		case 0x6C, 0x6D: // INS
			in.cycles += c.IOCycles
			var v uint32
			switch w {
			case 1:
				v = uint32(c.Bus.In8(c.DX()))
			case 2:
				v = uint32(c.Bus.In16(c.DX()))
			default:
				v = c.Bus.In32(c.DX())
			}
			c.wrw(w, SegES, getDI(), v)
			addDI()
		case 0x6E, 0x6F: // OUTS
			in.cycles += c.IOCycles
			v := c.rdw(w, srcSeg, getSI())
			switch w {
			case 1:
				c.Bus.Out8(c.DX(), uint8(v))
			case 2:
				c.Bus.Out16(c.DX(), uint16(v))
			default:
				c.Bus.Out32(c.DX(), v)
			}
			addSI()
		}
		in.cycles += 4
		if !rep {
			return nil
		}
		cx := c.cxGet() - 1
		c.cxSet(cx)
		if cx == 0 {
			return nil
		}
		// REPE/REPNE termination for CMPS/SCAS.
		if op == 0xA6 || op == 0xA7 || op == 0xAE || op == 0xAF {
			zf := c.Flags&FlagZF != 0
			if in.rep == 0xF3 && !zf {
				return nil
			}
			if in.rep == 0xF2 && zf {
				return nil
			}
		}
		// Allow interrupts between iterations: restart the instruction.
		if c.Flags&FlagIF != 0 && c.PendingIRQ != nil && c.PendingIRQ() >= 0 {
			c.EIP = in.start
			return nil
		}
	}
}
