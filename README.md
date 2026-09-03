# asm-emu — a DOS PC emulator with a built-in assembler

`asm-emu` runs real DOS `.COM` programs and NASM-syntax assembly sources on
an emulated 386-class PC with VGA graphics, written in Go.

- **Accurate CPU**: real x86 machine code (8086 through 386 real mode, plus
  the x87 FPU), validated instruction-by-instruction against the
  hardware-generated [SingleStepTests](https://github.com/SingleStepTests)
  suites (100% of the 8088 suite, 933/941 files of the 80386 suite; see
  [docs/ACCURACY.md](docs/ACCURACY.md)).
- **PC hardware**: PIT timer, PIC, keyboard controller, CMOS clock, and a
  register-level VGA (text modes, CGA modes, EGA/VGA planar modes, mode 13h,
  unchained "Mode X", DAC palette, real vertical/horizontal retrace timing on
  port `3DAh`).
- **Sound**: an AdLib-compatible OPL2 (Yamaha YM3812) FM synthesiser on
  ports `388h`/`389h` and the PC speaker (PIT channel 2 / port `61h`), mixed
  at 48 kHz into the window's audio output or a `-wav` file.
- **BIOS and DOS**: INT 10h/11h/12h/13h/15h/16h/1Ah services, a BIOS data
  area, an interrupt vector table you can hook (timer, keyboard, INT 1Ch),
  and the common INT 21h calls (console, files, memory, time, vectors,
  termination). Programs get a proper PSP, `CS=DS=ES=SS`, and `org 100h`.
- **NASM-compatible assembler**: macros, conditionals, `%include`, `times`,
  `$`/`$$`, local labels, full 16- and 32-bit addressing, all integer and
  x87 instructions. Real-world sources such as
  [fire-asm](https://github.com/runawaydevil/fire-asm) and the
  [Memories](https://github.com/cesarmiquel/memories-256b-msdos-intro) intro
  assemble byte-for-byte identical to NASM's output.
- **Front end**: an [Ebiten](https://ebitengine.org) window with keyboard
  input, plus headless runs, PNG screenshots and deterministic GIF
  recording.

## Build

```
go build -o asm-emu .
```

## Run

```
asm-emu run examples/plasma.asm          # assemble and run a source file
asm-emu run FIRE.COM                     # run a DOS .COM binary
asm-emu asm examples/fire.asm -o fire.com [-l fire.lst]   # assemble only
```

Useful `run` flags:

| Flag | Meaning |
|------|---------|
| `-speed 40` | virtual CPU speed in MHz (default 40); `-speed unlimited` runs as fast as possible |
| `-headless` | run without a window (console output goes to stdout) |
| `-max-insns N` | stop after N instructions |
| `-screenshot out.png` | write the final screen to a PNG |
| `-gif out.gif -gif-frames 150` | record the display to an animated GIF (deterministic, no window) |
| `-wav out.wav` | write the audio output (mono 16-bit 48 kHz) to a WAV file; works headless and with `-gif` |
| `-stats` | print instruction/cycle counts and the final CS:IP |
| `-debug` | start in the interactive debugger: step through the program and inspect registers and memory (see below) |

In the window, **F12** quits; all other keys are delivered to the program as
PC scancodes (ESC included). Programs that `int 21h`/`4Ch` keep their final
frame on screen until a key is pressed.

Files opened by the program are resolved inside the directory containing the
program (drive letters are ignored, `..` is refused).

## Debugging

`asm-emu run -debug program.asm` stops at the first instruction and reads
commands from the terminal (with or without `-headless`; in the window the
program runs while you type in the terminal). Running a `.asm` file gives the
debugger the source lines and labels, so breakpoints can be set by label or
by `file.asm:line` and every stop shows the source line.

```
$ asm-emu run -headless -debug examples/plasma.asm
asm-emu debugger: plasma.asm loaded at 0800:0100. Type ? for help.
AX=0000  BX=0000  CX=0000  DX=0000  SP=FFFE  BP=0000  SI=0000  DI=0000
DS=0800  ES=0800  SS=0800  CS=0800  IP=0100   NV UP EI PL NZ NA PO NC
plasma.asm:30: mov ax, 0x13
start:
0800:0100 B81300           mov ax,0x13
dbg> b x_loop
breakpoint 1 at 0800:0194 (x_loop)
dbg> g
breakpoint 1 hit
AX=0000  BX=01EF  CX=0140  DX=7F00  SP=FFFA  BP=0000  SI=02EF  DI=0000
DS=0800  ES=7000  SS=0800  CS=0800  IP=0194   NV UP EI PL ZR NA PE NC
plasma.asm:124: lodsb                   ; wave 1 for this X
x_loop:
0800:0194 AC               lodsb
dbg> n
```

| Command | Meaning |
|---------|---------|
| `s [n]` | step n instructions (into calls; hardware interrupts are stepped over) |
| `n [n]` | step over `call`, `int`, `loop` and `rep` string instructions |
| `g [addr]` | run until a breakpoint, program exit, or `addr`; Ctrl-C pauses |
| `b addr`, `bl`, `bc n` | set, list, clear breakpoints |
| `r [reg=val]` | show registers, or set one (`r ax=1234`, `r zf=1`, `r ip=200`) |
| `u [addr] [n]`, `d [addr] [len]`, `e addr bytes...` | disassemble, dump, edit memory |
| `k [n]`, `f`, `l [addr]`, `= expr` | stack words, x87 stack, source listing, evaluate |
| `q` | quit |

Numbers are hexadecimal (`0x100`, `100h` and `100` are the same); counts are
decimal. An address is `off`, `seg:off`, a label, a register pair such as
`es:di`, or `file.asm:line`; `+` and `-` combine terms (`d ds:si+2`). An
empty line repeats the last command. The full command list is in
[MANUAL.md](MANUAL.md#debugger).

## Writing programs

Sources use NASM syntax and are assembled as flat `.COM` binaries:

```nasm
org 100h
    mov ax, 13h
    int 10h
    push 0A000h
    pop es
    xor di, di
    mov cx, 64000
    xor al, al
.fill:
    stosb
    inc al
    loop .fill
    xor ah, ah
    int 16h          ; wait for a key
    mov ax, 4C00h
    int 21h
```

New to assembly? [docs/tutorial](docs/tutorial) is a guide in six parts that
starts at "hello world" and ends with plasma, fire, copper bars, a
starfield, a sine scroller, AdLib music and a small demo with a timeline —
every listing is a complete program you can run.

See [MANUAL.md](MANUAL.md) for the assembler reference, the supported
instruction set, BIOS/DOS services and I/O ports.

## Examples

The `examples/` directory holds mode 13h demos; run any of them with
`asm-emu run examples/<name>.asm` (ESC quits).

| | |
|---|---|
| **[plasma](examples/plasma.asm)** — classic plasma from a sine table<br>![plasma](examples/gifs/plasma.gif) | **[fire](examples/fire.asm)** — demoscene fire, DAC palette via 3C8h/3C9h<br>![fire](examples/gifs/fire.gif) |
| **[tunnel](examples/tunnel.asm)** — x87-built angle/depth tables (FPATAN, FSQRT)<br>![tunnel](examples/gifs/tunnel.gif) | **[rotozoom](examples/rotozoom.asm)** — 8.8 fixed-point rotating, zooming texture<br>![rotozoom](examples/gifs/rotozoom.gif) |
| **[cube](examples/cube.asm)** — rotating 3D wireframe cube<br>![cube](examples/gifs/cube.gif) | **[starfield](examples/starfield.asm)** — perspective starfield with rotation<br>![starfield](examples/gifs/starfield.gif) |
| **[sine_scroller](examples/sine_scroller.asm)** — text on a sine wave using the BIOS font<br>![sine_scroller](examples/gifs/sine_scroller.gif) | **[bouncing-line](examples/bouncing-line.asm)** — Bresenham line, double buffer + VSync<br>![bouncing-line](examples/gifs/bouncing-line.gif) |
| **[cracktro](examples/cracktro.asm)** — crack intro: DAC copper bars, starfield, typewriter pages, sine scroller and a three-channel AdLib (OPL2) tune<br>![cracktro](examples/gifs/cracktro.gif) | |

Others without a GIF: `mandelbrot` (x87, FCOMP/FSTSW/SAHF), `fpu_plasma`
(FSIN-generated table), `noise`, `gradient`, `rainbow`, `palette`,
`cp437_13h`, and a few small pixel/buffer sanity tests.

## Testing

```
go test ./...                          # unit tests, examples, corpus (fast sample of SingleStepTests)
SINGLESTEP_FULL=1 go test ./tests/singlestep   # the complete CPU suites (~10 min)
```

The CPU suites and the real-program corpus are optional test data; see
[docs/ACCURACY.md](docs/ACCURACY.md) and `tests/corpus_test.go` for where to
put them (`~/.cache/asm-emu/`).

## Limitations

Real mode only (no protected mode, no EMS/XMS beyond the HMA), no Sound
Blaster digital audio (only the OPL2 and the speaker), no mouse, no `.EXE`
loader yet, no MASM/TASM syntax. Timing is a simple virtual clock, not
cycle-accurate.
