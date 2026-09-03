package debugger

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"assembly-emulator/assembler"
	"assembly-emulator/machine"
)

const testProgram = `org 100h
start:
    mov ah, 9
    mov dx, msg
    int 21h
    call work
    mov ax, 4C00h
    int 21h
work:
    mov cx, 3
.loop:
    loop .loop
    ret
msg db 'Hi$'
`

func newSession(t *testing.T, program, script string) (*Debugger, *machine.Machine, *bytes.Buffer) {
	t.Helper()
	src := NewSource("prog.asm")
	image, err := assembler.Assemble([]byte(program), assembler.Options{
		Filename: "prog.asm", Listing: src.AddListing, Symbol: src.AddSymbol,
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	m := machine.New(machine.Options{Unlimited: true, Stdout: &out, Root: t.TempDir(), Clock: func() time.Time {
		return time.Date(1992, 1, 1, 12, 0, 0, 0, time.UTC)
	}})
	if err := m.LoadCOM(image, ""); err != nil {
		t.Fatal(err)
	}
	d := New(m, src, strings.NewReader(script), &out)
	m.Opts.Stdout = d.Output()
	d.Attach()
	return d, m, &out
}

func TestSession(t *testing.T) {
	script := strings.Join([]string{
		"s",      // mov ah,9
		"n",      // mov dx,msg
		"n",      // over int 21h
		"b work", // breakpoint on the label
		"bl",     //
		"g",      // run to it
		"k 2",    // return address on the stack
		"s",      // mov cx,3
		"n",      // over the loop
		"u start 3",
		"d msg 8",
		"= msg",
		"= 'A'+1",
		"r ax=1234",
		"r",
		"e msg 'Y' 24h",
		"bc 1",
		"g",
		"r", // post-mortem
		"q",
	}, "\n") + "\n"
	d, m, out := newSession(t, testProgram, script)
	err := m.Run()
	d.Finish(err)
	got := out.String()
	if err != nil {
		t.Fatal(err)
	}
	if !m.Exited() {
		t.Fatal("program did not exit")
	}
	for _, want := range []string{
		"prog.asm loaded at 0800:0100",
		"start:\n0800:0100 B409             mov ah,0x9",
		"AX=0900",                          // after the first step
		"IP=0107",                          // after stepping over int 21h
		"breakpoint 1 at 0800:010F (work)", // by label
		" 1  0800:010F (work)  hits=0",     // listing
		"breakpoint 1 hit",                 //
		"work:\n0800:010F B90300           mov cx,0x3",
		"SP-> 0800:FFFC  010A  start+0xa", // return address
		"CX=0003",                         // after mov cx,3
		"IP=0114",                         // stepped over the loop
		"CX=0000",                         //
		"0800:0102 BA1501           mov dx,0x115",
		"48 69 24",                         // dump of msg
		"0x115  277  msg",                  // expression
		"0x42  66  'B'",                    // character arithmetic
		"AX=1234",                          // register write
		"wrote 2 bytes at 0800:0115 (msg)", //
		"breakpoint 1 cleared",             //
		"program exited with code 0",       //
		"Hi\nAX=0924",                      // prompt on a fresh line after program output
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q\n---\n%s", want, got)
		}
	}
	if m.Mem.Read8(0x8000+0x115) != 'Y' {
		t.Errorf("e did not write memory")
	}
}

func TestPostMortemRefusesRun(t *testing.T) {
	d, m, out := newSession(t, testProgram, "g\ng\nq\n")
	err := m.Run()
	d.Finish(err)
	got := out.String()
	if !strings.Contains(got, "program exited with code 0") || !strings.Contains(got, "the program is no longer running") {
		t.Errorf("unexpected output:\n%s", got)
	}
	if !strings.Contains(got, "Hi") {
		t.Errorf("program output missing:\n%s", got)
	}
}

func TestQuit(t *testing.T) {
	d, m, out := newSession(t, testProgram, "s\nq\n")
	err := m.Run()
	d.Finish(err)
	if err != nil {
		t.Fatal(err)
	}
	if m.Exited() {
		t.Error("program should not have run to completion")
	}
	if !m.QuitRequested() {
		t.Error("QuitRequested not set")
	}
	if strings.Contains(out.String(), "exited") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
}

func TestEOFQuits(t *testing.T) {
	d, m, _ := newSession(t, testProgram, "")
	err := m.Run()
	d.Finish(err)
	if err != nil || !m.QuitRequested() {
		t.Errorf("err=%v quit=%v", err, m.QuitRequested())
	}
}

// Stepping must not land inside hardware interrupt handlers: a pending
// IRQ 0 is delivered after the nop and the debugger runs the handler to
// completion before stopping.
func TestStepHidesIRQ(t *testing.T) {
	prog := "org 100h\nsti\nnop\nnop\nmov ax,4C00h\nint 21h\n"
	d, m, out := newSession(t, prog, "s\ns\ns\nq\n")
	m.PIC.Raise(0)
	err := m.Run()
	d.Finish(err)
	got := out.String()
	if strings.Contains(got, "CS=F000") {
		t.Errorf("stepped into an interrupt handler:\n%s", got)
	}
	for _, want := range []string{"IP=0101", "IP=0102", "IP=0103"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	if m.CPU.LastVector != 8 && !strings.Contains(got, "IP=0103") {
		t.Errorf("IRQ 0 was not delivered")
	}
}

func TestBreakpointInLoopAndGoAddr(t *testing.T) {
	d, m, out := newSession(t, testProgram, "b work.loop\ng\ng\ng\nbc *\ng 0800:010D\nq\n")
	err := m.Run()
	d.Finish(err)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Count(got, "breakpoint 1 hit") != 3 {
		t.Errorf("expected three hits of the loop breakpoint:\n%s", got)
	}
	if !strings.Contains(got, "IP=010D") {
		t.Errorf("g addr did not stop at 010D:\n%s", got)
	}
	if m.Exited() {
		t.Error("program should have been stopped before exiting")
	}
}

func TestSourceLines(t *testing.T) {
	d, m, out := newSession(t, testProgram, "b :10\ng\nl\nq\n")
	err := m.Run()
	d.Finish(err)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"breakpoint 1 at 0800:010F (work)", "IP=010F"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	// The source file does not exist on disk, so listing reports that.
	if !strings.Contains(got, "cannot read prog.asm") {
		t.Errorf("expected unreadable-source message:\n%s", got)
	}
}

func TestParseNum(t *testing.T) {
	d, _, _ := newSession(t, testProgram, "")
	d.m.CPU.SetBX(0x10)
	cases := map[string]uint64{
		"100": 0x100, "0x100": 0x100, "100h": 0x100, "'A'": 0x41, "bx": 0x10, "bx+2": 0x12,
		"msg": 0x115, "msg-1": 0x114, "start+bx": 0x110, "-1": 0xFFFFFFFFFFFFFFFF,
		"work.loop": 0x112,
	}
	for in, want := range cases {
		got, err := d.parseNum(in)
		if err != nil || got != want {
			t.Errorf("parseNum(%q) = %#x, %v; want %#x", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "zz", "1+", "+", "nosuch"} {
		if _, err := d.parseNum(bad); err == nil {
			t.Errorf("parseNum(%q) succeeded", bad)
		}
	}
	// Local labels resolve inside the routine at CS:IP.
	if _, err := d.parseNum(".loop"); err == nil {
		t.Error(".loop resolved outside its routine")
	}
	d.m.CPU.SetIP(0x110)
	if v, err := d.parseNum(".loop"); err != nil || v != 0x112 {
		t.Errorf(".loop = %#x, %v", v, err)
	}
	d.m.CPU.SetIP(0x100)
	seg, off, err := d.parseAddr("es:di", 0)
	if err != nil || seg != 0x800 || off != 0 {
		t.Errorf("es:di = %04X:%04X, %v", seg, off, err)
	}
	if _, _, err := d.parseAddr("prog.asm:999", 0); err == nil {
		t.Error("line past the end resolved")
	}
}
