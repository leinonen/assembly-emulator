package emulator

import "math"

// fpuOp executes an x87 escape opcode D8-DF.
func (c *CPU) fpuOp(op byte) error {
	in := &c.in
	f := &c.FPU
	c.fetchModRM()
	in.cycles += 20
	f.LastIP = uint16(in.start)
	f.LastCS = c.Segs[SegCS]
	f.LastOp = uint16(op&7)<<8 | uint16(in.modrm)
	if !in.isReg {
		f.LastDP = uint16(in.ea)
		f.LastDS = c.Segs[in.easeg]
	}

	if in.isReg {
		return c.fpuReg(op)
	}
	seg, ea := in.easeg, in.ea
	r := int(in.reg)

	switch op {
	case 0xD8: // m32fp arithmetic
		c.fpuArith(r, float64(math.Float32frombits(c.rd32(seg, ea))))
	case 0xDC: // m64fp arithmetic
		c.fpuArith(r, math.Float64frombits(uint64(c.rd32(seg, ea))|uint64(c.rd32(seg, ea+4))<<32))
	case 0xDA: // m32int arithmetic
		c.fpuArith(r, float64(int32(c.rd32(seg, ea))))
	case 0xDE: // m16int arithmetic
		c.fpuArith(r, float64(int16(c.rd16(seg, ea))))
	case 0xD9:
		switch r {
		case 0: // FLD m32
			f.push(float64(math.Float32frombits(c.rd32(seg, ea))))
		case 2, 3: // FST / FSTP m32
			c.wr32(seg, ea, math.Float32bits(float32(f.get(0))))
			if r == 3 {
				f.pop()
			}
		case 4: // FLDENV
			c.fldenv(seg, ea)
		case 5: // FLDCW
			f.CW = c.rd16(seg, ea)
		case 6: // FNSTENV
			c.fstenv(seg, ea)
		case 7: // FNSTCW
			c.wr16(seg, ea, f.CW)
		default:
			return c.ud()
		}
	case 0xDB:
		switch r {
		case 0: // FILD m32
			f.push(float64(int32(c.rd32(seg, ea))))
		case 1: // FISTTP m32
			c.wr32(seg, ea, uint32(int32(math.Trunc(f.get(0)))))
			f.pop()
		case 2, 3: // FIST / FISTP m32
			c.wr32(seg, ea, uint32(int32(f.toInt(f.get(0), 32))))
			if r == 3 {
				f.pop()
			}
		case 5: // FLD m80
			f.push(c.readF80(seg, ea))
		case 7: // FSTP m80
			c.writeF80(seg, ea, f.get(0))
			f.pop()
		default:
			return c.ud()
		}
	case 0xDD:
		switch r {
		case 0: // FLD m64
			f.push(math.Float64frombits(uint64(c.rd32(seg, ea)) | uint64(c.rd32(seg, ea+4))<<32))
		case 1: // FISTTP m64
			v := int64(math.Trunc(f.get(0)))
			c.wr32(seg, ea, uint32(v))
			c.wr32(seg, ea+4, uint32(v>>32))
			f.pop()
		case 2, 3: // FST / FSTP m64
			b := math.Float64bits(f.get(0))
			c.wr32(seg, ea, uint32(b))
			c.wr32(seg, ea+4, uint32(b>>32))
			if r == 3 {
				f.pop()
			}
		case 4: // FRSTOR
			c.frstor(seg, ea)
		case 6: // FNSAVE
			c.fsave(seg, ea)
		case 7: // FNSTSW m16
			c.wr16(seg, ea, f.statusWord())
		default:
			return c.ud()
		}
	case 0xDF:
		switch r {
		case 0: // FILD m16
			f.push(float64(int16(c.rd16(seg, ea))))
		case 1: // FISTTP m16
			c.wr16(seg, ea, uint16(int16(math.Trunc(f.get(0)))))
			f.pop()
		case 2, 3: // FIST / FISTP m16
			c.wr16(seg, ea, uint16(int16(f.toInt(f.get(0), 16))))
			if r == 3 {
				f.pop()
			}
		case 4: // FBLD
			f.push(c.readBCD(seg, ea))
		case 5: // FILD m64
			f.push(float64(int64(uint64(c.rd32(seg, ea)) | uint64(c.rd32(seg, ea+4))<<32)))
		case 6: // FBSTP
			c.writeBCD(seg, ea, f.get(0))
			f.pop()
		case 7: // FISTP m64
			v := f.toInt(f.get(0), 64)
			c.wr32(seg, ea, uint32(v))
			c.wr32(seg, ea+4, uint32(v>>32))
			f.pop()
		}
	}
	return nil
}

