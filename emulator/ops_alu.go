package emulator

// alu performs one of the eight basic ALU operations (ADD OR ADC SBB AND
// SUB XOR CMP), returning the result and whether it should be written back.
func (c *CPU) alu(kind int, w int, a, b uint32) (uint32, bool) {
	switch kind {
	case 0:
		return c.flagsAdd(w, a, b, 0), true
	case 1:
		return c.flagsLogic(w, a|b), true
	case 2:
		return c.flagsAdd(w, a, b, c.Flags&FlagCF), true
	case 3:
		return c.flagsSub(w, a, b, c.Flags&FlagCF), true
	case 4:
		return c.flagsLogic(w, a&b), true
	case 5:
		return c.flagsSub(w, a, b, 0), true
	case 6:
		return c.flagsLogic(w, a^b), true
	default:
		c.flagsSub(w, a, b, 0)
		return 0, false
	}
}

// group3 handles F6/F7: TEST NOT NEG MUL IMUL DIV IDIV.
func (c *CPU) group3(op byte) error {
	in := &c.in
	w := in.opsize
	if op == 0xF6 {
		w = 1
	}
	c.fetchModRM()
	switch in.reg {
	case 0, 1: // TEST (reg=1 is an undocumented alias)
		imm := c.fetchImm(w)
		c.flagsLogic(w, c.rdRM(w)&imm)
	case 2:
		c.wrRM(w, ^c.rdRM(w)&widthMask(w))
	case 3:
		c.wrRM(w, c.flagsSub(w, 0, c.rdRM(w), 0))
	case 4:
		c.mul(w, c.rdRM(w))
	case 5:
		c.imul1(w, c.rdRM(w))
	case 6:
		return c.div(w, c.rdRM(w))
	case 7:
		return c.idiv(w, c.rdRM(w))
	}
	return nil
}

func (c *CPU) mul(w int, b uint32) {
	c.in.cycles += 10
	var hi, lo uint32
	switch w {
	case 1:
		r := uint32(c.AL()) * b
		c.SetAX(uint16(r))
		hi, lo = r>>8, r&0xFF
	case 2:
		r := uint32(c.AX()) * b
		c.SetAX(uint16(r))
		c.SetDX(uint16(r >> 16))
		hi, lo = r>>16, r&0xFFFF
	default:
		r := uint64(c.Regs[RegAX]) * uint64(b)
		c.Regs[RegAX] = uint32(r)
		c.Regs[RegDX] = uint32(r >> 32)
		hi, lo = uint32(r>>32), uint32(r)
	}
	c.setSZP(w, lo)
	c.Flags &^= FlagCF | FlagOF | FlagAF
	if hi != 0 {
		c.Flags |= FlagCF | FlagOF
	}
	if c.Model == Model8088 {
		// 8086/8088: ZF is cleared? Undefined; the hardware clears it
		// unless result is zero. Masked by test metadata anyway.
	}
}

func (c *CPU) imul1(w int, b uint32) {
	c.in.cycles += 10
	var ovf bool
	var lo uint32
	switch w {
	case 1:
		r := int32(int8(c.AL())) * int32(int8(b))
		c.SetAX(uint16(r))
		ovf = r != int32(int8(r))
		lo = uint32(r) & 0xFF
	case 2:
		r := int32(int16(c.AX())) * int32(int16(b))
		c.SetAX(uint16(r))
		c.SetDX(uint16(r >> 16))
		ovf = r != int32(int16(r))
		lo = uint32(r) & 0xFFFF
	default:
		r := int64(int32(c.Regs[RegAX])) * int64(int32(b))
		c.Regs[RegAX] = uint32(r)
		c.Regs[RegDX] = uint32(r >> 32)
		ovf = r != int64(int32(r))
		lo = uint32(r)
	}
	c.setSZP(w, lo)
	c.Flags &^= FlagCF | FlagOF | FlagAF
	if ovf {
		c.Flags |= FlagCF | FlagOF
	}
}

