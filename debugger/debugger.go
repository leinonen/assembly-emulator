// Package debugger is an interactive machine-code debugger for the
// emulator, in the spirit of DOS DEBUG and gdb: it breaks on entry, single
// steps, steps over calls and interrupts, sets breakpoints (by address,
// label or source line), and shows registers, memory, the stack, the x87
// state and NASM disassembly.
//
// It drives the machine through Machine.StepHook, so it works with every
// front end: commands are read from a terminal while the program runs in
// the window or headless.
package debugger

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"assembly-emulator/disasm"
	"assembly-emulator/emulator"
	"assembly-emulator/machine"
)

// Debugger attaches to a machine and runs a command loop whenever the
// program is stopped.
type Debugger struct {
	m   *machine.Machine
	src *Source
	in  io.Reader
	out *console

	// Seg is the segment the program was loaded in: source addresses and
	// bare labels are offsets in it.
	Seg uint16

	lines       chan string
	interrupted chan struct{}
	pause       atomic.Bool
	sigStop     func()

	bps    []*breakpoint
	nextID int

	steps   int     // single steps left to take
	tmp     *tempBP // run until here (step over, "g addr")
	irqRet  *tempBP // return address of a hardware interrupt being hidden
	stepIRQ bool    // hide hardware interrupts (single-stepping)
	haltMsg bool    // reported a halt with interrupts disabled

	lastCmd  string
	dumpSeg  uint16 // "d" cursor
	dumpOff  uint32
	listSeg  uint16 // "u" cursor
	listOff  uint32
	srcFile  string // "l" cursor
	srcLine  int
	regs32   bool
	quit     bool
	running  bool // between Attach and Finish
	finished bool // the program is no longer running (post-mortem)
	started  bool
}

type breakpoint struct {
	id   int
	seg  uint16
	off  uint32
	lin  uint32
	hits int
}

type tempBP struct {
	lin uint32
	sp  uint16 // stop only when SP >= sp (skips recursion)
}

func linear(seg uint16, off uint32) uint32 { return uint32(seg)<<4 + off }

// New creates a debugger for m reading commands from in and writing to
// out. src may be nil when no source information is available.
func New(m *machine.Machine, src *Source, in io.Reader, out io.Writer) *Debugger {
	d := &Debugger{m: m, src: src, in: in, out: &console{w: out, nl: true}, Seg: m.CPU.CS(), nextID: 1}
	d.dumpSeg, d.dumpOff = m.CPU.DS(), 0x100
	d.listSeg, d.listOff = m.CPU.CS(), uint32(m.CPU.IP())
	d.lines = make(chan string)
	d.interrupted = make(chan struct{}, 1)
	return d
}

// Attach installs the step hook so the machine stops at its first
// instruction, and starts reading commands. Call before running.
func (d *Debugger) Attach() {
	d.running = true
	d.m.StepHook = d.hook
	go d.readLines()
}

// HandleInterrupt makes Ctrl-C pause the running program instead of
// killing the emulator. Returns a function that restores the default.
func (d *Debugger) HandleInterrupt() func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for range ch {
			d.pause.Store(true)
			select {
			case d.interrupted <- struct{}{}:
			default:
			}
		}
	}()
	d.sigStop = func() { signal.Stop(ch); close(ch) }
	return d.sigStop
}

// Output returns a writer for the program's console output that shares
// the debugger's stream, so prompts start on a fresh line after it.
func (d *Debugger) Output() io.Writer { return d.out }

// console tracks whether the last byte written was a newline.
type console struct {
	w  io.Writer
	nl bool
}

func (c *console) Write(p []byte) (int, error) {
	if len(p) > 0 {
		c.nl = p[len(p)-1] == '\n'
	}
	return c.w.Write(p)
}

func (d *Debugger) readLines() {
	sc := bufio.NewScanner(d.in)
	for sc.Scan() {
		d.lines <- sc.Text()
	}
	close(d.lines)
}

func (d *Debugger) printf(format string, args ...any) {
	fmt.Fprintf(d.out, format, args...)
}

// ---- execution control ----------------------------------------------------

