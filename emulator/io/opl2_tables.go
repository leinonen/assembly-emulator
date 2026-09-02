package io

import "math"

// Lookup tables for the OPL2 synthesiser. They are generated from the
// formulas the chip's ROMs implement (a quarter log-sine and an exponent
// table with 8 fractional bits) rather than copied from a ROM dump, so the
// output is close to but not bit-identical with a real YM3812.
var (
	// oplLogSin[i] = -log2(sin((i+0.5)*pi/512)) * 256 for the first quarter wave.
	oplLogSin [256]uint16
	// oplExp[i] = (2^(i/256) - 1) * 1024.
	oplExp [256]uint16
)

// oplSlot maps a register offset (0x00-0x15) to an operator index 0-17, or
// -1 for the unused gaps.
var oplSlot = [32]int8{
	0, 1, 2, 3, 4, 5, -1, -1,
	6, 7, 8, 9, 10, 11, -1, -1,
	12, 13, 14, 15, 16, 17, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1,
}

// oplChanMod is the modulator operator of each channel; the carrier is +3.
var oplChanMod = [9]int{0, 1, 2, 6, 7, 8, 12, 13, 14}

// oplMult is the frequency multiplier for register 20h bits 0-3, times 2.
var oplMult = [16]int{1, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 20, 24, 24, 30, 30}

// oplKSL is the key scale level ROM, indexed by the top four bits of the
// F-number, in 0.1875 dB units before the per-operator shift.
var oplKSL = [16]int32{0, 32, 40, 45, 48, 51, 53, 55, 56, 57, 59, 60, 61, 62, 63, 64}

// oplKSLShift converts the KSL ROM value for the 0 / 1.5 / 3 / 6 dB per
// octave settings of register 40h bits 6-7.
var oplKSLShift = [4]uint{8, 1, 2, 0}

func init() {
	for i := 0; i < 256; i++ {
		s := math.Sin((float64(i) + 0.5) * math.Pi / 512)
		oplLogSin[i] = uint16(math.Round(-math.Log2(s) * 256))
		oplExp[i] = uint16(math.Round((math.Pow(2, float64(i)/256) - 1) * 1024))
	}
}

// oplWave computes one operator sample: phase is a 10-bit index, level the
// attenuation in 0.1875 dB units (0-511), wave the waveform 0-3. The result
// is a 13-bit signed value like the chip's DAC input.
func oplWave(phase uint32, level int32, wave uint8) int32 {
	phase &= 0x3FF
	neg := false
	var ls uint32
	switch wave {
	case 0: // full sine
		neg = phase&0x200 != 0
		fallthrough
	case 2: // absolute sine
		if phase&0x100 != 0 {
			ls = uint32(oplLogSin[(phase&0xFF)^0xFF])
		} else {
			ls = uint32(oplLogSin[phase&0xFF])
		}
	case 1: // half sine
		if phase&0x200 != 0 {
			return 0
		}
		if phase&0x100 != 0 {
			ls = uint32(oplLogSin[(phase&0xFF)^0xFF])
		} else {
			ls = uint32(oplLogSin[phase&0xFF])
		}
	default: // quarter pulses
		if phase&0x100 != 0 {
			return 0
		}
		ls = uint32(oplLogSin[phase&0xFF])
	}
	l := ls + uint32(level)<<3
	if l > 0x1FFF {
		l = 0x1FFF
	}
	v := int32((uint32(oplExp[(l&0xFF)^0xFF])|0x400)<<1) >> (l >> 8)
	if neg {
		return -v
	}
	return v
}
