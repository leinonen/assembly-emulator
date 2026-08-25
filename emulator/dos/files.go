package dos

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"assembly-emulator/emulator"
)

type handle struct {
	name   string
	device bool
	in     bool
	out    bool
	f      *os.File
}

type findState struct {
	entries []os.DirEntry
	idx     int
	attr    uint8
}

// resolve maps a DOS path onto the sandbox directory. Drive letters and
// leading backslashes are stripped; ".." components are rejected.
func (d *DOS) resolve(name string) (string, error) {
	n := strings.ReplaceAll(name, "\\", "/")
	if len(n) >= 2 && n[1] == ':' {
		n = n[2:]
	}
	n = strings.TrimLeft(n, "/")
	for _, part := range strings.Split(n, "/") {
		if part == ".." {
			return "", errors.New("path escapes sandbox")
		}
	}
	full := filepath.Join(d.Root, filepath.FromSlash(n))
	// Case-insensitive lookup of an existing file.
	dir, base := filepath.Split(full)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if strings.EqualFold(e.Name(), base) {
				return filepath.Join(dir, e.Name()), nil
			}
		}
	}
	return full, nil
}

func (d *DOS) newHandle(h *handle) int {
	n := d.next
	for {
		if _, used := d.files[n]; !used {
			break
		}
		n++
	}
	d.files[n] = h
	return n
}

func isDevice(name string) (*handle, bool) {
	up := strings.ToUpper(strings.TrimSuffix(filepath.Base(name), ":"))
	switch up {
	case "CON":
		return &handle{name: "CON", device: true, in: true, out: true}, true
	case "NUL":
		return &handle{name: "NUL", device: true}, true
	case "PRN", "LPT1":
		return &handle{name: "PRN", device: true, out: true}, true
	case "AUX", "COM1":
		return &handle{name: "AUX", device: true}, true
	}
	return nil, false
}

func (d *DOS) create(c *emulator.CPU) bool {
	name := d.readCString(c.DS(), c.DX())
	if h, ok := isDevice(name); ok {
		c.SetAX(uint16(d.newHandle(h)))
		d.setCF(c, false)
		return true
	}
	path, err := d.resolve(name)
	if err != nil {
		return d.fail(c, 5)
	}
	f, err := os.Create(path)
	if err != nil {
		return d.fail(c, 3)
	}
	c.SetAX(uint16(d.newHandle(&handle{name: name, f: f, in: true, out: true})))
	d.setCF(c, false)
	return true
}

func (d *DOS) open(c *emulator.CPU) bool {
	name := d.readCString(c.DS(), c.DX())
	if h, ok := isDevice(name); ok {
		c.SetAX(uint16(d.newHandle(h)))
		d.setCF(c, false)
		return true
	}
	path, err := d.resolve(name)
	if err != nil {
		return d.fail(c, 5)
	}
	mode := c.AL() & 3
	flag := os.O_RDONLY
	switch mode {
	case 1:
		flag = os.O_WRONLY
	case 2:
		flag = os.O_RDWR
	}
	f, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return d.fail(c, 2)
		}
		return d.fail(c, 5)
	}
	c.SetAX(uint16(d.newHandle(&handle{name: name, f: f, in: mode != 1, out: mode != 0})))
	d.setCF(c, false)
	return true
}

func (d *DOS) close(c *emulator.CPU) bool {
	h, ok := d.files[int(c.BX())]
	if !ok {
		return d.fail(c, 6)
	}
	if h.f != nil {
		h.f.Close()
	}
	if c.BX() > 4 {
		delete(d.files, int(c.BX()))
	}
	d.setCF(c, false)
	return true
}

