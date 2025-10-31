# Assembly Emulator

A command-line x86 assembly emulator with VGA Mode 13h graphics support, written in Go.

## Quick Start

```bash
make                              # Build
./asm-emu examples/pixels.asm    # Run (opens graphics window)
make test                         # Test
```

Press **ESC** or close window to exit.

## Usage

```bash
./asm-emu <file.asm>
```

**Examples:** `pixels.asm` (colored pixels), `bars.asm` (color bars), `noise.asm` (random pattern)

## Assembly Syntax

```asm
.code
    MOV AX, 13h        ; Set VGA Mode 13h
    INT 10h

    MOV AX, 0xA000     ; VGA memory base
    ADD AX, 1000       ; Offset
    MOV BX, 4          ; Red color
    MOV [AX], BX       ; Write pixel (2 bytes)
    HLT
```

**Number formats:** `100` (decimal), `0x64` / `64h` (hex), `0A000h` (hex with letter)

**Memory:** `[BX]`, `[SI]`, `[DI+10]` - 16-bit word operations (2 bytes)

## VGA Programming

**Mode 13h:** 320×200, 256 colors, linear memory at 0xA000

**Pixel address:** `0xA000 + (Y * 320 + X)`

**Colors 0-15:** Black, Blue, Green, Cyan, Red, Magenta, Brown, LightGray, DarkGray, LightBlue, LightGreen, LightCyan, LightRed, LightMagenta, Yellow, White

## Supported Instructions

**Data:** MOV, PUSH, POP, XCHG
**Arithmetic:** ADD, SUB, MUL, DIV, IMUL, IDIV, INC, DEC, NEG
**Logical:** AND, OR, XOR, NOT, SHL, SHR, SAL, SAR, ROL, ROR
**Control:** CMP, TEST, JMP, JE/JZ, JNE/JNZ, JG, JGE, JL, JLE, JA, JAE, JB, JBE, CALL, RET, LOOP
**Special:** INT 10h/16h/21h, NOP, HLT

## Registers

**16-bit:** AX, BX, CX, DX, SI, DI, BP, SP, IP
**8-bit:** AL/AH, BL/BH, CL/CH, DL/DH
**Flags:** CF, ZF, SF, OF

## Memory Map

```
0x0000-0x03FF : Protected (1KB)
0x0400-0x9FFF : VGA wrapped (39KB)
0xA000-0xFFFF : VGA primary (24KB)
```

VGA memory (64KB) accessed via wrapped addressing - values beyond 16-bit wrap to low memory.

## Troubleshooting

**Black screen:** Write to 0xA000+, check addresses, remember MOV writes 2 bytes
**Won't assemble:** Use `h` suffix for hex starting with letters (`0A000h`)
**Partial fill:** ~256 bytes in protected area unreachable

## Limitations

- 16-bit mode, no segments (flat model with VGA wrapping)
- Word-based memory (2 bytes per write)
- Limited instruction set
- No FPU

## Dependencies

- **github.com/hajimehoshi/ebiten/v2**: Graphics rendering

## License

Educational project for learning x86 assembly and emulator development.
