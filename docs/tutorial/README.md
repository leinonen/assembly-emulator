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
| **[Part 1 — The basics](01-basics.md)** | hello world and the shape of a `.COM` · registers, flags and loops · memory, the stack and procedures · the keyboard · macros and the preprocessor |
| **[Part 2 — Pixels](02-pixels.md)** | mode 13h and your first pixel · the palette · the frame loop: back buffer and vsync · lookup tables and the sine table · fixed-point maths |
| **[Part 3 — Effects](03-effects.md)** | plasma · fire · copper bars · starfield · a sine scroller |
| **[Part 4 — Sound](04-sound.md)** | the PC speaker · AdLib (OPL2) FM |
| **[Part 5 — Making a demo](05-demo.md)** | structure and a timeline · going faster · size coding |
| **[Debugging and reference](06-reference.md)** | what to try when it misbehaves · the mistakes everyone makes · a cheat sheet |

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
