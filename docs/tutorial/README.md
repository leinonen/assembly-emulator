# From `mov` to demoscene

A hands-on guide to writing x86 assembly for `asm-emu`. It starts with a
program that prints one line of text and ends with a plasma, a fire, copper
bars, a starfield, a sine scroller and an AdLib tune — the ingredients of a
1990s DOS intro.

Every numbered listing is a complete program. Save it, run it, change a
number, run it again: that loop is how assembly is learned.

## Contents

| | |
|---|---|
| **[Part 1 — The basics](01-basics.md)** | [1. Hello, DOS](01-basics.md#1-hello-dos) · [2. Registers, flags and loops](01-basics.md#2-registers-flags-and-loops) · [3. Memory, the stack and procedures](01-basics.md#3-memory-the-stack-and-procedures) · [4. Reading the keyboard](01-basics.md#4-reading-the-keyboard) · [5. Macros and the preprocessor](01-basics.md#5-macros-and-the-preprocessor) |
| **[Part 2 — Pixels](02-pixels.md)** | [6. Mode 13h and your first pixel](02-pixels.md#6-mode-13h-and-your-first-pixel) · [7. The palette](02-pixels.md#7-the-palette) · [8. The frame loop: back buffer and vsync](02-pixels.md#8-the-frame-loop-back-buffer-and-vsync) · [9. Lookup tables and the sine table](02-pixels.md#9-lookup-tables-and-the-sine-table) · [10. Fixed-point maths](02-pixels.md#10-fixed-point-maths) |
| **[Part 3 — Effects](03-effects.md)** | [11. Plasma](03-effects.md#11-plasma) · [12. Fire](03-effects.md#12-fire) · [13. Copper bars](03-effects.md#13-copper-bars) · [14. Starfield](03-effects.md#14-starfield) · [15. A sine scroller](03-effects.md#15-a-sine-scroller) |
| **[Part 4 — Sound](04-sound.md)** | [16. The PC speaker](04-sound.md#16-the-pc-speaker) · [17. AdLib (OPL2) FM](04-sound.md#17-adlib-opl2-fm) |
| **[Part 5 — Making a demo](05-demo.md)** | [18. Structure and a timeline](05-demo.md#18-structure-and-a-timeline) · [19. Going faster](05-demo.md#19-going-faster) · [20. Size coding](05-demo.md#20-size-coding) |
| **[Debugging and reference](06-reference.md)** | [Debugging](06-reference.md#debugging) · [Cheat sheet](06-reference.md#cheat-sheet) |

## Getting set up

Build the emulator once:

```
go build -o asm-emu .
```

Then, for every listing in this guide:

```
./asm-emu run lesson01.asm
```

`run` assembles the source in memory and boots it as a DOS `.COM` program.
Useful flags while you learn:

| Flag | What it does |
|------|--------------|
| `-headless` | no window; console output goes to your terminal |
| `-stats` | print instruction/cycle counts and the final `CS:IP` |
| `-max-insns N` | stop after N instructions (rescues infinite loops) |
| `-screenshot out.png` | write the final screen to a PNG |
| `-gif out.gif -gif-frames 150` | record an animated GIF (deterministic) |
| `-speed 8` | run at 8 MHz instead of the default 40 — a 286-ish feel |

In the window **F12** quits the emulator; every other key, ESC included, is
delivered to your program. Graphics programs in this guide quit on ESC.

You can also assemble to a real `.COM` file, with an optional listing file
that shows the bytes each line produced:

```
./asm-emu asm lesson01.asm -o lesson01.com -l lesson01.lst
```

The syntax is NASM's, so anything you learn here transfers to real NASM and
to real DOS machines. [MANUAL.md](../../MANUAL.md) is the reference for the
assembler, the instruction set, the BIOS/DOS calls and the I/O ports; this
guide is the tour.

---

[Part 1 — The basics →](01-basics.md)
