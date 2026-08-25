package emulator

import "math/bits"

// exec0F executes two-byte (0F xx) opcodes.
func (c *CPU) exec0F() error {
	in := &c.in
	w := in.opsize
	op := c.fetch8()
	if in.lock {
		c.lockCheck(op, true)
	}
	switch {
	case op == 0x06: // CLTS
	case op == 0x0B: // UD2
		return c.ud()
	case op == 0x1F: // multi-byte NOP
		c.fetchModRM()
	case op == 0x31: // RDTSC
		c.Regs[RegAX] = uint32(c.Cycles)
		c.Regs[RegDX] = uint32(c.Cycles >> 32)
	case op >= 0x40 && op <= 0x4F: // CMOVcc
		c.fetchModRM()
		v := c.rdRM(w)
		if c.cond(op & 0xF) {
			c.wrReg(w, v)
		}
	case op >= 0x80 && op <= 0x8F: // Jcc rel16/32
		var d int32
		if w == 2 {
			d = int32(int16(c.fetch16()))
		} else {
			d = int32(c.fetch32())
		}
		if c.cond(op & 0xF) {
			c.jumpRel(d)
		}
	case op >= 0x90 && op <= 0x9F: // SETcc
		c.fetchModRM()
		if c.cond(op & 0xF) {
			c.wrRM(1, 1)
		} else {
			c.wrRM(1, 0)
		}
	case op == 0xA0:
		c.push(w, uint32(c.Segs[SegFS]))
	case op == 0xA1:
		c.Segs[SegFS] = c.popSeg(w)
	case op == 0xA8:
		c.push(w, uint32(c.Segs[SegGS]))
	case op == 0xA9:
		c.Segs[SegGS] = c.popSeg(w)
	case op == 0xA2: // CPUID: minimal, report a 386-class CPU
		switch c.Regs[RegAX] {
		case 0:
			c.Regs[RegAX] = 0
			c.Regs[RegBX] = 0x756E6547
			c.Regs[RegDX] = 0x49656E69
			c.Regs[RegCX] = 0x6C65746E
		default:
			c.Regs[RegAX] = 0x0300
			c.Regs[RegBX] = 0
			c.Regs[RegCX] = 0
			c.Regs[RegDX] = 1 // FPU present
		}
	case op == 0xA3, op == 0xAB, op == 0xB3, op == 0xBB: // BT/BTS/BTR/BTC r/m, r
		c.fetchModRM()
		c.bitOp(int(op>>3)&3, w, c.rdReg(w), true)
	case op == 0xBA: // BT group with imm8
		c.fetchModRM()
		if in.reg < 4 {
			return c.ud()
		}
		imm := uint32(c.fetch8())
		c.bitOp(int(in.reg)&3, w, imm, false)
	case op == 0xA4, op == 0xA5, op == 0xAC, op == 0xAD: // SHLD/SHRD
		c.fetchModRM()
		var cnt uint32
		if op&1 == 0 {
			cnt = uint32(c.fetch8())
		} else {
			cnt = uint32(c.CL())
		}
		c.wrRM(w, c.shiftDouble(op < 0xA8, w, c.rdRM(w), c.rdReg(w), cnt))
	case op == 0xAF: // IMUL r, r/m
		c.fetchModRM()
		c.wrReg(w, c.imul2(w, c.rdReg(w), c.rdRM(w)))
	case op == 0xB0, op == 0xB1: // CMPXCHG
		ww := w
		if op == 0xB0 {
			ww = 1
		}
		c.fetchModRM()
		dst := c.rdRM(ww)
		acc := c.reg(ww, RegAX)
		c.flagsSub(ww, acc, dst, 0)
		if acc == dst {
			c.wrRM(ww, c.rdReg(ww))
		} else {
			c.setReg(ww, RegAX, dst)
			c.wrRM(ww, dst)
		}
	case op == 0xB2, op == 0xB4, op == 0xB5: // LSS/LFS/LGS
		c.fetchModRM()
		if in.isReg {
			return c.ud()
		}
		seg := map[byte]int{0xB2: SegSS, 0xB4: SegFS, 0xB5: SegGS}[op]
		c.loadFarPtr(seg)
	case op == 0xB6: // MOVZX r, r/m8
		c.fetchModRM()
		c.wrReg(w, c.rdRM(1))
	case op == 0xB7: // MOVZX r32, r/m16
		c.fetchModRM()
		c.wrReg(w, c.rdRM(2))
	case op == 0xBE:
		c.fetchModRM()
		c.wrReg(w, uint32(int32(int8(c.rdRM(1)))))
	case op == 0xBF:
		c.fetchModRM()
		c.wrReg(w, uint32(int32(int16(c.rdRM(2)))))
	case op == 0xBC, op == 0xBD: // BSF / BSR
		c.fetchModRM()
		v := c.rdRM(w)
		if v == 0 {
			c.Flags |= FlagZF
		} else {
			c.Flags &^= FlagZF
			var r int
			if op == 0xBC {
				r = bits.TrailingZeros32(v)
			} else {
				r = 31 - bits.LeadingZeros32(v)
			}
			c.wrReg(w, uint32(r))
		}
	case op == 0xC0, op == 0xC1: // XADD
		ww := w
		if op == 0xC0 {
			ww = 1
		}
		c.fetchModRM()
		dst := c.rdRM(ww)
		src := c.rdReg(ww)
		c.wrReg(ww, dst)
		c.wrRM(ww, c.flagsAdd(ww, dst, src, 0))
	case op >= 0xC8 && op <= 0xCF: // BSWAP
		r := int(op & 7)
		c.Regs[r] = bits.ReverseBytes32(c.Regs[r])
	default:
		return c.ud()
	}
	return nil
}

// bitOp implements BT (0), BTS (1), BTR (2), BTC (3).
// For memory operands with a register bit offset the address is adjusted
// by the signed bit offset in operand-size units; immediates are taken
// modulo the operand size.
func (c *CPU) bitOp(kind int, w int, off uint32, regOff bool) {
	in := &c.in
	bitsN := uint32(w * 8)
	var v uint32
	var bit uint32
	if in.isReg {
		bit = off % bitsN
		v = c.reg(w, int(in.rm))
	} else {
		if regOff {
			var so int32
			if w == 2 {
				so = int32(int16(off))
			} else {
				so = int32(off)
			}
			shift := uint(4)
			if w == 4 {
				shift = 5
			}
			idx := so >> shift // arithmetic shift = floor division
			bit = uint32(so) & (bitsN - 1)
			in.ea += uint32(idx) * uint32(w)
			if in.addrsize == 2 {
				in.ea &= 0xFFFF
			}
		} else {
			bit = off % bitsN
		}
		v = c.rdw(w, in.easeg, in.ea)
	}
	c.SetFlag(FlagCF, (v>>bit)&1 != 0)
	switch kind {
	case 1:
		v |= 1 << bit
	case 2:
		v &^= 1 << bit
	case 3:
		v ^= 1 << bit
	}
	if kind != 0 {
		if in.isReg {
			c.setReg(w, int(in.rm), v)
		} else {
			c.wrw(w, in.easeg, in.ea, v)
		}
	}
}
