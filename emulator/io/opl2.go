package io

import "math"

// OPL2Clock is the YM3812 master clock; the chip produces one sample every
// 72 clocks (49716 Hz).
const OPL2Clock = 3579545

// OPL2 models the Yamaha YM3812 (AdLib) FM synthesiser: nine two-operator
// channels, the envelope generators, tremolo/vibrato LFOs and the two
// timers. Register writes and timer ticks are driven from CPU cycles; audio
// is rendered on demand by Render at the mixer's sample rate, with the
// envelope and LFO clocks scaled to that rate.
type OPL2 struct {
	regs [256]uint8
	addr uint8
	ch   [9]oplChannel
	op   [18]oplOperator

	wse       bool // waveform select enable (reg 01 bit 5)
	tremDeep  bool
	vibDeep   bool
	rhythm    bool
	tremPos   uint32 // 0..209 triangle position
	tremCnt   uint32 // samples until the next tremolo step
	vibPos    uint32 // 0..7
	vibCnt    uint32
	lfoStep   uint32 // samples per LFO step at the output rate
	rate      float64
	phaseUnit float64 // 2^32 * 49716 / 2^20 / rate: phase increment per F-number unit at block 0, mult 1

	attackInc [64]uint32 // 16.16 attack steps per sample by effective rate
	decayInc  [64]uint32 // 16.16 level units per sample by effective rate

	// Timers run in the CPU cycle domain.
	cpuHz              uint64
	t1Period, t2Period uint64
	t1Acc, t2Acc       uint64
	t1Cnt, t2Cnt       uint8
	t1On, t2On         bool
	t1Mask, t2Mask     bool
	flags              uint8 // status bits 7 (IRQ), 6 (T1), 5 (T2)

	// Counters for tests and diagnostics.
	Writes uint64 // register writes
	KeyOns uint64 // key-on transitions
}

type oplChannel struct {
	fnum  uint16
	block uint8
	keyOn bool
	fb    uint8
	alg   uint8
}

const (
	egOff = iota
	egAttack
	egDecay
	egSustain
	egRelease
)

type oplOperator struct {
	am, vib, egType, ksr bool
	mult                 uint8
	kslBits, tl          uint8
	ar, dr, sl, rr       uint8
	wave                 uint8

	phase   uint32
	env     int32 // attenuation 0 (loud) .. 511 (silent)
	state   uint8
	acc     uint32 // 16.16 fraction for envelope steps
	out     int32
	prevOut int32

	// Derived from the channel's F-number/block and the operator registers.
	baseInc  float64 // phase increment per F-number unit
	ksl      int32
	rAttack  uint8
	rDecay   uint8
	rRelease uint8
	slLevel  int32
}

// NewOPL2 returns a silent chip that renders at the given sample rate.
func NewOPL2(cpuHz uint64, rate int) *OPL2 {
	o := &OPL2{cpuHz: cpuHz, rate: float64(rate)}
	o.t1Period = cpuHz * 80 / 1_000_000
	o.t2Period = cpuHz * 320 / 1_000_000
	if o.t1Period == 0 {
		o.t1Period = 1
	}
	if o.t2Period == 0 {
		o.t2Period = 1
	}
	o.phaseUnit = math.Ldexp(1, 32) * (float64(OPL2Clock) / 72) / math.Ldexp(1, 20) / o.rate
	o.lfoStep = uint32(math.Round(64 * o.rate / (float64(OPL2Clock) / 72)))
	if o.lfoStep == 0 {
		o.lfoStep = 1
	}
	// Envelope rates from the YM3812 data sheet: the full 96 dB decay takes
	// 39280 ms at rate 4 and halves every four rate steps; the attack takes
	// 2826 ms at rate 4 with the same scaling. Rate 0 never moves.
	for r := 4; r < 64; r++ {
		decay := 39.28064 / math.Pow(2, float64(r-4)/4) // seconds
		o.decayInc[r] = uint32(math.Min(511*65536/(decay*o.rate), 511*65536))
		attack := 2.82624 / math.Pow(2, float64(r-4)/4)
		o.attackInc[r] = uint32(math.Min(20*65536/(attack*o.rate), 64*65536))
	}
	o.Reset()
	return o
}