// hook is called by the machine before every instruction.
func (d *Debugger) hook() bool {
	c := d.m.CPU
	if !d.started {
		d.started = true
		d.banner()
		return d.prompt("")
	}
	if d.pause.Swap(false) {
		d.tmp, d.irqRet, d.steps = nil, nil, 0
		return d.prompt("paused")
	}
	lin := linear(c.CS(), uint32(c.IP()))

	// Hardware interrupts delivered while single-stepping are stepped
	// over: run until the handler returns to the interrupted code.
	if d.stepIRQ && d.irqRet == nil && c.LastIRQ {
		ip := d.peek16(c.SS(), uint32(c.SP()))
		cs := d.peek16(c.SS(), uint32(c.SP())+2)
		d.irqRet = &tempBP{lin: linear(cs, uint32(ip)), sp: c.SP() + 6}
	}
	if d.irqRet != nil {
		if lin == d.irqRet.lin && c.SP() >= d.irqRet.sp {
			d.irqRet = nil
		} else {
			if bp := d.bpAt(lin); bp != nil {
				d.tmp, d.irqRet, d.steps = nil, nil, 0
				return d.prompt(d.bpHit(bp))
			}
			return true
		}
	}

	if c.Halted {
		if c.Flags&emulator.FlagIF == 0 {
			if !d.haltMsg {
				d.haltMsg = true
				d.tmp, d.irqRet, d.steps = nil, nil, 0
				return d.prompt("CPU halted with interrupts disabled")
			}
			return true
		}
		return true // waiting for an interrupt: nothing to show yet
	}
	d.haltMsg = false

	if d.steps > 0 {
		d.steps--
		if d.steps == 0 {
			return d.prompt("")
		}
		return true
	}
	if d.tmp != nil && lin == d.tmp.lin && c.SP() >= d.tmp.sp {
		d.tmp = nil
		return d.prompt("")
	}
	if bp := d.bpAt(lin); bp != nil {
		d.tmp = nil
		return d.prompt(d.bpHit(bp))
	}
	return true
}

func (d *Debugger) bpAt(lin uint32) *breakpoint {
	for _, b := range d.bps {
		if b.lin == lin {
			return b
		}
	}
	return nil
}

func (d *Debugger) bpHit(b *breakpoint) string {
	b.hits++
	return fmt.Sprintf("breakpoint %d hit", b.id)
}

func (d *Debugger) banner() {
	name := "program"
	if d.src != nil && d.src.Main != "" {
		name = filepath.Base(d.src.Main)
	}
	d.printf("asm-emu debugger: %s loaded at %04X:%04X. Type ? for help.\n", name, d.m.CPU.CS(), d.m.CPU.IP())
}

// Finish is called once the run loop has returned. It reports why and,
// when the program ended on its own, offers a post-mortem prompt.
func (d *Debugger) Finish(runErr error) {
	d.running = false
	d.finished = true
	if d.sigStop != nil {
		d.sigStop()
		d.sigStop = nil
	}
	switch {
	case d.quit || d.m.QuitRequested():
		return
	case runErr != nil:
		d.printf("\nemulator fault: %v\n", runErr)
	case d.m.Exited():
		d.printf("\nprogram exited with code %d\n", d.m.ExitCode())
	case d.m.Opts.MaxInsns > 0 && d.m.CPU.InsnCount >= d.m.Opts.MaxInsns:
		d.printf("\nstopped after %d instructions\n", d.m.CPU.InsnCount)
	default:
		// Stopped by the front end (window closed): the run goroutine may
		// still be parked at a prompt, so don't start another.
		return
	}
	d.prompt("")
}

// prompt shows the state and runs commands until one resumes execution.
// It returns false when the debugger wants the machine to stop.
func (d *Debugger) prompt(why string) bool {
	d.m.ForceFrame()
	if !d.out.nl {
		d.printf("\n")
	}
	if why != "" {
		d.printf("%s\n", why)
	}
	d.showState()
	for {
		d.printf("dbg> ")
		var line string
		var ok bool
		select {
		case line, ok = <-d.lines:
			if !ok {
				d.printf("\n")
				d.quit = true
				return false
			}
		case <-d.interrupted:
			d.pause.Store(false)
			d.printf("\n(interrupted; q quits)\n")
			continue
		}
		d.out.nl = true // the terminal echoed the newline
		line = strings.TrimSpace(line)
		if line == "" {
			line = d.lastCmd
		}
		resume := d.exec(line)
		if d.quit {
			return false
		}
		if resume {
			if d.finished {
				d.printf("the program is no longer running\n")
				continue
			}
			return true
		}
	}
}

