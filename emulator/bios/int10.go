package bios

import (
	"assembly-emulator/emulator"
	"assembly-emulator/font"
)

// Text-mode geometry helpers read the BDA so programs that poke it see
// consistent behaviour.

func (b *BIOS) cols() int   { return int(b.M.Mem.Read16(bdaCols)) }
func (b *BIOS) rows() int   { return int(b.M.Mem.Read8(bdaRows)) + 1 }
func (b *BIOS) page() int   { return int(b.M.Mem.Read8(bdaPage)) }
func (b *BIOS) mode() uint8 { return b.M.Mem.Read8(bdaVideoMode) }

func (b *BIOS) isText() bool {
	m := b.mode()
	return m <= 3 || m == 7
}

func (b *BIOS) cursor(page int) (int, int) {
	v := b.M.Mem.Read16(bdaCursor + uint32(page)*2)
	return int(v & 0xFF), int(v >> 8)
}

func (b *BIOS) setCursor(page, x, y int) {
	b.M.Mem.Write16(bdaCursor+uint32(page)*2, uint16(y)<<8|uint16(x))
	if page == b.page() {
		b.syncHWCursor()
	}
}

// syncHWCursor programs the CRTC cursor position so the renderer can draw it.
func (b *BIOS) syncHWCursor() {
	x, y := b.cursor(b.page())
	pos := b.textCell(b.page(), x, y) / 2
	v := b.M.VGA
	v.CRTC[0x0E] = uint8(pos >> 8)
	v.CRTC[0x0F] = uint8(pos)
}

// textCell returns the byte offset in the text window of (x,y) on page.
func (b *BIOS) textCell(page, x, y int) int {
	pageSize := int(b.M.Mem.Read16(bdaPageSize))
	return page*pageSize + (y*b.cols()+x)*2
}

// textBase returns the linear address of the text framebuffer.
func (b *BIOS) textBase() uint32 {
	if b.mode() == 7 {
		return 0xB0000
	}
	return 0xB8000
}

// setMode implements INT 10h AH=00h.
func (b *BIOS) setMode(mode uint8, noClear bool) {
	mem := b.M.Mem
	v := b.M.VGA
	m := mode & 0x7F
	switch m {
	case 0, 1, 2, 3, 4, 5, 6, 7, 0x0D, 0x0E, 0x10, 0x12, 0x13:
	default:
		return // unsupported: leave state
	}
	v.SetMode(m)
	mem.Write8(bdaVideoMode, m)
	cols, rows := 80, 25
	pageSize := 4096
	charH := 16
	switch m {
	case 0, 1:
		cols = 40
		pageSize = 2048
	case 4, 5, 6:
		cols = 40
		pageSize = 16384
		charH = 8
	case 0x0D:
		cols = 40
		pageSize = 8192
		charH = 8
	case 0x0E:
		pageSize = 16384
		charH = 8
	case 0x10:
		pageSize = 32768
		rows = 25
		charH = 14
	case 0x12:
		pageSize = 38400
		rows = 30
	case 0x13:
		cols = 40
		pageSize = 64000
		charH = 8
	}
	if m == 6 || m == 0x0E {
		cols = 80
	}
	mem.Write16(bdaCols, uint16(cols))
	mem.Write16(bdaPageSize, uint16(pageSize))
	mem.Write16(bdaPageOff, 0)
	mem.Write8(bdaRows, uint8(rows-1))
	mem.Write16(bdaCharH, uint16(charH))
	mem.Write8(bdaPage, 0)
	for p := 0; p < 8; p++ {
		mem.Write16(bdaCursor+uint32(p)*2, 0)
	}
	mem.Write16(bdaCursorEnd, 0x0607)
	if m == 7 {
		mem.Write16(bdaCRTC, 0x3B4)
	} else {
		mem.Write16(bdaCRTC, 0x3D4)
	}
	ctl := mem.Read8(bdaVideoCtl) &^ 0x80
	if mode&0x80 != 0 {
		ctl |= 0x80
	}
	mem.Write8(bdaVideoCtl, ctl)
	v.CRTC[0x0A] = 6
	v.CRTC[0x0B] = 7
	if b.isText() && !noClear && mode&0x80 == 0 {
		b.clearText(0, 0, cols-1, rows-1, 0x07, 0)
	}
	b.syncHWCursor()
}

