package emulator

import "math"

// FPU is the x87 coprocessor state. Register values are kept as float64,
// which is enough for DOS-era code (the 80-bit formats are converted on
// load/store).
type FPU struct {
	R   [8]float64 // physical registers
	Tag [8]uint8   // 0 valid, 1 zero, 2 special, 3 empty
	Top uint8
	SW  uint16
	CW  uint16

	// Last instruction / operand pointers for FNSTENV/FNSAVE.
	LastIP, LastCS, LastDP, LastDS uint16
	LastOp                         uint16
}

// Status word bits.
const (
	fswIE = 1 << 0
	fswDE = 1 << 1
	fswZE = 1 << 2
	fswOE = 1 << 3
	fswUE = 1 << 4
	fswPE = 1 << 5
	fswSF = 1 << 6
	fswES = 1 << 7
	fswC0 = 1 << 8
	fswC1 = 1 << 9
	fswC2 = 1 << 10
	fswC3 = 1 << 14
	fswB  = 1 << 15

	tagValid   = 0
	tagZero    = 1
	tagSpecial = 2
	tagEmpty   = 3
)

func (f *FPU) Init() {
	*f = FPU{}
	f.CW = 0x037F
	for i := range f.Tag {
		f.Tag[i] = tagEmpty
	}
}

func (f *FPU) phys(i int) int { return (int(f.Top) + i) & 7 }

// ST returns st(i).
func (f *FPU) ST(i int) float64 { return f.R[f.phys(i)] }

func (f *FPU) setST(i int, v float64) {
	p := f.phys(i)
	f.R[p] = v
	f.Tag[p] = tagFor(v)
}

func tagFor(v float64) uint8 {
	switch {
	case v == 0:
		return tagZero
	case math.IsInf(v, 0) || math.IsNaN(v):
		return tagSpecial
	default:
		return tagValid
	}
}

func (f *FPU) empty(i int) bool { return f.Tag[f.phys(i)] == tagEmpty }

func (f *FPU) setC(c0, c1, c2, c3 bool) {
	f.SW &^= fswC0 | fswC1 | fswC2 | fswC3
	if c0 {
		f.SW |= fswC0
	}
	if c1 {
		f.SW |= fswC1
	}
	if c2 {
		f.SW |= fswC2
	}
	if c3 {
		f.SW |= fswC3
	}
}

func (f *FPU) setC1(v bool) {
	if v {
		f.SW |= fswC1
	} else {
		f.SW &^= fswC1
	}
}

// push decrements TOP and stores v. Overflow (target not empty) raises a
// stack fault and stores the indefinite NaN.
func (f *FPU) push(v float64) {
	f.Top = (f.Top - 1) & 7
	if f.Tag[f.Top] != tagEmpty {
		f.SW |= fswIE | fswSF | fswC1
		v = math.NaN()
	}
	f.R[f.Top] = v
	f.Tag[f.Top] = tagFor(v)
}

func (f *FPU) pop() {
	f.Tag[f.Top] = tagEmpty
	f.Top = (f.Top + 1) & 7
}

// read st(i) with underflow check.
func (f *FPU) get(i int) float64 {
	if f.empty(i) {
		f.SW |= fswIE | fswSF
		f.SW &^= fswC1
		return math.NaN()
	}
	return f.R[f.phys(i)]
}

// round applies the rounding-control field to v.
func (f *FPU) round(v float64) float64 {
	switch (f.CW >> 10) & 3 {
	case 0:
		return math.RoundToEven(v)
	case 1:
		return math.Floor(v)
	case 2:
		return math.Ceil(v)
	default:
		return math.Trunc(v)
	}
}

// statusWord returns SW with the TOP field filled in.
func (f *FPU) statusWord() uint16 {
	return f.SW&^(7<<11) | uint16(f.Top)<<11
}

