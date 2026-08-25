package emulator

// shift implements group 2: ROL ROR RCL RCR SHL SHR SAL SAR.
// count is the raw count; the 386 masks it to 5 bits, the 8088 does not.
func (c *CPU) shift(kind int, w int, a uint32, count uint32) uint32 {
	if c.Model != Model8088 {
		count &= 0x1F
	}
	if count == 0 {
		return a
	}
	bits := uint32(w * 8)
	m := widthMask(w)
	msb := signBit(w)
	a &= m
	var r uint32
	cf := c.Flags&FlagCF != 0
	var of bool

	switch kind {
	case 0: // ROL
		n := count % bits
		r = (a<<n | a>>(bits-n)) & m
		cf = r&1 != 0
		of = (r&msb != 0) != cf
	case 1: // ROR
		n := count % bits
		r = (a>>n | a<<(bits-n)) & m
		cf = r&msb != 0
		of = (r&msb != 0) != (r&(msb>>1) != 0)
	case 2: // RCL
		n := count % (bits + 1)
		r = a
		for i := uint32(0); i < n; i++ {
			nc := r&msb != 0
			r = (r << 1) & m
			if cf {
				r |= 1
			}
			cf = nc
		}
		of = (r&msb != 0) != cf
	case 3: // RCR
		n := count % (bits + 1)
		r = a
		for i := uint32(0); i < n; i++ {
			nc := r&1 != 0
			r >>= 1
			if cf {
				r |= msb
			}
			cf = nc
		}
		of = (r&msb != 0) != (r&(msb>>1) != 0)
	case 4, 6: // SHL / SAL
		if kind == 6 && c.Model == Model8088 {
			// Undocumented 8086 SETMO: result is all ones.
			r = m
			c.SetFlag(FlagCF, false)
			c.SetFlag(FlagOF, false)
			c.setSZP(w, r)
			c.Flags &^= FlagAF
			return r
		}
		// 386 quirk: 8-bit shifts by 16 or 24 report CF as a shift by 8.
		if w == 1 && count > 8 && count%8 == 0 && c.Model != Model8088 {
			count = 8
		}
		if count < bits {
			r = (a << count) & m
			cf = (a>>(bits-count))&1 != 0
		} else if count == bits {
			r = 0
			cf = a&1 != 0
		} else {
			r = 0
			cf = false
		}
		of = (r&msb != 0) != cf
	case 5: // SHR
		if w == 1 && count > 8 && count%8 == 0 && c.Model != Model8088 {
			count = 8
		}
		if count < bits {
			r = a >> count
			cf = (a>>(count-1))&1 != 0
		} else if count == bits {
			r = 0
			cf = a&msb != 0
		} else {
			r = 0
			cf = false
		}
		// OF = MSB(result) XOR next bit; equals MSB(original) for count 1.
		of = (r&msb != 0) != (r&(msb>>1) != 0)
	case 7: // SAR
		s := int32(a)
		if w == 1 {
			s = int32(int8(a))
		} else if w == 2 {
			s = int32(int16(a))
		}
		if count < bits {
			r = uint32(s>>count) & m
			cf = (s>>(count-1))&1 != 0
		} else {
			if s < 0 {
				r = m
				cf = true
			} else {
				r = 0
				cf = false
			}
		}
		of = false
	}

	c.SetFlag(FlagCF, cf)
	if kind >= 4 {
		c.setSZP(w, r)
		c.Flags &^= FlagAF
	}
	c.SetFlag(FlagOF, of)
	return r
}

// shiftDouble implements SHLD/SHRD. For 16-bit operands with a count
// above 16 the 386 produces src<<(n-16) | src>>(32-n) (verified against
// hardware traces); CF is the last bit shifted out of the 32-bit
// concatenation, OF = MSB(result) XOR CF.
func (c *CPU) shiftDouble(left bool, w int, dst, src uint32, count uint32) uint32 {
	count &= 0x1F
	if count == 0 {
		return dst
	}
	m := widthMask(w)
	msb := signBit(w)
	dst &= m
	src &= m
	var r uint32
	var cf bool
	if w == 2 {
		if left {
			if count <= 16 {
				r = (dst<<count | src>>(16-count)) & m
				cf = (dst>>(16-count))&1 != 0
			} else {
				r = (src<<(count-16) | src>>(32-count)) & m
				cf = (src>>(32-count))&1 != 0
			}
		} else {
			if count <= 16 {
				r = (dst>>count | src<<(16-count)) & m
				cf = (dst>>(count-1))&1 != 0
			} else {
				r = (src>>(count-16) | src<<(32-count)) & m
				cf = (src>>(count-17))&1 != 0
			}
		}
	} else {
		if left {
			r = dst<<count | src>>(32-count)
			cf = (dst>>(32-count))&1 != 0
		} else {
			r = dst>>count | src<<(32-count)
			cf = (dst>>(count-1))&1 != 0
		}
	}
	c.setSZP(w, r)
	c.SetFlag(FlagCF, cf)
	if count > 16 && w == 2 {
		c.SetFlag(FlagOF, (r&msb != 0) != cf)
	} else {
		c.SetFlag(FlagOF, (r&msb != 0) != (dst&msb != 0))
	}
	c.Flags &^= FlagAF
	return r
}
