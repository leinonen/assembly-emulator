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

    MOV AX, 0xA000     ; Set ES to VGA segment
    MOV ES, AX

    XOR DI, DI         ; DI = 0 (offset)
    MOV AL, 4          ; Red color
    MOV [DI], AL       ; Write pixel to ES:DI (VGA memory)
    HLT
```

**Number formats:** `100` (decimal), `0x64` / `64h` (hex), `0A000h` (hex with letter)

**Memory:** `[BX]`, `[SI]`, `[DI+10]` - supports both byte and word operations

**Segments:** CS, DS, ES, SS - full x86 real mode segment support

## VGA Programming

**Mode 13h:** 320×200, 256 colors

**Memory access:** VGA memory is at segment 0xA000 (linear address 0xA0000)

```asm
; Set ES to VGA segment
MOV AX, 0xA000
MOV ES, AX

; Write pixel at position (X, Y)
; Offset = Y * 320 + X
MOV DI, 0           ; offset
MOV AL, 15          ; white color
MOV [DI], AL        ; writes to ES:DI
```

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

**General Purpose (16-bit):** AX, BX, CX, DX, SI, DI, BP, SP
**General Purpose (8-bit):** AL/AH, BL/BH, CL/CH, DL/DH
**Segment:** CS (Code), DS (Data), ES (Extra), SS (Stack)
**Special:** IP (Instruction Pointer)
**Flags:** CF, ZF, SF, OF

## Memory Map

**Total addressable memory:** 1MB (x86 real mode)

**VGA Memory:** Linear address 0xA0000-0xAFFFF (64,000 bytes)
- Access via segment 0xA000, offset 0x0000-0xF9FF
- 320×200 pixels = 64,000 bytes

**Segmentation:** Uses authentic x86 real mode addressing
- Linear address = (segment << 4) + offset
- All segments default to 0x0000 for backward compatibility

## Features

- **VGA Mode 13h graphics** - 320×200 resolution with 256-color palette
- **x86 real mode segments** - Full CS, DS, ES, SS support with authentic addressing
- **1MB addressable memory** - True 20-bit address space
- **Customizable palette** - Modify colors via VGA DAC ports (0x3C8/0x3C9)
- **Keyboard input** - INT 16h for interactive programs
- **Window control** - Press ESC or close window to exit (works with infinite loops)
- **Complete x86 instruction set** - Data movement, arithmetic, logic, control flow

## Troubleshooting

**Black screen:**
- Set ES to 0xA000: `MOV AX, 0xA000; MOV ES, AX`
- Use segment-based addressing: `MOV [DI], AL` writes to ES:DI
- Ensure program stays in a loop (busy-wait on keyboard) for graphics to render

**Won't assemble:** Use `h` suffix for hex starting with letters (`0A000h`)

**Program won't exit:** Close the VGA window or press ESC - the emulator will terminate gracefully

## Limitations

- 16-bit real mode only (no protected mode)
- Limited instruction set (no advanced x86 instructions)
- No FPU
- INT 16h function 0x00 is non-blocking (use function 0x01 in a loop for keyboard waits)

## Dependencies

- **github.com/hajimehoshi/ebiten/v2**: Graphics rendering

## About

This is a weekend project built together with Claude Code - an exploration of x86 assembly, emulator development, and the joy of retro programming. Educational project for learning assembly and bringing back the magic of the demoscene era.

## License

MIT
