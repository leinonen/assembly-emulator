// Package io implements the PC I/O port devices: PIT, PIC, keyboard
// controller, speaker and the VGA register interface.
package io

// Device handles a range of I/O ports.
type Device interface {
	In8(port uint16) uint8
	Out8(port uint16, v uint8)
}

// Bus routes port accesses to registered devices. Unmapped ports read as
// 0xFF and ignore writes, like an open ISA bus.
type Bus struct {
	devices map[uint16]Device
}

func NewBus() *Bus {
	return &Bus{devices: map[uint16]Device{}}
}

// Map assigns dev to every port in [lo, hi].
func (b *Bus) Map(lo, hi uint16, dev Device) {
	for p := int(lo); p <= int(hi); p++ {
		b.devices[uint16(p)] = dev
	}
}

func (b *Bus) In8(port uint16) uint8 {
	if d, ok := b.devices[port]; ok {
		return d.In8(port)
	}
	return 0xFF
}

func (b *Bus) Out8(port uint16, v uint8) {
	if d, ok := b.devices[port]; ok {
		d.Out8(port, v)
	}
}

func (b *Bus) In16(port uint16) uint16 {
	return uint16(b.In8(port)) | uint16(b.In8(port+1))<<8
}

func (b *Bus) Out16(port uint16, v uint16) {
	b.Out8(port, uint8(v))
	b.Out8(port+1, uint8(v>>8))
}

func (b *Bus) In32(port uint16) uint32 {
	return uint32(b.In16(port)) | uint32(b.In16(port+2))<<16
}

func (b *Bus) Out32(port uint16, v uint32) {
	b.Out16(port, uint16(v))
	b.Out16(port+2, uint16(v>>16))
}
