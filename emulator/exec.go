package emulator

// exec1 executes a one-byte opcode (prefixes already consumed).
func (c *CPU) exec1(op byte) error {
	in := &c.in
	w := in.opsize

	// 8088 aliases for opcodes that later CPUs assigned new meanings.
	if c.Model == Model8088 {
		switch {
		case op == 0x0F:
			c.Segs[SegCS] = c.pop16()
			return nil
		case op >= 0x60 && op <= 0x6F:
			op += 0x10 // Jcc aliases
		case op == 0xC0 || op == 0xC1:
			op += 2 // RET aliases
		case op == 0xC8 || op == 0xC9:
			op += 2 // RETF aliases
		}
	}

	if in.lock && c.Model != Model8088 && op != 0x0F {
		c.lockCheck(op, false)
	}

	switch {
	// ---- ALU: 00-3F ----------------------------------------------------
	case op < 0x40:
		kind := int(op>>3) & 7
		switch op & 7 {
		case 0, 1, 2, 3:
			ww := w
			if op&1 == 0 {
				ww = 1
			}
			c.fetchModRM()
			if op&2 == 0 { // rm, reg
				r, wr := c.alu(kind, ww, c.rdRM(ww), c.rdReg(ww))
				if wr {
					c.wrRM(ww, r)
				}
			} else { // reg, rm
				r, wr := c.alu(kind, ww, c.rdReg(ww), c.rdRM(ww))
				if wr {
					c.wrReg(ww, r)
				}
			}
		case 4:
			r, wr := c.alu(kind, 1, uint32(c.AL()), uint32(c.fetch8()))
			if wr {
				c.SetAL(uint8(r))
			}
		case 5:
			r, wr := c.alu(kind, w, c.reg(w, RegAX), c.fetchImm(w))
			if wr {
				c.setReg(w, RegAX, r)
			}
		case 6: // PUSH seg / DAA / AAA (0x26/0x2E/0x36/0x3E are prefixes)
			switch op {
			case 0x06, 0x0E, 0x16, 0x1E:
				c.push(w, uint32(c.Segs[op>>3]))
			}
		case 7: // POP seg / DAA / DAS / AAA / AAS
			switch op {
			case 0x07, 0x17, 0x1F:
				c.Segs[op>>3] = c.popSeg(w)
				if op == 0x17 {
					c.intInhibit = true
				}
			case 0x0F:
				return c.exec0F()
			case 0x27:
				c.daa()
			case 0x2F:
				c.das()
			case 0x37:
				c.aaa()
			case 0x3F:
				c.aas()
			}
		}

	// ---- INC/DEC/PUSH/POP r: 40-5F ---------------------------------------
	case op < 0x48:
		r := int(op & 7)
		c.setReg(w, r, c.flagsInc(w, c.reg(w, r)))
	case op < 0x50:
		r := int(op & 7)
		c.setReg(w, r, c.flagsDec(w, c.reg(w, r)))
	case op < 0x58:
		r := int(op & 7)
		v := c.reg(w, r)
		if r == RegSP && c.Model == Model8088 {
			v -= 2
		}
		c.push(w, v)
	case op < 0x60:
		r := int(op & 7)
		c.setReg(w, r, c.pop(w))

	// ---- 60-6F (186+) ----------------------------------------------------
	case op == 0x60:
		c.pusha()
	case op == 0x61:
		c.popa()
	case op == 0x62:
		return c.bound()
	case op == 0x63: // ARPL: invalid in real mode
		return c.ud()
	case op == 0x68:
		c.push(w, c.fetchImm(w))
	case op == 0x69, op == 0x6B:
		c.fetchModRM()
		a := c.rdRM(w)
		var b uint32
		if op == 0x6B {
			b = uint32(int32(int8(c.fetch8())))
		} else {
			b = c.fetchImm(w)
		}
		c.wrReg(w, c.imul2(w, a, b))
	case op == 0x6A:
		c.push(w, uint32(int32(int8(c.fetch8()))))
	case op >= 0x6C && op <= 0x6F:
		return c.stringOp(op)

	// ---- Jcc rel8: 70-7F --------------------------------------------------
	case op < 0x80:
		d := int32(int8(c.fetch8()))
		if c.cond(op & 0xF) {
			c.jumpRel(d)
			in.cycles += 4
		}

	// ---- Group 1: 80-83 --------------------------------------------------
	case op <= 0x83:
		ww := w
		if op&1 == 0 {
			ww = 1
		}
		c.fetchModRM()
		var imm uint32
		switch op {
		case 0x80, 0x82:
			imm = uint32(c.fetch8())
		case 0x81:
			imm = c.fetchImm(w)
		case 0x83:
			imm = uint32(int32(int8(c.fetch8())))
		}
		r, wr := c.alu(int(in.reg), ww, c.rdRM(ww), imm)
		if wr {
			c.wrRM(ww, r)
		}

	case op == 0x84 || op == 0x85:
		ww := w
		if op == 0x84 {
			ww = 1
		}
		c.fetchModRM()
		c.flagsLogic(ww, c.rdRM(ww)&c.rdReg(ww))
	case op == 0x86 || op == 0x87:
		ww := w
		if op == 0x86 {
			ww = 1
		}
		c.fetchModRM()
		a := c.rdRM(ww)
		b := c.rdReg(ww)
		c.wrRM(ww, b)
		c.wrReg(ww, a)

	// ---- MOV: 88-8B --------------------------------------------------------
	case op == 0x88:
		c.fetchModRM()
		c.wrRM(1, c.rdReg(1))
	case op == 0x89:
		c.fetchModRM()
		c.wrRM(w, c.rdReg(w))
	case op == 0x8A:
		c.fetchModRM()
		c.wrReg(1, c.rdRM(1))
	case op == 0x8B:
		c.fetchModRM()
		c.wrReg(w, c.rdRM(w))
	case op == 0x8C: // MOV r/m16, sreg
		c.fetchModRM()
		s := int(in.reg)
		if c.Model == Model8088 {
			s &= 3
		} else if s > SegGS {
			return c.ud()
		}
		if in.isReg {
			c.setReg(w, int(in.rm), uint32(c.Segs[s]))
		} else {
			c.wr16(in.easeg, in.ea, c.Segs[s])
		}
	case op == 0x8D: // LEA
		c.fetchModRM()
		if in.isReg {
			return c.ud()
		}
		c.wrReg(w, in.ea)
	case op == 0x8E: // MOV sreg, r/m16
		c.fetchModRM()
		s := int(in.reg)
		if c.Model == Model8088 {
			s &= 3
		} else if s == SegCS || s > SegGS {
			return c.ud()
		}
		c.Segs[s] = uint16(c.rdRM(2))
		if s == SegSS {
			c.intInhibit = true
		}
	case op == 0x8F: // POP r/m
		c.fetchModRM()
		if in.reg != 0 && c.Model != Model8088 {
			return c.ud()
		}
		v := c.pop(w)
		if !in.isReg && c.Model != Model8088 {
			// EA computed after SP increment: re-decode not needed for
			// 16-bit addressing (SP is never a base), but for 32-bit
			// addressing with ESP base we recompute.
			if in.addrsize == 4 {
				c.EIP = in.start + uint32(c.prefixLen())
				c.fetchModRM()
			}
		}
		c.wrRM(w, v)

	// ---- 90-9F -----------------------------------------------------------
	case op == 0x90:
		// NOP (also XCHG AX,AX); F3 90 = PAUSE
	case op < 0x98:
		r := int(op & 7)
		a := c.reg(w, RegAX)
		c.setReg(w, RegAX, c.reg(w, r))
		c.setReg(w, r, a)
	case op == 0x98:
		if w == 2 {
			c.SetAX(uint16(int16(int8(c.AL()))))
		} else {
			c.Regs[RegAX] = uint32(int32(int16(c.AX())))
		}
	case op == 0x99:
		if w == 2 {
			if c.AX()&0x8000 != 0 {
				c.SetDX(0xFFFF)
			} else {
				c.SetDX(0)
			}
		} else {
			if c.Regs[RegAX]&0x80000000 != 0 {
				c.Regs[RegDX] = 0xFFFFFFFF
			} else {
				c.Regs[RegDX] = 0
			}
		}
	case op == 0x9A: // CALL far ptr
		off := c.fetchImm(w)
		seg := c.fetch16()
		c.push(w, uint32(c.Segs[SegCS]))
		c.push(w, c.EIP)
		c.Segs[SegCS] = seg
		c.setIP(off)
	case op == 0x9B: // WAIT
	case op == 0x9C: // PUSHF
		f := c.Flags
		if c.Model == Model8088 {
			f |= 0xF002
		}
		if w == 2 {
			c.push16(uint16(f))
		} else {
			c.push32(f & 0xFFFF) // RF/VM read as 0
		}
	case op == 0x9D: // POPF
		c.popf(w, c.pop(w))
	case op == 0x9E: // SAHF
		c.Flags = c.Flags&^0xFF | uint32(c.AH())&0xD5 | 0x02
	case op == 0x9F: // LAHF
		c.SetAH(uint8(c.Flags&0xD7 | 0x02))

	// ---- A0-AF -------------------------------------------------------------
	case op == 0xA0:
		off := c.fetchImm(in.addrsize)
		c.SetAL(c.rd8(c.segOr(SegDS), off))
	case op == 0xA1:
		off := c.fetchImm(in.addrsize)
		c.setReg(w, RegAX, c.rdw(w, c.segOr(SegDS), off))
	case op == 0xA2:
		off := c.fetchImm(in.addrsize)
		c.wr8(c.segOr(SegDS), off, c.AL())
	case op == 0xA3:
		off := c.fetchImm(in.addrsize)
		c.wrw(w, c.segOr(SegDS), off, c.reg(w, RegAX))
	case op >= 0xA4 && op <= 0xA7, op >= 0xAA && op <= 0xAF:
		return c.stringOp(op)
	case op == 0xA8:
		c.flagsLogic(1, uint32(c.AL())&uint32(c.fetch8()))
	case op == 0xA9:
		c.flagsLogic(w, c.reg(w, RegAX)&c.fetchImm(w))

	// ---- MOV r, imm: B0-BF --------------------------------------------------
	case op < 0xB8:
		c.SetR8(int(op&7), c.fetch8())
	case op < 0xC0:
		c.setReg(w, int(op&7), c.fetchImm(w))

	// ---- C0-CF --------------------------------------------------------------
	case op == 0xC0 || op == 0xC1:
		ww := w
		if op == 0xC0 {
			ww = 1
		}
		c.fetchModRM()
		cnt := c.fetch8()
		c.wrRM(ww, c.shift(int(in.reg), ww, c.rdRM(ww), uint32(cnt)))
	case op == 0xC2:
		n := c.fetch16()
		c.setIP(c.pop(w))
		c.SetR16(RegSP, c.SP()+n)
	case op == 0xC3:
		c.setIP(c.pop(w))
	case op == 0xC4 || op == 0xC5: // LES / LDS
		c.fetchModRM()
		if in.isReg {
			return c.ud()
		}
		seg := SegES
		if op == 0xC5 {
			seg = SegDS
		}
		c.loadFarPtr(seg)
	case op == 0xC6:
		c.fetchModRM()
		if in.reg != 0 && c.Model != Model8088 {
			return c.ud()
		}
		c.wrRM(1, uint32(c.fetch8()))
	case op == 0xC7:
		c.fetchModRM()
		if in.reg != 0 && c.Model != Model8088 {
			return c.ud()
		}
		c.wrRM(w, c.fetchImm(w))
	case op == 0xC8:
		c.enter()
	case op == 0xC9: // LEAVE
		c.SetR16(RegSP, c.BP())
		c.setReg(w, RegBP, c.pop(w))
	case op == 0xCA:
		n := c.fetch16()
		ip := c.pop(w)
		c.Segs[SegCS] = uint16(c.pop(w))
		c.setIP(ip)
		c.SetR16(RegSP, c.SP()+n)
	case op == 0xCB:
		ip := c.pop(w)
		c.Segs[SegCS] = uint16(c.pop(w))
		c.setIP(ip)
	case op == 0xCC:
		c.Interrupt(3)
	case op == 0xCD:
		c.Interrupt(c.fetch8())
	case op == 0xCE:
		if c.Flags&FlagOF != 0 {
			c.Interrupt(4)
		}
	case op == 0xCF: // IRET
		ip := c.pop(w)
		cs := uint16(c.pop(w))
		f := c.pop(w)
		c.Segs[SegCS] = cs
		c.setIP(ip)
		c.popf(w, f)

	// ---- D0-DF --------------------------------------------------------------
	case op <= 0xD3:
		ww := w
		if op&1 == 0 {
			ww = 1
		}
		c.fetchModRM()
		cnt := uint32(1)
		if op&2 != 0 {
			cnt = uint32(c.CL())
		}
		c.wrRM(ww, c.shift(int(in.reg), ww, c.rdRM(ww), cnt))
	case op == 0xD4:
		return c.aam(c.fetch8())
	case op == 0xD5:
		c.aad(c.fetch8())
	case op == 0xD6: // SALC
		if c.Flags&FlagCF != 0 {
			c.SetAL(0xFF)
		} else {
			c.SetAL(0)
		}
	case op == 0xD7: // XLAT
		var off uint32
		if in.addrsize == 4 {
			off = c.Regs[RegBX] + uint32(c.AL())
		} else {
			off = uint32(c.BX() + uint16(c.AL()))
		}
		c.SetAL(c.rd8(c.segOr(SegDS), off))
	case op >= 0xD8:
		if op <= 0xDF {
			return c.fpuOp(op)
		}
		return c.execE(op)
	}
	return nil
}

