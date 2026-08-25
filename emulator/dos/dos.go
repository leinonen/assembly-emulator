// Package dos implements the subset of MS-DOS needed to run .COM programs:
// PSP construction, INT 20h/21h services (console, files, memory, time,
// vectors, termination) with file access sandboxed to a directory.
package dos

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"assembly-emulator/emulator"
	"assembly-emulator/emulator/bios"
)

// PSPSeg is the segment where the program segment prefix is placed.
const PSPSeg = 0x0800

// MemTop is the first paragraph past conventional memory usable by DOS.
const MemTop = 0x9FC0 // 640K minus the EBDA-ish area

type block struct {
	seg   uint16
	paras uint16
	free  bool
}

// DOS holds emulated DOS state.
type DOS struct {
	B        *bios.BIOS
	CPU      *emulator.CPU
	Mem      *emulator.Memory
	Root     string // sandbox directory
	Stdout   io.Writer
	Stdin    io.Reader
	Clock    func() time.Time
	Exited   bool
	ExitCode int

	files      map[int]*handle
	next       int
	dta        uint32 // linear address of the DTA
	blocks     []block
	psp        uint16
	stdinBuf   []byte
	inputLine  []byte // pending buffered-input characters (AH=0Ah echo)
	verify     bool
	ctrlBreak  bool
	lastFind   *findState
	stdoutLine []byte
}

// New creates the DOS layer bound to a BIOS instance.
func New(b *bios.BIOS, root string) *DOS {
	d := &DOS{B: b, CPU: b.M.CPU, Mem: b.M.Mem, Root: root, files: map[int]*handle{}, next: 5}
	d.blocks = []block{{seg: PSPSeg, paras: MemTop - PSPSeg, free: false}}
	d.psp = PSPSeg
	d.files[0] = &handle{name: "STDIN", device: true, in: true}
	d.files[1] = &handle{name: "STDOUT", device: true, out: true}
	d.files[2] = &handle{name: "STDERR", device: true, out: true}
	d.files[3] = &handle{name: "AUX", device: true}
	d.files[4] = &handle{name: "PRN", device: true, out: true}
	return d
}

// BuildPSP writes a program segment prefix at PSPSeg.
func (d *DOS) BuildPSP(cmdTail string) {
	mem := d.Mem
	base := uint32(PSPSeg) << 4
	for i := uint32(0); i < 256; i++ {
		mem.Write8(base+i, 0)
	}
	mem.Write16(base+0x00, 0x20CD) // int 20h
	mem.Write16(base+0x02, MemTop) // first paragraph beyond program
	mem.Write8(base+0x05, 0x9A)    // far call to DOS (CP/M compat)
	mem.Write16(base+0x06, 0xFEF0)
	mem.Write16(base+0x08, 0x00F0)
	mem.Write16(base+0x0A, mem.Read16(0x22*4)) // saved INT 22h/23h/24h
	mem.Write16(base+0x0C, mem.Read16(0x22*4+2))
	mem.Write16(base+0x0E, mem.Read16(0x23*4))
	mem.Write16(base+0x10, mem.Read16(0x23*4+2))
	mem.Write16(base+0x12, mem.Read16(0x24*4))
	mem.Write16(base+0x14, mem.Read16(0x24*4+2))
	mem.Write16(base+0x16, PSPSeg)    // parent PSP
	for i := uint32(0); i < 20; i++ { // job file table
		v := uint8(0xFF)
		if i < 5 {
			v = uint8(i)
		}
		mem.Write8(base+0x18+i, v)
	}
	mem.Write16(base+0x2C, 0) // environment segment (none)
	mem.Write16(base+0x32, 20)
	mem.Write32(base+0x34, uint32(PSPSeg)<<16|0x0018)
	mem.Write8(base+0x50, 0xCD) // int 21h / retf
	mem.Write8(base+0x51, 0x21)
	mem.Write8(base+0x52, 0xCB)
	if len(cmdTail) > 126 {
		cmdTail = cmdTail[:126]
	}
	mem.Write8(base+0x80, uint8(len(cmdTail)))
	for i := 0; i < len(cmdTail); i++ {
		mem.Write8(base+0x81+uint32(i), cmdTail[i])
	}
	mem.Write8(base+0x81+uint32(len(cmdTail)), 0x0D)
	d.dta = base + 0x80
	// INT 22h terminate address: point at our own exit stub (PSP:0000 int 20h).
	mem.Write16(0x22*4, 0)
	mem.Write16(0x22*4+2, PSPSeg)
}

