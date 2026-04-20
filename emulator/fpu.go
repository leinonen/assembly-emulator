package emulator

import "math"

// FPU represents the x87 floating-point unit state
type FPU struct {
	ST  [8]float64 // Register stack (ST[Top] = ST(0))
	Top int        // Stack top pointer (0-7)
	SW  uint16     // Status word (C0,C1,C2,C3 flags)
	CW  uint16     // Control word
}

// Init resets the FPU to a clean initial state
func (f *FPU) Init() {
	for i := range f.ST {
		f.ST[i] = 0
	}
	f.Top = 0
	f.SW = 0
	f.CW = 0x037F // round-to-nearest, 64-bit, all exceptions masked
}

// Push pushes a value onto the FPU stack (decrements Top)
func (f *FPU) Push(val float64) {
	f.Top = (f.Top - 1) & 7
	f.ST[f.Top] = val
}

// Pop pops and returns the top of stack, advancing Top
func (f *FPU) Pop() float64 {
	val := f.ST[f.Top]
	f.ST[f.Top] = math.NaN()
	f.Top = (f.Top + 1) & 7
	return val
}

// ST0 returns the value in ST(0)
func (f *FPU) ST0() float64 { return f.ST[f.Top] }

// SetST0 sets ST(0)
func (f *FPU) SetST0(v float64) { f.ST[f.Top] = v }

// STn returns ST(n)
func (f *FPU) STn(n int) float64 { return f.ST[(f.Top+n)&7] }

// SetSTn sets ST(n)
func (f *FPU) SetSTn(n int, v float64) { f.ST[(f.Top+n)&7] = v }

// Compare sets SW C0/C2/C3 by comparing st0 with other
func (f *FPU) Compare(st0, other float64) {
	f.SW &^= 0x4500 // clear C0(0x0100), C2(0x0400), C3(0x4000)
	if math.IsNaN(st0) || math.IsNaN(other) {
		f.SW |= 0x4500 // unordered
	} else if st0 < other {
		f.SW |= 0x0100 // C0=1
	} else if st0 == other {
		f.SW |= 0x4000 // C3=1
	}
	// st0 > other: all clear
}
