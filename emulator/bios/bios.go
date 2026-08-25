// Package bios implements the PC BIOS: interrupt vector table, ROM stubs,
// the BIOS data area, and services for INT 08h-1Ah. Services run in Go;
// ROM stubs trap into Go with the reserved F1 <svc> sequence so that
// programs hooking or chaining interrupt vectors keep working.
package bios

import (
	"time"

	"assembly-emulator/emulator"
	"assembly-emulator/emulator/io"
	"assembly-emulator/font"
)

// Machine gives the BIOS access to the hardware it drives.
type Machine struct {
	CPU      *emulator.CPU
	Mem      *emulator.Memory
	Bus      *io.Bus
	VGA      *io.VGA
	Keyboard *io.Keyboard
	PIT      *io.PIT
	PIC      *io.PIC
	Stdout   func(b byte) // host console mirror (may be nil)
	Clock    func() time.Time
	// ExtCall handles services not implemented by the BIOS (DOS INT 20h/21h
	// etc.). It returns false if the service must be retried after idling.
	ExtCall func(c *emulator.CPU, svc byte) bool
}

// ROM layout (segment F000).
const (
	ROMSeg      = 0xF000
	romIRet     = 0xE000 // plain IRET for unhandled vectors
	romInt08    = 0x0100
	romInt09    = 0x0140
	romStubBase = 0x0200 // per-vector "F1 xx / iret" stubs, 16 bytes each
	romWaitBase = 0x0800 // per-vector idle-loop stubs, 16 bytes each
	romFont8x16 = 0xA000
	romFont8x8  = 0xFA6E
	romReset    = 0xFFF0
)

// BDA addresses.
const (
	bdaEquip      = 0x410
	bdaMemSize    = 0x413
	bdaKbFlags1   = 0x417
	bdaKbFlags2   = 0x418
	bdaKbHead     = 0x41A
	bdaKbTail     = 0x41C
	bdaKbBuf      = 0x41E
	bdaKbBufEnd   = 0x43E
	bdaVideoMode  = 0x449
	bdaCols       = 0x44A
	bdaPageSize   = 0x44C
	bdaPageOff    = 0x44E
	bdaCursor     = 0x450 // 8 words
	bdaCursorEnd  = 0x460
	bdaCursorBeg  = 0x461
	bdaPage       = 0x462
	bdaCRTC       = 0x463
	bdaTicks      = 0x46C
	bdaMidnight   = 0x470
	bdaBreak      = 0x471
	bdaKbBufStart = 0x480
	bdaKbBufEndP  = 0x482
	bdaRows       = 0x484
	bdaCharH      = 0x485
	bdaVideoCtl   = 0x487
	bdaVideoSw    = 0x488
	bdaKbFlags3   = 0x496
	bdaKbLED      = 0x497
)

// Vectors with a Go service behind a simple stub.
var stubVectors = []uint8{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x17, 0x1A, 0x33, 0x2F, 0x20, 0x21}

// Vectors whose stub includes an idle loop (they can block).
var waitVectors = []uint8{0x16, 0x21, 0x15}

// BIOS holds service state.
type BIOS struct {
	M         *Machine
	waitUntil uint64 // for INT 15h/86h
	Exited    bool
	ExitCode  int
}