// Call handles the F1 trap for vectors 20h and 21h. It returns true when
// the call is complete (false = keep idling for input).
func (d *DOS) Call(c *emulator.CPU, svc byte) bool {
	switch svc {
	case 0x20:
		d.exit(0)
		return true
	case 0x21:
		return d.int21(c)
	}
	return true
}

func (d *DOS) exit(code int) {
	d.Exited = true
	d.ExitCode = code
	d.flushStdout()
	d.B.Exit(code)
}

func (d *DOS) setCF(c *emulator.CPU, on bool) {
	addr := uint32(c.SS())<<4 + uint32(c.SP()+4)
	f := d.Mem.Read16(addr)
	if on {
		f |= emulator.FlagCF
	} else {
		f &^= emulator.FlagCF
	}
	d.Mem.Write16(addr, f)
}

func (d *DOS) setZF(c *emulator.CPU, on bool) {
	addr := uint32(c.SS())<<4 + uint32(c.SP()+4)
	f := d.Mem.Read16(addr)
	if on {
		f |= emulator.FlagZF
	} else {
		f &^= emulator.FlagZF
	}
	d.Mem.Write16(addr, f)
}

func (d *DOS) fail(c *emulator.CPU, code uint16) bool {
	c.SetAX(code)
	d.setCF(c, true)
	return true
}

func (d *DOS) now() time.Time {
	if d.Clock != nil {
		return d.Clock()
	}
	return time.Now()
}

// readString reads a '$'-terminated string at DS:DX.
func (d *DOS) readDollarString(c *emulator.CPU) []byte {
	addr := uint32(c.DS())<<4 + uint32(c.DX())
	var out []byte
	for i := uint32(0); i < 65536; i++ {
		b := d.Mem.Read8(addr + i)
		if b == '$' {
			break
		}
		out = append(out, b)
	}
	return out
}

// readCString reads an ASCIIZ string at seg:off.
func (d *DOS) readCString(seg, off uint16) string {
	addr := uint32(seg)<<4 + uint32(off)
	var sb strings.Builder
	for i := uint32(0); i < 256; i++ {
		b := d.Mem.Read8(addr + i)
		if b == 0 {
			break
		}
		sb.WriteByte(b)
	}
	return sb.String()
}

