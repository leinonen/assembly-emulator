package io

import (
	"sync"

	"assembly-emulator/font"
)

// VGA models the register-level interface of a VGA card: sequencer,
// graphics controller, attribute controller, CRTC, DAC, and the input
// status register with simulated vertical/horizontal retrace timing.
// Video memory is organised as four 64K planes.
type VGA struct {
	mu sync.Mutex

	Planes [4][65536]byte
	latch  [4]byte

	miscOut uint8
	seqIdx  uint8
	Seq     [8]uint8
	gcIdx   uint8
	GC      [16]uint8
	attrIdx uint8
	attrFF  bool // attribute controller flip-flop: false = index
	Attr    [32]uint8
	crtcIdx uint8
	CRTC    [32]uint8

	DAC         [256][3]uint8 // 6-bit components
	dacWrIdx    uint8
	dacRdIdx    uint8
	dacState    uint8
	dacMask     uint8
	dacReadMode bool

	// BIOS-visible mode number (set by INT 10h); used by the renderer.
	Mode uint8

	// CRT timing (in CPU cycles).
	cpuHz      uint64
	framePos   uint64
	frameLen   uint64
	lineLen    uint64
	vblankFrom uint64
	Frames     uint64
	OnVBlank   func()
}

func NewVGA(cpuHz uint64) *VGA {
	v := &VGA{cpuHz: cpuHz}
	v.dacMask = 0xFF
	v.setTiming(70)
	v.SetMode(3)
	return v
}

func (v *VGA) SetCPUHz(hz uint64) {
	v.cpuHz = hz
	v.setTiming(70)
}

// setTiming configures the frame length for the given refresh rate.
// Vertical retrace occupies the last ~8% of the frame; each of 449
// scanlines has a horizontal blank in its last 20%.
func (v *VGA) setTiming(refresh uint64) {
	v.frameLen = v.cpuHz / refresh
	v.lineLen = v.frameLen / 449
	v.vblankFrom = v.frameLen * 92 / 100
}

// Advance moves the CRT beam by the given number of CPU cycles.
func (v *VGA) Advance(cycles uint64) {
	before := v.framePos
	v.framePos += cycles
	if before < v.vblankFrom && v.framePos >= v.vblankFrom {
		v.Frames++
		if v.OnVBlank != nil {
			v.OnVBlank()
		}
	}
	if v.framePos >= v.frameLen {
		v.framePos %= v.frameLen
		if v.framePos >= v.vblankFrom {
			v.Frames++
		}
	}
}

// InVBlank reports whether the beam is in vertical retrace.
func (v *VGA) InVBlank() bool { return v.framePos >= v.vblankFrom }

// CyclesToVBlank returns cycles until the next vertical retrace begins.
func (v *VGA) CyclesToVBlank() uint64 {
	if v.framePos < v.vblankFrom {
		return v.vblankFrom - v.framePos
	}
	return v.frameLen - v.framePos + v.vblankFrom
}

func (v *VGA) inHBlank() bool {
	if v.lineLen == 0 {
		return false
	}
	return v.framePos%v.lineLen >= v.lineLen*80/100
}

// Lock/Unlock guard Planes and DAC for the renderer.
func (v *VGA) Lock()   { v.mu.Lock() }
func (v *VGA) Unlock() { v.mu.Unlock() }

// ---- register I/O ------------------------------------------------------

