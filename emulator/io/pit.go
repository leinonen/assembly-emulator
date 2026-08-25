package io

// PITClock is the 8254 input clock in Hz.
const PITClock = 1193182

// PIT models the 8254 programmable interval timer. Channel 0 drives IRQ0,
// channel 2 drives the speaker. Time is advanced by CPU cycles through
// Advance; the conversion to PIT ticks uses fixed-point accumulation so
// the emulation is deterministic.
type PIT struct {
	ch      [3]pitChannel
	pic     *PIC
	cpuHz   uint64
	accum   uint64 // cpu cycles not yet converted to PIT ticks
	Ticks   uint64 // total PIT ticks elapsed
	gate2   bool
	spkData bool
}

type pitChannel struct {
	count   uint16 // current counter
	reload  uint16
	mode    uint8
	rw      uint8 // 1=lsb 2=msb 3=lsb then msb
	bcd     bool
	latch   uint16
	latched bool
	wrLow   bool // expecting high byte of a 16-bit write
	rdLow   bool // expecting to return high byte
	output  bool
	armed   bool // counting
	nullCnt bool
}

func NewPIT(pic *PIC, cpuHz uint64) *PIT {
	p := &PIT{pic: pic, cpuHz: cpuHz}
	for i := range p.ch {
		p.ch[i].reload = 0
		p.ch[i].count = 0
		p.ch[i].mode = 3
		p.ch[i].rw = 3
		p.ch[i].armed = true
	}
	return p
}

// SetCPUHz changes the CPU clock used to derive PIT ticks.
func (p *PIT) SetCPUHz(hz uint64) { p.cpuHz = hz }

// Advance accounts for CPU cycles and fires channel 0 as needed.
func (p *PIT) Advance(cycles uint64) {
	p.accum += cycles * PITClock
	ticks := p.accum / p.cpuHz
	p.accum -= ticks * p.cpuHz
	if ticks == 0 {
		return
	}
	p.Ticks += ticks
	for i := range p.ch {
		p.tick(i, ticks)
	}
}

// NextEventCycles returns an estimate of CPU cycles until channel 0
// fires next (used to fast-forward while halted).
func (p *PIT) NextEventCycles() uint64 {
	c := &p.ch[0]
	remaining := uint64(c.count)
	if remaining == 0 {
		remaining = 0x10000
	}
	return remaining * p.cpuHz / PITClock
}

func (p *PIT) tick(i int, n uint64) {
	c := &p.ch[i]
	if !c.armed {
		return
	}
	period := uint64(c.reload)
	if period == 0 {
		period = 0x10000
	}
	for n > 0 {
		cur := uint64(c.count)
		if cur == 0 {
			cur = 0x10000
		}
		if n < cur {
			c.count = uint16(cur - n)
			// Mode 3 output toggles at half period; good enough for the speaker.
			if c.mode == 3 || c.mode == 2 {
				c.output = uint64(c.count) > period/2
			}
			return
		}
		n -= cur
		c.count = uint16(period) // reload (0x10000 wraps to 0)
		switch c.mode {
		case 0: // interrupt on terminal count
			c.output = true
			c.armed = false
			if i == 0 {
				p.pic.Raise(0)
			}
			return
		default: // rate generator / square wave
			if i == 0 {
				p.pic.Raise(0)
			}
		}
	}
}

func (p *PIT) In8(port uint16) uint8 {
	switch port {
	case 0x40, 0x41, 0x42:
		c := &p.ch[port-0x40]
		var v uint16
		if c.latched {
			v = c.latch
		} else {
			v = c.count
		}
		switch c.rw {
		case 1:
			c.latched = false
			return uint8(v)
		case 2:
			c.latched = false
			return uint8(v >> 8)
		default:
			if !c.rdLow {
				c.rdLow = true
				return uint8(v)
			}
			c.rdLow = false
			c.latched = false
			return uint8(v >> 8)
		}
	case 0x61:
		v := uint8(0)
		if p.gate2 {
			v |= 1
		}
		if p.spkData {
			v |= 2
		}
		if p.ch[2].output {
			v |= 0x20
		}
		// bit 4 toggles with refresh; use tick parity
		if p.Ticks&8 != 0 {
			v |= 0x10
		}
		return v
	}
	return 0xFF
}

func (p *PIT) Out8(port uint16, v uint8) {
	switch port {
	case 0x40, 0x41, 0x42:
		c := &p.ch[port-0x40]
		switch c.rw {
		case 1:
			c.reload = uint16(v)
			c.load(i16(port))
		case 2:
			c.reload = uint16(v) << 8
			c.load(i16(port))
		default:
			if !c.wrLow {
				c.reload = c.reload&0xFF00 | uint16(v)
				c.wrLow = true
				if c.mode == 0 {
					c.armed = false
				}
			} else {
				c.reload = c.reload&0x00FF | uint16(v)<<8
				c.wrLow = false
				c.load(i16(port))
			}
		}
	case 0x43:
		ch := v >> 6
		if ch == 3 { // read-back command
			for i := 0; i < 3; i++ {
				if v&(0x02<<uint(i)) != 0 && v&0x20 == 0 {
					p.ch[i].latch = p.ch[i].count
					p.ch[i].latched = true
				}
			}
			return
		}
		c := &p.ch[ch]
		rw := (v >> 4) & 3
		if rw == 0 { // counter latch
			if !c.latched {
				c.latch = c.count
				c.latched = true
			}
			return
		}
		c.rw = rw
		c.mode = (v >> 1) & 7
		if c.mode > 5 {
			c.mode -= 4
		}
		c.bcd = v&1 != 0
		c.wrLow = false
		c.rdLow = false
		c.output = c.mode != 0
	case 0x61:
		p.gate2 = v&1 != 0
		p.spkData = v&2 != 0
	}
}

func i16(port uint16) int { return int(port - 0x40) }

func (c *pitChannel) load(_ int) {
	c.count = c.reload
	c.armed = true
	if c.mode == 0 {
		c.output = false
	}
}

// SpeakerOn reports whether the speaker is being driven by channel 2.
func (p *PIT) SpeakerOn() bool { return p.gate2 && p.spkData }

// SpeakerFreq returns the channel 2 frequency in Hz (0 if not running).
func (p *PIT) SpeakerFreq() float64 {
	r := uint64(p.ch[2].reload)
	if r == 0 {
		r = 0x10000
	}
	return float64(PITClock) / float64(r)
}