// clearText fills a text window with spaces.
func (b *BIOS) clearText(x0, y0, x1, y1 int, attr uint8, page int) {
	mem := b.M.Mem
	base := b.textBase()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			off := uint32(b.textCell(page, x, y))
			mem.Write8(base+off, ' ')
			mem.Write8(base+off+1, attr)
		}
	}
}

// scroll moves a text window up (n>0) or down (n<0); n==0 clears.
func (b *BIOS) scroll(n int, attr uint8, x0, y0, x1, y1 int) {
	mem := b.M.Mem
	base := b.textBase()
	page := b.page()
	if x1 >= b.cols() {
		x1 = b.cols() - 1
	}
	if y1 >= b.rows() {
		y1 = b.rows() - 1
	}
	if !b.isText() {
		b.scrollGfx(n, attr, x0, y0, x1, y1)
		return
	}
	if n == 0 || n > y1-y0 || -n > y1-y0 {
		b.clearText(x0, y0, x1, y1, attr, page)
		return
	}
	if n > 0 {
		for y := y0; y <= y1-n; y++ {
			for x := x0; x <= x1; x++ {
				src := base + uint32(b.textCell(page, x, y+n))
				dst := base + uint32(b.textCell(page, x, y))
				mem.Write16(dst, mem.Read16(src))
			}
		}
		b.clearText(x0, y1-n+1, x1, y1, attr, page)
	} else {
		n = -n
		for y := y1; y >= y0+n; y-- {
			for x := x0; x <= x1; x++ {
				src := base + uint32(b.textCell(page, x, y-n))
				dst := base + uint32(b.textCell(page, x, y))
				mem.Write16(dst, mem.Read16(src))
			}
		}
		b.clearText(x0, y0, x1, y0+n-1, attr, page)
	}
}

// scrollGfx scrolls in graphics modes by moving pixels.
func (b *BIOS) scrollGfx(n int, attr uint8, x0, y0, x1, y1 int) {
	v := b.M.VGA
	ch := int(b.M.Mem.Read16(bdaCharH))
	w := (x1 - x0 + 1) * 8
	px0 := x0 * 8
	py0 := y0 * ch
	py1 := (y1+1)*ch - 1
	if n == 0 || n*ch > py1-py0+1 || -n*ch > py1-py0+1 {
		for y := py0; y <= py1; y++ {
			for x := px0; x < px0+w; x++ {
				v.PutPixel(x, y, attr)
			}
		}
		return
	}
	dy := n * ch
	if dy > 0 {
		for y := py0; y <= py1-dy; y++ {
			for x := px0; x < px0+w; x++ {
				v.PutPixel(x, y, v.GetPixel(x, y+dy))
			}
		}
		for y := py1 - dy + 1; y <= py1; y++ {
			for x := px0; x < px0+w; x++ {
				v.PutPixel(x, y, attr)
			}
		}
	} else {
		dy = -dy
		for y := py1; y >= py0+dy; y-- {
			for x := px0; x < px0+w; x++ {
				v.PutPixel(x, y, v.GetPixel(x, y-dy))
			}
		}
		for y := py0; y < py0+dy; y++ {
			for x := px0; x < px0+w; x++ {
				v.PutPixel(x, y, attr)
			}
		}
	}
}