// ---- commands ------------------------------------------------------------

const helpText = `commands (numbers are hex; counts are decimal; addr is off, seg:off,
a label, a register such as cs:ip, or file.asm:line)
  r [reg=val]      show registers / set one (r ax=1234, r zf=1, r ip=200)
  s [n]            step n instructions (into calls; hardware IRQs are hidden)
  n [n]            step over calls, ints, loops and rep strings
  g [addr]         go: run until a breakpoint, exit, or addr (Ctrl-C pauses)
  b [addr]         set a breakpoint (b alone lists them; bc n|* clears)
  u [addr] [n]     disassemble n instructions (u alone continues)
  d [addr] [len]   dump memory (d alone continues; default seg is ds)
  e addr b...      enter bytes ("strings" allowed) at addr
  k [n]            show n words of stack from ss:sp
  l [addr|line]    list source around addr (assembled sources only)
  f                show the x87 stack
  = expr           evaluate an expression (label, reg, hex, +/-)
  q                quit
An empty line repeats the last command.
`

// exec runs one command line; it returns true to resume execution.
func (d *Debugger) exec(line string) bool {
	args := strings.Fields(line)
	if len(args) == 0 {
		return false
	}
	cmd := strings.ToLower(args[0])
	rest := args[1:]
	d.lastCmd = line
	switch cmd {
	case "?", "h", "help":
		d.printf("%s", helpText)
	case "q", "quit", "exit":
		d.quit = true
	case "r", "reg", "regs":
		d.cmdRegs(rest)
	case "s", "t", "step", "si":
		n := d.count(rest, 1)
		d.steps, d.stepIRQ, d.tmp = n, true, nil
		return true
	case "n", "p", "next":
		return d.cmdNext(d.count(rest, 1))
	case "g", "c", "go", "cont", "continue":
		d.stepIRQ = false
		d.tmp = nil
		if len(rest) > 0 {
			seg, off, err := d.parseAddr(rest[0], d.m.CPU.CS())
			if err != nil {
				d.printf("%v\n", err)
				return false
			}
			d.tmp = &tempBP{lin: linear(seg, off)}
		}
		return true
	case "b", "bp", "break":
		d.cmdBreak(rest)
	case "bl":
		d.listBreakpoints()
	case "bc", "delete", "del":
		d.cmdClear(rest)
	case "u", "dis", "disas":
		d.cmdUnassemble(rest)
	case "d", "dump", "x":
		d.cmdDump(rest)
	case "e", "enter", "set":
		d.cmdEnter(rest)
	case "k", "stack", "bt":
		d.cmdStack(d.count(rest, 8))
	case "l", "list", "src":
		d.cmdList(rest)
	case "f", "fpu":
		d.showFPU()
	case "=", "?=", "eval", "print":
		d.cmdEval(rest)
	default:
		d.printf("unknown command %q (? for help)\n", args[0])
	}
	return false
}

// count parses a decimal count argument.
func (d *Debugger) count(args []string, def int) int {
	if len(args) == 0 {
		return def
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		d.printf("bad count %q, using %d\n", args[0], def)
		return def
	}
	return n
}

func (d *Debugger) cmdNext(n int) bool {
	c := d.m.CPU
	in := d.decodeAt(c.CS(), uint32(c.IP()))
	d.stepIRQ = true
	if in.Call || in.Int || in.Loop || in.RepString {
		d.steps = 0
		d.tmp = &tempBP{lin: linear(c.CS(), (uint32(c.IP())+uint32(in.Len()))&0xFFFF), sp: c.SP()}
		if n > 1 {
			d.printf("(stepping over one instruction)\n")
		}
		return true
	}
	d.tmp = nil
	d.steps = n
	return true
}

func (d *Debugger) cmdBreak(args []string) {
	if len(args) == 0 {
		d.listBreakpoints()
		return
	}
	seg, off, err := d.parseAddr(args[0], d.m.CPU.CS())
	if err != nil {
		d.printf("%v\n", err)
		return
	}
	bp := &breakpoint{id: d.nextID, seg: seg, off: off, lin: linear(seg, off)}
	d.nextID++
	d.bps = append(d.bps, bp)
	d.printf("breakpoint %d at %s\n", bp.id, d.addrString(seg, off))
}