// prefixLen returns the number of prefix bytes before the opcode (used to
// re-decode an instruction).
func (c *CPU) prefixLen() int {
	n := 0
	ip := c.in.start
	for {
		b := c.Mem.Read8(uint32(c.Segs[SegCS])<<4 + ip&0xFFFF)
		switch b {
		case 0x26, 0x2E, 0x36, 0x3E, 0x64, 0x65, 0x66, 0x67, 0xF0, 0xF2, 0xF3:
			n++
			ip++
			continue
		}
		return n + 1 // include opcode byte
	}
}

// execE handles E0-FF.
func (c *CPU) execE(op byte) error {
	in := &c.in
	w := in.opsize
	switch op {
	case 0xE0, 0xE1, 0xE2: // LOOPNE / LOOPE / LOOP
		d := int32(int8(c.fetch8()))
		cx := c.cxGet() - 1
		c.cxSet(cx)
		take := cx != 0
		if op == 0xE0 {
			take = take && c.Flags&FlagZF == 0
		} else if op == 0xE1 {
			take = take && c.Flags&FlagZF != 0
		}
		if take {
			c.jumpRel(d)
		}
	case 0xE3: // JCXZ / JECXZ
		d := int32(int8(c.fetch8()))
		if c.cxGet() == 0 {
			c.jumpRel(d)
		}
	case 0xE4:
		in.cycles += c.IOCycles
		c.SetAL(c.Bus.In8(uint16(c.fetch8())))
	case 0xE5:
		in.cycles += c.IOCycles
		c.setReg(w, RegAX, c.inw(w, uint16(c.fetch8())))
	case 0xE6:
		in.cycles += c.IOCycles
		c.Bus.Out8(uint16(c.fetch8()), c.AL())
	case 0xE7:
		in.cycles += c.IOCycles
		c.outw(w, uint16(c.fetch8()), c.reg(w, RegAX))
	case 0xE8: // CALL rel
		var d int32
		if w == 2 {
			d = int32(int16(c.fetch16()))
		} else {
			d = int32(c.fetch32())
		}
		ret := c.EIP
		c.jumpRel(d)
		c.push(w, ret)
	case 0xE9:
		var d int32
		if w == 2 {
			d = int32(int16(c.fetch16()))
		} else {
			d = int32(c.fetch32())
		}
		c.jumpRel(d)
	case 0xEA:
		off := c.fetchImm(w)
		seg := c.fetch16()
		c.Segs[SegCS] = seg
		c.setIP(off)
	case 0xEB:
		d := int32(int8(c.fetch8()))
		c.jumpRel(d)
	case 0xEC:
		in.cycles += c.IOCycles
		c.SetAL(c.Bus.In8(c.DX()))
	case 0xED:
		in.cycles += c.IOCycles
		c.setReg(w, RegAX, c.inw(w, c.DX()))
	case 0xEE:
		in.cycles += c.IOCycles
		c.Bus.Out8(c.DX(), c.AL())
	case 0xEF:
		in.cycles += c.IOCycles
		c.outw(w, c.DX(), c.reg(w, RegAX))
	case 0xF4:
		c.Halted = true
		if c.OnHalt != nil {
			c.OnHalt(c)
		}
	case 0xF5:
		c.Flags ^= FlagCF
	case 0xF6, 0xF7:
		return c.group3(op)
	case 0xF8:
		c.Flags &^= FlagCF
	case 0xF9:
		c.Flags |= FlagCF
	case 0xFA:
		c.Flags &^= FlagIF
	case 0xFB:
		c.Flags |= FlagIF
		c.intInhibit = true
	case 0xFC:
		c.Flags &^= FlagDF
	case 0xFD:
		c.Flags |= FlagDF
	case 0xFE:
		c.fetchModRM()
		switch in.reg {
		case 0:
			c.wrRM(1, c.flagsInc(1, c.rdRM(1)))
		case 1:
			c.wrRM(1, c.flagsDec(1, c.rdRM(1)))
		default:
			if c.Model == Model8088 {
				return c.group5(op)
			}
			return c.ud()
		}
	case 0xFF:
		c.fetchModRM()
		return c.group5(op)
	case 0xF1:
		// ICEBP: reserved BIOS trap when executing from the ROM segment.
		if c.Segs[SegCS] == 0xF000 && c.BIOSCall != nil {
			svc := c.fetch8()
			c.BIOSCall(c, svc)
			return nil
		}
		c.Interrupt(1)
	default:
		return c.ud()
	}
	return nil
}