func (v *VGA) In8(port uint16) uint8 {
	switch port {
	case 0x3C0:
		return v.attrIdx
	case 0x3C1:
		return v.Attr[v.attrIdx&0x1F]
	case 0x3C2:
		return 0x10 // no switch sense
	case 0x3C4:
		return v.seqIdx
	case 0x3C5:
		return v.Seq[v.seqIdx&7]
	case 0x3C6:
		return v.dacMask
	case 0x3C7:
		if v.dacReadMode {
			return 0x03
		}
		return 0x00
	case 0x3C8:
		return v.dacWrIdx
	case 0x3C9:
		c := v.DAC[v.dacRdIdx][v.dacState]
		v.dacState++
		if v.dacState == 3 {
			v.dacState = 0
			v.dacRdIdx++
		}
		return c
	case 0x3CA:
		return 0
	case 0x3CC:
		return v.miscOut
	case 0x3CE:
		return v.gcIdx
	case 0x3CF:
		return v.GC[v.gcIdx&0xF]
	case 0x3D4, 0x3B4:
		return v.crtcIdx
	case 0x3D5, 0x3B5:
		return v.CRTC[v.crtcIdx&0x1F]
	case 0x3DA, 0x3BA:
		v.attrFF = false
		var s uint8
		if v.InVBlank() {
			s |= 0x08 | 0x01
		} else if v.inHBlank() {
			s |= 0x01
		}
		return s
	}
	return 0xFF
}

func (v *VGA) Out8(port uint16, val uint8) {
	switch port {
	case 0x3C0:
		if !v.attrFF {
			v.attrIdx = val
		} else {
			v.mu.Lock()
			v.Attr[v.attrIdx&0x1F] = val
			v.mu.Unlock()
		}
		v.attrFF = !v.attrFF
	case 0x3C2:
		v.miscOut = val
	case 0x3C4:
		v.seqIdx = val
	case 0x3C5:
		v.Seq[v.seqIdx&7] = val
	case 0x3C6:
		v.dacMask = val
	case 0x3C7:
		v.dacRdIdx = val
		v.dacState = 0
		v.dacReadMode = true
	case 0x3C8:
		v.dacWrIdx = val
		v.dacState = 0
		v.dacReadMode = false
	case 0x3C9:
		v.mu.Lock()
		v.DAC[v.dacWrIdx][v.dacState] = val & 0x3F
		v.mu.Unlock()
		v.dacState++
		if v.dacState == 3 {
			v.dacState = 0
			v.dacWrIdx++
		}
	case 0x3CE:
		v.gcIdx = val
	case 0x3CF:
		v.GC[v.gcIdx&0xF] = val
	case 0x3D4, 0x3B4:
		v.crtcIdx = val
	case 0x3D5, 0x3B5:
		v.CRTC[v.crtcIdx&0x1F] = val
	}
}

// ---- convenience accessors -------------------------------------------------

func (v *VGA) chain4() bool  { return v.Seq[4]&0x08 != 0 }
func (v *VGA) oddEven() bool { return v.Seq[4]&0x04 == 0 }

// StartAddress returns the CRTC start address (regs 0Ch/0Dh).
func (v *VGA) StartAddress() int { return int(v.CRTC[0x0C])<<8 | int(v.CRTC[0x0D]) }

// LineCompare returns the split-screen scanline.
func (v *VGA) LineCompare() int {
	return int(v.CRTC[0x18]) | int(v.CRTC[0x07]&0x10)<<4 | int(v.CRTC[0x09]&0x40)<<3
}

// Pitch returns the logical line width in bytes (CRTC offset register * 2).
func (v *VGA) Pitch() int { return int(v.CRTC[0x13]) * 2 }

// SetDAC sets one palette entry (6-bit components).
func (v *VGA) SetDAC(i uint8, r, g, b uint8) {
	v.mu.Lock()
	v.DAC[i] = [3]uint8{r & 0x3F, g & 0x3F, b & 0x3F}
	v.mu.Unlock()
}