func (d *Debugger) listBreakpoints() {
	if len(d.bps) == 0 {
		d.printf("no breakpoints\n")
		return
	}
	for _, b := range d.bps {
		d.printf("%2d  %s  hits=%d\n", b.id, d.addrString(b.seg, b.off), b.hits)
	}
}

func (d *Debugger) cmdClear(args []string) {
	if len(args) == 0 {
		d.printf("usage: bc <n>|*\n")
		return
	}
	if args[0] == "*" {
		d.bps = nil
		d.printf("all breakpoints cleared\n")
		return
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		d.printf("bad breakpoint number %q\n", args[0])
		return
	}
	for i, b := range d.bps {
		if b.id == id {
			d.bps = append(d.bps[:i], d.bps[i+1:]...)
			d.printf("breakpoint %d cleared\n", id)
			return
		}
	}
	d.printf("no breakpoint %d\n", id)
}

func (d *Debugger) cmdUnassemble(args []string) {
	seg, off := d.listSeg, d.listOff
	if len(args) > 0 {
		var err error
		if seg, off, err = d.parseAddr(args[0], d.m.CPU.CS()); err != nil {
			d.printf("%v\n", err)
			return
		}
	}
	n := 16
	if len(args) > 1 {
		n = d.count(args[1:], 16)
	}
	for i := 0; i < n; i++ {
		off = d.printInsn(seg, off&0xFFFF)
	}
	d.listSeg, d.listOff = seg, off&0xFFFF
}

func (d *Debugger) cmdDump(args []string) {
	seg, off := d.dumpSeg, d.dumpOff
	if len(args) > 0 {
		var err error
		if seg, off, err = d.parseAddr(args[0], d.m.CPU.DS()); err != nil {
			d.printf("%v\n", err)
			return
		}
	}
	n := 128
	if len(args) > 1 {
		v, err := d.parseNum(args[1])
		if err != nil || v == 0 {
			d.printf("bad length %q\n", args[1])
			return
		}
		n = int(v)
	}
	for n > 0 {
		row := 16 - int(off&0xF)
		if row > n {
			row = n
		}
		d.printf("%04X:%04X ", seg, off&0xFFFF)
		var asc strings.Builder
		for i := 0; i < 16; i++ {
			if i < int(off&0xF) || i >= int(off&0xF)+row {
				d.printf("   ")
				asc.WriteByte(' ')
				continue
			}
			b := d.peek8(seg, (off&^0xF)+uint32(i))
			sep := " "
			if i == 8 {
				sep = "-"
			}
			d.printf("%s%02X", sep, b)
			if b >= 0x20 && b < 0x7F {
				asc.WriteByte(b)
			} else {
				asc.WriteByte('.')
			}
		}
		d.printf("  %s\n", asc.String())
		off = (off + uint32(row)) & 0xFFFF
		n -= row
	}
	d.dumpSeg, d.dumpOff = seg, off
}

func (d *Debugger) cmdEnter(args []string) {
	if len(args) < 2 {
		d.printf("usage: e <addr> <byte|\"string\">...\n")
		return
	}
	seg, off, err := d.parseAddr(args[0], d.m.CPU.DS())
	if err != nil {
		d.printf("%v\n", err)
		return
	}
	var data []byte
	tail := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.Join(args, " ")), args[0]))
	for tail != "" {
		tail = strings.TrimLeft(tail, " \t,")
		if tail == "" {
			break
		}
		if tail[0] == '"' || tail[0] == '\'' {
			end := strings.IndexByte(tail[1:], tail[0])
			if end < 0 {
				d.printf("unterminated string\n")
				return
			}
			data = append(data, tail[1:1+end]...)
			tail = tail[end+2:]
			continue
		}
		sp := strings.IndexAny(tail, " \t,")
		tok := tail
		if sp >= 0 {
			tok, tail = tail[:sp], tail[sp:]
		} else {
			tail = ""
		}
		v, err := d.parseNum(tok)
		if err != nil {
			d.printf("%v\n", err)
			return
		}
		if v > 0xFF {
			d.printf("%s does not fit in a byte\n", tok)
			return
		}
		data = append(data, byte(v))
	}
	for i, b := range data {
		d.m.Mem.Write8(linear(seg, (off+uint32(i))&0xFFFF), b)
	}
	d.printf("wrote %d bytes at %s\n", len(data), d.addrString(seg, off))
}

