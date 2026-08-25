# CPU accuracy

The CPU core is validated against the hardware-generated
[SingleStepTests](https://github.com/SingleStepTests) suites: every opcode
file contains thousands of randomised instructions captured from a real
Intel 8088 and a real 80386EX, with the full register and memory state
before and after.

Run them with:

```
# fast sample (300 tests per opcode file, ~1 minute)
go test ./tests/singlestep

# everything (~10 minutes)
SINGLESTEP_FULL=1 go test ./tests/singlestep
```

The suites are looked for in `~/.cache/asm-emu/singlestep/{8088,80386}`
(override with `SINGLESTEP_8088_DIR` / `SINGLESTEP_386_DIR`); clone
`SingleStepTests/8088` and `SingleStepTests/80386` there. Tests are
skipped when the data is absent.

## Results

| Suite | Files | Status |
|-------|-------|--------|
| 8088 v2 (3.2 M instructions) | 324 | all pass |
| 80386 real mode (2.3 M instructions) | 941 | 933 pass, 8 excluded (below) |

Undefined flag bits are masked as the suites' metadata specifies.

## Modelled quirks

Things the emulator does because the hardware traces showed them:

- 8088: `PUSH SP` pushes the decremented value; divide faults push the
  address of the next instruction; `IDIV` faults on a quotient of exactly
  -128/-32768; a `REP` prefix on `IDIV` negates the quotient; shift counts
  are not masked; `SETMO` (`D0-D3 /6`); `0F` = `POP CS`; `60-6F`, `C0/C1`,
  `C8/C9` alias later opcodes; `F1` = `LOCK`; `AAA`/`AAS` adjust AL and AH
  as separate bytes; `DAA`/`DAS` use the 8086 gate logic for the high
  digit (`CF | b7·(b6 | b5 | b4·(¬AF ∧ low>9))`).
- 386: real-mode segment limit checks (`#GP`/`#SS`, no error code pushed,
  registers rolled back to the instruction start); `LOCK` on a
  non-lockable form or register destination raises `#UD`; 32-bit
  control transfers to an offset above `FFFFh` raise `#GP`; `POPA`
  commits registers popped before a fault; `POPAD` loads the high half
  of ESP from the skipped slot; 16-bit `SHLD`/`SHRD` with a count above 16
  compute `src<<(n-16) | src>>(32-n)`; 8-bit shifts by 16 or 24 report CF
  as a shift by 8; the SIB "no index" rows scale the base register; `BT`
  with a register bit offset accesses memory at operand-size granularity;
  far pointer loads wrap the second word within the segment; `o32 POP
  sreg` reads only 16 bits; `AAM 0` updates flags before the fault.

## Known deviations (excluded test files)

| Opcode | Deviation |
|--------|-----------|
| `60` (`PUSHAD`) | Partial stack writes before a `#SS` near the 64K wrap are not reproduced (8 / 2500 tests). |
| `A5`, `AB` with `a32 rep` | A `REP MOVS/STOS` that overwrites its own instruction bytes keeps executing from the prefetch queue on real hardware; there is no prefetch queue here (1 / 2500 each). |
| `F6 /7` (`IDIV r/m8`) | For some negative quotients that overflow, the 386 returns `AL=80h` and a truncated remainder instead of `#DE` (9 / 5000). |
| `D4 00` (`AAM 0`) | The flags image pushed by the divide fault differs in PF for some AH values (5 / 2500). |
| `E5`/`ED` (`IN eax`) | The 386EX test board returns `7FFFFFFF` from an open bus nondeterministically. |

## Other simplifications

- No prefetch queue, no cycle-accurate timing. Instruction costs are a
  small fixed table used only to drive the virtual clock (PIT, vertical
  retrace, keyboard repeat).
- Real mode only (no protected/V86 mode, no paging). `MOV CR0`/`LMSW`
  are accepted but do not switch modes.
- x87 arithmetic is done in IEEE double precision; the 80-bit formats are
  converted on load/store. Denormals and exceptions are not modelled
  (all exceptions masked).
- A20 is enabled by default so the HMA is reachable (as with HIMEM.SYS).
