package io

// PIC models a pair of cascaded 8259A interrupt controllers well enough
// for DOS software: IRQ masking, EOI, and vector lookup. IRQ0-7 map to
// vectors 08h-0Fh and IRQ8-15 to 70h-77h.
type PIC struct {
	imr     [2]uint8 // interrupt mask registers
	irr     [2]uint8 // pending requests
	isr     [2]uint8 // in service
	icw     [2][4]uint8
	icwStep [2]int
	base    [2]uint8
	readISR [2]bool
}

func NewPIC() *PIC {
	p := &PIC{}
	p.base = [2]uint8{0x08, 0x70}
	p.imr = [2]uint8{0xB8, 0x8F} // typical DOS state: timer, kbd, cascade, fdc on
	return p
}

// Raise asserts an IRQ line (edge).
func (p *PIC) Raise(irq int) {
	if irq < 8 {
		p.irr[0] |= 1 << uint(irq)
	} else {
		p.irr[1] |= 1 << uint(irq-8)
		p.irr[0] |= 1 << 2 // cascade
	}
}

// Pending returns the vector of the highest-priority unmasked pending IRQ
// that is not blocked by an in-service interrupt, or -1.
func (p *PIC) Pending() int {
	for i := 0; i < 8; i++ {
		bit := uint8(1) << uint(i)
		if p.irr[0]&bit != 0 && p.imr[0]&bit == 0 {
			if p.isr[0]&(bit-1) != 0 {
				return -1 // higher priority in service
			}
			if i == 2 {
				for j := 0; j < 8; j++ {
					b2 := uint8(1) << uint(j)
					if p.irr[1]&b2 != 0 && p.imr[1]&b2 == 0 {
						if p.isr[1]&(b2-1) != 0 {
							return -1
						}
						return int(p.base[1]) + j
					}
				}
				p.irr[0] &^= bit
				continue
			}
			return int(p.base[0]) + i
		}
		if p.isr[0]&bit != 0 {
			return -1
		}
	}
	return -1
}

// Acknowledge moves the IRQ for vector v from pending to in-service.
func (p *PIC) Acknowledge(v int) {
	if v >= int(p.base[0]) && v < int(p.base[0])+8 {
		i := v - int(p.base[0])
		p.irr[0] &^= 1 << uint(i)
		p.isr[0] |= 1 << uint(i)
	} else if v >= int(p.base[1]) && v < int(p.base[1])+8 {
		i := v - int(p.base[1])
		p.irr[1] &^= 1 << uint(i)
		p.isr[1] |= 1 << uint(i)
		p.isr[0] |= 1 << 2
	}
}

func (p *PIC) In8(port uint16) uint8 {
	n := 0
	if port >= 0xA0 {
		n = 1
	}
	if port&1 == 0 {
		if p.readISR[n] {
			return p.isr[n]
		}
		return p.irr[n]
	}
	return p.imr[n]
}

func (p *PIC) Out8(port uint16, v uint8) {
	n := 0
	if port >= 0xA0 {
		n = 1
	}
	if port&1 == 0 {
		switch {
		case v&0x10 != 0: // ICW1
			p.icwStep[n] = 1
			p.icw[n][0] = v
		case v&0x08 == 0: // OCW2
			if v&0x20 != 0 { // EOI
				if v&0x40 != 0 { // specific
					p.isr[n] &^= 1 << (v & 7)
				} else {
					// non-specific: clear highest priority in-service
					for i := 0; i < 8; i++ {
						if p.isr[n]&(1<<uint(i)) != 0 {
							p.isr[n] &^= 1 << uint(i)
							break
						}
					}
				}
			}
		default: // OCW3
			if v&0x02 != 0 {
				p.readISR[n] = v&0x01 != 0
			}
		}
		return
	}
	// Odd port: ICW2-4 during init, else OCW1 (mask).
	switch p.icwStep[n] {
	case 1:
		p.base[n] = v & 0xF8
		p.icwStep[n] = 2
	case 2:
		if p.icw[n][0]&0x02 == 0 { // cascade mode: ICW3 follows
			p.icwStep[n] = 3
		} else if p.icw[n][0]&0x01 != 0 {
			p.icwStep[n] = 4
		} else {
			p.icwStep[n] = 0
		}
	case 3:
		if p.icw[n][0]&0x01 != 0 {
			p.icwStep[n] = 4
		} else {
			p.icwStep[n] = 0
		}
	case 4:
		p.icwStep[n] = 0
	default:
		p.imr[n] = v
	}
}