// writeCharAt puts a character at (x,y) on page. In graphics modes the
// glyph is drawn with colour attr (bit 7 = XOR).
func (b *BIOS) writeCharAt(page, x, y int, ch, attr uint8, setAttr bool) {
	mem := b.M.Mem
	if b.isText() {
		off := b.textBase() + uint32(b.textCell(page, x, y))
		mem.Write8(off, ch)
		if setAttr {
			mem.Write8(off+1, attr)
		}
		return
	}
	v := b.M.VGA
	h := int(mem.Read16(bdaCharH))
	color := attr
	xor := attr&0x80 != 0 && b.mode() != 0x13
	if xor {
		color &= 0x7F
	}
	for gy := 0; gy < h; gy++ {
		var bits uint8
		if h == 16 {
			bits = font.CP437Font[ch][gy]
		} else {
			bits = font.CP437Font[ch][gy*2]
		}
		for gx := 0; gx < 8; gx++ {
			px, py := x*8+gx, y*h+gy
			if bits&(0x80>>uint(gx)) != 0 {
				if xor {
					v.PutPixel(px, py, v.GetPixel(px, py)^color)
				} else {
					v.PutPixel(px, py, color)
				}
			} else if b.mode() == 0x13 || !xor {
				// background: in graphics modes the BIOS writes 0 for
				// unset pixels unless XOR mode.
				if !xor {
					v.PutPixel(px, py, 0)
				}
			}
		}
	}
}

// teletype implements INT 10h AH=0Eh.
func (b *BIOS) teletype(ch uint8, color uint8) {
	page := b.page()
	x, y := b.cursor(page)
	cols, rows := b.cols(), b.rows()
	switch ch {
	case 0x07: // bell
	case 0x08:
		if x > 0 {
			x--
		}
	case 0x0A:
		y++
	case 0x0D:
		x = 0
	default:
		b.writeCharAt(page, x, y, ch, color, !b.isText())
		x++
		if x >= cols {
			x = 0
			y++
		}
	}
	if y >= rows {
		attr := uint8(0x07)
		if b.isText() {
			// use attribute of last character on screen
			off := b.textBase() + uint32(b.textCell(page, cols-1, rows-1)) + 1
			attr = b.M.Mem.Read8(off)
		} else {
			attr = 0
		}
		b.scroll(1, attr, 0, 0, cols-1, rows-1)
		y = rows - 1
	}
	b.setCursor(page, x, y)
	if b.M.Stdout != nil && page == 0 {
		b.M.Stdout(ch)
	}
}

// Teletype is exported for DOS console output.
func (b *BIOS) Teletype(ch uint8) { b.teletype(ch, 0x07) }

