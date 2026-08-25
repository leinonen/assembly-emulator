package bios

import "assembly-emulator/emulator"

// ---- INT 09h: keyboard interrupt -> BIOS buffer ----------------------------------

func (b *BIOS) keyboardIRQ() {
	mem := b.M.Mem
	code := b.M.Bus.In8(0x60)
	defer b.M.Bus.Out8(0x20, 0x20) // EOI

	flags3 := mem.Read8(bdaKbFlags3)
	if code == 0xE0 {
		mem.Write8(bdaKbFlags3, flags3|kbE0Prefix)
		return
	}
	if code == 0xE1 { // pause: ignore sequence
		return
	}
	ext := flags3&kbE0Prefix != 0
	mem.Write8(bdaKbFlags3, flags3&^kbE0Prefix)
	if code == 0xFA {
		return // ACK
	}
	release := code&0x80 != 0
	code &= 0x7F
	f1 := mem.Read8(bdaKbFlags1)
	f2 := mem.Read8(bdaKbFlags2)

	// Modifier keys.
	switch code {
	case 0x2A: // left shift
		f1 = setBit(f1, kbLShift, !release)
	case 0x36: // right shift
		f1 = setBit(f1, kbRShift, !release)
	case 0x1D: // ctrl
		f1 = setBit(f1, kbCtrl, !release)
		if ext {
			f2 = setBit(f2, 0x04, !release)
		} else {
			f2 = setBit(f2, kbLCtrl, !release)
		}
	case 0x38: // alt
		f1 = setBit(f1, kbAlt, !release)
		if ext {
			f2 = setBit(f2, 0x08, !release)
		} else {
			f2 = setBit(f2, kbLAlt, !release)
		}
	case 0x3A: // caps lock
		if !release {
			f1 ^= kbCaps
		}
	case 0x45: // num lock
		if !release && !ext {
			f1 ^= kbNum
		}
	case 0x46: // scroll lock
		if !release {
			f1 ^= kbScroll
		}
	case 0x52: // insert
		if !release && !ext {
			f1 ^= kbInsert
		}
	}
	mem.Write8(bdaKbFlags1, f1)
	mem.Write8(bdaKbFlags2, f2)
	switch code {
	case 0x2A, 0x36, 0x1D, 0x38, 0x3A, 0x45, 0x46:
		return
	}
	if release {
		return
	}

	shift := f1&(kbLShift|kbRShift) != 0
	ctrl := f1&kbCtrl != 0
	alt := f1&kbAlt != 0
	caps := f1&kbCaps != 0
	num := f1&kbNum != 0

	// Ctrl+Break -> INT 1Bh flag; Ctrl+Alt+Del ignored.
	if ctrl && (code == 0x46 || (ext && code == 0x46)) {
		mem.Write8(bdaBreak, 0x80)
		b.pushKey(0x0000)
		return
	}

	var ascii uint8
	var scan uint8 = code
	if int(code) < len(keyTable) {
		e := keyTable[code]
		switch {
		case alt:
			ascii = e.alt
			if ascii == 0 && code >= 0x02 && code <= 0x0D {
				// Alt+digit row: scan code with no ASCII
				scan = code + 0x76
			}
		case ctrl:
			ascii = e.ctrl
			if code == 0x03 { // Ctrl+2 = NUL
				scan = 0x03
			}
		case shift:
			ascii = e.shift
		default:
			ascii = e.normal
		}
		if caps && ascii != 0 {
			if ascii >= 'a' && ascii <= 'z' && !shift {
				ascii -= 0x20
			} else if ascii >= 'A' && ascii <= 'Z' && shift {
				ascii += 0x20
			}
		}
	}

	// Function keys and navigation keys produce scan<<8 with ASCII 0
	// (E0 for extended keys, following the enhanced BIOS convention).
	switch {
	case code >= 0x3B && code <= 0x44: // F1-F10
		ascii = 0
		switch {
		case shift:
			scan = code + 0x19
		case ctrl:
			scan = code + 0x23
		case alt:
			scan = code + 0x2D
		}
	case code == 0x57 || code == 0x58: // F11/F12
		ascii = 0
		scan = code + 0x2E
		switch {
		case shift:
			scan = code + 0x30
		case ctrl:
			scan = code + 0x32
		case alt:
			scan = code + 0x34
		}
	case code >= 0x47 && code <= 0x53: // keypad / nav cluster
		if ext || !num {
			if ext {
				ascii = 0xE0
			} else {
				ascii = 0
			}
			if ctrl {
				switch code {
				case 0x47:
					scan = 0x77
				case 0x48:
					scan = 0x8D
				case 0x49:
					scan = 0x84
				case 0x4B:
					scan = 0x73
				case 0x4D:
					scan = 0x74
				case 0x4F:
					scan = 0x75
				case 0x50:
					scan = 0x91
				case 0x51:
					scan = 0x76
				case 0x52:
					scan = 0x92
				case 0x53:
					scan = 0x93
				}
				ascii = 0
			}
		} else if shift && !ext {
			// shifted keypad with numlock: navigation
			ascii = 0
		}
		if ext && code == 0x35 { // keypad /
			ascii = '/'
		}
	case ext && code == 0x1C: // keypad enter
		ascii = 0x0D
		if ctrl {
			ascii = 0x0A
		}
	case ext && code == 0x35:
		ascii = '/'
	}
	if ascii == 0 && scan == 0 {
		return
	}
	b.pushKey(uint16(scan)<<8 | uint16(ascii))
}