// SetMode programs the registers the way the BIOS would for a standard
// mode and clears video memory.
func (v *VGA) SetMode(mode uint8) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Mode = mode
	for i := range v.Planes {
		for j := range v.Planes[i] {
			v.Planes[i][j] = 0
		}
	}
	v.Seq = [8]uint8{0x03, 0x00, 0x0F, 0x00, 0x03} // odd/even addressing
	v.GC = [16]uint8{}
	v.GC[6] = 0x0E // B8000 window, odd/even
	v.CRTC = [32]uint8{}
	v.CRTC[0x13] = 40
	v.CRTC[0x09] = 0x0F
	v.CRTC[0x0A] = 0x0D
	v.CRTC[0x0B] = 0x0E
	v.CRTC[0x18] = 0xFF
	v.CRTC[0x07] = 0x1F
	for i := 0; i < 16; i++ {
		v.Attr[i] = uint8(i)
	}
	v.Attr[0x10] = 0x0C // text mode: blink enable + line graphics
	v.Attr[0x11] = 0
	v.Attr[0x12] = 0x0F
	v.Attr[0x13] = 0x08
	v.Attr[0x14] = 0
	switch mode {
	case 0x00, 0x01, 0x02, 0x03, 0x07:
		// text: odd/even, EGA palette values with intensity
		for i := 0; i < 16; i++ {
			v.Attr[i] = egaAttr[i]
		}
		v.CRTC[0x13] = 40
		if mode < 2 {
			v.CRTC[0x13] = 20
		}
	case 0x04, 0x05, 0x06:
		v.Seq[4] = 0x02
		v.GC[5] = 0x30 // odd/even, shift interleave
		v.GC[6] = 0x0F
		v.CRTC[0x13] = 40
		v.Attr[0x10] = 0x01
	case 0x0D, 0x0E, 0x10, 0x12:
		v.Seq[4] = 0x06 // planar
		v.GC[5] = 0x00
		v.GC[6] = 0x05 // A0000 64K, graphics
		v.CRTC[0x13] = 40
		if mode != 0x0D {
			v.CRTC[0x13] = 40
		}
		v.CRTC[0x13] = 40
		if mode == 0x0D {
			v.CRTC[0x13] = 20
		}
		for i := 0; i < 16; i++ {
			v.Attr[i] = egaAttr[i]
		}
		v.Attr[0x10] = 0x01
	case 0x13:
		v.Seq[4] = 0x0E // chain-4
		v.GC[5] = 0x40  // 256-colour shift
		v.GC[6] = 0x05
		v.CRTC[0x13] = 40
		v.CRTC[0x14] = 0x40 // dword mode
		v.CRTC[0x17] = 0xA3
		v.Attr[0x10] = 0x41
	}
	v.loadDefaultPalette()
}

var egaAttr = [16]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x14, 0x07, 0x38, 0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F}

// ---- memory access (called through the CPU memory MMIO hook) -------------

// windowOffset maps a linear address to an offset within the active
// window, or -1 if outside it.
func (v *VGA) windowOffset(addr uint32) int {
	a := addr - 0xA0000
	switch (v.GC[6] >> 2) & 3 {
	case 0: // A0000-BFFFF 128K
		return int(a)
	case 1: // A0000-AFFFF
		if a < 0x10000 {
			return int(a)
		}
	case 2: // B0000-B7FFF
		if a >= 0x10000 && a < 0x18000 {
			return int(a - 0x10000)
		}
	case 3: // B8000-BFFFF
		if a >= 0x18000 {
			return int(a - 0x18000)
		}
	}
	return -1
}

func (v *VGA) Read(addr uint32) byte {
	off := v.windowOffset(addr)
	if off < 0 {
		return 0xFF
	}
	if v.chain4() {
		return v.Planes[off&3][(off>>2)&0xFFFF]
	}
	if v.oddEven() {
		return v.Planes[off&1][(off>>1)&0xFFFF]
	}
	off &= 0xFFFF
	for i := 0; i < 4; i++ {
		v.latch[i] = v.Planes[i][off]
	}
	if v.GC[5]&0x08 != 0 { // read mode 1: colour compare
		var r uint8 = 0xFF
		cmp := v.GC[2]
		dc := v.GC[7]
		for p := 0; p < 4; p++ {
			if dc&(1<<uint(p)) == 0 {
				continue
			}
			var want uint8
			if cmp&(1<<uint(p)) != 0 {
				want = 0xFF
			}
			r &^= v.latch[p] ^ want
		}
		return r
	}
	return v.latch[v.GC[4]&3]
}