// Reset silences the chip and clears every register.
func (o *OPL2) Reset() {
	o.regs = [256]uint8{}
	o.addr = 0
	o.ch = [9]oplChannel{}
	for i := range o.op {
		o.op[i] = oplOperator{env: 511, state: egOff}
	}
	o.wse, o.tremDeep, o.vibDeep, o.rhythm = false, false, false, false
	o.tremPos, o.tremCnt, o.vibPos, o.vibCnt = 0, o.lfoStep, 0, o.lfoStep
	o.t1On, o.t2On, o.t1Mask, o.t2Mask = false, false, false, false
	o.t1Acc, o.t2Acc, o.t1Cnt, o.t2Cnt = 0, 0, 0, 0
	o.flags = 0
	for i := range o.op {
		o.updateOperator(i)
	}
}

// In8 returns the status register on the even port (the odd port is
// write-only). The low bits read as on a real YM3812 so OPL3 probes fail.
func (o *OPL2) In8(port uint16) uint8 {
	if port&1 == 0 {
		return o.flags | 0x06
	}
	return 0xFF
}

// Out8 latches the register index on the even port and writes the selected
// register on the odd port.
func (o *OPL2) Out8(port uint16, v uint8) {
	if port&1 == 0 {
		o.addr = v
		return
	}
	o.write(o.addr, v)
}

func (o *OPL2) write(reg, v uint8) {
	o.Writes++
	o.regs[reg] = v
	switch {
	case reg == 0x01:
		o.wse = v&0x20 != 0
	case reg == 0x02:
		// timer 1 preset; takes effect on the next start
	case reg == 0x03:
		// timer 2 preset
	case reg == 0x04:
		if v&0x80 != 0 {
			o.flags = 0 // IRQ reset; the other bits are ignored
			return
		}
		o.t1Mask = v&0x40 != 0
		o.t2Mask = v&0x20 != 0
		if v&0x01 != 0 && !o.t1On {
			o.t1Cnt = o.regs[0x02]
			o.t1Acc = 0
		}
		if v&0x02 != 0 && !o.t2On {
			o.t2Cnt = o.regs[0x03]
			o.t2Acc = 0
		}
		o.t1On = v&0x01 != 0
		o.t2On = v&0x02 != 0
	case reg == 0xBD:
		o.tremDeep = v&0x80 != 0
		o.vibDeep = v&0x40 != 0
		o.rhythm = v&0x20 != 0 // stored only: channels 6-8 stay melodic
	case reg >= 0x20 && reg <= 0x35:
		if s := oplSlot[reg-0x20]; s >= 0 {
			op := &o.op[s]
			op.am = v&0x80 != 0
			op.vib = v&0x40 != 0
			op.egType = v&0x20 != 0
			op.ksr = v&0x10 != 0
			op.mult = v & 0x0F
			o.updateOperator(int(s))
		}
	case reg >= 0x40 && reg <= 0x55:
		if s := oplSlot[reg-0x40]; s >= 0 {
			op := &o.op[s]
			op.kslBits = v >> 6
			op.tl = v & 0x3F
			o.updateOperator(int(s))
		}
	case reg >= 0x60 && reg <= 0x75:
		if s := oplSlot[reg-0x60]; s >= 0 {
			op := &o.op[s]
			op.ar = v >> 4
			op.dr = v & 0x0F
			o.updateOperator(int(s))
		}
	case reg >= 0x80 && reg <= 0x95:
		if s := oplSlot[reg-0x80]; s >= 0 {
			op := &o.op[s]
			op.sl = v >> 4
			op.rr = v & 0x0F
			o.updateOperator(int(s))
		}
	case reg >= 0xE0 && reg <= 0xF5:
		if s := oplSlot[reg-0xE0]; s >= 0 {
			o.op[s].wave = v & 0x03
		}
	case reg >= 0xA0 && reg <= 0xA8:
		c := &o.ch[reg-0xA0]
		c.fnum = c.fnum&0x300 | uint16(v)
		o.updateChannel(int(reg - 0xA0))
	case reg >= 0xB0 && reg <= 0xB8:
		n := int(reg - 0xB0)
		c := &o.ch[n]
		c.fnum = c.fnum&0xFF | uint16(v&0x03)<<8
		c.block = (v >> 2) & 7
		key := v&0x20 != 0
		o.updateChannel(n)
		if key != c.keyOn {
			c.keyOn = key
			m := oplChanMod[n]
			if key {
				o.KeyOns++
				o.keyOn(&o.op[m])
				o.keyOn(&o.op[m+3])
			} else {
				o.keyOff(&o.op[m])
				o.keyOff(&o.op[m+3])
			}
		}
	case reg >= 0xC0 && reg <= 0xC8:
		c := &o.ch[reg-0xC0]
		c.fb = (v >> 1) & 7
		c.alg = v & 1
	}
}