func (d *Debugger) cmdStack(n int) {
	c := d.m.CPU
	sp := uint32(c.SP())
	for i := 0; i < n; i++ {
		off := (sp + uint32(2*i)) & 0xFFFF
		v := d.peek16(c.SS(), off)
		mark := "    "
		if i == 0 {
			mark = "SP->"
		} else if off == uint32(c.BP()) {
			mark = "BP->"
		}
		note := ""
		if name, dist, ok := d.src.Nearest(uint32(v)); ok && dist < 0x100 && v >= 0x100 {
			if dist == 0 {
				note = "  " + name
			} else {
				note = fmt.Sprintf("  %s+0x%x", name, dist)
			}
		}
		d.printf("%s %04X:%04X  %04X%s\n", mark, c.SS(), off, v, note)
	}
}

func (d *Debugger) cmdList(args []string) {
	if d.src == nil || len(d.src.lines) == 0 {
		d.printf("no source information (run a .asm file to get it)\n")
		return
	}
	file, line := d.srcFile, d.srcLine
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil {
			file, line = d.src.Main, n
		} else {
			seg, off, err := d.parseAddr(args[0], d.m.CPU.CS())
			if err != nil {
				d.printf("%v\n", err)
				return
			}
			l, ok := d.src.Containing(d.progOffset(seg, off))
			if !ok {
				d.printf("no source for %s\n", d.addrString(seg, off))
				return
			}
			file, line = l.File, l.Line
		}
		line -= 5
	} else if file == "" {
		l, ok := d.src.Containing(d.progOffset(d.m.CPU.CS(), uint32(d.m.CPU.IP())))
		if !ok {
			d.printf("no source for the current instruction\n")
			return
		}
		file, line = l.File, l.Line-5
	}
	if line < 1 {
		line = 1
	}
	cur, _ := d.src.Containing(d.progOffset(d.m.CPU.CS(), uint32(d.m.CPU.IP())))
	for i := 0; i < 10; i++ {
		text, ok := d.src.Text(file, line+i)
		if !ok {
			if i == 0 {
				d.printf("cannot read %s\n", file)
			}
			break
		}
		mark := "  "
		if cur.File == file && cur.Line == line+i {
			mark = "=>"
		}
		d.printf("%s %4d  %s\n", mark, line+i, text)
	}
	d.srcFile, d.srcLine = file, line+10
}

func (d *Debugger) cmdEval(args []string) {
	if len(args) == 0 {
		d.printf("usage: = <expr>\n")
		return
	}
	expr := strings.Join(args, "")
	if strings.Contains(expr, ":") {
		seg, off, err := d.parseAddr(expr, d.m.CPU.DS())
		if err != nil {
			d.printf("%v\n", err)
			return
		}
		d.printf("%04X:%04X  linear %05X\n", seg, off, linear(seg, off))
		return
	}
	v, err := d.parseNum(expr)
	if err != nil {
		d.printf("%v\n", err)
		return
	}
	d.printf("0x%X  %d", v, v)
	if v >= 0x20 && v < 0x7F {
		d.printf("  '%c'", v)
	}
	if name, ok := d.src.Label(uint32(v)); ok {
		d.printf("  %s", name)
	}
	d.printf("\n")
}

// ---- registers -----------------------------------------------------------

var regNames16 = []string{"ax", "bx", "cx", "dx", "sp", "bp", "si", "di"}
var regIndex = map[string]int{"ax": 0, "cx": 1, "dx": 2, "bx": 3, "sp": 4, "bp": 5, "si": 6, "di": 7}
var segIndex = map[string]int{"es": 0, "cs": 1, "ss": 2, "ds": 3, "fs": 4, "gs": 5}
var flagBits = map[string]uint32{
	"cf": emulator.FlagCF, "pf": emulator.FlagPF, "af": emulator.FlagAF, "zf": emulator.FlagZF,
	"sf": emulator.FlagSF, "tf": emulator.FlagTF, "if": emulator.FlagIF, "df": emulator.FlagDF, "of": emulator.FlagOF,
}