func setBit(v, bit uint8, on bool) uint8 {
	if on {
		return v | bit
	}
	return v &^ bit
}

// pushKey inserts a key into the 16-entry BIOS ring buffer.
func (b *BIOS) pushKey(key uint16) bool {
	mem := b.M.Mem
	head := mem.Read16(bdaKbHead)
	tail := mem.Read16(bdaKbTail)
	start := mem.Read16(bdaKbBufStart)
	end := mem.Read16(bdaKbBufEndP)
	next := tail + 2
	if next >= end {
		next = start
	}
	if next == head {
		return false // full
	}
	mem.Write16(0x400+uint32(tail), key)
	mem.Write16(bdaKbTail, next)
	return true
}

// peekKey returns the next key without removing it.
func (b *BIOS) peekKey() (uint16, bool) {
	mem := b.M.Mem
	head := mem.Read16(bdaKbHead)
	tail := mem.Read16(bdaKbTail)
	if head == tail {
		return 0, false
	}
	return mem.Read16(0x400 + uint32(head)), true
}

func (b *BIOS) popKey() (uint16, bool) {
	mem := b.M.Mem
	k, ok := b.peekKey()
	if !ok {
		return 0, false
	}
	head := mem.Read16(bdaKbHead) + 2
	if head >= mem.Read16(bdaKbBufEndP) {
		head = mem.Read16(bdaKbBufStart)
	}
	mem.Write16(bdaKbHead, head)
	return k, true
}

// ReadKey is used by DOS console input: returns the next key or false.
func (b *BIOS) ReadKey() (uint16, bool) { return b.popKey() }
func (b *BIOS) PeekKey() (uint16, bool) { return b.peekKey() }

// ---- INT 16h ---------------------------------------------------------------------

// int16 returns true when the call is complete.
func (b *BIOS) int16(c *emulator.CPU) bool {
	mem := b.M.Mem
	switch c.AH() {
	case 0x00, 0x10:
		k, ok := b.popKey()
		if !ok {
			return false
		}
		if c.AH() == 0x00 && k&0xFF == 0xE0 {
			k &= 0xFF00 // legacy: extended keys report ASCII 0
		}
		c.SetAX(k)
	case 0x01, 0x11:
		k, ok := b.peekKey()
		if !ok {
			b.setZF(c, true)
		} else {
			if c.AH() == 0x01 && k&0xFF == 0xE0 {
				k &= 0xFF00
			}
			c.SetAX(k)
			b.setZF(c, false)
		}
	case 0x02:
		c.SetAL(mem.Read8(bdaKbFlags1))
	case 0x12:
		c.SetAL(mem.Read8(bdaKbFlags1))
		c.SetAH(mem.Read8(bdaKbFlags2))
	case 0x03: // typematic rate: accept
	case 0x05: // store key
		if b.pushKey(uint16(c.CH())<<8 | uint16(c.CL())) {
			c.SetAL(0)
		} else {
			c.SetAL(1)
		}
	default:
		// unsupported: leave registers
	}
	return true
}