func (o *OPL2) keyOn(op *oplOperator) {
	op.phase = 0
	op.acc = 0
	if op.rAttack >= 60 {
		op.env = 0
		op.state = egDecay
	} else {
		op.state = egAttack
	}
}

func (o *OPL2) keyOff(op *oplOperator) {
	if op.state != egOff {
		op.state = egRelease
		op.acc = 0
	}
}

// updateChannel recomputes the derived operator parameters after an
// F-number or block change.
func (o *OPL2) updateChannel(n int) {
	m := oplChanMod[n]
	o.updateOperator(m)
	o.updateOperator(m + 3)
}

func (o *OPL2) channelOf(slot int) (int, *oplChannel) {
	for n, m := range oplChanMod {
		if slot == m || slot == m+3 {
			return n, &o.ch[n]
		}
	}
	return 0, &o.ch[0]
}

func (o *OPL2) updateOperator(slot int) {
	op := &o.op[slot]
	_, c := o.channelOf(slot)
	op.baseInc = o.phaseUnit * float64(oplMult[op.mult]) / 2 * math.Ldexp(1, int(c.block))

	keycode := uint8(c.block)<<1 | uint8(c.fnum>>9)
	rof := keycode
	if !op.ksr {
		rof >>= 2
	}
	rate := func(r uint8) uint8 {
		if r == 0 {
			return 0
		}
		e := r*4 + rof
		if e > 63 {
			e = 63
		}
		return e
	}
	op.rAttack = rate(op.ar)
	op.rDecay = rate(op.dr)
	op.rRelease = rate(op.rr)

	if op.sl == 15 {
		op.slLevel = 511
	} else {
		op.slLevel = int32(op.sl) * 16
	}

	ksl := oplKSL[c.fnum>>6] - int32(8-c.block)<<3
	if ksl < 0 {
		ksl = 0
	}
	op.ksl = ksl >> oplKSLShift[op.kslBits]
}

// Advance runs the timers for the given number of CPU cycles.
func (o *OPL2) Advance(cycles uint64) {
	if !o.t1On && !o.t2On {
		return
	}
	if o.t1On {
		o.t1Acc += cycles
		for o.t1Acc >= o.t1Period {
			o.t1Acc -= o.t1Period
			o.t1Cnt++
			if o.t1Cnt == 0 {
				o.t1Cnt = o.regs[0x02]
				if !o.t1Mask {
					o.flags |= 0x80 | 0x40
				}
			}
		}
	}
	if o.t2On {
		o.t2Acc += cycles
		for o.t2Acc >= o.t2Period {
			o.t2Acc -= o.t2Period
			o.t2Cnt++
			if o.t2Cnt == 0 {
				o.t2Cnt = o.regs[0x03]
				if !o.t2Mask {
					o.flags |= 0x80 | 0x20
				}
			}
		}
	}
}

// NextEventCycles returns the CPU cycles until a running timer next
// overflows, or MaxUint64 when no timer is running.
func (o *OPL2) NextEventCycles() uint64 {
	n := uint64(math.MaxUint64)
	if o.t1On {
		v := (256-uint64(o.t1Cnt))*o.t1Period - o.t1Acc
		if v < n {
			n = v
		}
	}
	if o.t2On {
		v := (256-uint64(o.t2Cnt))*o.t2Period - o.t2Acc
		if v < n {
			n = v
		}
	}
	return n
}