func (c *CPU) inw(w int, port uint16) uint32 {
	if w == 2 {
		return uint32(c.Bus.In16(port))
	}
	return c.Bus.In32(port)
}

func (c *CPU) outw(w int, port uint16, v uint32) {
	if w == 2 {
		c.Bus.Out16(port, uint16(v))
	} else {
		c.Bus.Out32(port, v)
	}
}

// group5 handles FF /0-/7 (and the 8088 FE aliases).
func (c *CPU) group5(op byte) error {
	in := &c.in
	w := in.opsize
	if op == 0xFE {
		w = 1
	}
	switch in.reg {
	case 0:
		c.wrRM(w, c.flagsInc(w, c.rdRM(w)))
	case 1:
		c.wrRM(w, c.flagsDec(w, c.rdRM(w)))
	case 2: // CALL near r/m
		t := c.rdRM(w)
		if w == 2 {
			t &= 0xFFFF
		}
		c.push(w, c.EIP)
		c.setIP(t)
	case 3: // CALL far m
		if in.isReg {
			if c.Model == Model8088 {
				// Undefined on 8088; behaves like a far call using the
				// last EA. We approximate with #UD-free nonsense: skip.
				return nil
			}
			return c.ud()
		}
		off := c.rdw(w, in.easeg, in.ea)
		seg := c.rd16(in.easeg, c.ea2(in.ea+uint32(w)))
		c.push(w, uint32(c.Segs[SegCS]))
		c.push(w, c.EIP)
		c.Segs[SegCS] = seg
		c.setIP(off)
	case 4: // JMP near r/m
		t := c.rdRM(w)
		if w == 2 {
			t &= 0xFFFF
		}
		c.setIP(t)
	case 5: // JMP far m
		if in.isReg {
			if c.Model == Model8088 {
				return nil
			}
			return c.ud()
		}
		off := c.rdw(w, in.easeg, in.ea)
		seg := c.rd16(in.easeg, c.ea2(in.ea+uint32(w)))
		c.Segs[SegCS] = seg
		c.setIP(off)
	case 6, 7:
		if in.reg == 7 && c.Model != Model8088 {
			return c.ud()
		}
		v := c.rdRM(w)
		if in.isReg && in.rm == RegSP && c.Model == Model8088 {
			v -= 2
		}
		c.push(w, v)
	}
	return nil
}

