# assembly emulator

A DOS PC emulator in Go that executes **real x86 machine code** (386 real mode
+ x87) and assembles **NASM syntax**. Never reintroduce a private bytecode or
a custom assembler dialect: the goal is that unmodified old DOS programs and
NASM sources run unchanged.

## Layout

- `emulator/` CPU core: `cpu.go` (registers, memory access with real-mode
  limit checks, Step loop), `decode.go` (prefixes, ModR/M, exceptions via
  panic/recover with register rollback), `exec.go` + `ops_*.go` (one-byte and
  `0F` opcodes), `fpu*.go` (x87), `memory.go` (1 MB + HMA, A20, MMIO hook).
  `Model8088`/`Model386` switch the few places where the chips differ.
- `emulator/io/` devices: PIT, PIC, 8042 keyboard, CMOS, OPL2 (`opl2.go`,
  AdLib FM synth + timers), speaker (`speaker.go`, from PIT channel 2), a
  cycle-driven `mixer.go` (48 kHz, ring buffer + tap), VGA (registers,
  planar memory, CRT timing, renderer to RGBA).
- `emulator/bios/` BIOS in ROM at F000: stubs are real x86 that trap to Go
  with `F1 <svc>` (ICEBP, only honoured when CS=F000) so hooked/chained
  vectors work; BDA at 0040:0000; INT 10h/16h/1Ah/15h services.
- `emulator/dos/` INT 20h/21h, PSP, sandboxed files.
- `loader/` .COM loader. `machine/` wires everything, virtual-clock throttle.
- `assembler/` NASM-compatible assembler (lexer, preprocessor, expr, parser,
  operand, table, encoder, only-grow relaxation).
- `graphics/` Ebiten window, scancode keymap, GIF recorder, and `audio.go`
  (the only Ebiten-audio import; feeds the mixer to a player). `cp437/`,
  `font/`.
- `tests/singlestep/` SingleStepTests harness; `tests/corpus_test.go` runs
  every example and the real-program corpus.

## Conventions

- Flags are computed eagerly into a real EFLAGS image per operand width.
- Guest-visible faults are raised with `c.raise(vec)` (never Go errors);
  device reads never block — waiting is done by the guest (`sti; hlt`).
- All timing derives from `CPU.Cycles` (deterministic headless runs).
- Encoder choices should match NASM's output; `fire.asm` and `fx2.asm` in the
  corpus are byte-identical checks.

## Testing

`go test ./...` (fast). `SINGLESTEP_FULL=1 go test ./tests/singlestep` for
the full CPU suites; data lives in `~/.cache/asm-emu/singlestep/`. Known
deviations are documented in `docs/ACCURACY.md`; keep that list current.