// fpuArith performs st(0) = st(0) op src for the memory forms.
func (c *CPU) fpuArith(r int, src float64) {
	f := &c.FPU
	a := f.get(0)
	switch r {
	case 0:
		f.setST(0, a+src)
	case 1:
		f.setST(0, a*src)
	case 2:
		f.fcom(a, src)
	case 3:
		f.fcom(a, src)
		f.pop()
	case 4:
		f.setST(0, a-src)
	case 5:
		f.setST(0, src-a)
	case 6:
		f.setST(0, a/src)
	case 7:
		f.setST(0, src/a)
	}
}

// fcom sets C0/C2/C3 from comparing a with b.
func (f *FPU) fcom(a, b float64) {
	switch {
	case math.IsNaN(a) || math.IsNaN(b):
		f.setC(true, false, true, true)
	case a > b:
		f.setC(false, false, false, false)
	case a < b:
		f.setC(true, false, false, false)
	default:
		f.setC(false, false, false, true)
	}
}

// fcomi sets ZF/PF/CF from comparing a with b.
func (c *CPU) fcomi(a, b float64) {
	c.Flags &^= FlagZF | FlagPF | FlagCF | FlagOF | FlagSF | FlagAF
	switch {
	case math.IsNaN(a) || math.IsNaN(b):
		c.Flags |= FlagZF | FlagPF | FlagCF
	case a > b:
	case a < b:
		c.Flags |= FlagCF
	default:
		c.Flags |= FlagZF
	}
	c.FPU.setC1(false)
}