// Install writes the ROM image, IVT and BDA and hooks the CPU trap.
func Install(m *Machine) *BIOS {
	b := &BIOS{M: m}
	mem := m.Mem
	rom := func(off uint16, data []byte) { mem.WriteROM(uint32(ROMSeg)<<4+uint32(off), data) }

	// Default handler: IRET.
	rom(romIRet, []byte{0xCF})
	// INT 08h: push ax / F1 08 (Go: tick) / int 1Ch / mov al,20h / out 20h,al / pop ax / iret
	rom(romInt08, []byte{0x50, 0xF1, 0x08, 0xCD, 0x1C, 0xB0, 0x20, 0xE6, 0x20, 0x58, 0xCF})
	// INT 09h: F1 09 / iret (Go handles EOI)
	rom(romInt09, []byte{0xF1, 0x09, 0xCF})
	for _, v := range stubVectors {
		rom(romStubBase+uint16(v)*16, []byte{0xF1, v, 0xCF})
	}
	// Idle-loop stubs: L: F1 vv / sti / hlt / jmp L / iret. The Go service
	// sets IP to L+6 when the call is complete.
	for _, v := range waitVectors {
		rom(romWaitBase+uint16(v)*16, []byte{0xF1, v, 0xFB, 0xF4, 0xEB, 0xFA, 0xCF})
	}
	// Fonts.
	var f16 [256 * 16]byte
	var f8 [256 * 8]byte
	for ch := 0; ch < 256; ch++ {
		copy(f16[ch*16:], font.CP437Font[ch][:])
		for y := 0; y < 8; y++ {
			f8[ch*8+y] = font.CP437Font[ch][y*2]
		}
	}
	rom(romFont8x16, f16[:])
	rom(romFont8x8, f8[:])
	// Reset vector and model byte.
	rom(romReset, []byte{0xEA, 0x00, 0x00, 0x00, 0xF0}) // jmp far F000:0000
	rom(0xFFF5, []byte("01/01/92"))
	rom(0xFFFE, []byte{0xFC})
	rom(0x0000, []byte{0xF4}) // hlt

	// IVT.
	for v := 0; v < 256; v++ {
		b.setVector(uint8(v), ROMSeg, romIRet)
	}
	b.setVector(0x08, ROMSeg, romInt08)
	b.setVector(0x09, ROMSeg, romInt09)
	for _, v := range stubVectors {
		b.setVector(v, ROMSeg, romStubBase+uint16(v)*16)
	}
	for _, v := range waitVectors {
		b.setVector(v, ROMSeg, romWaitBase+uint16(v)*16)
	}
	b.setVector(0x1F, ROMSeg, romFont8x8+128*8) // graphics chars 80h-FFh
	b.setVector(0x43, ROMSeg, romFont8x16)
	b.setVector(0x1D, ROMSeg, romIRet) // video parameter table (unused)
	b.setVector(0x1E, ROMSeg, romIRet) // diskette params

	// BDA.
	mem.Write16(bdaEquip, 0x0021|0x0002) // 80x25 colour, FPU, 1 floppy
	mem.Write16(bdaMemSize, 640)
	mem.Write16(bdaKbHead, bdaKbBuf)
	mem.Write16(bdaKbTail, bdaKbBuf)
	mem.Write16(bdaKbBufStart, bdaKbBuf&0xFF|0x1E)
	mem.Write16(bdaKbBufStart, 0x1E)
	mem.Write16(bdaKbBufEndP, 0x3E)
	mem.Write8(bdaKbFlags1, kbNum)
	mem.Write16(bdaCRTC, 0x3D4)
	mem.Write8(bdaRows, 24)
	mem.Write16(bdaCharH, 16)
	mem.Write8(bdaVideoCtl, 0x60)
	mem.Write8(bdaVideoSw, 0x09)
	b.setMode(0x03, false)
	mem.ROMProtect = true

	// Boot-time tick count from the wall clock.
	if m.Clock != nil {
		t := m.Clock()
		secs := t.Hour()*3600 + t.Minute()*60 + t.Second()
		mem.Write32(bdaTicks, uint32(float64(secs)*18.2065))
	}

	m.CPU.BIOSCall = b.call
	return b
}

func (b *BIOS) setVector(v uint8, seg, off uint16) {
	b.M.Mem.Write16(uint32(v)*4, off)
	b.M.Mem.Write16(uint32(v)*4+2, seg)
}

// ---- helpers for services -----------------------------------------------

// setCF sets the carry flag in the FLAGS image saved by the INT that
// invoked the current stub (at SS:SP+4).
func (b *BIOS) setCF(c *emulator.CPU, on bool) {
	addr := uint32(c.SS())<<4 + uint32(c.SP()+4)
	f := b.M.Mem.Read16(addr)
	if on {
		f |= emulator.FlagCF
	} else {
		f &^= emulator.FlagCF
	}
	b.M.Mem.Write16(addr, f)
}

func (b *BIOS) setZF(c *emulator.CPU, on bool) {
	addr := uint32(c.SS())<<4 + uint32(c.SP()+4)
	f := b.M.Mem.Read16(addr)
	if on {
		f |= emulator.FlagZF
	} else {
		f &^= emulator.FlagZF
	}
	b.M.Mem.Write16(addr, f)
}

// done marks a wait-stub call complete by skipping the idle loop.
func done(c *emulator.CPU) {
	c.EIP += 4 // past sti / hlt / jmp rel8
}

// call is the F1 trap dispatcher.
func (b *BIOS) call(c *emulator.CPU, svc byte) {
	switch svc {
	case 0x08:
		b.timerTick()
	case 0x09:
		b.keyboardIRQ()
	case 0x10:
		b.int10(c)
	case 0x11:
		c.SetAX(b.M.Mem.Read16(bdaEquip))
	case 0x12:
		c.SetAX(b.M.Mem.Read16(bdaMemSize))
	case 0x13:
		b.int13(c)
	case 0x14:
		c.SetAX(0x8000) // no serial port
	case 0x15:
		if b.int15(c) {
			done(c)
		}
	case 0x16:
		if b.int16(c) {
			done(c)
		}
	case 0x17:
		c.SetAH(0x90) // printer: selected, no error
	case 0x1A:
		b.int1A(c)
	case 0x33:
		if c.AX() == 0 {
			c.SetAX(0) // no mouse
		}
	case 0x2F:
		// Multiplex: report nothing installed.
	default:
		if b.M.ExtCall != nil {
			if b.M.ExtCall(c, svc) {
				if svc == 0x21 {
					done(c)
				}
			}
		}
	}
}

