# Debugging and reference

## Debugging

There is no debugger here, so lean on the tools that exist:

| Symptom | Try |
|---------|-----|
| nothing happens, program hangs | `-max-insns 5000000 -stats` — the final `CS:IP` tells you where it spun |
| the screen is black | `-screenshot out.png`; check `ES`, and check that the palette is not all zeros |
| it works then crashes | a fault vectors through the IVT like real hardware: divide error (INT 0), invalid opcode (INT 6), segment limit (INT 13) |
| output looks *almost* right | `-gif out.gif -gif-frames 60` and step through the frames |
| "did it assemble the way I meant?" | `asm ... -l out.lst` shows the bytes for every line |

The mistakes everyone makes at least once:

- **Forgetting `cld`.** With `DF` set, `stosb` walks backwards through
  memory and paints the screen in reverse.
- **Confusing `DS` and `ES`.** `stosb` writes to `ES:DI`, `lodsb` reads from
  `DS:SI`, and ordinary `[label]` uses `DS`. If your palette buffer ends up
  drawn on screen, `ES` was still pointing at video memory.
- **Signed versus unsigned jumps.** `jl` after `cmp ax, 200` is a different
  test from `jb`. Coordinates: unsigned. Anything that can be negative:
  signed.
- **`loop` with `CX = 0`** runs 65536 times, not zero.
- **Not clearing `DX` before `div`,** or not doing `cwd`/`cbw` before
  `idiv`. A stale `DX` gives a divide error or a wild quotient.
- **A byte counter where you needed a word.** `mov cl, 200` then
  `loop` is fine; `mov cx, 320` and then testing `cl` is not.
- **Crossing a 64 KB segment boundary.** Offsets wrap inside a segment: a
  buffer at `7000h` is exactly 64 KB, and `[di+640]` at the very end wraps
  to the start. Real-mode addressing hides this until it does not.
- **Writing the DAC while the beam is drawing.** Palette changes belong
  inside the vertical blank; otherwise you see them tear across the picture.
- **Assuming the assembler knows the operand size.** `mov [bx], 1` is
  ambiguous — write `mov byte [bx], 1`.

## Cheat sheet

**Program skeleton**

```nasm
org 100h
bits 16
start:
    mov ax, 0013h
    int 10h
    ; ... your code ...
    mov ax, 0003h
    int 10h
    mov ax, 4C00h
    int 21h
```

**The calls used in this guide**

| Call | Registers | Does |
|------|-----------|------|
| `int 10h` | `AX=0013h` | 320×200×256 graphics |
| `int 10h` | `AX=0003h` | 80×25 text |
| `int 10h` | `AH=0Ch`, `AL`=colour, `CX`=x, `DX`=y | plot a pixel (slow but easy) |
| `int 16h` | `AH=00h` | wait for a key → `AL` ASCII, `AH` scancode |
| `int 16h` | `AH=01h` | key waiting? `ZF=1` if not |
| `int 15h` | `AH=86h`, `CX:DX`=µs | wait |
| `int 1Ah` | `AH=00h` | tick count in `CX:DX` (18.2 Hz) |
| `int 21h` | `AH=02h`, `DL` | print a character |
| `int 21h` | `AH=09h`, `DX` | print a `$`-terminated string |
| `int 21h` | `AH=4Ch`, `AL`=code | exit |

**Ports**

| Port | Use |
|------|-----|
| `3C8h` / `3C9h` | DAC index / RGB data (6-bit values) |
| `3DAh` | bit 3 = vertical retrace, bit 0 = blanking |
| `40h`-`43h` | PIT; `43h`←`0B6h` then `42h`←divisor for the speaker |
| `61h` | bits 0-1 gate the speaker |
| `388h` / `389h` | AdLib OPL2 register index / data |

**Mode 13h arithmetic**

```nasm
offset = y * 320 + x        ; = (y << 8) + (y << 6) + x
segment A000h, 64000 bytes, one byte per pixel, colour = DAC index
```

**Where to go next**

- [MANUAL.md](../../MANUAL.md) — the full instruction set, every BIOS and DOS
  call, every emulated port.
- [examples/](../../examples) — plasma, fire, tunnel, rotozoom, cube,
  starfield, mandelbrot, sine scroller and a full cracktro, all runnable
  with `asm-emu run`.
- `asm-emu asm file.asm -l file.lst` — read what your source became.
- The corpus in `docs/ACCURACY.md` and the `tests/` directory show how the
  emulator is validated against real hardware traces, if you want to know
  how far you can trust it. (Answer: far enough that programs written here
  run on a real 386.)

---

[← Part 5 — Making a demo](05-demo.md) · [Contents](README.md)