func (d *DOS) int21(c *emulator.CPU) bool {
	switch c.AH() {
	case 0x00:
		d.exit(0)
	case 0x01: // read char with echo
		k, ok := d.readChar()
		if !ok {
			return false
		}
		c.SetAL(k)
		if k != 0 {
			d.echo(k)
		}
	case 0x02: // write char
		d.putc(c.DL())
		c.SetAL(c.DL())
	case 0x03, 0x04: // aux I/O
		c.SetAL(0)
	case 0x05: // printer
	case 0x06: // direct console I/O
		if c.DL() == 0xFF {
			k, ok := d.readChar()
			if !ok {
				c.SetAL(0)
				d.setZF(c, true)
				return true
			}
			c.SetAL(k)
			d.setZF(c, false)
		} else {
			d.putc(c.DL())
			c.SetAL(c.DL())
		}
	case 0x07, 0x08: // read char, no echo
		k, ok := d.readChar()
		if !ok {
			return false
		}
		c.SetAL(k)
	case 0x09:
		for _, b := range d.readDollarString(c) {
			d.putc(b)
		}
		c.SetAL('$')
	case 0x0A: // buffered input
		return d.bufferedInput(c)
	case 0x0B: // input status
		if _, ok := d.B.PeekKey(); ok {
			c.SetAL(0xFF)
		} else {
			c.SetAL(0)
		}
	case 0x0C: // flush + call function
		d.B.M.Mem.Write16(0x41A, d.B.M.Mem.Read16(0x41C))
		fn := c.AL()
		if fn == 1 || fn == 6 || fn == 7 || fn == 8 || fn == 0x0A {
			c.SetAH(fn)
			return d.int21(c)
		}
		c.SetAL(0)
	case 0x0D: // disk reset
	case 0x0E: // select disk
		c.SetAL(1) // number of drives
	case 0x19: // current disk
		c.SetAL(2) // C:
	case 0x1A: // set DTA
		d.dta = uint32(c.DS())<<4 + uint32(c.DX())
	case 0x25: // set interrupt vector
		d.Mem.Write16(uint32(c.AL())*4, c.DX())
		d.Mem.Write16(uint32(c.AL())*4+2, c.DS())
	case 0x26: // create PSP
		d.copyPSP(c.DX())
	case 0x2A: // get date
		t := d.now()
		c.SetCX(uint16(t.Year()))
		c.SetDH(uint8(t.Month()))
		c.SetDL(uint8(t.Day()))
		c.SetAL(uint8(t.Weekday()))
	case 0x2B, 0x2D: // set date/time
		c.SetAL(0)
	case 0x2C: // get time
		t := d.now()
		c.SetCH(uint8(t.Hour()))
		c.SetCL(uint8(t.Minute()))
		c.SetDH(uint8(t.Second()))
		c.SetDL(uint8(t.Nanosecond() / 10000000))
	case 0x2E:
		d.verify = c.AL() != 0
	case 0x2F: // get DTA
		c.Segs[emulator.SegES] = uint16(d.dta >> 4)
		c.SetBX(uint16(d.dta & 0xF))
	case 0x30: // DOS version
		c.SetAX(0x0005) // 5.00
		c.SetBX(0xFF00) // MS-DOS
		c.SetCX(0)
	case 0x33: // ctrl-break
		if c.AL() == 0 {
			if d.ctrlBreak {
				c.SetDL(1)
			} else {
				c.SetDL(0)
			}
		} else if c.AL() == 1 {
			d.ctrlBreak = c.DL() != 0
		} else if c.AL() == 6 {
			c.SetBX(0x0005)
		}
	case 0x34: // InDOS flag address
		c.Segs[emulator.SegES] = 0x0080
		c.SetBX(0x0000)
	case 0x35: // get vector
		c.Segs[emulator.SegES] = d.Mem.Read16(uint32(c.AL())*4 + 2)
		c.SetBX(d.Mem.Read16(uint32(c.AL()) * 4))
	case 0x36: // disk free space
		c.SetAX(8)
		c.SetBX(0xFFFF)
		c.SetCX(512)
		c.SetDX(0xFFFF)
	case 0x37: // switch char
		c.SetAL(0)
		c.SetDL('/')
	case 0x38: // country info: US
		c.SetAX(1)
		c.SetBX(1)
		d.setCF(c, false)
	case 0x39, 0x3A: // mkdir / rmdir
		return d.fail(c, 5)
	case 0x3B: // chdir
		d.setCF(c, false)
	case 0x3C:
		return d.create(c)
	case 0x3D:
		return d.open(c)
	case 0x3E:
		return d.close(c)
	case 0x3F:
		return d.read(c)
	case 0x40:
		return d.write(c)
	case 0x41:
		return d.unlink(c)
	case 0x42:
		return d.seek(c)
	case 0x43: // attributes
		if c.AL() == 0 {
			c.SetCX(0x20)
		}
		d.setCF(c, false)
	case 0x44:
		return d.ioctl(c)
	case 0x45, 0x46: // dup
		return d.dup(c)
	case 0x47: // get cwd
		addr := uint32(c.DS())<<4 + uint32(c.SI())
		d.Mem.Write8(addr, 0)
		d.setCF(c, false)
	case 0x48:
		return d.alloc(c)
	case 0x49:
		return d.free(c)
	case 0x4A:
		return d.resize(c)
	case 0x4B: // exec: not supported
		return d.fail(c, 2)
	case 0x4C:
		d.exit(int(c.AL()))
	case 0x4D: // get return code
		c.SetAX(uint16(d.ExitCode))
	case 0x4E:
		return d.findFirst(c)
	case 0x4F:
		return d.findNext(c)
	case 0x50: // set PSP
		d.psp = c.BX()
	case 0x51, 0x62: // get PSP
		c.SetBX(d.psp)
	case 0x54: // verify flag
		if d.verify {
			c.SetAL(1)
		} else {
			c.SetAL(0)
		}
	case 0x56: // rename
		return d.rename(c)
	case 0x57: // file date/time
		if c.AL() == 0 {
			t := d.now()
			c.SetCX(uint16(t.Hour())<<11 | uint16(t.Minute())<<5 | uint16(t.Second()/2))
			c.SetDX(uint16(t.Year()-1980)<<9 | uint16(t.Month())<<5 | uint16(t.Day()))
		}
		d.setCF(c, false)
	case 0x58: // allocation strategy
		c.SetAX(0)
		d.setCF(c, false)
	case 0x59: // extended error
		c.SetAX(0)
		c.SetBH(0)
		c.SetBL(0)
		c.SetCH(0)
	case 0x5B:
		return d.create(c)
	case 0x63: // DBCS lead table
		c.SetAL(0)
	case 0x64, 0x65, 0x66: // localisation stubs
		d.setCF(c, false)
	default:
		c.SetAL(0)
		d.setCF(c, true)
	}
	return true
}

