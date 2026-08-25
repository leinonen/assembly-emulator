// Package machine wires the CPU, devices, BIOS and DOS into a PC and runs
// it with virtual-clock throttling.
package machine

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"assembly-emulator/emulator"
	"assembly-emulator/emulator/bios"
	"assembly-emulator/emulator/dos"
	emio "assembly-emulator/emulator/io"
	"assembly-emulator/loader"
)

// Options configure a machine.
type Options struct {
	CPUHz     uint64    // virtual CPU clock (default 40 MHz)
	Unlimited bool      // don't throttle to real time
	Root      string    // DOS file sandbox
	Stdout    io.Writer // host mirror of console output (nil = none)
	Clock     func() time.Time
	MaxInsns  uint64 // stop after this many instructions (0 = unlimited)
}

// Machine is a complete emulated PC.
type Machine struct {
	Opts Options
	CPU  *emulator.CPU
	Mem  *emulator.Memory
	Bus  *emio.Bus
	PIC  *emio.PIC
	PIT  *emio.PIT
	KBD  *emio.Keyboard
	VGA  *emio.VGA
	CMOS *emio.CMOS
	BIOS *bios.BIOS
	DOS  *dos.DOS

	// RenderFrames enables rendering a frame snapshot at every vertical
	// retrace (front ends set this; headless runs skip the cost).
	RenderFrames bool

	// Frame snapshot taken at every vertical retrace (for renderers).
	frameMu sync.Mutex
	frame   emio.Frame
	frameN  uint64

	cursorBlink uint64
	stdoutBuf   []byte
}

