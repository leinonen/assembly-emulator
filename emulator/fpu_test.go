package emulator

import (
	"math"
	"testing"
)

func TestF80RoundTrip(t *testing.T) {
	for _, v := range []float64{0, 1, -1, 3.14159, 1e300, -1e-300, 65535, 0.1, math.Inf(1), math.Inf(-1), math.SmallestNonzeroFloat64} {
		m, se := encodeF80(v)
		got := decodeF80(m, se)
		if got != v {
			t.Errorf("f80 roundtrip %g -> %g (mant=%016X se=%04X)", v, got, m, se)
		}
	}
	m, se := encodeF80(math.NaN())
	if !math.IsNaN(decodeF80(m, se)) {
		t.Errorf("NaN roundtrip failed")
	}
	// Known encoding: 1.0 = 3FFF 8000000000000000
	if m, se := encodeF80(1); m != 0x8000000000000000 || se != 0x3FFF {
		t.Errorf("encode 1.0: %016X %04X", m, se)
	}
}

func TestFPUBasic(t *testing.T) {
	// fninit ; fld1 ; fldpi ; faddp st1,st0 ; fistp word [0x400]
	code := []byte{0xDB, 0xE3, 0xD9, 0xE8, 0xD9, 0xEB, 0xDE, 0xC1, 0xDF, 0x1E, 0x00, 0x04}
	c := run(t, code, 5, nil)
	if v := c.Mem.Read16(0x400); v != 4 {
		t.Errorf("fistp (1+pi rounded nearest) = %d want 4", v)
	}
	if c.FPU.Top != 0 {
		t.Errorf("stack not empty: top=%d", c.FPU.Top)
	}
}

func TestFPURounding(t *testing.T) {
	// fninit; fldcw [0x500] (RC=trunc) ; fld dword [0x504] (=2.7) ; fistp word [0x400]
	code := []byte{0xDB, 0xE3, 0xD9, 0x2E, 0x00, 0x05, 0xD9, 0x06, 0x04, 0x05, 0xDF, 0x1E, 0x00, 0x04}
	c := run(t, code, 4, func(c *CPU) {
		c.Mem.Write16(0x500, 0x0F7F) // RC=11 truncate
		c.Mem.Write32(0x504, math.Float32bits(2.7))
	})
	if v := c.Mem.Read16(0x400); v != 2 {
		t.Errorf("fistp trunc(2.7) = %d want 2", v)
	}
	// default rounding: 2.5 -> 2 (even), 3.5 -> 4
	c = run(t, []byte{0xDB, 0xE3, 0xD9, 0x06, 0x04, 0x05, 0xDF, 0x1E, 0x00, 0x04}, 3, func(c *CPU) {
		c.Mem.Write32(0x504, math.Float32bits(2.5))
	})
	if v := c.Mem.Read16(0x400); v != 2 {
		t.Errorf("fistp nearest-even(2.5) = %d want 2", v)
	}
}

func TestFPUCompareAndStatus(t *testing.T) {
	// fninit ; fld1 ; fldz ; fcompp ; fnstsw ax ; sahf  -> st0(0) < st1(1): C0 set -> CF set
	code := []byte{0xDB, 0xE3, 0xD9, 0xE8, 0xD9, 0xEE, 0xDE, 0xD9, 0xDF, 0xE0, 0x9E}
	c := run(t, code, 6, nil)
	if c.Flags&FlagCF == 0 || c.Flags&FlagZF != 0 {
		t.Errorf("fcompp 0 vs 1: flags=%s AX=%04X", flagsStr(c.Flags), c.AX())
	}
}

func TestFPUSinFprem(t *testing.T) {
	// fninit ; fld dword [0x504] (=7.5) ; fld dword [0x508] (=2.0) ; fxch ; fprem ; fstp dword [0x400] ; -> 1.5
	code := []byte{0xDB, 0xE3, 0xD9, 0x06, 0x04, 0x05, 0xD9, 0x06, 0x08, 0x05, 0xD9, 0xC9, 0xD9, 0xF8, 0xD9, 0x1E, 0x00, 0x04}
	c := run(t, code, 6, func(c *CPU) {
		c.Mem.Write32(0x504, math.Float32bits(7.5))
		c.Mem.Write32(0x508, math.Float32bits(2.0))
	})
	if v := math.Float32frombits(c.Mem.Read32(0x400)); v != 1.5 {
		t.Errorf("fprem 7.5 mod 2 = %g", v)
	}
	if c.FPU.SW&fswC2 != 0 {
		t.Errorf("fprem: C2 should be clear")
	}
	// fsin(pi/2) ≈ 1
	code = []byte{0xDB, 0xE3, 0xD9, 0xEB, 0xD9, 0xE8, 0xD9, 0xE8, 0xDE, 0xC1, 0xDE, 0xF9, 0xD9, 0xFE}
	// fninit; fldpi; fld1; fld1; faddp (2); fdivp st1,st0 (pi/2); fsin
	c = run(t, code, 7, nil)
	if v := c.FPU.ST(0); math.Abs(v-1) > 1e-12 {
		t.Errorf("fsin(pi/2)=%g", v)
	}
}

func TestFPUStackOverflow(t *testing.T) {
	// fninit then 9 x fld1: the 9th push overflows -> NaN + stack fault
	code := []byte{0xDB, 0xE3}
	for i := 0; i < 9; i++ {
		code = append(code, 0xD9, 0xE8)
	}
	c := run(t, code, 10, nil)
	if !math.IsNaN(c.FPU.ST(0)) || c.FPU.SW&fswSF == 0 {
		t.Errorf("stack overflow: st0=%g sw=%04X", c.FPU.ST(0), c.FPU.SW)
	}
}