// copyPSP implements AH=26h.
func (d *DOS) copyPSP(seg uint16) {
	src := uint32(d.psp) << 4
	dst := uint32(seg) << 4
	for i := uint32(0); i < 256; i++ {
		d.Mem.Write8(dst+i, d.Mem.Read8(src+i))
	}
}

// ---- console -------------------------------------------------------------

func (d *DOS) putc(b byte) {
	d.B.Teletype(b)
}

func (d *DOS) echo(k uint8) {
	switch k {
	case 0x0D:
		d.putc(0x0D)
		d.putc(0x0A)
	default:
		d.putc(k)
	}
}

func (d *DOS) flushStdout() {}

// readChar returns the next console character; extended keys yield 0
// first and the scancode on the following call (DOS convention).
func (d *DOS) readChar() (uint8, bool) {
	if len(d.stdinBuf) > 0 {
		k := d.stdinBuf[0]
		d.stdinBuf = d.stdinBuf[1:]
		return k, true
	}
	key, ok := d.B.ReadKey()
	if !ok {
		return 0, false
	}
	ascii := uint8(key)
	if ascii == 0 || ascii == 0xE0 {
		d.stdinBuf = append(d.stdinBuf, uint8(key>>8))
		return 0, true
	}
	return ascii, true
}

// bufferedInput implements AH=0Ah (line input). It is re-entered while
// idling until Enter is pressed.
func (d *DOS) bufferedInput(c *emulator.CPU) bool {
	addr := uint32(c.DS())<<4 + uint32(c.DX())
	max := int(d.Mem.Read8(addr))
	if max == 0 {
		return true
	}
	for {
		k, ok := d.readChar()
		if !ok {
			return false
		}
		switch k {
		case 0x0D:
			d.putc(0x0D)
			n := len(d.inputLine)
			d.Mem.Write8(addr+1, uint8(n))
			for i := 0; i < n; i++ {
				d.Mem.Write8(addr+2+uint32(i), d.inputLine[i])
			}
			d.Mem.Write8(addr+2+uint32(n), 0x0D)
			d.inputLine = d.inputLine[:0]
			return true
		case 0x08:
			if len(d.inputLine) > 0 {
				d.inputLine = d.inputLine[:len(d.inputLine)-1]
				d.putc(0x08)
				d.putc(' ')
				d.putc(0x08)
			}
		case 0:
			// extended key: discard scancode
			d.readChar()
		default:
			if len(d.inputLine) < max-1 {
				d.inputLine = append(d.inputLine, k)
				d.putc(k)
			} else {
				d.putc(0x07)
			}
		}
	}
}

