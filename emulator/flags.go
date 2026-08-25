package emulator

var parityTable [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		n := 0
		for b := i; b != 0; b >>= 1 {
			n += b & 1
		}
		if n%2 == 0 {
			parityTable[i] = FlagPF
		}
	}
}

func widthMask(w int) uint32 {
	switch w {
	case 1:
		return 0xFF
	case 2:
		return 0xFFFF
	default:
		return 0xFFFFFFFF
	}
}

func signBit(w int) uint32 {
	switch w {
	case 1:
		return 0x80
	case 2:
		return 0x8000
	default:
		return 0x80000000
	}
}

// setSZP sets SF, ZF and PF from a result of the given width, leaving other flags.
func (c *CPU) setSZP(w int, r uint32) {
	r &= widthMask(w)
	c.Flags &^= FlagSF | FlagZF | FlagPF
	if r == 0 {
		c.Flags |= FlagZF
	}
	if r&signBit(w) != 0 {
		c.Flags |= FlagSF
	}
	c.Flags |= parityTable[r&0xFF]
}

// flagsAdd computes flags for r = a + b + cin.
func (c *CPU) flagsAdd(w int, a, b, cin uint32) uint32 {
	m := widthMask(w)
	a &= m
	b &= m
	r64 := uint64(a) + uint64(b) + uint64(cin)
	r := uint32(r64) & m
	c.Flags &^= flagsArith
	if r64 > uint64(m) {
		c.Flags |= FlagCF
	}
	if (a^b^r)&0x10 != 0 {
		c.Flags |= FlagAF
	}
	if (^(a ^ b) & (a ^ r) & signBit(w)) != 0 {
		c.Flags |= FlagOF
	}
	c.setSZP(w, r)
	return r
}

// flagsSub computes flags for r = a - b - bin.
func (c *CPU) flagsSub(w int, a, b, bin uint32) uint32 {
	m := widthMask(w)
	a &= m
	b &= m
	r := (a - b - bin) & m
	c.Flags &^= flagsArith
	if uint64(a) < uint64(b)+uint64(bin) {
		c.Flags |= FlagCF
	}
	if (a^b^r)&0x10 != 0 {
		c.Flags |= FlagAF
	}
	if ((a ^ b) & (a ^ r) & signBit(w)) != 0 {
		c.Flags |= FlagOF
	}
	c.setSZP(w, r)
	return r
}

// flagsLogic: CF=OF=0, AF undefined (cleared), SZP from result.
func (c *CPU) flagsLogic(w int, r uint32) uint32 {
	r &= widthMask(w)
	c.Flags &^= flagsArith
	c.setSZP(w, r)
	return r
}

// flagsInc / flagsDec preserve CF.
func (c *CPU) flagsInc(w int, a uint32) uint32 {
	cf := c.Flags & FlagCF
	r := c.flagsAdd(w, a, 1, 0)
	c.Flags = c.Flags&^FlagCF | cf
	return r
}

func (c *CPU) flagsDec(w int, a uint32) uint32 {
	cf := c.Flags & FlagCF
	r := c.flagsSub(w, a, 1, 0)
	c.Flags = c.Flags&^FlagCF | cf
	return r
}