func (b *BIOS) int10(c *emulator.CPU) {
	mem := b.M.Mem
	v := b.M.VGA
	switch c.AH() {
	case 0x00:
		b.setMode(c.AL(), false)
	case 0x01: // cursor shape
		mem.Write16(bdaCursorEnd, c.CX())
		v.CRTC[0x0A] = c.CH() & 0x3F
		v.CRTC[0x0B] = c.CL() & 0x1F
	case 0x02:
		page := int(c.BH() & 7)
		b.setCursor(page, int(c.DL()), int(c.DH()))
	case 0x03:
		x, y := b.cursor(int(c.BH() & 7))
		c.SetDL(uint8(x))
		c.SetDH(uint8(y))
		c.SetCX(mem.Read16(bdaCursorEnd))
	case 0x04: // light pen
		c.SetAH(0)
	case 0x05:
		page := int(c.AL() & 7)
		mem.Write8(bdaPage, uint8(page))
		off := page * int(mem.Read16(bdaPageSize))
		mem.Write16(bdaPageOff, uint16(off))
		start := off / 2
		if !b.isText() {
			start = off
		}
		v.CRTC[0x0C] = uint8(start >> 8)
		v.CRTC[0x0D] = uint8(start)
		b.syncHWCursor()
	case 0x06:
		b.scroll(int(c.AL()), c.BH(), int(c.CL()), int(c.CH()), int(c.DL()), int(c.DH()))
	case 0x07:
		b.scroll(-int(c.AL()), c.BH(), int(c.CL()), int(c.CH()), int(c.DL()), int(c.DH()))
	case 0x08: // read char+attr at cursor
		page := int(c.BH() & 7)
		x, y := b.cursor(page)
		if b.isText() {
			off := b.textBase() + uint32(b.textCell(page, x, y))
			c.SetAL(mem.Read8(off))
			c.SetAH(mem.Read8(off + 1))
		} else {
			c.SetAX(0)
		}
	case 0x09, 0x0A: // write char (+attr) N times
		page := int(c.BH() & 7)
		x, y := b.cursor(page)
		n := int(c.CX())
		for i := 0; i < n; i++ {
			b.writeCharAt(page, x, y, c.AL(), c.BL(), c.AH() == 0x09)
			x++
			if x >= b.cols() {
				x = 0
				y++
				if y >= b.rows() {
					break
				}
			}
		}
	case 0x0B: // CGA palette / background
		if c.BH() == 0 {
			v.Attr[0x11] = c.BL() & 0x3F // overscan
		} else {
			// palette select: 0 = green/red/brown, 1 = cyan/magenta/white
			if c.BL()&1 != 0 {
				v.Attr[1], v.Attr[2], v.Attr[3] = 3, 5, 7
			} else {
				v.Attr[1], v.Attr[2], v.Attr[3] = 2, 4, 6
			}
		}
	case 0x0C:
		color := c.AL()
		if color&0x80 != 0 && b.mode() != 0x13 {
			color = v.GetPixel(int(c.CX()), int(c.DX())) ^ (color & 0x7F)
		}
		v.PutPixel(int(c.CX()), int(c.DX()), color)
	case 0x0D:
		c.SetAL(v.GetPixel(int(c.CX()), int(c.DX())))
	case 0x0E:
		b.teletype(c.AL(), c.BL())
	case 0x0F:
		c.SetAL(b.mode() | mem.Read8(bdaVideoCtl)&0x80)
		c.SetAH(uint8(b.cols()))
		c.SetBH(uint8(b.page()))
	case 0x10:
		b.int10Palette(c)
	case 0x11:
		b.int10Font(c)
	case 0x12:
		switch c.BL() {
		case 0x10: // EGA info
			c.SetBH(0) // colour
			c.SetBL(3) // 256K
			c.SetCX(0x0009)
		case 0x30: // scan lines
			c.SetAL(0x12)
		case 0x31, 0x32, 0x33, 0x34, 0x35, 0x36:
			c.SetAL(0x12)
		}
	case 0x13: // write string
		page := int(c.BH() & 7)
		x, y := int(c.DL()), int(c.DH())
		n := int(c.CX())
		addr := uint32(c.ES())<<4 + uint32(c.BP())
		attr := c.BL()
		oldx, oldy := b.cursor(page)
		b.setCursor(page, x, y)
		for i := 0; i < n; i++ {
			ch := mem.Read8(addr)
			addr++
			if c.AL()&2 != 0 {
				attr = mem.Read8(addr)
				addr++
			}
			switch ch {
			case 0x07, 0x08, 0x0A, 0x0D:
				b.teletype(ch, attr)
			default:
				cx, cy := b.cursor(page)
				b.writeCharAt(page, cx, cy, ch, attr, true)
				b.teletype(ch, attr) // advances cursor (rewrites char)
			}
		}
		if c.AL()&1 == 0 {
			b.setCursor(page, oldx, oldy)
		}
	case 0x1A:
		if c.AL() == 0 {
			c.SetAL(0x1A)
			c.SetBX(0x0008) // VGA colour
		}
	case 0x1B: // functionality/state info
		c.SetAL(0)
	case 0xFE, 0xFF: // topview/desqview: ignore
	}
}