func (d *Debugger) cmdRegs(args []string) {
	if len(args) == 0 {
		d.showState()
		return
	}
	spec := strings.ToLower(strings.Join(args, ""))
	if spec == "32" {
		d.regs32 = !d.regs32
		d.showRegs()
		return
	}
	name, val, ok := strings.Cut(spec, "=")
	if !ok {
		if len(args) >= 2 {
			name, val = strings.ToLower(args[0]), strings.Join(args[1:], "")
		} else {
			// Show a single register.
			if v, ok := d.regValue(name); ok {
				d.printf("%s=%X\n", strings.ToUpper(name), v)
			} else {
				d.printf("unknown register %q\n", name)
			}
			return
		}
	}
	v, err := d.parseNum(val)
	if err != nil {
		d.printf("%v\n", err)
		return
	}
	if err := d.setReg(name, uint32(v)); err != nil {
		d.printf("%v\n", err)
		return
	}
	d.showRegs()
}

// regValue returns the value of a named register.
func (d *Debugger) regValue(name string) (uint32, bool) {
	c := d.m.CPU
	name = strings.ToLower(name)
	if i, ok := regIndex[name]; ok {
		return uint32(c.R16(i)), true
	}
	if i, ok := segIndex[name]; ok {
		return uint32(c.Segs[i]), true
	}
	if strings.HasPrefix(name, "e") {
		if i, ok := regIndex[name[1:]]; ok {
			return c.R32(i), true
		}
	}
	if len(name) == 2 && (name[1] == 'l' || name[1] == 'h') {
		if i := strings.IndexByte("acdb", name[0]); i >= 0 {
			if name[1] == 'l' {
				return uint32(c.R8(i)), true
			}
			return uint32(c.R8(i + 4)), true
		}
	}
	switch name {
	case "ip":
		return uint32(c.IP()), true
	case "eip":
		return c.EIP, true
	case "fl", "flags", "eflags":
		return c.Flags, true
	}
	if bit, ok := flagBits[name]; ok {
		if c.Flags&bit != 0 {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func (d *Debugger) setReg(name string, v uint32) error {
	c := d.m.CPU
	name = strings.ToLower(name)
	if i, ok := regIndex[name]; ok {
		c.SetR16(i, uint16(v))
		return nil
	}
	if i, ok := segIndex[name]; ok {
		c.Segs[i] = uint16(v)
		return nil
	}
	if strings.HasPrefix(name, "e") {
		if i, ok := regIndex[name[1:]]; ok {
			c.SetR32(i, v)
			return nil
		}
	}
	if len(name) == 2 && (name[1] == 'l' || name[1] == 'h') {
		if i := strings.IndexByte("acdb", name[0]); i >= 0 {
			if name[1] == 'l' {
				c.SetR8(i, uint8(v))
			} else {
				c.SetR8(i+4, uint8(v))
			}
			return nil
		}
	}
	switch name {
	case "ip":
		c.EIP = uint32(uint16(v))
		return nil
	case "eip":
		c.EIP = v
		return nil
	case "fl", "flags", "eflags":
		c.Flags = v&^0x28 | 2
		return nil
	}
	if bit, ok := flagBits[name]; ok {
		c.SetFlag(bit, v != 0)
		return nil
	}
	return fmt.Errorf("unknown register %q", name)
}

// showState prints registers, the source line and the current instruction.
func (d *Debugger) showState() {
	d.showRegs()
	c := d.m.CPU
	if l, ok := d.src.LineAt(d.progOffset(c.CS(), uint32(c.IP()))); ok {
		if text, ok := d.src.Text(l.File, l.Line); ok {
			d.printf("%s:%d: %s\n", filepath.Base(l.File), l.Line, strings.TrimSpace(text))
		}
	}
	d.printInsn(c.CS(), uint32(c.IP()))
	d.listSeg, d.listOff = c.CS(), uint32(c.IP())
}

func (d *Debugger) showRegs() {
	c := d.m.CPU
	wide := d.regs32
	for i := 0; i < 8; i++ {
		if c.Regs[i]>>16 != 0 {
			wide = true
		}
	}
	if wide {
		d.printf("EAX=%08X EBX=%08X ECX=%08X EDX=%08X ESI=%08X EDI=%08X EBP=%08X ESP=%08X\n",
			c.Regs[0], c.Regs[3], c.Regs[1], c.Regs[2], c.Regs[6], c.Regs[7], c.Regs[5], c.Regs[4])
	} else {
		d.printf("AX=%04X  BX=%04X  CX=%04X  DX=%04X  SP=%04X  BP=%04X  SI=%04X  DI=%04X\n",
			c.AX(), c.BX(), c.CX(), c.DX(), c.SP(), c.BP(), c.SI(), c.DI())
	}
	d.printf("DS=%04X  ES=%04X  SS=%04X  CS=%04X  IP=%04X   %s\n",
		c.DS(), c.ES(), c.SS(), c.CS(), c.IP(), flagString(c.Flags))
	if c.Segs[emulator.SegFS] != 0 || c.Segs[emulator.SegGS] != 0 || c.EIP > 0xFFFF {
		d.printf("FS=%04X  GS=%04X  EIP=%08X\n", c.Segs[emulator.SegFS], c.Segs[emulator.SegGS], c.EIP)
	}
}

// flagString renders the flags DEBUG-style.
func flagString(f uint32) string {
	pick := func(bit uint32, on, off string) string {
		if f&bit != 0 {
			return on
		}
		return off
	}
	return strings.Join([]string{
		pick(emulator.FlagOF, "OV", "NV"), pick(emulator.FlagDF, "DN", "UP"), pick(emulator.FlagIF, "EI", "DI"),
		pick(emulator.FlagSF, "NG", "PL"), pick(emulator.FlagZF, "ZR", "NZ"), pick(emulator.FlagAF, "AC", "NA"),
		pick(emulator.FlagPF, "PE", "PO"), pick(emulator.FlagCF, "CY", "NC"),
	}, " ")
}

func (d *Debugger) showFPU() {
	f := &d.m.CPU.FPU
	tags := []string{"valid", "zero", "special", "empty"}
	for i := 0; i < 8; i++ {
		phys := (int(f.Top) + i) & 7
		tag := f.Tag[phys] & 3
		if tag == 3 {
			d.printf("ST%d  empty\n", i)
			continue
		}
		d.printf("ST%d  %-24.17g %s\n", i, f.R[phys], tags[tag])
	}
	d.printf("CW=%04X SW=%04X TOP=%d\n", f.CW, f.SW, f.Top)
}

// ---- memory, disassembly, addresses ----------------------------------------

func (d *Debugger) peek8(seg uint16, off uint32) byte {
	return d.m.Peek8(linear(seg, off&0xFFFF))
}

func (d *Debugger) peek16(seg uint16, off uint32) uint16 {
	return uint16(d.peek8(seg, off)) | uint16(d.peek8(seg, off+1))<<8
}

func (d *Debugger) decodeAt(seg uint16, off uint32) disasm.Insn {
	return disasm.Decode(func(o uint32) byte { return d.peek8(seg, o) }, off)
}

// progOffset converts seg:off to an offset in the program's segment when
// the address lies in it (for source lookups); otherwise it returns a
// value no source maps to.
func (d *Debugger) progOffset(seg uint16, off uint32) uint32 {
	lin := linear(seg, off)
	base := uint32(d.Seg) << 4
	if lin < base || lin-base > 0xFFFF {
		return 0xFFFFFFFF
	}
	return lin - base
}

// printInsn prints one instruction and returns the offset of the next.
func (d *Debugger) printInsn(seg uint16, off uint32) uint32 {
	if name, ok := d.src.Label(d.progOffset(seg, off)); ok {
		d.printf("%s:\n", name)
	}
	in := d.decodeAt(seg, off)
	var hex strings.Builder
	for _, b := range in.Bytes {
		fmt.Fprintf(&hex, "%02X", b)
	}
	note := ""
	if in.HasTarget {
		if name, ok := d.src.Label(d.progOffset(seg, in.Target)); ok {
			note = "  ; " + name
		}
	}
	d.printf("%04X:%04X %-16s %s%s\n", seg, off, hex.String(), in.Text, note)
	return (off + uint32(in.Len())) & 0xFFFF
}

func (d *Debugger) addrString(seg uint16, off uint32) string {
	s := fmt.Sprintf("%04X:%04X", seg, off)
	if name, dist, ok := d.src.Nearest(d.progOffset(seg, off)); ok {
		if dist == 0 {
			s += " (" + name + ")"
		} else if dist < 0x100 {
			s += fmt.Sprintf(" (%s+0x%x)", name, dist)
		}
	}
	return s
}

// parseAddr parses off, seg:off, a label, or file:line.
func (d *Debugger) parseAddr(s string, defSeg uint16) (uint16, uint32, error) {
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		left, right := s[:i], s[i+1:]
		if line, err := strconv.Atoi(right); err == nil && (left == "" || d.src.HasFile(left) || strings.Contains(left, ".")) {
			off, ok := d.src.Resolve(left, line)
			if !ok {
				return 0, 0, fmt.Errorf("no code at line %d of %s", line, orMain(left, d.src))
			}
			return d.Seg, off, nil
		}
		seg, err := d.parseNum(left)
		if err != nil {
			return 0, 0, err
		}
		off, err := d.parseNum(right)
		if err != nil {
			return 0, 0, err
		}
		return uint16(seg), uint32(off) & 0xFFFF, nil
	}
	if v, ok := d.lookupSymbol(s); ok {
		return d.Seg, v & 0xFFFF, nil
	}
	v, err := d.parseNum(s)
	if err != nil {
		return 0, 0, err
	}
	return defSeg, uint32(v) & 0xFFFF, nil
}

// lookupSymbol resolves a label or constant; a NASM local label (.name)
// is looked up inside the routine containing the current instruction.
func (d *Debugger) lookupSymbol(name string) (uint32, bool) {
	if v, ok := d.src.Lookup(name); ok {
		return v, true
	}
	if strings.HasPrefix(name, ".") {
		c := d.m.CPU
		if parent, _, ok := d.src.Nearest(d.progOffset(c.CS(), uint32(c.IP()))); ok {
			if i := strings.IndexByte(parent, '.'); i > 0 {
				parent = parent[:i]
			}
			return d.src.Lookup(parent + name)
		}
	}
	return 0, false
}

func orMain(name string, src *Source) string {
	if name == "" && src != nil {
		return filepath.Base(src.Main)
	}
	return name
}

// parseNum evaluates a value: hex by default (0x prefix or h suffix
// accepted), 'c' characters, registers, symbols, joined by + and -.
func (d *Debugger) parseNum(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("missing value")
	}
	var total int64
	sign := int64(1)
	i := 0
	if s[0] == '-' {
		sign, i = -1, 1
	}
	for {
		j := i
		for j < len(s) && s[j] != '+' && s[j] != '-' {
			j++
		}
		if j == i {
			return 0, fmt.Errorf("bad expression %q", s)
		}
		v, err := d.parseTerm(s[i:j])
		if err != nil {
			return 0, err
		}
		total += sign * int64(v)
		if j == len(s) {
			return uint64(total), nil
		}
		sign = 1
		if s[j] == '-' {
			sign = -1
		}
		i = j + 1
	}
}

func (d *Debugger) parseTerm(t string) (uint64, error) {
	if len(t) == 3 && t[0] == '\'' && t[2] == '\'' {
		return uint64(t[1]), nil
	}
	if v, ok := d.regValue(t); ok {
		return uint64(v), nil
	}
	if v, ok := d.lookupSymbol(t); ok {
		return uint64(v), nil
	}
	lower := strings.ToLower(t)
	switch {
	case strings.HasPrefix(lower, "0x"):
		return strconv.ParseUint(t[2:], 16, 64)
	case strings.HasSuffix(lower, "h"):
		return strconv.ParseUint(t[:len(t)-1], 16, 64)
	}
	v, err := strconv.ParseUint(t, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("bad value %q (hex, 'c', register or label)", t)
	}
	return v, nil
}

// Breakpoints returns the breakpoint addresses (for tests and tooling).
func (d *Debugger) Breakpoints() []uint32 {
	var out []uint32
	for _, b := range d.bps {
		out = append(out, b.lin)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