func (d *DOS) read(c *emulator.CPU) bool {
	h, ok := d.files[int(c.BX())]
	if !ok {
		return d.fail(c, 6)
	}
	addr := uint32(c.DS())<<4 + uint32(c.DX())
	n := int(c.CX())
	if h.device {
		if !h.in {
			c.SetAX(0)
			d.setCF(c, false)
			return true
		}
		// Console read: line-oriented like DOS.
		for {
			k, ok := d.readChar()
			if !ok {
				return false
			}
			if k == 0x0D {
				d.putc(0x0D)
				d.putc(0x0A)
				d.inputLine = append(d.inputLine, 0x0D, 0x0A)
				break
			}
			if k == 0x08 {
				if len(d.inputLine) > 0 {
					d.inputLine = d.inputLine[:len(d.inputLine)-1]
					d.putc(0x08)
					d.putc(' ')
					d.putc(0x08)
				}
				continue
			}
			if k == 0 {
				d.readChar()
				continue
			}
			d.inputLine = append(d.inputLine, k)
			d.putc(k)
			if len(d.inputLine) >= n {
				break
			}
		}
		cnt := len(d.inputLine)
		if cnt > n {
			cnt = n
		}
		for i := 0; i < cnt; i++ {
			d.Mem.Write8(addr+uint32(i), d.inputLine[i])
		}
		d.inputLine = d.inputLine[cnt:]
		c.SetAX(uint16(cnt))
		d.setCF(c, false)
		return true
	}
	buf := make([]byte, n)
	got, err := h.f.Read(buf)
	if err != nil && err != io.EOF {
		return d.fail(c, 5)
	}
	for i := 0; i < got; i++ {
		d.Mem.Write8(addr+uint32(i), buf[i])
	}
	c.SetAX(uint16(got))
	d.setCF(c, false)
	return true
}

func (d *DOS) write(c *emulator.CPU) bool {
	h, ok := d.files[int(c.BX())]
	if !ok {
		return d.fail(c, 6)
	}
	addr := uint32(c.DS())<<4 + uint32(c.DX())
	n := int(c.CX())
	if h.device {
		if h.out {
			for i := 0; i < n; i++ {
				d.putc(d.Mem.Read8(addr + uint32(i)))
			}
		}
		c.SetAX(uint16(n))
		d.setCF(c, false)
		return true
	}
	if n == 0 { // truncate
		pos, _ := h.f.Seek(0, io.SeekCurrent)
		h.f.Truncate(pos)
		c.SetAX(0)
		d.setCF(c, false)
		return true
	}
	buf := make([]byte, n)
	for i := 0; i < n; i++ {
		buf[i] = d.Mem.Read8(addr + uint32(i))
	}
	got, err := h.f.Write(buf)
	if err != nil {
		return d.fail(c, 5)
	}
	c.SetAX(uint16(got))
	d.setCF(c, false)
	return true
}

func (d *DOS) seek(c *emulator.CPU) bool {
	h, ok := d.files[int(c.BX())]
	if !ok {
		return d.fail(c, 6)
	}
	if h.device {
		c.SetAX(0)
		c.SetDX(0)
		d.setCF(c, false)
		return true
	}
	off := int64(int32(uint32(c.CX())<<16 | uint32(c.DX())))
	whence := int(c.AL())
	if whence > 2 {
		return d.fail(c, 1)
	}
	pos, err := h.f.Seek(off, whence)
	if err != nil {
		return d.fail(c, 25)
	}
	c.SetAX(uint16(pos))
	c.SetDX(uint16(pos >> 16))
	d.setCF(c, false)
	return true
}

func (d *DOS) unlink(c *emulator.CPU) bool {
	path, err := d.resolve(d.readCString(c.DS(), c.DX()))
	if err != nil {
		return d.fail(c, 5)
	}
	if err := os.Remove(path); err != nil {
		return d.fail(c, 2)
	}
	d.setCF(c, false)
	return true
}

func (d *DOS) rename(c *emulator.CPU) bool {
	from, err := d.resolve(d.readCString(c.DS(), c.DX()))
	if err != nil {
		return d.fail(c, 5)
	}
	to, err := d.resolve(d.readCString(c.ES(), c.DI()))
	if err != nil {
		return d.fail(c, 5)
	}
	if err := os.Rename(from, to); err != nil {
		return d.fail(c, 2)
	}
	d.setCF(c, false)
	return true
}

