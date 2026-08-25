package bios

import "assembly-emulator/emulator/io"

// pitHz returns the CPU clock the PIT was configured with. The PIT does
// not expose it directly; we keep a package-level registry.
var pitClocks = map[*io.PIT]uint64{}

// RegisterClock records the CPU frequency for a PIT (called by machine).
func RegisterClock(p *io.PIT, hz uint64) { pitClocks[p] = hz }

func pitHz(p *io.PIT) uint64 {
	if hz, ok := pitClocks[p]; ok {
		return hz
	}
	return 40000000
}