// loadFarPtr implements LDS/LES/LFS/LGS/LSS: reg <- [m], seg <- [m+w].
func (c *CPU) loadFarPtr(seg int) {
	in := &c.in
	w := in.opsize
	off := c.rdw(w, in.easeg, in.ea)
	s := c.rd16(in.easeg, c.ea2(in.ea+uint32(w)))
	c.wrReg(w, off)
	c.Segs[seg] = s
	if seg == SegSS {
		c.intInhibit = true
	}
}

// popf loads the flags from a popped value, honouring model quirks.
func (c *CPU) popf(w int, v uint32) {
	if c.Model == Model8088 {
		c.Flags = v&0x0FD5 | 0xF002
		return
	}
	if w == 2 {
		c.Flags = c.Flags&0xFFFF0000 | v&0x7FD5 | 0x0002
	} else {
		// Real mode: VM (17) cannot be changed by POPFD; RF can.
		c.Flags = c.Flags&^0x00017FFF | v&0x00017FD5 | 0x0002
	}
}

func (c *CPU) pusha() {
	w := c.in.opsize
	sp := c.reg(w, RegSP)
	c.push(w, c.reg(w, RegAX))
	c.push(w, c.reg(w, RegCX))
	c.push(w, c.reg(w, RegDX))
	c.push(w, c.reg(w, RegBX))
	c.push(w, sp)
	c.push(w, c.reg(w, RegBP))
	c.push(w, c.reg(w, RegSI))
	c.push(w, c.reg(w, RegDI))
}