// New builds a machine with BIOS and DOS installed (no program loaded).
func New(opts Options) *Machine {
	if opts.CPUHz == 0 {
		opts.CPUHz = 40_000_000
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	m := &Machine{Opts: opts}
	m.CPU = emulator.NewCPU(emulator.Model386)
	m.Mem = m.CPU.Mem
	m.Bus = emio.NewBus()
	m.CPU.Bus = m.Bus
	m.PIC = emio.NewPIC()
	m.PIT = emio.NewPIT(m.PIC, opts.CPUHz)
	bios.RegisterClock(m.PIT, opts.CPUHz)
	m.KBD = emio.NewKeyboard(m.PIC, opts.CPUHz)
	m.KBD.SetA20(&m.Mem.A20)
	m.VGA = emio.NewVGA(opts.CPUHz)
	m.CMOS = emio.NewCMOS()
	m.CMOS.Clock = func() (int, int, int) {
		t := opts.Clock()
		return t.Hour(), t.Minute(), t.Second()
	}

	m.Bus.Map(0x20, 0x21, m.PIC)
	m.Bus.Map(0xA0, 0xA1, m.PIC)
	m.Bus.Map(0x40, 0x43, m.PIT)
	m.Bus.Map(0x61, 0x61, m.PIT)
	m.Bus.Map(0x60, 0x60, m.KBD)
	m.Bus.Map(0x64, 0x64, m.KBD)
	m.Bus.Map(0x70, 0x71, m.CMOS)
	m.Bus.Map(0x92, 0x92, &emio.A20Gate{Flag: &m.Mem.A20})
	m.Bus.Map(0x3B0, 0x3DF, m.VGA)
	m.Bus.Map(0x00, 0x0F, emio.Stub{}) // DMA
	m.Bus.Map(0x80, 0x8F, emio.Stub{}) // DMA pages / POST
	m.Bus.Map(0xC0, 0xDF, emio.Stub{})
	m.Bus.Map(0x201, 0x201, emio.Stub{Value: 0xF0}) // joystick: none

	m.Mem.MMIORead = m.VGA.Read
	m.Mem.MMIOWrite = m.VGA.Write

	m.CPU.PendingIRQ = m.PIC.Pending
	m.CPU.AckIRQ = m.PIC.Acknowledge
	m.CPU.Clock = m.advance
	m.CPU.OnHalt = m.onHalt
	m.VGA.OnVBlank = func() {
		if m.RenderFrames {
			m.snapshotFrame()
		}
	}

	bm := &bios.Machine{
		CPU: m.CPU, Mem: m.Mem, Bus: m.Bus, VGA: m.VGA, Keyboard: m.KBD,
		PIT: m.PIT, PIC: m.PIC, Clock: opts.Clock,
	}
	if opts.Stdout != nil {
		bm.Stdout = m.stdoutByte
	}
	m.BIOS = bios.Install(bm)
	m.DOS = dos.New(m.BIOS, opts.Root)
	m.DOS.Clock = opts.Clock
	bm.ExtCall = m.DOS.Call
	m.snapshotFrame()
	return m
}

// LoadCOM loads a .COM image and prepares it to run.
func (m *Machine) LoadCOM(image []byte, cmdTail string) error {
	m.DOS.BuildPSP(cmdTail)
	return loader.LoadCOM(m.CPU, dos.PSPSeg, image)
}

// advance is called by the CPU after each instruction.
func (m *Machine) advance(cycles uint64) {
	m.PIT.Advance(cycles)
	m.KBD.Advance(cycles)
	m.VGA.Advance(cycles)
}

// onHalt fast-forwards the clocks to the next timer or retrace event
// when the CPU halts with interrupts enabled.
func (m *Machine) onHalt(c *emulator.CPU) {
	if c.Flags&emulator.FlagIF == 0 {
		return
	}
	// Nothing to do here: the run loop handles halted stepping.
}

// snapshotFrame copies the visible screen for renderers.
func (m *Machine) snapshotFrame() {
	m.frameMu.Lock()
	defer m.frameMu.Unlock()
	m.cursorBlink++
	blink := (m.cursorBlink/8)%2 == 0
	cur := int(m.VGA.CRTC[0x0E])<<8 | int(m.VGA.CRTC[0x0F])
	m.VGA.Render(&m.frame, blink, cur, int(m.VGA.CRTC[0x0A]&0x1F), int(m.VGA.CRTC[0x0B]&0x1F))
	m.frameN++
}

// ForceFrame renders the current screen state immediately.
func (m *Machine) ForceFrame() { m.snapshotFrame() }

// Frame returns a copy of the latest rendered frame and its sequence number.
func (m *Machine) Frame(dst *emio.Frame) uint64 {
	m.frameMu.Lock()
	defer m.frameMu.Unlock()
	if dst.W != m.frame.W || dst.H != m.frame.H || len(dst.Pix) != len(m.frame.Pix) {
		dst.W, dst.H = m.frame.W, m.frame.H
		dst.Pix = make([]byte, len(m.frame.Pix))
	}
	copy(dst.Pix, m.frame.Pix)
	return m.frameN
}

// FrameNumber returns the latest frame sequence number without copying.
func (m *Machine) FrameNumber() uint64 {
	m.frameMu.Lock()
	defer m.frameMu.Unlock()
	return m.frameN
}

func (m *Machine) stdoutByte(b byte) {
	if m.Opts.Stdout == nil {
		return
	}
	// Translate CP437 to UTF-8 lazily per byte; CR is dropped.
	if b == '\r' {
		return
	}
	r := cp437ToRune(b)
	if b == '\n' || b == '\t' || (b >= 0x20 && b < 0x7F) {
		fmt.Fprintf(m.Opts.Stdout, "%c", b)
		return
	}
	fmt.Fprintf(m.Opts.Stdout, "%c", r)
}

// Exited reports whether the program terminated through DOS.
func (m *Machine) Exited() bool { return m.DOS.Exited }

// ExitCode returns the DOS exit code.
func (m *Machine) ExitCode() int { return m.DOS.ExitCode }

// Step executes one instruction (or an idle tick when halted).
func (m *Machine) Step() error {
	if m.CPU.Halted {
		// Fast-forward to the next event so idle loops don't spin.
		return m.idle()
	}
	return m.CPU.Step()
}

// idle advances the virtual clock while the CPU is halted until an
// interrupt becomes pending.
func (m *Machine) idle() error {
	if m.CPU.Flags&emulator.FlagIF == 0 || m.CPU.PendingIRQ == nil {
		// Halted with interrupts disabled: nothing will ever wake us.
		m.CPU.Cycles += 1000
		m.CPU.InsnCount++
		m.advance(1000)
		return nil
	}
	n := m.PIT.NextEventCycles()
	if v := m.VGA.CyclesToVBlank(); v < n {
		n = v
	}
	if n == 0 {
		n = 1
	}
	if n > m.Opts.CPUHz/1000 {
		n = m.Opts.CPUHz / 1000
	}
	m.CPU.Cycles += n
	m.CPU.InsnCount++ // an idle tick counts as one instruction for limits
	m.advance(n)
	return m.CPU.Step() // delivers the pending IRQ if any
}

// RunCycles runs for at least n virtual cycles.
func (m *Machine) RunCycles(n uint64) error {
	target := m.CPU.Cycles + n
	for m.CPU.Cycles < target && !m.CPU.Stopped() {
		if err := m.Step(); err != nil {
			return err
		}
		if m.Opts.MaxInsns > 0 && m.CPU.InsnCount >= m.Opts.MaxInsns {
			m.CPU.Stop()
		}
	}
	return nil
}

// Run executes until the program exits or Stop is called, throttling to
// the configured CPU speed unless Unlimited is set.
func (m *Machine) Run() error {
	start := time.Now()
	startCycles := m.CPU.Cycles
	slice := m.Opts.CPUHz / 1000 // 1 ms of virtual time
	for !m.CPU.Stopped() {
		if err := m.RunCycles(slice); err != nil {
			return err
		}
		if m.Opts.Unlimited {
			continue
		}
		virtual := time.Duration(float64(m.CPU.Cycles-startCycles) / float64(m.Opts.CPUHz) * float64(time.Second))
		real := time.Since(start)
		if virtual > real {
			time.Sleep(virtual - real)
		} else if real-virtual > 200*time.Millisecond {
			// We fell far behind (e.g. the host was paused): resync.
			start = time.Now()
			startCycles = m.CPU.Cycles
		}
	}
	return nil
}

// Stop halts execution.
func (m *Machine) Stop() { m.CPU.Stop() }

// KeyDown / KeyUp inject scancode-set-1 keys from a host front end.
func (m *Machine) KeyDown(code uint8, ext bool) { m.KBD.KeyDown(code, ext) }
func (m *Machine) KeyUp(code uint8, ext bool)   { m.KBD.KeyUp(code, ext) }

// TypeKey presses and releases a key.
func (m *Machine) TypeKey(code uint8, ext bool) {
	m.KBD.KeyDown(code, ext)
	m.KBD.KeyUp(code, ext)
}

var _ = os.Stdout
