# asm-emu manual

1. [Assembler](#assembler)
2. [Instruction set](#instruction-set)
3. [Machine](#machine)
4. [BIOS services](#bios-services)
5. [DOS services](#dos-services)
6. [I/O ports](#io-ports)
7. [Timing](#timing)
8. [Testing](#testing)

For a step-by-step introduction rather than a reference, see
[docs/TUTORIAL.md](docs/TUTORIAL.md).

## Assembler

`asm-emu asm file.asm [-o out.com] [-l out.lst]` assembles NASM syntax to a
flat binary; `asm-emu run file.asm` assembles in memory and runs the result
as a `.COM` program. Output is 16-bit code (`bits 16`), so use `org 100h`
for DOS programs.

### Syntax

- **Comments** start with `;`. Lines may be continued with a trailing `\`.
- **Labels**: `name:` or `name` at the start of a line. Local labels start
  with `.` and are scoped to the previous non-local label (`.loop` after
  `main:` is `main.loop`). Macro-local labels use `%%name`.
- **Numbers**: `255`, `0FFh`, `0xFF`, `$FF`, `0b1111_1111`, `11111111b`,
  `377o`/`377q`, `255d`. Character constants `'A'`, `'AB'` (little-endian).
- **Strings**: `'...'` and `"..."` are literal; `` `...` `` processes C
  escapes (`\n`, `\r`, `\t`, `\xHH`, ...). Non-ASCII characters are
  transcoded to CP437 (`db '═══'` works).
- **Expressions**: NASM precedence: `||` `^^` `&&`, comparisons (`= == !=
  <> < > <= >=`), `|`, `^`, `&`, `<< >>`, `+ -`, `* / // % %%`, unary `- + ~
  ! seg`, `?:`. `$` is the current address, `$$` the start of the section.
  Labels may be used anywhere (`len equ $-msg`, `dw table+2`).

### Directives

| Directive | Meaning |
|-----------|---------|
| `org addr` | load address of the first byte (use `100h` for `.COM`) |
| `bits 16` / `cpu 386` / `use16` | accepted (only 16-bit code is generated) |
| `section .text` / `.data` / `.bss` (`segment` too) | `.text` and `.data` are emitted in that order; `.bss` reserves space after them without emitting bytes |
| `db dw dd dq dt` | data; `dd 1.5`/`dq 1.5`/`dt 1.5` accept floats; `db 'text', 13, 10, '$'` |
| `resb resw resd resq` | reserve zeroed space (`buf resb 256`) |
| `times n item` | repeat an instruction or data item (`times 510-($-$$) db 0`) |
| `align n[, item]` | pad to a multiple of `n` (with `nop` in `.text`, zero elsewhere, or the given item) |
| `incbin "file"[, skip[, len]]` | include a binary file |
| `name equ expr` | constant |
| `global`, `extern`, `default`, `absolute` | accepted and ignored |

### Preprocessor

`%define`/`%idefine`/`%xdefine` (with parameters: `%define SQ(x) ((x)*(x))`),
`%undef`, `%assign`, `%strlen`, `%macro name nparams[+]` ... `%endmacro`
(`%1`..`%9`, `%0`, `%{1:-1}`, `%%local`, `%+`), `%rep n` ... `%endrep`,
`%exitrep`, `%if expr` / `%elif` / `%else` / `%endif`, `%ifdef`, `%ifndef`,
`%ifidn`, `%ifidni`, `%ifmacro`, `%ifnum`, `%ifstr`, `%ifempty`,
`%include "file"` (searched next to the including file), `%error`,
`%warning`. `%push`/`%pop`/`%line`/`%pragma` are ignored.

### Operands

- Registers: `al ah ax eax ... sp bp si di esp ebp esi edi`, `es cs ss ds
  fs gs`, `st0`..`st7` (also `st(i)`), `cr0 cr2 cr3 cr4`, `dr0`-`dr7`.
- Memory: `[expr]`, `[bx+si+disp]`, `[bp-4]`, `[label+di]`, `[es:di]`,
  `es:[di]`, `[eax+ebx*4+8]`, `[ebx*4]`. `[byte bx+disp]` forces an 8-bit
  displacement. Size keywords `byte word dword qword tword` before the
  operand (`ptr` is accepted and ignored); an instruction whose only
  operands are memory and an immediate needs one (`mov word [bx], 1`).
- Jumps: `jmp short l`, `jmp near l`, `jmp far [m]`, `jmp seg:off`, `call
  far [m]`. Conditional jumps and `jmp` are automatically short when the
  target is in range, near otherwise.
- Prefixes: `rep repe repz repne repnz lock`, `o16 o32 a16 a32`, and a
  segment override before the mnemonic (`es lodsb`).

## Instruction set

Everything a 386 executes in real mode, plus the x87 FPU:

- **8086/8088**: `mov push pop xchg xlat lea lds les lahf sahf pushf popf`,
  `add adc sub sbb inc dec neg cmp mul imul div idiv aaa aas aam aad daa das
  cbw cwd`, `and or xor not test shl sal shr sar rol ror rcl rcr`, `jmp jcc
  jcxz loop loope loopne call ret retf int int3 into iret`, `movsb/w cmpsb/w
  scasb/w lodsb/w stosb/w` with `rep/repe/repne`, `in out`, `clc stc cmc
  cld std cli sti hlt wait nop`, undocumented `salc`.
- **186/286**: `push imm`, `imul r, r/m, imm`, shifts by immediate, `enter
  leave pusha popa bound insb/w outsb/w`.
- **386**: 32-bit registers and operand/address-size prefixes, `pushad popad
  pushfd popfd iretd cwde cdq movzx movsx setcc bt bts btr btc bsf bsr shld
  shrd`, `push/pop fs/gs`, `lfs lgs lss`, `jecxz`, `movsd cmpsd stosd lodsd
  scasd insd outsd`, `cmpxchg xadd bswap cmovcc rdtsc cpuid` (486+/Pentium
  extras that DOS programs commonly probe for).
- **x87**: `fld fst fstp fild fist fistp fisttp fbld fbstp fxch ffree
  fincstp fdecstp`, `fadd fsub fsubr fmul fdiv fdivr` (+`p` and integer
  forms), `fcom fcomp fcompp fucom fucomp fucompp fcomi fcomip fucomi fucomip
  ficom ficomp ftst fxam`, `fchs fabs fsqrt frndint fscale fprem fprem1
  fxtract f2xm1 fyl2x fyl2xp1 fsin fcos fsincos fptan fpatan`, `fld1 fldz
  fldpi fldl2e fldl2t fldlg2 fldln2`, `fldcw fnstcw fstcw fnstsw fstsw
  fldenv fnstenv fstenv fnsave fsave frstor fninit finit fnclex fclex fnop
  fwait fcmovcc`.

Faults are delivered through the interrupt vector table like on real
hardware: `#DE` (INT 0) for division errors, `#UD` (INT 6) for invalid
opcodes, `#BR` (INT 5) for `bound`, `#GP`/`#SS` (INT 13/12) for accesses
past a 64K segment limit.

## Machine

| Item | Value |
|------|-------|
| CPU | 386 real mode, default 40 MHz virtual clock, A20 enabled |
| Memory | 1 MB conventional + 64 KB HMA; IVT at 0000:0000, BIOS data area at 0040:0000, ROM at F000:0000 |
| Program load | PSP at segment 0800h (`int 20h` at offset 0, top-of-memory at 2, command tail at 80h); image at PSP:0100; `CS=DS=ES=SS=PSP`, `SP=FFFEh` with 0 pushed; free memory up to 9FC0h |
| Video | VGA: modes 00-03 (text), 04-06 (CGA), 07 (mono text), 0D/0E/10/12 (planar 16-colour), 13h (320x200x256); unchained "Mode X" via the sequencer/CRTC registers; text framebuffer at B800:0000 |
| Keyboard | 8042 at ports 60h/64h with scancode set 1 make/break codes and typematic repeat; INT 9 fills the BIOS 16-key buffer |
| Timer | 8254 PIT at 1.193182 MHz; channel 0 raises IRQ 0 (INT 8, 18.2 Hz by default), channel 2 drives the speaker |
| Sound | AdLib OPL2 (YM3812) at ports 388h/389h (also 228h/229h), PC speaker from PIT channel 2 / port 61h; mixed at 48 kHz to the window's audio output or `-wav` |
| Exit | `int 20h`, `int 21h/4Ch`, `int 21h/00h`, or `ret` with the pushed 0 (returns to PSP:0000 → `int 20h`) |

## BIOS services

**INT 10h (video)**: 00 set mode (bit 7 keeps memory), 01 cursor shape, 02
set cursor, 03 get cursor, 05 select page, 06/07 scroll up/down, 08 read
char/attribute, 09/0A write char (with/without attribute), 0B CGA
palette/border, 0C write pixel, 0D read pixel, 0E teletype (text and
graphics modes, scrolling), 0F get mode, 10h palette: 00/01/02/03/07/08/09
(attribute registers), 10 set DAC entry (BX, DH/CH/CL = R/G/B), 12 set DAC
block (ES:DX), 13 colour select, 15/17 read DAC, 18/19 PEL mask, 1A, 1B
grey-scale; 11h font info (1130h: 8x8 at F000:FA6E, 8x16 at F000:A000, INT
1Fh/43h pointers), 12h/BL=10h EGA info, 13h write string, 1Ah display
combination (VGA colour).

**INT 11h** equipment, **INT 12h** 640 KB, **INT 13h** no drives (CF=1),
**INT 14h/17h** no serial/printer, **INT 15h** 24h A20 control, 86h wait
(CX:DX µs), 88h no extended memory, C0h unsupported, **INT 16h** 00/10 read
key (blocks by halting the CPU), 01/11 check key (ZF), 02/12 shift flags, 05
push key, **INT 1Ah** 00/01 tick count (0040:006C, midnight flag), 02/04
RTC time/date (BCD), **INT 33h** no mouse (AX=0), **INT 2Fh** nothing
installed.

Hooking works as on a PC: change the vector with `int 21h/25h` or directly
in the IVT; INT 8 (timer) calls INT 1Ch, INT 9 reads port 60h and sends EOI
to the PIC. Handlers chained to the BIOS ones keep working.

## DOS services

**INT 20h** terminate. **INT 21h**: 00 terminate; 01/07/08 read char
(blocking, 01 echoes); 02 write char; 06 direct console I/O (DL=FFh polls
with ZF); 09 write `$`-string; 0A buffered line input; 0B input status; 0C
flush+read; 0E/19 drive (C:); 1A/2F set/get DTA; 25/35 set/get interrupt
vector; 26 create PSP; 2A/2C date/time (host clock), 2B/2D accepted; 30
version 5.00; 33 break flag; 34 InDOS; 36 disk space; 37 switch char; 38
country (US); 3C create, 3D open (`CON`, `NUL`, `PRN` devices supported), 3E
close, 3F read, 40 write (handles 1/2 print to the screen and stdout), 41
delete, 42 seek, 43 attributes, 44 ioctl (00/01/06/07/08), 45/46 dup, 47
cwd, 48/49/4A allocate/free/resize memory, 4C exit with code, 4D return
code, 4E/4F find first/next (DTA layout), 50/51/62 PSP, 56 rename, 57 file
date. File names are resolved case-insensitively inside the program's
directory; `\` and `/` are both accepted.

## I/O ports

| Ports | Device |
|-------|--------|
| 20h/21h, A0h/A1h | 8259 PIC (mask, EOI, ICW init) |
| 40h-43h | 8254 PIT (all read/write modes, latch, read-back) |
| 60h, 64h | keyboard controller (scancodes, A20 via output port) |
| 61h | speaker gate and data bits (audible), channel 2 output, refresh toggle bit |
| 70h/71h | CMOS RTC |
| 92h | fast A20 gate |
| 3C0h/3C1h | attribute controller (flip-flop reset by reading 3DAh) |
| 3C2h/3CCh | misc output |
| 3C4h/3C5h | sequencer (map mask, memory mode / chain-4 for Mode X) |
| 3C6h-3C9h | DAC: PEL mask, read/write index, data (6-bit, auto-increment, readable) |
| 3CEh/3CFh | graphics controller (set/reset, read map, write modes 0-3, bit mask, memory map) |
| 3D4h/3D5h (3B4h/3B5h) | CRTC (start address, cursor position/shape, offset, line compare, vertical display end) |
| 3DAh (3BAh) | input status 1: bit 3 = vertical retrace, bit 0 = display disabled (horizontal or vertical blanking) |
| 388h/389h, 228h/229h | AdLib OPL2: index/status and data (timers, status bits 5-7, the standard detection sequence) |

Unmapped ports read `FFh`. Every port access costs about 1 µs of virtual
time (ISA wait states), so the classic "read port 388h N times" delays used
by AdLib code take the intended time.

The classic retrace wait works exactly as on hardware:

```nasm
    mov dx, 3DAh
.w1: in al, dx
    test al, 8
    jnz .w1          ; wait until not in retrace
.w2: in al, dx
    test al, 8
    jz .w2           ; wait for the retrace to begin
```

## Timing

Every instruction advances a virtual cycle counter (roughly 2-4 cycles per
simple instruction, more for multiply/divide/string iterations). The PIT,
the CRT (70 Hz frames, vertical retrace in the last 8% of each frame,
horizontal blanking in the last 20% of each of 449 lines) and keyboard
repeat all run from that counter, so headless runs and GIF recordings are
deterministic. Audio is rendered from the same counter (48 000 samples per
virtual second), so `-wav` output is deterministic too. The window front end
throttles the counter to real time at the configured `-speed`; `hlt` skips
ahead to the next timer, retrace or OPL2 timer event.

## Testing

- `go test ./...` runs unit tests for the CPU, FPU, assembler, machine, the
  examples, and a fast sample of the SingleStepTests CPU suites.
- `SINGLESTEP_FULL=1 go test ./tests/singlestep` runs every test in the
  8088 and 80386 suites (see `docs/ACCURACY.md` for results and the list of
  known deviations).
- `go test ./tests -run TestCorpus` assembles and runs real-world programs
  from `~/.cache/asm-emu/corpus` (or `$ASM_EMU_CORPUS_DIR`): `fire-asm`,
  `memories`, `blobz`. The first two must assemble byte-identical to the
  `.COM` files their authors built with NASM.
