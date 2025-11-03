# x86 Assembly Emulator - Instruction Set Manual

This manual documents all implemented x86 assembly instructions in the assembly emulator. The emulator supports a subset of the x86 instruction set with focus on 16-bit real mode operations and VGA graphics programming.

## Table of Contents

1. [Overview](#overview)
2. [Register Set](#register-set)
3. [Flags Register](#flags-register)
4. [Operand Types](#operand-types)
5. [Data Movement Instructions](#data-movement-instructions)
6. [Arithmetic Instructions](#arithmetic-instructions)
7. [Logical Instructions](#logical-instructions)
8. [Comparison Instructions](#comparison-instructions)
9. [Control Flow Instructions](#control-flow-instructions)
10. [I/O Port Instructions](#io-port-instructions)
11. [String Instructions](#string-instructions)
12. [Interrupt Instructions](#interrupt-instructions)
13. [Instruction Prefixes](#instruction-prefixes)
14. [VGA Graphics Programming](#vga-graphics-programming)

---

## Overview

The emulator implements **46 unique x86 assembly instructions** with support for:
- 16-bit and 8-bit register operations
- Memory addressing modes
- VGA Mode 13h graphics (320x200, 256 colors)
- Software interrupts (INT 10h, 16h, 21h)
- Stack operations
- String manipulation

---

## Register Set

### General Purpose Registers (16-bit)
- **AX** - Accumulator (AH:AL)
- **BX** - Base (BH:BL)
- **CX** - Counter (CH:CL)
- **DX** - Data (DH:DL)

### Index and Pointer Registers
- **SI** - Source Index
- **DI** - Destination Index
- **BP** - Base Pointer
- **SP** - Stack Pointer

### Segment Registers
- **CS** - Code Segment
- **DS** - Data Segment
- **ES** - Extra Segment
- **SS** - Stack Segment

### Special Registers
- **IP** - Instruction Pointer
- **FLAGS** - Flags Register

---

## Flags Register

The FLAGS register contains condition code bits that are set or cleared by various instructions:

| Flag | Bit | Name | Description |
|------|-----|------|-------------|
| **CF** | 0 | Carry Flag | Set when arithmetic operation generates a carry/borrow |
| **ZF** | 6 | Zero Flag | Set when result is zero |
| **SF** | 7 | Sign Flag | Set when result is negative (MSB = 1) |
| **OF** | 11 | Overflow Flag | Set when signed arithmetic overflow occurs |

---

## Operand Types

Instructions support the following operand types:

| Type | Syntax | Example | Description |
|------|--------|---------|-------------|
| **Register (16-bit)** | reg16 | `AX`, `BX`, `CX` | 16-bit general purpose register |
| **Register (8-bit)** | reg8 | `AL`, `AH`, `BL` | 8-bit register (high/low byte) |
| **Immediate (16-bit)** | imm16 | `1234`, `0x1000` | 16-bit constant value |
| **Immediate (8-bit)** | imm8 | `42`, `0xFF` | 8-bit constant value |
| **Memory** | [addr] | `[0x1000]` | Direct memory address |
| **Memory Indexed** | [reg] | `[BX]`, `[SI+10]` | Memory address in register with optional offset |

---

## Data Movement Instructions

### MOV - Move Data
**Opcode:** 0x01

Copies data from source to destination.

**Syntax:**
```assembly
MOV dest, src
```

**Examples:**
```assembly
MOV AX, 1234        ; Load immediate value into AX
MOV BX, AX          ; Copy AX to BX
MOV [0x1000], AX    ; Store AX to memory address 0x1000
MOV CX, [BX]        ; Load value from memory at [BX] into CX
MOV AL, 0xFF        ; Load 8-bit immediate into AL
```

**Flags:** None affected

---

### PUSH - Push to Stack
**Opcode:** 0x02

Pushes a 16-bit value onto the stack. Decrements SP by 2.

**Syntax:**
```assembly
PUSH src
```

**Examples:**
```assembly
PUSH AX             ; Push AX onto stack
PUSH 1234           ; Push immediate value onto stack
```

**Flags:** None affected

---

### POP - Pop from Stack
**Opcode:** 0x03

Pops a 16-bit value from the stack. Increments SP by 2.

**Syntax:**
```assembly
POP dest
```

**Examples:**
```assembly
POP AX              ; Pop top of stack into AX
POP BX              ; Pop top of stack into BX
```

**Flags:** None affected

---

### XCHG - Exchange
**Opcode:** 0x04

Exchanges values between two operands.

**Syntax:**
```assembly
XCHG dest, src
```

**Examples:**
```assembly
XCHG AX, BX         ; Swap AX and BX
XCHG CX, DX         ; Swap CX and DX
```

**Flags:** None affected

---

## Arithmetic Instructions

### ADD - Addition
**Opcode:** 0x10

Adds source to destination and stores result in destination.

**Syntax:**
```assembly
ADD dest, src
```

**Examples:**
```assembly
ADD AX, 10          ; AX = AX + 10
ADD BX, CX          ; BX = BX + CX
ADD [SI], AL        ; Add AL to byte at [SI]
```

**Flags:** CF, OF, ZF, SF

---

### SUB - Subtraction
**Opcode:** 0x11

Subtracts source from destination and stores result in destination.

**Syntax:**
```assembly
SUB dest, src
```

**Examples:**
```assembly
SUB AX, 5           ; AX = AX - 5
SUB BX, CX          ; BX = BX - CX
```

**Flags:** CF, OF, ZF, SF

---

### MUL - Unsigned Multiply
**Opcode:** 0x12

Multiplies AX by operand. Result stored in DX:AX (DX = high word, AX = low word).

**Syntax:**
```assembly
MUL src
```

**Examples:**
```assembly
MOV AX, 100
MUL BX              ; DX:AX = AX * BX
```

**Flags:** CF, OF (set if result exceeds 16 bits)

---

### DIV - Unsigned Division
**Opcode:** 0x13

Divides DX:AX by operand. Quotient stored in AX, remainder in DX.

**Syntax:**
```assembly
DIV src
```

**Examples:**
```assembly
MOV DX, 0
MOV AX, 1000
DIV BX              ; AX = quotient, DX = remainder
```

**Flags:** Undefined (division by zero causes error)

---

### INC - Increment
**Opcode:** 0x16

Increments operand by 1.

**Syntax:**
```assembly
INC dest
```

**Examples:**
```assembly
INC AX              ; AX = AX + 1
INC CX              ; CX = CX + 1
INC BYTE [SI]       ; Increment byte at [SI]
```

**Flags:** OF, ZF, SF (CF not affected)

---

### DEC - Decrement
**Opcode:** 0x17

Decrements operand by 1.

**Syntax:**
```assembly
DEC dest
```

**Examples:**
```assembly
DEC AX              ; AX = AX - 1
DEC CX              ; CX = CX - 1
```

**Flags:** OF, ZF, SF (CF not affected)

---

### NEG - Negate
**Opcode:** 0x18

Negates operand (two's complement).

**Syntax:**
```assembly
NEG dest
```

**Examples:**
```assembly
MOV AX, 100
NEG AX              ; AX = -100 (0xFF9C in two's complement)
```

**Flags:** CF, OF, ZF, SF

---

## Logical Instructions

### AND - Logical AND
**Opcode:** 0x20

Performs bitwise AND operation.

**Syntax:**
```assembly
AND dest, src
```

**Examples:**
```assembly
AND AX, 0x00FF      ; Mask high byte of AX
AND BX, CX          ; BX = BX AND CX
```

**Flags:** CF=0, OF=0, ZF, SF

---

### OR - Logical OR
**Opcode:** 0x21

Performs bitwise OR operation.

**Syntax:**
```assembly
OR dest, src
```

**Examples:**
```assembly
OR AX, 0x0080       ; Set bit 7 in AL
OR BX, BX           ; Test if BX is zero (sets ZF)
```

**Flags:** CF=0, OF=0, ZF, SF

---

### XOR - Logical XOR
**Opcode:** 0x22

Performs bitwise XOR operation.

**Syntax:**
```assembly
XOR dest, src
```

**Examples:**
```assembly
XOR AX, AX          ; Clear AX (faster than MOV AX, 0)
XOR BX, 0xFFFF      ; Invert all bits in BX
```

**Flags:** CF=0, OF=0, ZF, SF

---

### NOT - Logical NOT
**Opcode:** 0x23

Inverts all bits in operand.

**Syntax:**
```assembly
NOT dest
```

**Examples:**
```assembly
MOV AL, 0x0F
NOT AL              ; AL = 0xF0
```

**Flags:** None affected

---

### SHL / SAL - Shift Left
**Opcode:** 0x24 / 0x26

Shifts bits left. Zero fills from right. Last bit shifted out goes to CF.

**Syntax:**
```assembly
SHL dest, count
SAL dest, count     ; Same as SHL
```

**Examples:**
```assembly
SHL AX, 1           ; Multiply AX by 2
SHL BX, 4           ; Multiply BX by 16
```

**Flags:** CF, ZF, SF

---

### SHR - Shift Right Logical
**Opcode:** 0x25

Shifts bits right. Zero fills from left. Last bit shifted out goes to CF.

**Syntax:**
```assembly
SHR dest, count
```

**Examples:**
```assembly
SHR AX, 1           ; Divide AX by 2 (unsigned)
SHR BX, 3           ; Divide BX by 8
```

**Flags:** CF, ZF, SF

---

### SAR - Shift Right Arithmetic
**Opcode:** 0x27

Shifts bits right while preserving sign bit. Last bit shifted out goes to CF.

**Syntax:**
```assembly
SAR dest, count
```

**Examples:**
```assembly
SAR AX, 1           ; Divide AX by 2 (signed)
```

**Flags:** CF, ZF, SF

---

### ROL - Rotate Left
**Opcode:** 0x28

Rotates bits left. Leftmost bit moves to rightmost position.

**Syntax:**
```assembly
ROL dest, count
```

**Examples:**
```assembly
ROL AX, 1           ; Rotate AX left by 1 bit
```

**Flags:** CF (limited flag support)

---

### ROR - Rotate Right
**Opcode:** 0x29

Rotates bits right. Rightmost bit moves to leftmost position.

**Syntax:**
```assembly
ROR dest, count
```

**Examples:**
```assembly
ROR AX, 1           ; Rotate AX right by 1 bit
```

**Flags:** CF (limited flag support)

---

## Comparison Instructions

### CMP - Compare
**Opcode:** 0x30

Compares destination and source by performing subtraction without storing result.

**Syntax:**
```assembly
CMP dest, src
```

**Examples:**
```assembly
CMP AX, 100         ; Compare AX with 100
JE  equal_label     ; Jump if AX == 100
```

**Flags:** CF, OF, ZF, SF

---

### TEST - Logical Test
**Opcode:** 0x31

Performs AND operation without storing result. Used to test if bits are set.

**Syntax:**
```assembly
TEST dest, src
```

**Examples:**
```assembly
TEST AX, 0x0001     ; Test if bit 0 is set
JNZ  bit_set        ; Jump if bit is set
```

**Flags:** CF=0, OF=0, ZF, SF

---

## Control Flow Instructions

### JMP - Unconditional Jump
**Opcode:** 0x40

Jumps unconditionally to target address.

**Syntax:**
```assembly
JMP label
```

**Examples:**
```assembly
JMP start
JMP [BX]            ; Indirect jump
```

**Flags:** None affected

---

### Conditional Jumps (Signed)

| Instruction | Opcode | Condition | Description |
|------------|--------|-----------|-------------|
| **JE / JZ** | 0x41 | ZF=1 | Jump if Equal / Zero |
| **JNE / JNZ** | 0x42 | ZF=0 | Jump if Not Equal / Not Zero |
| **JG / JNLE** | 0x43 | ZF=0 AND SF=OF | Jump if Greater |
| **JGE / JNL** | 0x44 | SF=OF | Jump if Greater or Equal |
| **JL / JNGE** | 0x45 | SF≠OF | Jump if Less |
| **JLE / JNG** | 0x46 | ZF=1 OR SF≠OF | Jump if Less or Equal |

**Examples:**
```assembly
CMP AX, BX
JE  equal           ; Jump if AX == BX
JG  greater         ; Jump if AX > BX (signed)
JL  less            ; Jump if AX < BX (signed)
```

---

### Conditional Jumps (Unsigned)

| Instruction | Opcode | Condition | Description |
|------------|--------|-----------|-------------|
| **JA / JNBE** | 0x47 | CF=0 AND ZF=0 | Jump if Above |
| **JAE / JNB** | 0x48 | CF=0 | Jump if Above or Equal |
| **JB / JNAE** | 0x49 | CF=1 | Jump if Below |
| **JBE / JNA** | 0x4A | CF=1 OR ZF=1 | Jump if Below or Equal |

**Examples:**
```assembly
CMP AX, BX
JA  above           ; Jump if AX > BX (unsigned)
JB  below           ; Jump if AX < BX (unsigned)
```

---

### CALL - Call Subroutine
**Opcode:** 0x4B

Calls a subroutine by pushing return address onto stack and jumping to target.

**Syntax:**
```assembly
CALL label
```

**Examples:**
```assembly
CALL draw_pixel
CALL [BX]           ; Indirect call
```

**Flags:** None affected

---

### RET - Return from Subroutine
**Opcode:** 0x4C

Returns from subroutine by popping return address from stack.

**Syntax:**
```assembly
RET
```

**Flags:** None affected

---

### Loop Instructions

| Instruction | Opcode | Condition | Description |
|------------|--------|-----------|-------------|
| **LOOP** | 0x4D | CX≠0 after decrement | Decrement CX and jump if CX≠0 |
| **LOOPZ** | 0x4E | CX≠0 AND ZF=1 | Loop while zero flag set |
| **LOOPNZ** | 0x4F | CX≠0 AND ZF=0 | Loop while zero flag clear |

**Examples:**
```assembly
MOV CX, 100
loop_start:
    ; ... loop body ...
    LOOP loop_start     ; Repeat 100 times
```

**Flags:** None affected (except for decrementing CX)

---

## I/O Port Instructions

### IN - Input from Port
**Opcode:** 0x60

Reads a byte from I/O port into AL.

**Syntax:**
```assembly
IN AL, port
IN AL, DX           ; Port number in DX
```

**Examples:**
```assembly
MOV DX, 0x3DA
IN  AL, DX          ; Read VGA status register
```

**Flags:** None affected

---

### OUT - Output to Port
**Opcode:** 0x61

Writes a byte from AL to I/O port.

**Syntax:**
```assembly
OUT port, AL
OUT DX, AL          ; Port number in DX
```

**Examples:**
```assembly
MOV DX, 0x3C8
MOV AL, 0
OUT DX, AL          ; Select palette index 0
```

**Flags:** None affected

---

## String Instructions

### MOVSB - Move String Byte
**Opcode:** 0x70

Moves byte from DS:SI to ES:DI, then increments SI and DI.

**Syntax:**
```assembly
MOVSB
REP MOVSB           ; Repeat CX times
```

**Examples:**
```assembly
MOV SI, source
MOV DI, dest
MOV CX, 100
REP MOVSB           ; Copy 100 bytes
```

**Flags:** None affected

---

### MOVSW - Move String Word
**Opcode:** 0x71

Moves word from DS:SI to ES:DI, then increments SI and DI by 2.

**Syntax:**
```assembly
MOVSW
REP MOVSW           ; Repeat CX times
```

**Flags:** None affected

---

### STOSB - Store String Byte
**Opcode:** 0x72

Stores AL at ES:DI, then increments DI.

**Syntax:**
```assembly
STOSB
REP STOSB           ; Repeat CX times
```

**Examples:**
```assembly
MOV AL, 0
MOV DI, buffer
MOV CX, 1000
REP STOSB           ; Fill 1000 bytes with zero
```

**Flags:** None affected

---

### STOSW - Store String Word
**Opcode:** 0x73

Stores AX at ES:DI, then increments DI by 2.

**Syntax:**
```assembly
STOSW
REP STOSW           ; Repeat CX times
```

**Examples:**
```assembly
MOV AX, 0xFFFF
MOV DI, buffer
MOV CX, 500
REP STOSW           ; Fill 1000 bytes with 0xFF
```

**Flags:** None affected

---

## Interrupt Instructions

### INT - Software Interrupt
**Opcode:** 0x50

Calls interrupt handler specified by interrupt number.

**Syntax:**
```assembly
INT number
```

**Supported Interrupts:**

#### INT 10h - Video BIOS Services

**Function AH=00h - Set Video Mode**
```assembly
MOV AH, 0x00
MOV AL, 0x13        ; Mode 13h: 320x200, 256 colors
INT 0x10
```

**Function AH=10h - Set Palette Register**
```assembly
MOV AH, 0x10
MOV AL, 0x10        ; Set individual DAC register
MOV BX, index       ; Palette index (0-255)
MOV DH, red         ; Red component (0-63)
MOV CH, green       ; Green component (0-63)
MOV CL, blue        ; Blue component (0-63)
INT 0x10
```

#### INT 16h - Keyboard BIOS Services

**Function AH=00h - Read Keystroke (Blocking)**
```assembly
MOV AH, 0x00
INT 0x16
; Returns: AH = scan code, AL = ASCII character
```

**Function AH=01h - Check for Keystroke (Non-blocking)**
```assembly
MOV AH, 0x01
INT 0x16
; Sets ZF=1 if key available, ZF=0 if no key
```

#### INT 21h - DOS Services

**Function AH=4Ch - Exit Program**
```assembly
MOV AH, 0x4C
MOV AL, 0           ; Exit code
INT 0x21
```

**Flags:** Varies by interrupt service

---

### NOP - No Operation
**Opcode:** 0x51

Does nothing. Used for timing or code alignment.

**Syntax:**
```assembly
NOP
```

**Flags:** None affected

---

### HLT - Halt
**Opcode:** 0x52

Halts CPU execution.

**Syntax:**
```assembly
HLT
```

**Flags:** None affected

---

## Instruction Prefixes

### REP - Repeat String Operation
**Byte:** 0xF3

Repeats the following string instruction CX times.

**Syntax:**
```assembly
REP instruction
```

**Works with:** MOVSB, MOVSW, STOSB, STOSW

**Examples:**
```assembly
MOV CX, 1000
REP MOVSB           ; Move 1000 bytes

MOV CX, 500
REP STOSW           ; Store 500 words
```

---

## VGA Graphics Programming

### Mode 13h - 320x200 256-color Graphics

**Setting Video Mode:**
```assembly
MOV AH, 0x00
MOV AL, 0x13
INT 0x10
```

**Video Memory:**
- Base address: 0xA0000
- Resolution: 320 x 200 pixels
- Format: Linear framebuffer, 1 byte per pixel
- Total size: 64000 bytes

**Writing Pixels:**
```assembly
; Calculate offset: Y * 320 + X
MOV AX, Y
MOV BX, 320
MUL BX              ; DX:AX = Y * 320
ADD AX, X           ; AX = Y * 320 + X
MOV DI, AX

; Write pixel
MOV AX, 0xA000
MOV ES, AX
MOV AL, color       ; Color index (0-255)
MOV [ES:DI], AL
```

### VGA Palette Programming

The VGA DAC (Digital-to-Analog Converter) provides 256 palette entries, each with 6-bit RGB components (0-63).

**I/O Ports:**
- **0x3C8** - DAC Write Index (select palette entry to write)
- **0x3C9** - DAC Data (write R, G, B sequentially)
- **0x3C7** - DAC Read Index
- **0x3DA** - Input Status Register 1 (VBlank status)

**Setting a Palette Entry:**
```assembly
MOV DX, 0x3C8
MOV AL, index       ; Palette index (0-255)
OUT DX, AL

MOV DX, 0x3C9
MOV AL, red         ; Red component (0-63)
OUT DX, AL
MOV AL, green       ; Green component (0-63)
OUT DX, AL
MOV AL, blue        ; Blue component (0-63)
OUT DX, AL
```

**VBlank Synchronization:**
```assembly
wait_vblank:
    MOV DX, 0x3DA
    IN  AL, DX
    AND AL, 0x08        ; Test bit 3 (VBlank)
    JZ  wait_vblank     ; Wait until VBlank active
```

### Example: Drawing a Pixel

```assembly
; Set Mode 13h
MOV AH, 0x00
MOV AL, 0x13
INT 0x10

; Set ES to video segment
MOV AX, 0xA000
MOV ES, AX

; Draw pixel at (100, 100) with color 15
MOV AX, 100         ; Y coordinate
MOV BX, 320
MUL BX              ; AX = Y * 320
ADD AX, 100         ; AX = Y * 320 + X
MOV DI, AX

MOV AL, 15          ; Color
MOV [ES:DI], AL
```

### Example: Filling Screen

```assembly
; Set Mode 13h
MOV AH, 0x00
MOV AL, 0x13
INT 0x10

; Fill screen with color
MOV AX, 0xA000
MOV ES, AX
MOV DI, 0
MOV AL, 1           ; Color
MOV CX, 64000       ; 320 * 200 pixels
REP STOSB           ; Fill entire screen
```

---

## Programming Examples

### Example 1: Simple Loop
```assembly
; Count from 0 to 9
MOV CX, 10
MOV BX, 0
count_loop:
    INC BX
    LOOP count_loop
HLT
```

### Example 2: Subroutine
```assembly
; Main program
CALL add_numbers
HLT

; Subroutine: Add AX and BX, result in AX
add_numbers:
    ADD AX, BX
    RET
```

### Example 3: Conditional Branch
```assembly
; Compare two numbers
MOV AX, 100
MOV BX, 50
CMP AX, BX
JG  greater
JL  less
JE  equal

greater:
    ; AX > BX
    JMP done
less:
    ; AX < BX
    JMP done
equal:
    ; AX == BX
done:
    HLT
```

### Example 4: VGA Graphics - Horizontal Line
```assembly
; Set Mode 13h
MOV AH, 0x00
MOV AL, 0x13
INT 0x10

; Draw horizontal line at Y=100
MOV AX, 0xA000
MOV ES, AX

MOV AX, 100         ; Y coordinate
MOV BX, 320
MUL BX              ; AX = Y * 320
MOV DI, AX

MOV AL, 15          ; White color
MOV CX, 320         ; Line width
REP STOSB           ; Draw line

HLT
```

---

## Notes and Limitations

1. **16-bit Mode Only:** The emulator operates in 16-bit real mode. No 32-bit or 64-bit instructions are supported.

2. **Limited Interrupt Support:** Only INT 10h (video), INT 16h (keyboard), and INT 21h (DOS exit) are implemented.

3. **Memory Model:** Simple linear memory model without segment:offset calculations for most operations.

4. **Floating Point:** No FPU instructions are supported.

5. **String Direction:** String instructions always increment (DF flag not implemented).

6. **Instruction Set:** This is a subset of the full x86 instruction set, focused on educational and graphics programming purposes.

---

## Quick Reference Card

### Data Movement
`MOV`, `PUSH`, `POP`, `XCHG`

### Arithmetic
`ADD`, `SUB`, `MUL`, `DIV`, `INC`, `DEC`, `NEG`

### Logical
`AND`, `OR`, `XOR`, `NOT`, `SHL`, `SHR`, `SAR`, `ROL`, `ROR`

### Comparison
`CMP`, `TEST`

### Control Flow
`JMP`, `JE/JZ`, `JNE/JNZ`, `JG/JNLE`, `JGE/JNL`, `JL/JNGE`, `JLE/JNG`, `JA`, `JAE`, `JB`, `JBE`, `CALL`, `RET`, `LOOP`, `LOOPZ`, `LOOPNZ`

### I/O
`IN`, `OUT`

### String
`MOVSB`, `MOVSW`, `STOSB`, `STOSW`, `REP`

### System
`INT`, `NOP`, `HLT`

### VGA Ports
- `0x3C8` - Palette Write Index
- `0x3C9` - Palette Data
- `0x3DA` - Status Register

---

*For more information and examples, see the included example programs in the `examples/` directory.*
