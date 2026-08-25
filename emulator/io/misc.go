package io

// A20Gate exposes the fast A20 port 92h.
type A20Gate struct {
	Flag *bool
}

func (a *A20Gate) In8(port uint16) uint8 {
	if a.Flag != nil && *a.Flag {
		return 0x02
	}
	return 0
}

func (a *A20Gate) Out8(port uint16, v uint8) {
	if a.Flag != nil {
		*a.Flag = v&0x02 != 0
	}
}

// CMOS implements the RTC ports 70h/71h with a fixed date and a minimal
// register set (enough for programs that read the time or the equipment
// byte).
type CMOS struct {
	index uint8
	Regs  [128]uint8
	Clock func() (h, m, s int)
}

func NewCMOS() *CMOS {
	c := &CMOS{}
	c.Regs[0x0A] = 0x26
	c.Regs[0x0B] = 0x02
	c.Regs[0x0D] = 0x80
	c.Regs[0x14] = 0x2D // equipment: FPU, 2 floppies, 80 col colour
	c.Regs[0x15] = 0x80 // base memory 640K
	c.Regs[0x16] = 0x02
	return c
}

func bcd(v int) uint8 { return uint8(v/10<<4 | v%10) }

func (c *CMOS) In8(port uint16) uint8 {
	if port == 0x70 {
		return c.index
	}
	switch c.index & 0x7F {
	case 0x00, 0x02, 0x04:
		if c.Clock != nil {
			h, m, s := c.Clock()
			switch c.index & 0x7F {
			case 0:
				return bcd(s)
			case 2:
				return bcd(m)
			case 4:
				return bcd(h)
			}
		}
	}
	return c.Regs[c.index&0x7F]
}

func (c *CMOS) Out8(port uint16, v uint8) {
	if port == 0x70 {
		c.index = v
		return
	}
	c.Regs[c.index&0x7F] = v
}

// DMA / misc stub: absorbs writes and returns 0 for ports programs poke
// during init (DMA controller, page registers).
type Stub struct{ Value uint8 }

func (s Stub) In8(uint16) uint8   { return s.Value }
func (s Stub) Out8(uint16, uint8) {}
