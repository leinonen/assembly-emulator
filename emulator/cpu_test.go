package emulator

import "testing"

// run loads code at 0000:0100 and executes n instructions.
func run(t *testing.T, code []byte, n int, setup func(c *CPU)) *CPU {
	t.Helper()
	c := NewCPU(Model386)
	c.Segs[SegCS] = 0
	c.Segs[SegDS] = 0
	c.Segs[SegES] = 0
	c.Segs[SegSS] = 0x1000
	c.SetSP(0xFFFE)
	c.EIP = 0x100
	c.Flags = 0x0002
	copy(c.Mem.RAM[0x100:], code)
	if setup != nil {
		setup(c)
	}
	for i := 0; i < n; i++ {
		if err := c.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	return c
}

func flagsStr(f uint32) string {
	s := ""
	for _, b := range []struct {
		bit  uint32
		name string
	}{{FlagCF, "C"}, {FlagPF, "P"}, {FlagAF, "A"}, {FlagZF, "Z"}, {FlagSF, "S"}, {FlagOF, "O"}} {
		if f&b.bit != 0 {
			s += b.name
		} else {
			s += "-"
		}
	}
	return s
}

func TestAdd8Flags(t *testing.T) {
	cases := []struct {
		a, b  uint8
		r     uint8
		flags string
	}{
		{0xFF, 0x01, 0x00, "CPAZ--"},
		{0x7F, 0x01, 0x80, "--A-SO"},
		{0x80, 0x80, 0x00, "CP-Z-O"},
	}
	for _, tc := range cases {
		// mov al,a ; add al,b
		c := run(t, []byte{0xB0, tc.a, 0x04, tc.b}, 2, nil)
		if c.AL() != tc.r {
			t.Errorf("add %02X+%02X: AL=%02X want %02X", tc.a, tc.b, c.AL(), tc.r)
		}
		if got := flagsStr(c.Flags); got != tc.flags {
			t.Errorf("add %02X+%02X: flags %s want %s", tc.a, tc.b, got, tc.flags)
		}
	}
}

func TestSub8Signed(t *testing.T) {
	// mov al,90h ; cmp al,10h ; jl +2 ; mov bl,1 ; (target) mov cl,1
	c := run(t, []byte{0xB0, 0x90, 0x3C, 0x10, 0x7C, 0x02, 0xB3, 0x01, 0xB1, 0x01}, 4, nil)
	if c.BL() != 0 || c.CL() != 1 {
		t.Errorf("signed compare: JL not taken (BL=%d CL=%d)", c.BL(), c.CL())
	}
	if c.Flags&FlagSF == 0 || c.Flags&FlagOF != 0 {
		t.Errorf("flags after cmp 90h,10h: %s", flagsStr(c.Flags))
	}
}

func TestMulDiv8(t *testing.T) {
	// mov ax,1234h ; mov bl,10h ; mul bl -> AX = 34h*10h = 340h, DX untouched
	c := run(t, []byte{0xB8, 0x34, 0x12, 0xB3, 0x10, 0xF6, 0xE3}, 3, func(c *CPU) { c.SetDX(0x5555) })
	if c.AX() != 0x0340 || c.DX() != 0x5555 {
		t.Errorf("mul bl: AX=%04X DX=%04X", c.AX(), c.DX())
	}
	if c.Flags&FlagCF == 0 {
		t.Errorf("mul bl: CF should be set (AH != 0)")
	}
	// mov ax,0123h ; mov bl,10h ; div bl -> AL=12h AH=03h
	c = run(t, []byte{0xB8, 0x23, 0x01, 0xB3, 0x10, 0xF6, 0xF3}, 3, nil)
	if c.AL() != 0x12 || c.AH() != 0x03 {
		t.Errorf("div bl: AL=%02X AH=%02X", c.AL(), c.AH())
	}
	// imul bl with bl=-1: ax = 5 * -1 = -5
	c = run(t, []byte{0xB8, 0x05, 0x00, 0xB3, 0xFF, 0xF6, 0xEB}, 3, nil)
	if c.AX() != 0xFFFB {
		t.Errorf("imul bl: AX=%04X", c.AX())
	}
}

func TestDivByZeroRaisesInt0(t *testing.T) {
	c := run(t, []byte{0x31, 0xDB, 0xF6, 0xF3}, 2, func(c *CPU) {
		c.Mem.Write16(0, 0x2000) // int 0 -> 0000:2000
		c.Mem.Write16(2, 0x0000)
	})
	if c.EIP != 0x2000 || c.Segs[SegCS] != 0 {
		t.Errorf("div by zero: CS:IP=%04X:%04X", c.Segs[SegCS], c.EIP)
	}
	// 386: return address is the faulting instruction (0x102)
	if ip := c.Mem.Read16(0x1000<<4 + 0xFFF8); ip != 0x102 {
		t.Errorf("pushed IP=%04X want 0102", ip)
	}
}

func TestShiftFlags(t *testing.T) {
	// mov al,80h ; shl al,1 -> AL=0, CF=1, OF=1 (msb changed)
	c := run(t, []byte{0xB0, 0x80, 0xD0, 0xE0}, 2, nil)
	if c.AL() != 0 || c.Flags&FlagCF == 0 || c.Flags&FlagOF == 0 || c.Flags&FlagZF == 0 {
		t.Errorf("shl al,1: AL=%02X flags=%s", c.AL(), flagsStr(c.Flags))
	}
	// mov ax,1 ; rol ax,16 -> unchanged, CF = 1
	c = run(t, []byte{0xB8, 0x01, 0x00, 0xC1, 0xC0, 0x10}, 2, nil)
	if c.AX() != 1 || c.Flags&FlagCF == 0 {
		t.Errorf("rol ax,16: AX=%04X flags=%s", c.AX(), flagsStr(c.Flags))
	}
	// sar al,1 with al=0x81 -> 0xC0, CF=1
	c = run(t, []byte{0xB0, 0x81, 0xD0, 0xF8}, 2, nil)
	if c.AL() != 0xC0 || c.Flags&FlagCF == 0 {
		t.Errorf("sar al,1: AL=%02X flags=%s", c.AL(), flagsStr(c.Flags))
	}
}

func TestStringOpsDF(t *testing.T) {
	// std ; mov si,0x210 ; mov di,0x220 ; mov cx,4 ; rep movsb
	c := run(t, []byte{0xFD, 0xBE, 0x10, 0x02, 0xBF, 0x20, 0x02, 0xB9, 0x04, 0x00, 0xF3, 0xA4}, 5, func(c *CPU) {
		copy(c.Mem.RAM[0x20D:], []byte{1, 2, 3, 4})
	})
	if got := c.Mem.RAM[0x21D:0x221]; got[0] != 1 || got[1] != 2 || got[2] != 3 || got[3] != 4 {
		t.Errorf("rep movsb backwards: %v", got)
	}
	if c.SI() != 0x20C || c.DI() != 0x21C || c.CX() != 0 {
		t.Errorf("SI=%04X DI=%04X CX=%04X", c.SI(), c.DI(), c.CX())
	}
}

func TestMemDefaultSegments(t *testing.T) {
	// mov al,[di] must use DS, not ES.
	c := run(t, []byte{0x8A, 0x05}, 1, func(c *CPU) {
		c.Segs[SegDS] = 0x100
		c.Segs[SegES] = 0x200
		c.SetDI(0x10)
		c.Mem.RAM[0x1010] = 0xAA
		c.Mem.RAM[0x2010] = 0xBB
	})
	if c.AL() != 0xAA {
		t.Errorf("[di] used ES: AL=%02X", c.AL())
	}
	// mov al,[bp+2] uses SS.
	c = run(t, []byte{0x8A, 0x46, 0x02}, 1, func(c *CPU) {
		c.Segs[SegSS] = 0x300
		c.SetBP(0x10)
		c.Mem.RAM[0x3012] = 0xCC
	})
	if c.AL() != 0xCC {
		t.Errorf("[bp+2] did not use SS: AL=%02X", c.AL())
	}
}

func TestAddrModes32(t *testing.T) {
	// 67 8B 04 9B : mov ax,[ebx+ebx*4]  ; ebx=0x100 -> 0x500
	c := run(t, []byte{0x67, 0x8B, 0x04, 0x9B}, 1, func(c *CPU) {
		c.Regs[RegBX] = 0x100
		c.Mem.Write16(0x500, 0xBEEF)
	})
	if c.AX() != 0xBEEF {
		t.Errorf("SIB addressing: AX=%04X", c.AX())
	}
}

func TestPushSP386(t *testing.T) {
	c := run(t, []byte{0x54}, 1, nil)
	if v := c.Mem.Read16(0x1000<<4 + 0xFFFC); v != 0xFFFE {
		t.Errorf("push sp (386) pushed %04X want FFFE", v)
	}
}

func TestInterruptAndIret(t *testing.T) {
	// int 21h -> handler at 0000:0300 does iret. Vector at 0x84.
	c := run(t, []byte{0xCD, 0x21, 0x90}, 2, func(c *CPU) {
		c.Mem.Write16(0x84, 0x300)
		c.Mem.Write16(0x86, 0)
		c.Mem.RAM[0x300] = 0xCF
		c.Flags |= FlagIF
	})
	if c.EIP != 0x102 || c.Flags&FlagIF == 0 || c.SP() != 0xFFFE {
		t.Errorf("after int/iret: IP=%04X flags=%08X SP=%04X", c.EIP, c.Flags, c.SP())
	}
}

func TestEnterLeave(t *testing.T) {
	// enter 8,0 ; leave
	c := run(t, []byte{0xC8, 0x08, 0x00, 0x00, 0xC9}, 1, func(c *CPU) { c.SetBP(0x1234) })
	if c.BP() != 0xFFFC || c.SP() != 0xFFF4 {
		t.Errorf("enter: BP=%04X SP=%04X", c.BP(), c.SP())
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.BP() != 0x1234 || c.SP() != 0xFFFE {
		t.Errorf("leave: BP=%04X SP=%04X", c.BP(), c.SP())
	}
}

func TestBCD(t *testing.T) {
	// mov al,15h ; add al,27h ; daa -> 42h
	c := run(t, []byte{0xB0, 0x15, 0x04, 0x27, 0x27}, 3, nil)
	if c.AL() != 0x42 {
		t.Errorf("daa: AL=%02X", c.AL())
	}
	// mov ax,0x0107 ; aad -> al = 1*10+7 = 17
	c = run(t, []byte{0xB8, 0x07, 0x01, 0xD5, 0x0A}, 2, nil)
	if c.AX() != 17 {
		t.Errorf("aad: AX=%04X", c.AX())
	}
	// mov al,17 ; aam -> ah=1 al=7
	c = run(t, []byte{0xB0, 17, 0xD4, 0x0A}, 2, nil)
	if c.AX() != 0x0107 {
		t.Errorf("aam: AX=%04X", c.AX())
	}
}

func TestBitOps(t *testing.T) {
	// mov eax,0x80000000 ; bsr ebx,eax -> 31
	c := run(t, []byte{0x66, 0xB8, 0x00, 0x00, 0x00, 0x80, 0x66, 0x0F, 0xBD, 0xD8}, 2, nil)
	if c.Regs[RegBX] != 31 {
		t.Errorf("bsr: EBX=%d", c.Regs[RegBX])
	}
	// bts word [0x400], 17 with register offset: mov ax,17 ; bts [0x400],ax
	c = run(t, []byte{0xB8, 0x11, 0x00, 0x0F, 0xAB, 0x06, 0x00, 0x04}, 2, nil)
	if c.Mem.RAM[0x402] != 0x02 {
		t.Errorf("bts mem: %02X %02X", c.Mem.RAM[0x400], c.Mem.RAM[0x402])
	}
	// movsx ax, byte -1 : mov bl,0xFF ; movsx ax,bl
	c = run(t, []byte{0xB3, 0xFF, 0x0F, 0xBE, 0xC3}, 2, nil)
	if c.AX() != 0xFFFF {
		t.Errorf("movsx: AX=%04X", c.AX())
	}
}