// KeyOnMask returns a bit per channel that currently has its key on.
func (o *OPL2) KeyOnMask() uint16 {
	var m uint16
	for i := range o.ch {
		if o.ch[i].keyOn {
			m |= 1 << uint(i)
		}
	}
	return m
}

// Render adds len(dst) mono samples of chip output to dst. Each operator
// contributes at most +-4084 (13 bits), so nine channels can exceed the
// 16-bit range; the caller clamps.
func (o *OPL2) Render(dst []int32) {
	for i := range dst {
		o.stepLFO()
		trem := o.tremolo()
		var sum int32
		for n := range o.ch {
			c := &o.ch[n]
			m := &o.op[oplChanMod[n]]
			k := &o.op[oplChanMod[n]+3]
			if m.state == egOff && k.state == egOff {
				continue
			}
			var fb int32
			if c.fb != 0 {
				fb = (m.out + m.prevOut) >> (9 - c.fb)
			}
			mout := o.operator(m, c, fb, trem)
			if c.alg == 1 {
				sum += mout + o.operator(k, c, 0, trem)
			} else {
				sum += o.operator(k, c, mout, trem)
			}
		}
		dst[i] += sum
	}
}

func (o *OPL2) stepLFO() {
	o.tremCnt--
	if o.tremCnt == 0 {
		o.tremCnt = o.lfoStep
		o.tremPos++
		if o.tremPos >= 210 {
			o.tremPos = 0
		}
	}
	o.vibCnt--
	if o.vibCnt == 0 {
		o.vibCnt = o.lfoStep * 16
		o.vibPos = (o.vibPos + 1) & 7
	}
}

// tremolo returns the current AM attenuation in 0.1875 dB units.
func (o *OPL2) tremolo() int32 {
	t := int32(o.tremPos)
	if t >= 105 {
		t = 210 - t
	}
	if o.tremDeep {
		return t >> 2
	}
	return t >> 4
}

// vibrato returns the F-number offset for the current LFO position.
func (o *OPL2) vibrato(fnum uint16) int32 {
	r := int32(fnum>>7) & 7
	switch o.vibPos & 3 {
	case 0:
		return 0
	case 1, 3:
		r >>= 1
	}
	if !o.vibDeep {
		r >>= 1
	}
	if o.vibPos&4 != 0 {
		return -r
	}
	return r
}

// operator advances one operator by a sample and returns its output.
func (o *OPL2) operator(op *oplOperator, c *oplChannel, mod int32, trem int32) int32 {
	// Phase generator.
	fnum := int32(c.fnum)
	if op.vib {
		fnum += o.vibrato(c.fnum)
	}
	op.phase += uint32(op.baseInc * float64(fnum))
	idx := op.phase>>22 + uint32(mod)

	// Envelope generator.
	switch op.state {
	case egAttack:
		op.acc += o.attackInc[op.rAttack]
		for op.acc >= 1<<16 && op.env > 0 {
			op.acc -= 1 << 16
			op.env -= op.env>>2 + 1
		}
		if op.env <= 0 {
			op.env = 0
			op.state = egDecay
			op.acc = 0
		}
	case egDecay:
		op.acc += o.decayInc[op.rDecay]
		op.env += int32(op.acc >> 16)
		op.acc &= 0xFFFF
		if op.env >= op.slLevel {
			op.env = op.slLevel
			op.state = egSustain
			op.acc = 0
		}
	case egSustain:
		if !op.egType {
			op.acc += o.decayInc[op.rRelease]
			op.env += int32(op.acc >> 16)
			op.acc &= 0xFFFF
			if op.env >= 511 {
				op.env = 511
				op.state = egOff
			}
		}
	case egRelease:
		op.acc += o.decayInc[op.rRelease]
		op.env += int32(op.acc >> 16)
		op.acc &= 0xFFFF
		if op.env >= 511 {
			op.env = 511
			op.state = egOff
		}
	}

	att := op.env + int32(op.tl)<<2 + op.ksl
	if op.am {
		att += trem
	}
	if att > 511 {
		att = 511
	}
	wave := op.wave
	if !o.wse {
		wave = 0
	}
	v := int32(0)
	if att < 511 {
		v = oplWave(idx, att, wave)
	}
	op.prevOut = op.out
	op.out = v
	return v
}