// Peek returns what Read would return without disturbing the latches
// (for debuggers inspecting video memory).
func (v *VGA) Peek(addr uint32) byte {
	saved := v.latch
	b := v.Read(addr)
	v.latch = saved
	return b
}

func (v *VGA) Write(addr uint32, val byte) {
	off := v.windowOffset(addr)
	if off < 0 {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.chain4() {
		v.Planes[off&3][(off>>2)&0xFFFF] = val
		return
	}
	if v.oddEven() {
		v.Planes[off&1][(off>>1)&0xFFFF] = val
		return
	}
	off &= 0xFFFF
	mask := v.Seq[2] & 0x0F
	bitMask := v.GC[8]
	rot := v.GC[3] & 7
	fn := (v.GC[3] >> 3) & 3
	setReset := v.GC[0]
	enableSR := v.GC[1]
	for p := 0; p < 4; p++ {
		if mask&(1<<uint(p)) == 0 {
			continue
		}
		var out uint8
		switch v.GC[5] & 3 {
		case 0:
			d := val>>rot | val<<(8-rot)
			if enableSR&(1<<uint(p)) != 0 {
				if setReset&(1<<uint(p)) != 0 {
					d = 0xFF
				} else {
					d = 0
				}
			}
			d = applyFn(fn, d, v.latch[p])
			out = d&bitMask | v.latch[p]&^bitMask
		case 1:
			out = v.latch[p]
		case 2:
			var d uint8
			if val&(1<<uint(p)) != 0 {
				d = 0xFF
			}
			d = applyFn(fn, d, v.latch[p])
			out = d&bitMask | v.latch[p]&^bitMask
		case 3:
			d := val>>rot | val<<(8-rot)
			bm := bitMask & d
			var sr uint8
			if setReset&(1<<uint(p)) != 0 {
				sr = 0xFF
			}
			out = sr&bm | v.latch[p]&^bm
		}
		v.Planes[p][off] = out
	}
}

func applyFn(fn uint8, d, latch uint8) uint8 {
	switch fn {
	case 1:
		return d & latch
	case 2:
		return d | latch
	case 3:
		return d ^ latch
	}
	return d
}

// ---- pixel helpers for the BIOS ------------------------------------------

// PutPixel writes a pixel in the current graphics mode.
func (v *VGA) PutPixel(x, y int, color uint8) {
	v.mu.Lock()
	defer v.mu.Unlock()
	switch v.Mode {
	case 0x13:
		if x < 0 || x >= 320 || y < 0 || y >= 200 {
			return
		}
		off := y*320 + x
		v.Planes[off&3][off>>2] = color
	case 0x0D, 0x0E, 0x10, 0x12:
		w := v.Pitch()
		off := y*w + x/8
		if off >= 65536 || x < 0 || y < 0 {
			return
		}
		bit := uint8(0x80) >> uint(x&7)
		for p := 0; p < 4; p++ {
			if color&(1<<uint(p)) != 0 {
				v.Planes[p][off] |= bit
			} else {
				v.Planes[p][off] &^= bit
			}
		}
	case 0x04, 0x05:
		if x < 0 || x >= 320 || y < 0 || y >= 200 {
			return
		}
		off := (y&1)*0x2000 + (y>>1)*80 + x/4
		shift := uint(6 - 2*(x&3))
		b := v.Planes[off&1][off>>1]
		b = b&^(3<<shift) | (color&3)<<shift
		v.Planes[off&1][off>>1] = b
	case 0x06:
		if x < 0 || x >= 640 || y < 0 || y >= 200 {
			return
		}
		off := (y&1)*0x2000 + (y>>1)*80 + x/8
		bit := uint8(0x80) >> uint(x&7)
		b := v.Planes[off&1][off>>1]
		if color&1 != 0 {
			b |= bit
		} else {
			b &^= bit
		}
		v.Planes[off&1][off>>1] = b
	}
}

// GetPixel reads a pixel in the current graphics mode.
func (v *VGA) GetPixel(x, y int) uint8 {
	v.mu.Lock()
	defer v.mu.Unlock()
	switch v.Mode {
	case 0x13:
		if x < 0 || x >= 320 || y < 0 || y >= 200 {
			return 0
		}
		off := y*320 + x
		return v.Planes[off&3][off>>2]
	case 0x0D, 0x0E, 0x10, 0x12:
		w := v.Pitch()
		off := y*w + x/8
		if off >= 65536 || x < 0 || y < 0 {
			return 0
		}
		bit := uint8(0x80) >> uint(x&7)
		var c uint8
		for p := 0; p < 4; p++ {
			if v.Planes[p][off]&bit != 0 {
				c |= 1 << uint(p)
			}
		}
		return c
	case 0x04, 0x05:
		off := (y&1)*0x2000 + (y>>1)*80 + x/4
		shift := uint(6 - 2*(x&3))
		return (v.Planes[off&1][off>>1] >> shift) & 3
	case 0x06:
		off := (y&1)*0x2000 + (y>>1)*80 + x/8
		bit := uint8(0x80) >> uint(x&7)
		if v.Planes[off&1][off>>1]&bit != 0 {
			return 1
		}
	}
	return 0
}

// ---- default palette ---------------------------------------------------------

// loadDefaultPalette installs the IBM VGA BIOS default DAC contents for
// the current mode: 16 EGA colours, 16 greys and a 216-colour HSV ramp
// for 256-colour modes; the 64-entry EGA set otherwise.
func (v *VGA) loadDefaultPalette() {
	for i := range v.DAC {
		v.DAC[i] = [3]uint8{}
	}
	// Entries 0-15: EGA colours (also used by 16-colour modes via the
	// attribute controller mapping into the 64-entry set).
	for i := 0; i < 64; i++ {
		r := uint8(0)
		g := uint8(0)
		b := uint8(0)
		if i&4 != 0 {
			r = 0x2A
		}
		if i&2 != 0 {
			g = 0x2A
		}
		if i&1 != 0 {
			b = 0x2A
		}
		if i&0x20 != 0 {
			r += 0x15
		}
		if i&0x10 != 0 {
			g += 0x15
		}
		if i&8 != 0 {
			b += 0x15
		}
		v.DAC[i] = [3]uint8{r, g, b}
	}
	if v.Mode != 0x13 {
		return
	}
	copy(v.DAC[:16], defaultVGA16[:])
	greys := [16]uint8{0, 5, 8, 11, 14, 17, 20, 24, 28, 32, 36, 40, 45, 50, 56, 63}
	for i, g := range greys {
		v.DAC[16+i] = [3]uint8{g, g, g}
	}
	idx := 32
	// Three intensity groups, each with three saturation ramps of 24 hues.
	groups := []struct {
		m    int
		ramp [3][5]int
	}{
		{63, [3][5]int{{0, 16, 31, 47, 63}, {31, 39, 47, 55, 63}, {45, 49, 54, 58, 63}}},
		{28, [3][5]int{{0, 7, 14, 21, 28}, {14, 17, 21, 24, 28}, {20, 22, 24, 26, 28}}},
		{16, [3][5]int{{0, 4, 8, 12, 16}, {8, 10, 12, 14, 16}, {11, 12, 13, 15, 16}}},
	}
	for _, g := range groups {
		m := g.m
		for _, ramp := range g.ramp {
			lo := ramp[0]
			s1, s2, s3 := ramp[1], ramp[2], ramp[3]
			hues := [24][3]int{
				{lo, lo, m}, {s1, lo, m}, {s2, lo, m}, {s3, lo, m}, {m, lo, m},
				{m, lo, s3}, {m, lo, s2}, {m, lo, s1}, {m, lo, lo},
				{m, s1, lo}, {m, s2, lo}, {m, s3, lo}, {m, m, lo},
				{s3, m, lo}, {s2, m, lo}, {s1, m, lo}, {lo, m, lo},
				{lo, m, s1}, {lo, m, s2}, {lo, m, s3}, {lo, m, m},
				{lo, s3, m}, {lo, s2, m}, {lo, s1, m},
			}
			for _, h := range hues {
				v.DAC[idx] = [3]uint8{uint8(h[0]), uint8(h[1]), uint8(h[2])}
				idx++
			}
		}
	}
}

var defaultVGA16 = [16][3]uint8{
	{0, 0, 0}, {0, 0, 42}, {0, 42, 0}, {0, 42, 42}, {42, 0, 0}, {42, 0, 42}, {42, 21, 0}, {42, 42, 42},
	{21, 21, 21}, {21, 21, 63}, {21, 63, 21}, {21, 63, 63}, {63, 21, 21}, {63, 21, 63}, {63, 63, 21}, {63, 63, 63},
}

// ---- rendering --------------------------------------------------------------

// Frame is a rendered screen image.
type Frame struct {
	W, H int
	Pix  []byte // RGBA
}

// dacRGB expands a DAC entry (through the pel mask) to 8-bit RGB.
func (v *VGA) dacRGB(i uint8) (uint8, uint8, uint8) {
	c := v.DAC[i&v.dacMask]
	return c[0]<<2 | c[0]>>4, c[1]<<2 | c[1]>>4, c[2]<<2 | c[2]>>4
}

// attrColor maps a 4-bit attribute colour through the attribute
// controller palette (and colour select) to a DAC index.
func (v *VGA) attrColor(c uint8) uint8 {
	p := v.Attr[c&0xF] & 0x3F
	if v.Attr[0x10]&0x80 != 0 {
		p = p&0x0F | v.Attr[0x14]&0x0F<<4
	} else {
		p = p | v.Attr[0x14]&0x0C<<4
	}
	return p
}

// Render draws the current screen into f (reallocating as needed).
// blink toggles text cursor/blink attribute visibility.
func (v *VGA) Render(f *Frame, blink bool, cursorPos int, cursorStart, cursorEnd int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	switch v.Mode {
	case 0x13:
		if v.chain4() && v.CRTC[0x14]&0x40 != 0 {
			v.render13(f)
		} else {
			v.renderModeX(f)
		}
	case 0x0D, 0x0E, 0x10, 0x12:
		v.renderPlanar(f)
	case 0x04, 0x05, 0x06:
		v.renderCGA(f)
	default:
		v.renderText(f, blink, cursorPos, cursorStart, cursorEnd)
	}
}

func (f *Frame) resize(w, h int) {
	if f.W != w || f.H != h || len(f.Pix) != w*h*4 {
		f.W, f.H = w, h
		f.Pix = make([]byte, w*h*4)
	}
}

func (v *VGA) render13(f *Frame) {
	f.resize(320, 200)
	start := v.StartAddress()
	i := 0
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			off := start + y*320 + x
			c := v.Planes[off&3][(off>>2)&0xFFFF]
			r, g, b := v.dacRGB(c)
			f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3] = r, g, b, 255
			i += 4
		}
	}
}