// tagWord packs the tag array in physical order.
func (f *FPU) tagWord() uint16 {
	var t uint16
	for i := 7; i >= 0; i-- {
		t = t<<2 | uint16(f.Tag[i])
	}
	return t
}

func (f *FPU) setTagWord(t uint16) {
	for i := 0; i < 8; i++ {
		f.Tag[i] = uint8(t>>(2*uint(i))) & 3
	}
}

// ---- 80-bit extended conversion ----------------------------------------

// encodeF80 converts a float64 to the 10-byte x87 extended format.
func encodeF80(v float64) (mant uint64, se uint16) {
	bits := math.Float64bits(v)
	sign := uint16(bits>>63) << 15
	exp := int((bits >> 52) & 0x7FF)
	frac := bits & (1<<52 - 1)
	switch {
	case exp == 0x7FF: // inf / nan
		if frac == 0 {
			return 1 << 63, sign | 0x7FFF
		}
		return 1<<63 | frac<<11 | 1<<62, sign | 0x7FFF
	case exp == 0 && frac == 0:
		return 0, sign
	case exp == 0: // subnormal double: normalise
		e := -1022
		for frac&(1<<52) == 0 {
			frac <<= 1
			e--
		}
		return frac << 11, sign | uint16(e+16383)
	default:
		return 1<<63 | frac<<11, sign | uint16(exp-1023+16383)
	}
}

// decodeF80 converts the 10-byte x87 format to float64.
func decodeF80(mant uint64, se uint16) float64 {
	sign := 1.0
	if se&0x8000 != 0 {
		sign = -1
	}
	exp := int(se & 0x7FFF)
	if exp == 0x7FFF {
		if mant<<1 == 0 {
			return math.Inf(int(sign))
		}
		return math.NaN()
	}
	if mant == 0 {
		return math.Copysign(0, sign)
	}
	// value = mant * 2^(exp-16383-63)
	return sign * math.Ldexp(float64(mant), exp-16383-63)
}

func (c *CPU) readF80(seg int, off uint32) float64 {
	lo := c.rd32(seg, off)
	hi := c.rd32(seg, off+4)
	se := c.rd16(seg, off+8)
	return decodeF80(uint64(hi)<<32|uint64(lo), se)
}

func (c *CPU) writeF80(seg int, off uint32, v float64) {
	m, se := encodeF80(v)
	c.wr32(seg, off, uint32(m))
	c.wr32(seg, off+4, uint32(m>>32))
	c.wr16(seg, off+8, se)
}

// ---- packed BCD -----------------------------------------------------------

func (c *CPU) readBCD(seg int, off uint32) float64 {
	var v float64
	for i := 8; i >= 0; i-- {
		b := c.rd8(seg, off+uint32(i))
		v = v*100 + float64((b>>4)*10+b&0xF)
	}
	if c.rd8(seg, off+9)&0x80 != 0 {
		v = -v
	}
	return v
}

func (c *CPU) writeBCD(seg int, off uint32, v float64) {
	neg := v < 0
	if neg {
		v = -v
	}
	n := uint64(c.FPU.round(v))
	for i := 0; i < 9; i++ {
		d := n % 100
		n /= 100
		c.wr8(seg, off+uint32(i), uint8(d/10<<4|d%10))
	}
	if neg {
		c.wr8(seg, off+9, 0x80)
	} else {
		c.wr8(seg, off+9, 0)
	}
}

// ---- integer store with rounding / overflow ---------------------------------

// toInt rounds v per RC and range-checks it; out of range yields the
// "integer indefinite" value and sets IE.
func (f *FPU) toInt(v float64, bits int) int64 {
	r := f.round(v)
	lo := -math.Ldexp(1, bits-1)
	hi := math.Ldexp(1, bits-1) - 1
	if math.IsNaN(r) || r < lo || r > hi {
		f.SW |= fswIE
		return -1 << uint(bits-1)
	}
	if r != v {
		f.SW |= fswPE
	}
	return int64(r)
}
