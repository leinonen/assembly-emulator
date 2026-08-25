package io

import "sync"

// Keyboard models the 8042 keyboard controller data/status ports with a
// thread-safe scancode queue. Host code pushes scancode-set-1 make/break
// codes; each byte raises IRQ1 once the previous byte has been read.
type Keyboard struct {
	mu      sync.Mutex
	queue   []uint8
	pic     *PIC
	current uint8
	full    bool // output buffer full (byte waiting in 60h)
	cmd     uint8
	a20     *bool
	// Typematic repeat driven by the emulator clock.
	repeatCode  uint8
	repeatDelay int64 // cycles until next repeat, <0 = none
	cpuHz       uint64
}

func NewKeyboard(pic *PIC, cpuHz uint64) *Keyboard {
	return &Keyboard{pic: pic, cpuHz: cpuHz, repeatDelay: -1}
}

// SetA20 hooks the A20 gate for the 8042 output-port command.
func (k *Keyboard) SetA20(flag *bool) { k.a20 = flag }

// Push queues raw scancode bytes (E0-prefixed codes are two bytes).
func (k *Keyboard) Push(codes ...uint8) {
	k.mu.Lock()
	k.queue = append(k.queue, codes...)
	k.mu.Unlock()
	k.pump()
}

// KeyDown / KeyUp push make/break codes and manage typematic repeat.
func (k *Keyboard) KeyDown(code uint8, extended bool) {
	if extended {
		k.Push(0xE0, code)
	} else {
		k.Push(code)
	}
	k.mu.Lock()
	k.repeatCode = code
	if extended {
		k.repeatCode |= 0x80 // marker, cleared when emitting
	}
	k.repeatDelay = int64(k.cpuHz / 4) // 250 ms initial delay
	k.mu.Unlock()
}

func (k *Keyboard) KeyUp(code uint8, extended bool) {
	k.mu.Lock()
	rc := k.repeatCode
	if extended {
		rc &^= 0x80
	}
	if rc == code {
		k.repeatDelay = -1
	}
	k.mu.Unlock()
	if extended {
		k.Push(0xE0, code|0x80)
	} else {
		k.Push(code | 0x80)
	}
}

// Advance drives typematic repeat (30 Hz).
func (k *Keyboard) Advance(cycles uint64) {
	k.mu.Lock()
	if k.repeatDelay < 0 {
		k.mu.Unlock()
		return
	}
	k.repeatDelay -= int64(cycles)
	if k.repeatDelay > 0 {
		k.mu.Unlock()
		return
	}
	k.repeatDelay = int64(k.cpuHz / 30)
	code := k.repeatCode
	k.mu.Unlock()
	if code&0x80 != 0 {
		k.Push(0xE0, code&0x7F)
	} else {
		k.Push(code)
	}
}

// pump moves the next queued byte into the output buffer and raises IRQ1.
func (k *Keyboard) pump() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.full || len(k.queue) == 0 {
		return
	}
	k.current = k.queue[0]
	k.queue = k.queue[1:]
	k.full = true
	k.pic.Raise(1)
}

func (k *Keyboard) In8(port uint16) uint8 {
	switch port {
	case 0x60:
		k.mu.Lock()
		v := k.current
		k.full = false
		k.mu.Unlock()
		k.pump()
		return v
	case 0x64:
		k.mu.Lock()
		defer k.mu.Unlock()
		var s uint8 = 0x10 // keyboard enabled
		if k.full {
			s |= 0x01
		}
		return s
	}
	return 0xFF
}

func (k *Keyboard) Out8(port uint16, v uint8) {
	switch port {
	case 0x64:
		k.cmd = v
		switch v {
		case 0xD0: // read output port
			k.mu.Lock()
			out := uint8(0x01)
			if k.a20 != nil && *k.a20 {
				out |= 0x02
			}
			k.current = out
			k.full = true
			k.mu.Unlock()
		}
	case 0x60:
		switch k.cmd {
		case 0xD1: // write output port
			if k.a20 != nil {
				*k.a20 = v&0x02 != 0
			}
			k.cmd = 0
		default:
			// Keyboard commands (set LEDs, typematic, ...) get an ACK.
			k.cmd = 0
			k.Push(0xFA)
		}
	}
}

// HasPending reports whether a byte is waiting or queued (for tests).
func (k *Keyboard) HasPending() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.full || len(k.queue) > 0
}