// renderModeX handles unchained 256-colour modes (Mode X and friends):
// each plane byte is one pixel, four pixels per address.
func (v *VGA) renderModeX(f *Frame) {
	pitch := v.Pitch()
	if pitch == 0 {
		pitch = 80
	}
	w := pitch * 4 / 2 * 2
	if w > 640 {
		w = 640
	}
	// width in pixels: offset register counts in dwords? For Mode X the
	// standard is offset=40 -> 320 pixels; each address = 4 pixels and
	// pitch (bytes) = offset*2 = 80 addresses... use 4 pixels/address.
	w = pitch * 4 / 2
	if w <= 0 {
		w = 320
	}
	// Height: derive from vertical display end; default 200/240.
	h := int(v.CRTC[0x12]) | int(v.CRTC[0x07]&0x02)<<7 | int(v.CRTC[0x07]&0x40)<<3
	h++
	if v.CRTC[0x09]&0x80 != 0 { // double scan
		h /= 2
	}
	if h <= 0 || h > 480 {
		h = 200
	}
	if v.CRTC[0x12] == 0 {
		h = 200
	}
	f.resize(w, h)
	start := v.StartAddress()
	lineCmp := v.LineCompare()
	if v.CRTC[0x09]&0x80 != 0 {
		lineCmp /= 2
	}
	i := 0
	addrPerLine := pitch / 2
	for y := 0; y < h; y++ {
		var base int
		if y >= lineCmp {
			base = (y - lineCmp) * addrPerLine
		} else {
			base = start + y*addrPerLine
		}
		for x := 0; x < w; x++ {
			off := base + x/4
			c := v.Planes[x&3][off&0xFFFF]
			r, g, b := v.dacRGB(c)
			f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3] = r, g, b, 255
			i += 4
		}
	}
}