// imul2 computes the truncated signed product a*b for IMUL r, r/m[, imm].
func (c *CPU) imul2(w int, a, b uint32) uint32 {
	c.in.cycles += 10
	var r uint32
	var ovf bool
	switch w {
	case 2:
		p := int32(int16(a)) * int32(int16(b))
		r = uint32(uint16(p))
		ovf = p != int32(int16(p))
	default:
		p := int64(int32(a)) * int64(int32(b))
		r = uint32(p)
		ovf = p != int64(int32(p))
	}
	c.setSZP(w, r)
	c.Flags &^= FlagCF | FlagOF | FlagAF
	if ovf {
		c.Flags |= FlagCF | FlagOF
	}
	return r
}

func (c *CPU) div(w int, b uint32) error {
	c.in.cycles += 20
	if b == 0 {
		c.exception(0)
		return nil
	}
	switch w {
	case 1:
		n := uint32(c.AX())
		q := n / b
		if q > 0xFF {
			c.exception(0)
			return nil
		}
		c.SetAL(uint8(q))
		c.SetAH(uint8(n % b))
	case 2:
		n := uint32(c.DX())<<16 | uint32(c.AX())
		q := n / b
		if q > 0xFFFF {
			c.exception(0)
			return nil
		}
		c.SetAX(uint16(q))
		c.SetDX(uint16(n % b))
	default:
		n := uint64(c.Regs[RegDX])<<32 | uint64(c.Regs[RegAX])
		q := n / uint64(b)
		if q > 0xFFFFFFFF {
			c.exception(0)
			return nil
		}
		c.Regs[RegAX] = uint32(q)
		c.Regs[RegDX] = uint32(n % uint64(b))
	}
	return nil
}

func (c *CPU) idiv(w int, b uint32) error {
	c.in.cycles += 20
	// 8086/8088 quirk: a REP prefix on IDIV negates the quotient.
	neg := c.Model == Model8088 && c.in.rep != 0
	switch w {
	case 1:
		d := int32(int8(b))
		if d == 0 {
			c.exception(0)
			return nil
		}
		n := int32(int16(c.AX()))
		q := n / d
		if q > 127 || q < -128 || (c.Model == Model8088 && q == -128) {
			c.exception(0)
			return nil
		}
		if neg {
			q = -q
		}
		c.SetAL(uint8(q))
		c.SetAH(uint8(n % d))
	case 2:
		d := int64(int16(b))
		if d == 0 {
			c.exception(0)
			return nil
		}
		n := int64(int32(uint32(c.DX())<<16 | uint32(c.AX())))
		q := n / d
		if q > 32767 || q < -32768 || (c.Model == Model8088 && q == -32768) {
			c.exception(0)
			return nil
		}
		if neg {
			q = -q
		}
		c.SetAX(uint16(q))
		c.SetDX(uint16(n % d))
	default:
		d := int64(int32(b))
		if d == 0 {
			c.exception(0)
			return nil
		}
		n := int64(uint64(c.Regs[RegDX])<<32 | uint64(c.Regs[RegAX]))
		if n == -1<<63 && d == -1 {
			c.exception(0)
			return nil
		}
		q := n / d
		if q > 0x7FFFFFFF || q < -0x80000000 {
			c.exception(0)
			return nil
		}
		c.Regs[RegAX] = uint32(q)
		c.Regs[RegDX] = uint32(n % d)
	}
	return nil
}

// ---- BCD adjust -----------------------------------------------------------
//
// The 8086 differs from the 386 in several details (verified against
// hardware traces): its high-nibble test is (AL >> 4) > 9 rather than
// AL > 99h, DAS does not set CF from the low-nibble borrow, and AAA/AAS
// adjust AL and AH as separate bytes.