func (b *BIOS) int10Palette(c *emulator.CPU) {
	mem := b.M.Mem
	v := b.M.VGA
	switch c.AL() {
	case 0x00: // set palette register
		v.Attr[c.BL()&0x1F] = c.BH()
	case 0x01: // overscan
		v.Attr[0x11] = c.BH()
	case 0x02: // set all palette registers from ES:DX (17 bytes)
		addr := uint32(c.ES())<<4 + uint32(c.DX())
		for i := 0; i < 17; i++ {
			v.Attr[i] = mem.Read8(addr + uint32(i))
		}
	case 0x03: // blink/intensity
		if c.BL()&1 != 0 {
			v.Attr[0x10] |= 0x08
		} else {
			v.Attr[0x10] &^= 0x08
		}
	case 0x07:
		c.SetBH(v.Attr[c.BL()&0x1F])
	case 0x08:
		c.SetBH(v.Attr[0x11])
	case 0x09:
		addr := uint32(c.ES())<<4 + uint32(c.DX())
		for i := 0; i < 17; i++ {
			mem.Write8(addr+uint32(i), v.Attr[i])
		}
	case 0x10: // set DAC register: BX index, DH red, CH green, CL blue
		v.SetDAC(uint8(c.BX()), c.DH(), c.CH(), c.CL())
	case 0x12: // set block of DAC registers from ES:DX
		addr := uint32(c.ES())<<4 + uint32(c.DX())
		start := int(c.BX())
		n := int(c.CX())
		for i := 0; i < n; i++ {
			idx := uint8(start + i)
			v.SetDAC(idx, mem.Read8(addr), mem.Read8(addr+1), mem.Read8(addr+2))
			addr += 3
		}
	case 0x13: // select colour page
		if c.BL() == 0 {
			if c.BH()&1 != 0 {
				v.Attr[0x10] |= 0x80
			} else {
				v.Attr[0x10] &^= 0x80
			}
		} else {
			v.Attr[0x14] = c.BH()
		}
	case 0x15:
		d := v.DAC[uint8(c.BX())]
		c.SetDH(d[0])
		c.SetCH(d[1])
		c.SetCL(d[2])
	case 0x17:
		addr := uint32(c.ES())<<4 + uint32(c.DX())
		start := int(c.BX())
		n := int(c.CX())
		for i := 0; i < n; i++ {
			d := v.DAC[uint8(start+i)]
			mem.Write8(addr, d[0])
			mem.Write8(addr+1, d[1])
			mem.Write8(addr+2, d[2])
			addr += 3
		}
	case 0x18: // set PEL mask
		v.Out8(0x3C6, c.BL())
	case 0x19:
		c.SetBL(v.In8(0x3C6))
	case 0x1A:
		c.SetBH(v.Attr[0x14])
		c.SetBL(v.Attr[0x10] >> 7)
	case 0x1B: // sum to grey scale
		start := int(c.BX())
		for i := 0; i < int(c.CX()); i++ {
			d := v.DAC[uint8(start+i)]
			g := uint8((int(d[0])*30 + int(d[1])*59 + int(d[2])*11) / 100)
			v.SetDAC(uint8(start+i), g, g, g)
		}
	}
}

func (b *BIOS) int10Font(c *emulator.CPU) {
	mem := b.M.Mem
	switch c.AL() {
	case 0x30: // get font info
		switch c.BH() {
		case 0, 1: // int 1F / int 43 pointers
			vec := uint32(0x1F)
			if c.BH() == 1 {
				vec = 0x43
			}
			c.SetBP(mem.Read16(vec * 4))
			c.Segs[emulator.SegES] = mem.Read16(vec*4 + 2)
		case 2, 3, 4:
			c.Segs[emulator.SegES] = ROMSeg
			c.SetBP(romFont8x8)
			if c.BH() == 4 {
				c.SetBP(romFont8x8 + 128*8)
			}
		case 5, 6:
			c.Segs[emulator.SegES] = ROMSeg
			c.SetBP(romFont8x16)
		}
		c.SetCX(mem.Read16(bdaCharH))
		c.SetDL(mem.Read8(bdaRows))
	case 0x00, 0x10, 0x01, 0x11, 0x02, 0x12, 0x04, 0x14, 0x03:
		// Load font: keep 8x16 rendering; record the character height.
		switch c.AL() {
		case 0x01, 0x11:
			mem.Write16(bdaCharH, 14)
		case 0x02, 0x12:
			mem.Write16(bdaCharH, 8)
			mem.Write8(bdaRows, 49)
		case 0x04, 0x14:
			mem.Write16(bdaCharH, 16)
		}
	}
}