// fpuReg handles the register (mod==3) forms.
func (c *CPU) fpuReg(op byte) error {
	f := &c.FPU
	in := &c.in
	i := int(in.rm)
	r := int(in.reg)
	switch op {
	case 0xD8:
		c.fpuArith(r, f.get(i))
	case 0xDC: // st(i) = st(i) op st(0)  (note reversed sub/div encodings)
		a := f.get(i)
		b := f.get(0)
		switch r {
		case 0:
			f.setST(i, a+b)
		case 1:
			f.setST(i, a*b)
		case 2:
			f.fcom(b, a)
		case 3:
			f.fcom(b, a)
			f.pop()
		case 4: // FSUBR st(i),st(0): st(i) = st(0) - st(i)
			f.setST(i, b-a)
		case 5: // FSUB st(i),st(0)
			f.setST(i, a-b)
		case 6: // FDIVR st(i),st(0): st(i) = st(0)/st(i)
			f.setST(i, b/a)
		case 7: // FDIV st(i),st(0)
			f.setST(i, a/b)
		}
	case 0xDE:
		a := f.get(i)
		b := f.get(0)
		switch r {
		case 0:
			f.setST(i, a+b)
		case 1:
			f.setST(i, a*b)
		case 2:
			return c.ud()
		case 3: // FCOMPP
			if i != 1 {
				return c.ud()
			}
			f.fcom(b, f.get(1))
			f.pop()
			f.pop()
			return nil
		case 4: // FSUBRP
			f.setST(i, b-a)
		case 5: // FSUBP
			f.setST(i, a-b)
		case 6: // FDIVRP
			f.setST(i, b/a)
		case 7: // FDIVP
			f.setST(i, a/b)
		}
		f.pop()
	case 0xD9:
		switch r {
		case 0: // FLD st(i)
			v := f.get(i)
			f.push(v)
		case 1: // FXCH
			a, b := f.get(0), f.get(i)
			f.setST(0, b)
			f.setST(i, a)
			f.setC1(false)
		case 2: // FNOP (D9 D0)
		case 4:
			switch i {
			case 0: // FCHS
				f.setST(0, -f.get(0))
			case 1: // FABS
				f.setST(0, math.Abs(f.get(0)))
			case 4: // FTST
				f.fcom(f.get(0), 0)
			case 5: // FXAM
				c.fxam()
			default:
				return c.ud()
			}
		case 5: // constants
			switch i {
			case 0:
				f.push(1)
			case 1:
				f.push(math.Log2(10))
			case 2:
				f.push(math.Log2E)
			case 3:
				f.push(math.Pi)
			case 4:
				f.push(math.Log10(2))
			case 5:
				f.push(math.Ln2)
			case 6:
				f.push(0)
			default:
				return c.ud()
			}
		case 6:
			x := f.get(0)
			switch i {
			case 0: // F2XM1
				f.setST(0, math.Exp2(x)-1)
			case 1: // FYL2X: st1 = st1*log2(st0); pop
				f.setST(1, f.get(1)*math.Log2(x))
				f.pop()
			case 2: // FPTAN: st0 = tan(st0); push 1
				f.setST(0, math.Tan(x))
				f.push(1)
				f.SW &^= fswC2
			case 3: // FPATAN: st1 = atan2(st1, st0); pop
				f.setST(1, math.Atan2(f.get(1), x))
				f.pop()
			case 4: // FXTRACT
				fr, ex := math.Frexp(x)
				// frexp returns [0.5,1); x87 wants [1,2)
				f.setST(0, float64(ex-1))
				f.push(fr * 2)
			case 5: // FPREM1
				c.fprem(true)
			case 6: // FDECSTP
				f.Top = (f.Top - 1) & 7
			case 7: // FINCSTP
				f.Top = (f.Top + 1) & 7
			}
		case 7:
			x := f.get(0)
			switch i {
			case 0: // FPREM
				c.fprem(false)
			case 1: // FYL2XP1
				f.setST(1, f.get(1)*math.Log2(x+1))
				f.pop()
			case 2: // FSQRT
				f.setST(0, math.Sqrt(x))
			case 3: // FSINCOS
				s, co := math.Sincos(x)
				f.setST(0, s)
				f.push(co)
				f.SW &^= fswC2
			case 4: // FRNDINT
				f.setST(0, f.round(x))
			case 5: // FSCALE: st0 = st0 * 2^trunc(st1)
				f.setST(0, math.Ldexp(x, int(math.Trunc(f.get(1)))))
			case 6: // FSIN
				f.setST(0, math.Sin(x))
				f.SW &^= fswC2
			case 7: // FCOS
				f.setST(0, math.Cos(x))
				f.SW &^= fswC2
			}
		default:
			return c.ud()
		}
	case 0xDA:
		if in.modrm == 0xE9 { // FUCOMPP
			f.fcom(f.get(0), f.get(1))
			f.pop()
			f.pop()
			return nil
		}
		if r < 4 { // FCMOVcc (P6)
			take := false
			switch r {
			case 0:
				take = c.Flags&FlagCF != 0
			case 1:
				take = c.Flags&FlagZF != 0
			case 2:
				take = c.Flags&(FlagCF|FlagZF) != 0
			case 3:
				take = c.Flags&FlagPF != 0
			}
			if take {
				f.setST(0, f.get(i))
			}
			return nil
		}
		return c.ud()
	case 0xDB:
		switch in.modrm {
		case 0xE0, 0xE1, 0xE4: // FENI / FDISI / FSETPM: no-ops
		case 0xE2: // FNCLEX
			f.SW &^= 0x80FF
		case 0xE3: // FNINIT
			f.Init()
		default:
			switch r {
			case 0, 1, 2, 3: // FCMOVNcc
				take := false
				switch r {
				case 0:
					take = c.Flags&FlagCF == 0
				case 1:
					take = c.Flags&FlagZF == 0
				case 2:
					take = c.Flags&(FlagCF|FlagZF) == 0
				case 3:
					take = c.Flags&FlagPF == 0
				}
				if take {
					f.setST(0, f.get(i))
				}
			case 5, 6: // FUCOMI / FCOMI
				c.fcomi(f.get(0), f.get(i))
			default:
				return c.ud()
			}
		}
	case 0xDD:
		switch r {
		case 0: // FFREE
			f.Tag[f.phys(i)] = tagEmpty
		case 2: // FST st(i)
			f.setST(i, f.get(0))
		case 3: // FSTP st(i)
			f.setST(i, f.get(0))
			f.pop()
		case 4: // FUCOM
			f.fcom(f.get(0), f.get(i))
		case 5: // FUCOMP
			f.fcom(f.get(0), f.get(i))
			f.pop()
		default:
			return c.ud()
		}
	case 0xDF:
		switch {
		case in.modrm == 0xE0: // FNSTSW AX
			c.SetAX(f.statusWord())
		case r == 5, r == 6: // FUCOMIP / FCOMIP
			c.fcomi(f.get(0), f.get(i))
			f.pop()
		default:
			return c.ud()
		}
	}
	return nil
}