func (d *DOS) dup(c *emulator.CPU) bool {
	h, ok := d.files[int(c.BX())]
	if !ok {
		return d.fail(c, 6)
	}
	if c.AH() == 0x45 {
		c.SetAX(uint16(d.newHandle(h)))
	} else {
		d.files[int(c.CX())] = h
	}
	d.setCF(c, false)
	return true
}

func (d *DOS) ioctl(c *emulator.CPU) bool {
	switch c.AL() {
	case 0x00: // get device info
		h, ok := d.files[int(c.BX())]
		if !ok {
			return d.fail(c, 6)
		}
		var dx uint16
		if h.device {
			dx = 0x80 | 0x20 // device, binary
			if h.in {
				dx |= 0x01
			}
			if h.out {
				dx |= 0x02
			}
			if h.name == "NUL" {
				dx |= 0x04
			}
		} else {
			dx = 0x02 // C: drive
		}
		c.SetDX(dx)
		c.SetAX(dx)
		d.setCF(c, false)
	case 0x01: // set device info
		d.setCF(c, false)
	case 0x06: // input status
		c.SetAL(0xFF)
		d.setCF(c, false)
	case 0x07: // output status
		c.SetAL(0xFF)
		d.setCF(c, false)
	case 0x08: // removable?
		c.SetAX(1)
		d.setCF(c, false)
	case 0x0E, 0x0F: // logical drive map
		c.SetAL(0)
		d.setCF(c, false)
	default:
		return d.fail(c, 1)
	}
	return true
}

// ---- findfirst / findnext (DTA layout) ----------------------------------------

func matchPattern(pat, name string) bool {
	pat = strings.ToUpper(pat)
	name = strings.ToUpper(name)
	pn, pe := splitExt(pat)
	nn, ne := splitExt(name)
	return matchPart(pn, nn) && matchPart(pe, ne)
}

func splitExt(s string) (string, string) {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func matchPart(p, s string) bool {
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '*':
			return true
		case '?':
			if i < len(s) {
				continue
			}
		default:
			if i >= len(s) || p[i] != s[i] {
				return false
			}
		}
	}
	return len(s) <= len(p) || strings.HasSuffix(p, "*")
}

func (d *DOS) findFirst(c *emulator.CPU) bool {
	spec := d.readCString(c.DS(), c.DX())
	path, err := d.resolve(spec)
	if err != nil {
		return d.fail(c, 3)
	}
	dir, pat := filepath.Split(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return d.fail(c, 3)
	}
	var matched []os.DirEntry
	for _, e := range entries {
		if matchPattern(pat, e.Name()) {
			matched = append(matched, e)
		}
	}
	d.lastFind = &findState{entries: matched, attr: c.CL()}
	return d.findNext(c)
}

func (d *DOS) findNext(c *emulator.CPU) bool {
	fs := d.lastFind
	if fs == nil {
		return d.fail(c, 18)
	}
	for fs.idx < len(fs.entries) {
		e := fs.entries[fs.idx]
		fs.idx++
		if e.IsDir() && fs.attr&0x10 == 0 {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := strings.ToUpper(e.Name())
		if len(name) > 12 {
			name = name[:12]
		}
		attr := uint8(0x20)
		if e.IsDir() {
			attr = 0x10
		}
		t := info.ModTime()
		d.Mem.Write8(d.dta+0x15, attr)
		d.Mem.Write16(d.dta+0x16, uint16(t.Hour())<<11|uint16(t.Minute())<<5|uint16(t.Second()/2))
		d.Mem.Write16(d.dta+0x18, uint16(t.Year()-1980)<<9|uint16(t.Month())<<5|uint16(t.Day()))
		d.Mem.Write32(d.dta+0x1A, uint32(info.Size()))
		for i := 0; i < 13; i++ {
			var b byte
			if i < len(name) {
				b = name[i]
			}
			d.Mem.Write8(d.dta+0x1E+uint32(i), b)
		}
		c.SetAX(0)
		d.setCF(c, false)
		return true
	}
	return d.fail(c, 18)
}