func (v *VGA) renderPlanar(f *Frame) {
	w, h := 640, 480
	switch v.Mode {
	case 0x0D:
		w, h = 320, 200
	case 0x0E:
		w, h = 640, 200
	case 0x10:
		w, h = 640, 350
	}
	f.resize(w, h)
	pitch := v.Pitch()
	if pitch == 0 {
		pitch = w / 8
	}
	start := v.StartAddress()
	i := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (start + y*pitch + x/8) & 0xFFFF
			bit := uint8(0x80) >> uint(x&7)
			var c uint8
			for p := 0; p < 4; p++ {
				if v.Planes[p][off]&bit != 0 {
					c |= 1 << uint(p)
				}
			}
			r, g, b := v.dacRGB(v.attrColor(c))
			f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3] = r, g, b, 255
			i += 4
		}
	}
}

func (v *VGA) renderCGA(f *Frame) {
	w, h := 320, 200
	if v.Mode == 6 {
		w = 640
	}
	f.resize(w, h)
	i := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var c uint8
			if v.Mode == 6 {
				off := (y&1)*0x2000 + (y>>1)*80 + x/8
				if v.Planes[off&1][off>>1]&(0x80>>uint(x&7)) != 0 {
					c = 15
				}
			} else {
				off := (y&1)*0x2000 + (y>>1)*80 + x/4
				c = (v.Planes[off&1][off>>1] >> uint(6-2*(x&3))) & 3
				if c != 0 {
					// CGA palette 1 (cyan/magenta/white) by default
					c = [4]uint8{0, 3, 5, 7}[c] | 8
				}
			}
			r, g, b := v.dacRGB(v.attrColor(c))
			f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3] = r, g, b, 255
			i += 4
		}
	}
}

