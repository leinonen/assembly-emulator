# Assembly Emulator

A command-line x86 assembly emulator with VGA Mode 13h graphics support, written in Go.

A homage to the oldschool demoscene programming of the 90s, where cool graphics demos and effects were crafted in raw assembly language. Write retro VGA graphics programs just like the legends did!

## Quick Start

```bash
make                              # Build
./asm-emu examples/noise.asm      # Run (opens graphics window)
make test                         # Test
```

Press **ESC** or close window to exit.

## Usage

```bash
./asm-emu <file.asm>
```

**Examples:** 
- `pixels.asm` (colored pixels)
- `bars.asm` (color bars)
- `noise.asm` (random pattern with looping until ESC pressed)

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

### Palette Control

Programs can customize the 256-color palette using VGA DAC ports:

```asm
; Set palette color 42 to bright purple (RGB: 63, 0, 63)
MOV DX, 0x3C8      ; DAC write index port
MOV AL, 42         ; Color index to modify
OUT DX, AL

MOV DX, 0x3C9      ; DAC data port
MOV AL, 63         ; Red component (0-63)
OUT DX, AL
MOV AL, 0          ; Green component
OUT DX, AL
MOV AL, 63         ; Blue component
OUT DX, AL
```

**Palette ports:**
- `0x3C8`: DAC write index (set color to modify)
- `0x3C9`: DAC data (write R, G, B in sequence, values 0-63)

### Keyboard Input

Programs can detect and read keyboard input via BIOS INT 16h:

```asm
main_loop:
    ; ... render graphics ...

    MOV AH, 0x01       ; Check for keystroke (non-destructive)
    INT 0x16
    JZ main_loop       ; ZF=1 means no key available

    MOV AH, 0x00       ; Read keystroke
    INT 0x16
    ; Returns: AH = scan code, AL = ASCII character

    CMP AL, 0x1B       ; Check if ESC key
    JE exit_program
    JMP main_loop

exit_program:
    HLT
```

**Supported keys:** ESC, Enter, Space, Backspace, A-Z (lowercase)

## Supported Instructions

**Data:** MOV, PUSH, POP, XCHG
**Arithmetic:** ADD, SUB, MUL, DIV, IMUL, IDIV, INC, DEC, NEG
**Logical:** AND, OR, XOR, NOT, SHL, SHR, SAL, SAR, ROL, ROR
**Control:** CMP, TEST, JMP, JE/JZ, JNE/JNZ, JG, JGE, JL, JLE, JA, JAE, JB, JBE, CALL, RET, LOOP
**I/O:** IN, OUT (for VGA palette control)
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

## Features

- **VGA Mode 13h graphics** - 320×200 resolution with 256-color palette
- **Customizable palette** - Modify colors via VGA DAC ports (0x3C8/0x3C9)
- **Keyboard input** - INT 16h for interactive programs
- **Window control** - Press ESC or close window to exit (works with infinite loops)
- **Complete x86 instruction set** - Data movement, arithmetic, logic, control flow

## Troubleshooting

**Black screen:** Write to 0xA000+, check addresses, remember MOV writes 2 bytes
**Won't assemble:** Use `h` suffix for hex starting with letters (`0A000h`)
**Partial fill:** ~256 bytes in protected area unreachable
**Program won't exit:** Close the VGA window or press ESC - the emulator will terminate gracefully

## Limitations

- 16-bit mode, no segments (flat model with VGA wrapping)
- Word-based memory (2 bytes per write)
- Limited instruction set
- No FPU

## Dependencies

- **github.com/hajimehoshi/ebiten/v2**: Graphics rendering

## About

This is a weekend project built together with Claude Code - an exploration of x86 assembly, emulator development, and the joy of retro programming. Educational project for learning assembly and bringing back the magic of the demoscene era.

## License

MIT