func (c *CPU) popa() {
	w := c.in.opsize
	// Registers popped before a stack fault keep their new values.
	for _, r := range []int{RegDI, RegSI, RegBP} {
		c.setReg(w, r, c.pop(w))
		c.snapRegs[r] = c.Regs[r]
	}
	skipped := c.pop(w)
	if w == 4 {
		// 386 quirk: the upper half of ESP is loaded from the popped value.
		c.Regs[RegSP] = skipped&0xFFFF0000 | c.Regs[RegSP]&0xFFFF
	}
	c.setReg(w, RegBX, c.pop(w))
	c.setReg(w, RegDX, c.pop(w))
	c.setReg(w, RegCX, c.pop(w))
	c.setReg(w, RegAX, c.pop(w))
}

func (c *CPU) enter() {
	w := c.in.opsize
	size := uint32(c.fetch16())
	level := int(c.fetch8()) & 0x1F
	c.push(w, c.reg(w, RegBP))
	frame := uint32(c.SP())
	if level > 0 {
		bp := uint32(c.BP())
		for i := 1; i < level; i++ {
			bp -= uint32(w)
			c.push(w, c.rdw(w, SegSS, bp&0xFFFF))
		}
		c.push(w, frame)
	}
	c.setReg(w, RegBP, frame)
	c.SetR16(RegSP, uint16(uint32(c.SP())-size))
}