func (c *CPU) daa() {
	al := uint32(c.AL())
	old := al
	cf := c.Flags&FlagCF != 0
	oldAF := c.Flags&FlagAF != 0
	c.Flags &^= FlagCF
	if al&0x0F > 9 || c.Flags&FlagAF != 0 {
		al += 6
		if cf || al > 0xFF {
			c.Flags |= FlagCF
		}
		c.Flags |= FlagAF
	} else {
		c.Flags &^= FlagAF
	}
	if c.bcdHigh(old, cf, oldAF) {
		al += 0x60
		c.Flags |= FlagCF
	}
	c.SetAL(uint8(al))
	c.setSZP(1, al)
}

// bcdHigh decides whether DAA/DAS must correct the high digit. The 8086
// gate logic (per silicon reverse engineering) is
// CF | bit7·(bit6 | bit5 | bit4·(¬AF ∧ low>9)); later CPUs use AL > 99h.
func (c *CPU) bcdHigh(old uint32, cf, af bool) bool {
	if c.Model != Model8088 {
		return old > 0x99 || cf
	}
	low9 := old&0x0F > 9
	return cf || old >= 0xA0 || (old >= 0x90 && low9 && !af)
}

func (c *CPU) das() {
	al := uint32(c.AL())
	old := al
	cf := c.Flags&FlagCF != 0
	oldAF := c.Flags&FlagAF != 0
	c.Flags &^= FlagCF
	if al&0x0F > 9 || c.Flags&FlagAF != 0 {
		if cf || (al < 6 && c.Model != Model8088) {
			c.Flags |= FlagCF
		}
		al -= 6
		c.Flags |= FlagAF
	} else {
		c.Flags &^= FlagAF
	}
	if c.bcdHigh(old, cf, oldAF) {
		al -= 0x60
		c.Flags |= FlagCF
	}
	c.SetAL(uint8(al))
	c.setSZP(1, al)
}

func (c *CPU) aaa() {
	if c.AL()&0x0F > 9 || c.Flags&FlagAF != 0 {
		if c.Model == Model8088 {
			c.SetAL(c.AL() + 6)
			c.SetAH(c.AH() + 1)
		} else {
			c.SetAX(c.AX() + 0x106)
		}
		c.Flags |= FlagAF | FlagCF
	} else {
		c.Flags &^= FlagAF | FlagCF
	}
	c.SetAL(c.AL() & 0x0F)
	// SF/ZF/PF undefined; real hardware computes them from AL.
	c.setSZP(1, uint32(c.AL()))
}

func (c *CPU) aas() {
	if c.AL()&0x0F > 9 || c.Flags&FlagAF != 0 {
		if c.Model == Model8088 {
			c.SetAL(c.AL() - 6)
			c.SetAH(c.AH() - 1)
		} else {
			c.SetAX(c.AX() - 6)
			c.SetAH(c.AH() - 1)
		}
		c.Flags |= FlagAF | FlagCF
	} else {
		c.Flags &^= FlagAF | FlagCF
	}
	c.SetAL(c.AL() & 0x0F)
	c.setSZP(1, uint32(c.AL()))
}

func (c *CPU) aam(base uint8) error {
	if base == 0 {
		// Hardware traces show the flags being updated before the divide
		// fault is taken: 386 leaves PF set, ZF/SF clear; 8088 sets ZF/PF.
		if c.Model != Model8088 {
			c.Flags = c.Flags&^(FlagZF|FlagSF|FlagPF) | parityTable[c.AH()]
		} else {
			c.Flags = c.Flags&^(FlagZF|FlagSF|FlagPF) | FlagPF | FlagZF
		}
		c.snapshot()
		c.exception(0)
		return nil
	}
	al := c.AL()
	c.SetAH(al / base)
	c.SetAL(al % base)
	c.setSZP(1, uint32(c.AL()))
	c.Flags &^= FlagCF | FlagOF | FlagAF
	return nil
}

func (c *CPU) aad(base uint8) {
	al := uint8(uint32(c.AH())*uint32(base) + uint32(c.AL()))
	c.SetAX(uint16(al))
	c.setSZP(1, uint32(al))
	c.Flags &^= FlagCF | FlagOF | FlagAF
}
