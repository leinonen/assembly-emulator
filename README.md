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
| `-stats` | print instruction/cycle counts and the final CS:IP |

In the window, **F12** quits; all other keys are delivered to the program as
PC scancodes (ESC included). Programs that `int 21h`/`4Ch` keep their final
frame on screen until a key is pressed.

Files opened by the program are resolved inside the directory containing the
program (drive letters are ignored, `..` is refused).

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

Real mode only (no protected mode, no EMS/XMS beyond the HMA), no sound
output (the PIT speaker channel is modelled but silent), no mouse, no `.EXE`
loader yet, no MASM/TASM syntax. Timing is a simple virtual clock, not
cycle-accurate.