// renderText draws the 80x25 (or 40x25) text screen at 8x16 cells.
func (v *VGA) renderText(f *Frame, blink bool, cursorPos int, cursorStart, cursorEnd int) {
	cols := 80
	if v.Mode < 2 {
		cols = 40
	}
	rows := 25
	f.resize(cols*8, rows*16)
	start := v.StartAddress()
	blinkOn := v.Attr[0x10]&0x08 != 0
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cell := start + row*cols + col
			ch := v.Planes[0][cell&0xFFFF]
			at := v.Planes[1][cell&0xFFFF]
			fg := at & 0x0F
			bg := (at >> 4) & 0x0F
			hidden := false
			if blinkOn {
				bg &= 7
				if at&0x80 != 0 && !blink {
					hidden = true
				}
			}
			fr, fgc, fb := v.dacRGB(v.attrColor(fg))
			br, bgc, bb := v.dacRGB(v.attrColor(bg))
			glyph := font.CP437Font[ch]
			showCursor := cell == cursorPos && blink && cursorStart <= cursorEnd && cursorStart < 16
			for gy := 0; gy < 16; gy++ {
				bits := glyph[gy]
				if hidden {
					bits = 0
				}
				if showCursor && gy >= cursorStart && gy <= cursorEnd {
					bits = 0xFF
				}
				i := ((row*16+gy)*cols*8 + col*8) * 4
				for gx := 0; gx < 8; gx++ {
					if bits&(0x80>>uint(gx)) != 0 {
						f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3] = fr, fgc, fb, 255
					} else {
						f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3] = br, bgc, bb, 255
					}
					i += 4
				}
			}
		}
	}
}