// ---- INT 08h / 1Ah: timer ------------------------------------------------------

func (b *BIOS) timerTick() {
	mem := b.M.Mem
	t := mem.Read32(bdaTicks) + 1
	if t >= 0x1800B0 {
		t = 0
		mem.Write8(bdaMidnight, 1)
	}
	mem.Write32(bdaTicks, t)
}

func bcd(v int) uint8 { return uint8(v/10<<4 | v%10) }

func (b *BIOS) int1A(c *emulator.CPU) {
	mem := b.M.Mem
	switch c.AH() {
	case 0x00:
		t := mem.Read32(bdaTicks)
		c.SetCX(uint16(t >> 16))
		c.SetDX(uint16(t))
		c.SetAL(mem.Read8(bdaMidnight))
		mem.Write8(bdaMidnight, 0)
	case 0x01:
		mem.Write32(bdaTicks, uint32(c.CX())<<16|uint32(c.DX()))
		mem.Write8(bdaMidnight, 0)
	case 0x02:
		t := b.now()
		c.SetCH(bcd(t.Hour()))
		c.SetCL(bcd(t.Minute()))
		c.SetDH(bcd(t.Second()))
		c.SetDL(0)
		b.setCF(c, false)
	case 0x04:
		t := b.now()
		c.SetCH(bcd(t.Year() / 100))
		c.SetCL(bcd(t.Year() % 100))
		c.SetDH(bcd(int(t.Month())))
		c.SetDL(bcd(t.Day()))
		b.setCF(c, false)
	case 0x03, 0x05:
		b.setCF(c, false)
	default:
		b.setCF(c, true)
	}
}

func (b *BIOS) now() time.Time {
	if b.M.Clock != nil {
		return b.M.Clock()
	}
	return time.Now()
}

// ---- INT 13h: disk (none) ------------------------------------------------------

func (b *BIOS) int13(c *emulator.CPU) {
	switch c.AH() {
	case 0x00:
		c.SetAH(0)
		b.setCF(c, false)
	case 0x08:
		c.SetAH(0x01)
		b.setCF(c, true)
	default:
		c.SetAH(0x01) // invalid command
		b.setCF(c, true)
	}
}

// ---- INT 15h: system services ---------------------------------------------------

// int15 returns true when the call is complete (false = keep idling).
func (b *BIOS) int15(c *emulator.CPU) bool {
	switch c.AH() {
	case 0x86: // wait CX:DX microseconds
		if b.waitUntil == 0 {
			us := uint64(c.CX())<<16 | uint64(c.DX())
			b.waitUntil = c.Cycles + us*b.cpuHz()/1000000
			if b.waitUntil == c.Cycles {
				b.waitUntil = 0
				b.setCF(c, false)
				return true
			}
			return false
		}
		if c.Cycles >= b.waitUntil {
			b.waitUntil = 0
			c.SetAH(0)
			b.setCF(c, false)
			return true
		}
		return false
	case 0x24: // A20 gate
		switch c.AL() {
		case 0:
			b.M.Mem.A20 = false
		case 1:
			b.M.Mem.A20 = true
		case 2:
			if b.M.Mem.A20 {
				c.SetAL(1)
			} else {
				c.SetAL(0)
			}
		case 3:
			c.SetBX(3)
		}
		c.SetAH(0)
		b.setCF(c, false)
	case 0x88: // extended memory size
		c.SetAX(0)
		b.setCF(c, false)
	case 0xC0: // system configuration: not supported
		c.SetAH(0x80)
		b.setCF(c, true)
	case 0x4F: // keyboard intercept: pass through
		b.setCF(c, true)
	case 0x41, 0x83, 0x90, 0x91:
		c.SetAH(0)
		b.setCF(c, false)
	default:
		c.SetAH(0x86)
		b.setCF(c, true)
	}
	return true
}

func (b *BIOS) cpuHz() uint64 {
	if b.M.PIT != nil {
		return pitHz(b.M.PIT)
	}
	return 40000000
}

// Exit is called by DOS on program termination.
func (b *BIOS) Exit(code int) {
	b.Exited = true
	b.ExitCode = code
	b.M.CPU.Halted = true
	b.M.CPU.Stop()
}