func (c *CPU) bound() error {
	in := &c.in
	w := in.opsize
	c.fetchModRM()
	if in.isReg {
		return c.ud()
	}
	idx := c.rdReg(w)
	lo := c.rdw(w, in.easeg, in.ea)
	hi := c.rdw(w, in.easeg, c.ea2(in.ea+uint32(w)))
	var out bool
	if w == 2 {
		out = int16(idx) < int16(lo) || int16(idx) > int16(hi)
	} else {
		out = int32(idx) < int32(lo) || int32(idx) > int32(hi)
	}
	if out {
		c.exception(5)
	}
	return nil
}

// lockCheck raises #UD when a LOCK prefix is used with an instruction form
// that is not lockable (or whose destination is a register).
func (c *CPU) lockCheck(op byte, twoByte bool) {
	peek := c.Mem.Read8(uint32(c.Segs[SegCS])<<4 + c.EIP&0xFFFF)
	mod := peek >> 6
	reg := (peek >> 3) & 7
	ok := false
	if twoByte {
		switch op {
		case 0xAB, 0xB3, 0xBB, 0xB0, 0xB1, 0xC0, 0xC1:
			ok = true
		case 0xBA:
			ok = reg >= 5
		}
	} else {
		switch {
		case op < 0x40 && op&7 <= 1 && op&0x38 != 0x38:
			ok = true
		case op >= 0x80 && op <= 0x83:
			ok = reg != 7
		case op == 0x86 || op == 0x87:
			ok = true
		case op == 0xF6 || op == 0xF7:
			ok = reg == 2 || reg == 3
		case op == 0xFE || op == 0xFF:
			ok = reg == 0 || reg == 1
		}
	}
	if !ok || mod == 3 {
		c.raise(6)
	}
}

// popSeg pops a segment register value: only 16 bits are read even with a
// 32-bit operand size, but SP is adjusted by the operand size.
func (c *CPU) popSeg(w int) uint16 {
	sp := uint16(c.Regs[RegSP])
	v := c.rd16(SegSS, uint32(sp))
	c.SetR16(RegSP, sp+uint16(w))
	return v
}

// ea2 returns the address of the second word of a far pointer / bound
// pair; with 16-bit addressing it wraps within the segment.
func (c *CPU) ea2(off uint32) uint32 {
	if c.in.addrsize == 2 {
		return off & 0xFFFF
	}
	return off
}

// setIP loads a new instruction pointer, raising #GP if it exceeds the
// 64K code segment limit.
func (c *CPU) setIP(v uint32) {
	if v > 0xFFFF && c.Model != Model8088 {
		c.raise(13)
	}
	c.EIP = v
}
