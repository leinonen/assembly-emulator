package machine

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"assembly-emulator/emulator"
)

func newTestMachine(out *bytes.Buffer) *Machine {
	return New(Options{Unlimited: true, Stdout: out, Root: ".", Clock: func() time.Time {
		return time.Date(1992, 1, 1, 12, 0, 0, 0, time.UTC)
	}})
}

// hello.com: mov ah,9 / mov dx,msg / int 21h / mov ax,4C00h / int 21h / msg db 'Hello$'
var helloCOM = []byte{
	0xB4, 0x09, // mov ah,9
	0xBA, 0x0D, 0x01, // mov dx,010Dh
	0xCD, 0x21, // int 21h
	0xB8, 0x00, 0x4C, // mov ax,4C00h
	0xCD, 0x21, // int 21h
	0x90,
	'H', 'e', 'l', 'l', 'o', ',', ' ', 'D', 'O', 'S', '!', 0x0D, 0x0A, '$',
}

func TestHelloCOM(t *testing.T) {
	var out bytes.Buffer
	m := newTestMachine(&out)
	if err := m.LoadCOM(helloCOM, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if !m.Exited() {
		t.Fatal("program did not exit")
	}
	if got := out.String(); got != "Hello, DOS!\n" {
		t.Errorf("stdout = %q", got)
	}
	// Text should also be on the B8000 screen.
	var sb strings.Builder
	for i := 0; i < 11; i++ {
		sb.WriteByte(m.VGA.Planes[0][i])
	}
	if sb.String() != "Hello, DOS!" {
		t.Errorf("text buffer = %q", sb.String())
	}
}

// Program hooks INT 1Ch, spins until the counter reaches 3, then exits.
var timerCOM = []byte{
	// mov ax,cs / mov ds,ax
	0x8C, 0xC8, 0x8E, 0xD8,
	// mov ax,251Ch / mov dx,isr / int 21h
	0xB8, 0x1C, 0x25, 0xBA, 0x1E, 0x01, 0xCD, 0x21,
	// sti
	0xFB,
	// loop: cmp byte [count],3 / jb loop
	0x80, 0x3E, 0x24, 0x01, 0x03, 0x72, 0xF9,
	// mov ax,4C00h / int 21h
	0xB8, 0x00, 0x4C, 0xCD, 0x21,
	0x90, 0x90, 0x90, 0x90, 0x90,
	// isr at 011E: inc byte [cs:count] / iret   (2E FE 06 24 01 CF)
	0x2E, 0xFE, 0x06, 0x24, 0x01, 0xCF,
	// count at 0124
	0x00,
}

func TestTimerInterruptHook(t *testing.T) {
	m := newTestMachine(nil)
	if err := m.LoadCOM(timerCOM, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if !m.Exited() {
		t.Fatal("program did not exit (timer never fired?)")
	}
	if n := m.Mem.Read8(0x8000 + 0x124); n < 3 {
		t.Errorf("count=%d", n)
	}
	ticks := m.Mem.Read32(0x46C)
	if ticks == 0 {
		t.Errorf("BIOS tick counter did not advance")
	}
	// 3 ticks at 18.2 Hz ≈ 0.165 s of virtual time.
	if secs := float64(m.CPU.Cycles) / 40e6; secs < 0.1 || secs > 0.3 {
		t.Errorf("virtual time %.3fs, want ~0.165", secs)
	}
}

// Program waits for a key with INT 16h and prints it.
var keyCOM = []byte{
	0xB4, 0x00, 0xCD, 0x16, // mov ah,0 / int 16h
	0x8A, 0xD0, // mov dl,al
	0xB4, 0x02, 0xCD, 0x21, // mov ah,2 / int 21h
	0xB8, 0x00, 0x4C, 0xCD, 0x21,
}

func TestKeyboardBlockingRead(t *testing.T) {
	var out bytes.Buffer
	m := newTestMachine(&out)
	if err := m.LoadCOM(keyCOM, ""); err != nil {
		t.Fatal(err)
	}
	// Run a while with no key: must idle, not exit.
	if err := m.RunCycles(2_000_000); err != nil {
		t.Fatal(err)
	}
	if m.Exited() {
		t.Fatal("exited without a key")
	}
	m.TypeKey(0x1E, false) // 'a'
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "a" {
		t.Errorf("stdout=%q", out.String())
	}
}

func TestVSyncPolling(t *testing.T) {
	// mov ax,13h / int 10h ; wait for vblank end then start, twice; exit
	code := []byte{
		0xB8, 0x13, 0x00, 0xCD, 0x10,
		0xBA, 0xDA, 0x03, // mov dx,3DAh
		0xB9, 0x02, 0x00, // mov cx,2
		// w1: in al,dx / test al,8 / jnz w1
		0xEC, 0xA8, 0x08, 0x75, 0xFB,
		// w2: in al,dx / test al,8 / jz w2
		0xEC, 0xA8, 0x08, 0x74, 0xFB,
		0xE2, 0xF4, // loop w1
		0xB8, 0x00, 0x4C, 0xCD, 0x21,
	}
	m := newTestMachine(nil)
	if err := m.LoadCOM(code, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if !m.Exited() {
		t.Fatal("did not exit")
	}
	frames := m.VGA.Frames
	if frames < 2 || frames > 3 {
		t.Errorf("frames=%d want 2-3", frames)
	}
	if m.VGA.Mode != 0x13 {
		t.Errorf("mode=%02X", m.VGA.Mode)
	}
}

func TestDefaultPalette13h(t *testing.T) {
	m := newTestMachine(nil)
	m.VGA.SetMode(0x13)
	// Well-known entries of the VGA default palette.
	want := map[int][3]uint8{
		0: {0, 0, 0}, 1: {0, 0, 42}, 15: {63, 63, 63}, 16: {0, 0, 0}, 31: {63, 63, 63},
		32: {0, 0, 63}, 40: {63, 0, 0}, 44: {63, 63, 0}, 48: {0, 63, 0}, 56: {31, 31, 63},
		104: {0, 0, 28}, 176: {0, 0, 16}, 255: {0, 0, 0},
	}
	for i, w := range want {
		if m.VGA.DAC[i] != w {
			t.Errorf("DAC[%d]=%v want %v", i, m.VGA.DAC[i], w)
		}
	}
}

func TestFileIO(t *testing.T) {
	dir := t.TempDir()
	// create "OUT.TXT", write "hi", close, exit
	code := []byte{
		0xB8, 0x00, 0x3C, 0x31, 0xC9, 0xBA, 0x20, 0x01, 0xCD, 0x21, // mov ax,3C00h / xor cx,cx / mov dx,name / int 21h
		0x89, 0xC3, // mov bx,ax
		0xB4, 0x40, 0xB9, 0x02, 0x00, 0xBA, 0x28, 0x01, 0xCD, 0x21, // write 2 bytes from 0128
		0xB4, 0x3E, 0xCD, 0x21, // close
		0xB8, 0x00, 0x4C, 0xCD, 0x21,
		0x90,
		'O', 'U', 'T', '.', 'T', 'X', 'T', 0, // at 0120
		'h', 'i', // at 0128
	}
	m := New(Options{Unlimited: true, Root: dir})
	if err := m.LoadCOM(code, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	data, err := readFile(dir + "/OUT.TXT")
	if err != nil || string(data) != "hi" {
		t.Errorf("file: %q %v", data, err)
	}
	_ = emulator.FlagCF
}