// ---- memory ----------------------------------------------------------------

func (d *DOS) alloc(c *emulator.CPU) bool {
	want := c.BX()
	bestIdx := -1
	var largest uint16
	for i, b := range d.blocks {
		if !b.free {
			continue
		}
		if b.paras > largest {
			largest = b.paras
		}
		if b.paras >= want && (bestIdx < 0 || b.paras < d.blocks[bestIdx].paras) {
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		c.SetBX(largest)
		return d.fail(c, 8)
	}
	b := d.blocks[bestIdx]
	if b.paras > want {
		d.blocks = append(d.blocks[:bestIdx+1], d.blocks[bestIdx:]...)
		d.blocks[bestIdx] = block{seg: b.seg, paras: want, free: false}
		d.blocks[bestIdx+1] = block{seg: b.seg + want, paras: b.paras - want, free: true}
	} else {
		d.blocks[bestIdx].free = false
	}
	c.SetAX(b.seg)
	d.setCF(c, false)
	return true
}

func (d *DOS) free(c *emulator.CPU) bool {
	seg := c.ES()
	for i := range d.blocks {
		if d.blocks[i].seg == seg && !d.blocks[i].free {
			d.blocks[i].free = true
			d.coalesce()
			d.setCF(c, false)
			return true
		}
	}
	return d.fail(c, 9)
}

func (d *DOS) resize(c *emulator.CPU) bool {
	seg := c.ES()
	want := c.BX()
	for i := range d.blocks {
		b := &d.blocks[i]
		if b.seg != seg || b.free {
			continue
		}
		avail := b.paras
		if i+1 < len(d.blocks) && d.blocks[i+1].free {
			avail += d.blocks[i+1].paras
		}
		if want > avail {
			c.SetBX(avail)
			return d.fail(c, 8)
		}
		if i+1 < len(d.blocks) && d.blocks[i+1].free {
			d.blocks = append(d.blocks[:i+1], d.blocks[i+2:]...)
		}
		if avail > want {
			d.blocks = append(d.blocks[:i+1], d.blocks[i:]...)
			d.blocks[i+1] = block{seg: seg + want, paras: avail - want, free: true}
		}
		d.blocks[i].paras = want
		d.setCF(c, false)
		return true
	}
	return d.fail(c, 9)
}

func (d *DOS) coalesce() {
	for i := 0; i+1 < len(d.blocks); {
		if d.blocks[i].free && d.blocks[i+1].free {
			d.blocks[i].paras += d.blocks[i+1].paras
			d.blocks = append(d.blocks[:i+1], d.blocks[i+2:]...)
			continue
		}
		i++
	}
}

// Debug string for tests.
func (d *DOS) String() string {
	return fmt.Sprintf("dos{psp=%04X blocks=%v exited=%v}", d.psp, d.blocks, d.Exited)
}

// StdinFeed makes the given bytes available as console input (tests).
func (d *DOS) StdinFeed(s string) {
	for i := 0; i < len(s); i++ {
		d.stdinBuf = append(d.stdinBuf, s[i])
	}
}

var _ = os.Stdout
