// Package loader loads DOS executables into emulated memory.
package loader

import (
	"errors"

	"assembly-emulator/emulator"
)

// LoadCOM places a .COM image at psp:0100 and sets up registers as DOS
// does: CS=DS=ES=SS=PSP, SP=FFFE with a zero word pushed, IP=0100.
func LoadCOM(c *emulator.CPU, psp uint16, image []byte) error {
	if len(image) > 0xFF00 {
		return errors.New("COM image larger than 65280 bytes")
	}
	base := uint32(psp)<<4 + 0x100
	for i, b := range image {
		c.Mem.Write8(base+uint32(i), b)
	}
	c.Segs[emulator.SegCS] = psp
	c.Segs[emulator.SegDS] = psp
	c.Segs[emulator.SegES] = psp
	c.Segs[emulator.SegSS] = psp
	c.Regs = [8]uint32{}
	c.SetSP(0xFFFE)
	c.Mem.Write16(uint32(psp)<<4+0xFFFE, 0)
	c.EIP = 0x100
	c.Flags = 0x0202 // IF set
	c.Halted = false
	return nil
}