func (c *CPU) fxam() {
	f := &c.FPU
	if f.empty(0) {
		f.setC(true, false, false, true) // empty: C3=1 C0=1
		return
	}
	x := f.ST(0)
	sign := math.Signbit(x)
	switch {
	case math.IsNaN(x):
		f.setC(true, sign, false, false)
	case math.IsInf(x, 0):
		f.setC(true, sign, true, false)
	case x == 0:
		f.setC(false, sign, false, true)
	default:
		f.setC(false, sign, true, false)
	}
}

// fprem implements FPREM (truncating) and FPREM1 (IEEE remainder).
func (c *CPU) fprem(ieee bool) {
	f := &c.FPU
	a := f.get(0)
	b := f.get(1)
	if b == 0 || math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) {
		f.setST(0, math.NaN())
		f.SW |= fswIE
		return
	}
	var q, r float64
	if ieee {
		q = math.RoundToEven(a / b)
		r = math.Remainder(a, b)
	} else {
		q = math.Trunc(a / b)
		r = math.Mod(a, b)
	}
	f.setST(0, r)
	qi := int64(math.Abs(q))
	f.setC(qi&4 != 0, qi&1 != 0, false, qi&2 != 0)
}

// ---- environment / state images (real-mode 16-bit layout) ---------------------

func (c *CPU) fstenv(seg int, ea uint32) {
	f := &c.FPU
	if c.in.opsize == 4 {
		c.wr32(seg, ea, uint32(f.CW))
		c.wr32(seg, ea+4, uint32(f.statusWord()))
		c.wr32(seg, ea+8, uint32(f.tagWord()))
		c.wr32(seg, ea+12, uint32(f.LastIP))
		c.wr32(seg, ea+16, uint32(f.LastOp&0x7FF)<<16|uint32(f.LastCS))
		c.wr32(seg, ea+20, uint32(f.LastDP))
		c.wr32(seg, ea+24, uint32(f.LastDS))
	} else {
		c.wr16(seg, ea, f.CW)
		c.wr16(seg, ea+2, f.statusWord())
		c.wr16(seg, ea+4, f.tagWord())
		c.wr16(seg, ea+6, f.LastIP)
		c.wr16(seg, ea+8, f.LastOp&0x7FF|uint16(f.LastCS&0xF000)>>4)
		c.wr16(seg, ea+10, f.LastDP)
		c.wr16(seg, ea+12, uint16(f.LastDS)>>12<<12)
	}
	// FNSTENV masks all exceptions afterwards.
	f.CW |= 0x3F
}

func (c *CPU) fldenv(seg int, ea uint32) {
	f := &c.FPU
	if c.in.opsize == 4 {
		f.CW = c.rd16(seg, ea)
		f.SW = c.rd16(seg, ea+4)
		f.setTagWord(c.rd16(seg, ea+8))
	} else {
		f.CW = c.rd16(seg, ea)
		f.SW = c.rd16(seg, ea+2)
		f.setTagWord(c.rd16(seg, ea+4))
	}
	f.Top = uint8(f.SW>>11) & 7
}

func (c *CPU) envSize() uint32 {
	if c.in.opsize == 4 {
		return 28
	}
	return 14
}

func (c *CPU) fsave(seg int, ea uint32) {
	f := &c.FPU
	c.fstenv(seg, ea)
	off := ea + c.envSize()
	for i := 0; i < 8; i++ {
		c.writeF80(seg, off+uint32(i)*10, f.ST(i))
	}
	f.Init()
}

func (c *CPU) frstor(seg int, ea uint32) {
	f := &c.FPU
	c.fldenv(seg, ea)
	off := ea + c.envSize()
	for i := 0; i < 8; i++ {
		f.R[f.phys(i)] = c.readF80(seg, off+uint32(i)*10)
	}
}
