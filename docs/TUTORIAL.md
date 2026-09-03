# From `mov` to demoscene

A hands-on guide to writing x86 assembly for `asm-emu`. It starts with a
program that prints one line of text and ends with a plasma, a fire, copper
bars, a starfield, a sine scroller and an AdLib tune — the ingredients of a
1990s DOS intro.

Every numbered listing is a complete program. Save it, run it, change a
number, run it again: that loop is how assembly is learned.

**Contents**

- [0. Getting set up](#0-getting-set-up)
- [Part 1 — The basics](#part-1--the-basics)
  - [1. Hello, DOS](#1-hello-dos)
  - [2. Registers, flags and loops](#2-registers-flags-and-loops)
  - [3. Memory, the stack and procedures](#3-memory-the-stack-and-procedures)
  - [4. Reading the keyboard](#4-reading-the-keyboard)
  - [5. Macros and the preprocessor](#5-macros-and-the-preprocessor)
- [Part 2 — Pixels](#part-2--pixels)
  - [6. Mode 13h and your first pixel](#6-mode-13h-and-your-first-pixel)
  - [7. The palette](#7-the-palette)
  - [8. The frame loop: back buffer and vsync](#8-the-frame-loop-back-buffer-and-vsync)
  - [9. Lookup tables and the sine table](#9-lookup-tables-and-the-sine-table)
  - [10. Fixed-point maths](#10-fixed-point-maths)
- [Part 3 — Effects](#part-3--effects)
  - [11. Plasma](#11-plasma)
  - [12. Fire](#12-fire)
  - [13. Copper bars](#13-copper-bars)
  - [14. Starfield](#14-starfield)
  - [15. A sine scroller](#15-a-sine-scroller)
- [Part 4 — Sound](#part-4--sound)
  - [16. The PC speaker](#16-the-pc-speaker)
  - [17. AdLib (OPL2) FM](#17-adlib-opl2-fm)
- [Part 5 — Making a demo](#part-5--making-a-demo)
  - [18. Structure and a timeline](#18-structure-and-a-timeline)
  - [19. Going faster](#19-going-faster)
  - [20. Size coding](#20-size-coding)
- [Debugging and common mistakes](#debugging-and-common-mistakes)
- [Cheat sheet](#cheat-sheet)

---

## 0. Getting set up

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
to real DOS machines. [MANUAL.md](../MANUAL.md) is the reference for the
assembler, the instruction set, the BIOS/DOS calls and the I/O ports; this
guide is the tour.

---

## Part 1 — The basics

### 1. Hello, DOS

```nasm
; lesson01.asm - the smallest useful DOS program
org 100h                    ; a .COM image is loaded at offset 100h
bits 16                     ; 16-bit real mode

start:
    mov ah, 09h             ; DOS function 09h = print a '$'-terminated string
    mov dx, msg             ; DS:DX must point at the text
    int 21h                 ; ask DOS to do it

    mov ax, 4C00h           ; function 4Ch = exit, AL = exit code 0
    int 21h

msg: db 'Hello, world!', 13, 10, '$'
```

```
./asm-emu run -headless lesson01.asm
```

What each part means:

- **`org 100h`** — DOS loads a `.COM` program at offset `100h` inside its
  segment; the first `100h` bytes are the PSP (a DOS bookkeeping block).
  Without `org 100h` every address in your program would be off by 256.
- **`start:`** — a label, just a name for an address. Execution begins at
  the first byte of the file, so the first instruction is the entry point.
- **`mov ah, 09h`** — put the value `09h` into the register `AH`. `mov` is
  a copy, not a move; nothing is erased at the source.
- **`int 21h`** — a software interrupt. It hands control to DOS, which looks
  at `AH` to decide which service you wanted. This is how a DOS program does
  anything outside its own memory.
- **`db`** — "define bytes": the assembler writes those bytes into the
  program at that spot. `13, 10` are CR and LF, and `$` is the terminator
  that function `09h` insists on.

A `.COM` program is a *flat binary*: no headers, no relocations, just the
bytes. `CS`, `DS`, `ES` and `SS` all start pointing at the same segment, so
code, data and stack live in one 64 KB block. That is why the listings here
can refer to `msg` without any setup.

### 2. Registers, flags and loops

The 386 in real mode gives you eight 16-bit general registers. Four of them
split into byte halves:

| 16-bit | halves | traditional job |
|--------|--------|-----------------|
| `AX` | `AH`, `AL` | accumulator — arithmetic, I/O, DOS call numbers |
| `BX` | `BH`, `BL` | base — a memory pointer (`[bx]`), `xlat` base |
| `CX` | `CH`, `CL` | counter — `loop`, `rep`, shift counts |
| `DX` | `DH`, `DL` | data — I/O port number, high half of a product |
| `SI` | — | source index (`lodsb`, `movsb`) |
| `DI` | — | destination index (`stosb`, `movsb`) |
| `BP` | — | base pointer (stack frames) — free for you in a `.COM` |
| `SP` | — | stack pointer — leave it alone |

Each also has a 32-bit form (`EAX`, `ESI`, ...) which you can use freely
here, and there are the segment registers `CS DS ES SS FS GS` which pick
which 64 KB window of memory an address refers to.

Instructions set *flags* as a side effect: `ZF` (result was zero), `CF`
(carry/borrow), `SF` (sign), `OF` (signed overflow). Conditional jumps read
those flags. `cmp a, b` is a subtraction that throws the result away and
keeps only the flags.

```nasm
; lesson02.asm - print the alphabet
org 100h
bits 16

start:
    mov cx, 26              ; LOOP counts CX down to zero
    mov dl, 'A'             ; DOS function 02h prints the character in DL
.next:
    mov ah, 02h
    int 21h
    inc dl                  ; next letter
    loop .next              ; CX = CX - 1; jump if CX is not zero

    mov ah, 09h
    mov dx, crlf
    int 21h
    ret                     ; a .COM may also exit with RET

crlf: db 13, 10, '$'
```

Two new things:

- **`.next` is a local label.** Labels starting with a dot belong to the
  last ordinary label above them, so `.next` here really means `start.next`.
  Every procedure can have its own `.loop` without name clashes.
- **`ret` exits.** DOS pushes a zero on your stack before starting you, and
  offset 0 of the PSP holds an `int 20h`, so returning lands there and
  terminates. `mov ax, 4C00h` / `int 21h` is the tidier way (it can return an
  exit code), but you will see `ret` in size-optimised intros constantly:
  it is one byte.

The important loop instructions:

| Instruction | Meaning |
|-------------|---------|
| `jmp label` | always jump |
| `je`/`jz`, `jne`/`jnz` | jump if equal / not equal (`ZF`) |
| `jb`, `jae`, `ja`, `jbe` | **unsigned** below / above-or-equal / above / below-or-equal |
| `jl`, `jge`, `jg`, `jle` | **signed** less / greater-or-equal / greater / less-or-equal |
| `loop label` | decrement `CX`, jump while non-zero |
| `jcxz label` | jump if `CX` is already zero |

Mixing up the signed and unsigned families is the single most common
beginner bug. Screen coordinates are unsigned: use `jb`/`jae`. Anything that
can go negative (a velocity, a rotated coordinate) is signed: use
`jl`/`jge`.

### 3. Memory, the stack and procedures

Memory is addressed as `[segment:offset]`. In a `.COM` program `DS` already
points at your data, so `[label]` just works. Square brackets mean *the
contents of*: `mov ax, msg` loads the address, `mov ax, [msg]` loads the two
bytes stored there.

The stack grows downward from the top of your segment. `push` puts a word on
it, `pop` takes one off, `call` pushes the return address and `ret` pops it.
That makes procedures cheap, and it makes the stack a convenient place to
reverse a sequence — exactly what printing a number in decimal needs:

```nasm
; lesson03.asm - print numbers in decimal
org 100h
bits 16

start:
    mov ax, 12345
    call print_u16
    mov ax, 7
    call print_u16
    mov ax, 65535
    call print_u16
    mov ax, 4C00h
    int 21h

; print_u16: print AX as unsigned decimal, followed by CR/LF.
print_u16:
    mov bx, 10
    xor cx, cx              ; XOR with itself is the shortest way to zero
.split:
    xor dx, dx              ; DIV divides DX:AX, so clear the high half
    div bx                  ; AX = AX/10, DX = AX mod 10
    push dx                 ; stash the digit; the stack reverses them
    inc cx
    test ax, ax             ; TEST is AND without storing - here: is AX zero?
    jnz .split
.emit:
    pop dx
    add dl, '0'             ; 0..9 -> '0'..'9'
    mov ah, 02h
    int 21h
    loop .emit              ; one iteration per digit we pushed

    mov ah, 09h
    mov dx, crlf
    int 21h
    ret

crlf: db 13, 10, '$'
```

Notes worth keeping:

- **`div` is not symmetric.** `div bx` divides the 32-bit value `DX:AX` by
  `BX`. Forgetting to clear `DX` first gives nonsense — or a divide error
  (`INT 0`) when the quotient does not fit in 16 bits.
- **`xor reg, reg` and `test reg, reg`** are the idiomatic zero-and-compare.
  They are shorter than `mov reg, 0` and `cmp reg, 0`.
- **Procedures have no calling convention here.** You decide what is passed
  in which register and write it in a comment above the label. Do that
  every time; assembly you wrote last week is a stranger's code.

Ways to reserve and initialise memory:

```nasm
count:   dw 0                   ; one word (2 bytes), initialised
table:   db 1, 2, 3, 4          ; four bytes
text:    db 'hi', 0             ; bytes from a string
zeros:   times 256 db 0         ; 256 zero bytes
buffer:  resb 1024              ; 1024 bytes reserved, not stored in the file
LEN      equ $-text             ; $ is "here"; equ makes a constant
```

### 4. Reading the keyboard

DOS can read keys (`int 21h`, `AH=01h`), but demos use the BIOS keyboard
services because they can *check* for a key without waiting:

| Call | Effect |
|------|--------|
| `AH=00h`, `int 16h` | wait for a key; `AL` = ASCII, `AH` = scancode |
| `AH=01h`, `int 16h` | peek: `ZF=1` if no key is waiting, else `AL`/`AH` as above |

```nasm
; lesson04.asm - echo keys until ESC
org 100h
bits 16

start:
    mov ah, 09h
    mov dx, prompt
    int 21h
.loop:
    mov ah, 00h             ; wait for a key
    int 16h
    cmp al, 27              ; 27 = ESC
    je .done
    cmp al, 13              ; Enter: echo a CR/LF pair
    jne .print
    mov ah, 09h
    mov dx, crlf
    int 21h
    jmp .loop
.print:
    mov dl, al
    mov ah, 02h
    int 21h
    jmp .loop
.done:
    mov ah, 09h
    mov dx, bye
    int 21h
    ret

prompt: db 'Type something (ESC quits):', 13, 10, '$'
bye:    db 13, 10, 'bye!', 13, 10, '$'
crlf:   db 13, 10, '$'
```

The non-blocking form is what every listing from lesson 8 on uses to quit:

```nasm
    mov ah, 01h
    int 16h
    jz .no_key              ; nothing pressed - carry on with the frame
    xor ah, ah
    int 16h                 ; take the key out of the buffer
    cmp al, 27
    je .quit
.no_key:
```

### 5. Macros and the preprocessor

The assembler runs a NASM-compatible preprocessor before it assembles
anything. It costs zero bytes at runtime and saves an enormous amount of
typing.

```nasm
; lesson05.asm - constants, macros and repetition
org 100h
bits 16

%define GREETING 'compile-time'    ; plain text substitution
%assign LINES 3                    ; a preprocessor variable

%macro puts 1                      ; one parameter, referred to as %1
    mov dx, %1
    mov ah, 09h
    int 21h
%endmacro

%macro countdown 1                 ; a macro that expands a loop
    mov cx, %1
%%again:                           ; %% = a label unique to each expansion
    push cx
    mov dl, '*'
    mov ah, 02h
    int 21h
    pop cx
    loop %%again
%endmacro

start:
%rep LINES                         ; assemble the body LINES times
    puts msg
%endrep
    countdown 10
    puts crlf
    ret

msg:  db 'hello from ', GREETING, 13, 10, '$'
crlf: db 13, 10, '$'
```

The pieces you will actually use:

| Feature | Use |
|---------|-----|
| `NAME equ expr` | a constant: `VGA_SEG equ 0A000h` |
| `%define NAME body` | text substitution, optionally with parameters |
| `%macro name n` … `%endmacro` | a code template; `%1`..`%9` are the arguments, `%%label` a local label |
| `%rep n` … `%endrep` | unroll a loop at assembly time |
| `%if` / `%ifdef` / `%else` / `%endif` | conditional assembly |
| `%include "file.inc"` | pull in another source file |
| `times n item` | repeat one instruction or data item |

Constants deserve special emphasis. `mov ax, 0A000h` appears in every
graphics program ever written; `VGA_SEG equ 0A000h` and then
`mov ax, VGA_SEG` says *why*.

---

## Part 2 — Pixels

### 6. Mode 13h and your first pixel

Mode 13h is the mode the demoscene ran on: 320×200 pixels, 256 colours, and
— the important part — **one byte per pixel** in a single linear buffer at
segment `A000h`. The pixel at (x, y) is the byte at offset `y * 320 + x`.
No planes, no bit masks, no page registers. You write a byte, a pixel
changes.

```nasm
; lesson06.asm - set mode 13h and draw
org 100h
bits 16

VGA_SEG equ 0A000h

start:
    mov ax, 0013h           ; BIOS: AH=00h set video mode, AL=13h
    int 10h

    push VGA_SEG            ; ES -> video memory
    pop es                  ; (push imm / pop seg is the short way)

    ; a diagonal line
    xor cx, cx              ; CX = x
    xor dx, dx              ; DX = y
.diag:
    mov al, 15              ; colour 15 = white in the default palette
    call putpixel
    inc cx
    inc dx
    cmp dx, 200
    jb .diag

    ; a block of 80 colours
    mov dx, 60              ; y
.rows:
    mov cx, 120             ; x
.cols:
    mov al, dl              ; colour = the row number
    call putpixel
    inc cx
    cmp cx, 200
    jb .cols
    inc dx
    cmp dx, 140
    jb .rows

    xor ah, ah              ; wait for a key
    int 16h
    mov ax, 0003h           ; back to 80x25 text before exiting
    int 10h
    ret

; putpixel: CX = x (0..319), DX = y (0..199), AL = colour, ES = A000h
putpixel:
    push ax
    push bx
    push dx
    mov bx, dx
    shl dx, 6               ; y * 64
    shl bx, 8               ; y * 256
    add bx, dx              ; y * 320   (64 + 256, no multiply needed)
    add bx, cx              ; + x
    mov [es:bx], al
    pop dx
    pop bx
    pop ax
    ret
```

Three habits start here:

- **`y*320 = (y<<8) + (y<<6)`.** A shift-and-add pair beats `mul` by an
  order of magnitude on old hardware, and it leaves `DX` alone.
- **Always restore the text mode** (`mov ax, 0003h`, `int 10h`) before
  exiting, unless you deliberately want the last frame left on screen.
- **`ES` is the graphics segment, `DS` stays on your data.** The string
  instructions (`stosb`, `movsw`) write to `ES:DI` and read from `DS:SI`,
  which is exactly the split you want: tables in `DS`, pixels in `ES`.

`putpixel` is fine for a few hundred pixels and far too slow for a full
screen. Full-screen effects walk `DI` through the buffer with `stosb`
instead, which is what the rest of the guide does.

### 7. The palette

Each of the 256 colours is an entry in the VGA's DAC holding **6-bit** red,
green and blue values (0..63, not 0..255). You program it through two
ports:

- `3C8h` — write the index of the colour you want to change,
- `3C9h` — write R, then G, then B. The index auto-increments, so you can
  set the whole palette with one index write and 768 data writes.

```nasm
; lesson07.asm - a custom palette
org 100h
bits 16

start:
    mov ax, 0013h
    int 10h
    cld                     ; make string instructions count upward

    ; --- 256 colours: a blue-to-orange ramp ---
    mov dx, 3C8h
    xor al, al              ; start at colour 0
    out dx, al
    inc dx                  ; DX = 3C9h, the data port
    xor cx, cx
.pal:
    mov al, cl
    shr al, 2               ; 0..255 -> 0..63, the DAC is 6-bit
    out dx, al              ; red rises
    mov al, cl
    shr al, 3               ; green rises half as fast
    out dx, al
    mov al, cl
    shr al, 2
    xor al, 3Fh             ; blue falls (63 - i/4)
    out dx, al
    inc cl
    jnz .pal                ; CL wraps 255 -> 0 and ends the loop

    ; --- fill the screen so that colour = x ---
    push 0A000h
    pop es
    xor di, di
    mov dx, 200
.row:
    xor al, al
    mov cx, 320
.col:
    stosb                   ; [ES:DI] = AL, DI = DI + 1
    inc al
    loop .col
    dec dx
    jnz .row

    xor ah, ah
    int 16h
    mov ax, 0003h
    int 10h
    ret
```

Because the palette is a level of indirection, you can animate an image
without touching a single pixel — rotate the palette and the picture moves.
That trick is [lesson 13](#13-copper-bars).

The BIOS has palette calls too (`int 10h`, `AX=1010h` for one entry,
`AX=1012h` for a block from `ES:DX`), and they are perfectly fine outside
the inner loop. Direct port writes are what demos use because they are
faster and can be timed against the retrace.

### 8. The frame loop: back buffer and vsync

Draw straight into `A000h` and the monitor will show you half-finished
frames: tearing and flicker. The fix is the *back buffer*: build the frame
in ordinary RAM, then copy it to video memory in one burst, timed so the
copy happens while the CRT beam is off-screen.

Two facts make this easy here:

- Below the 640 KB DOS limit there is plenty of unused RAM. A `.COM`
  program is loaded at segment `0800h`; segments `7000h`, `8000h`, `9000h`
  are free playgrounds, each with 64 KB — and a 320×200 screen is 64000
  bytes, just under the limit.
- Port `3DAh` bit 3 is set while the CRT is in **vertical retrace**. Wait
  for it to go low and then high, and you are at the start of a fresh
  frame: the emulator models this at the real 70 Hz of mode 13h.

```nasm
; lesson08.asm - a proper frame loop with a bouncing square
org 100h
bits 16

BACK    equ 07000h          ; back buffer segment (64000 bytes)
VGA_SEG equ 0A000h

start:
    mov ax, 0013h
    int 10h
    cld
    mov ax, BACK
    mov es, ax              ; ES = where we draw

frame:
    ; ---- clear the back buffer (32000 words = 64000 bytes) ----
    xor di, di
    xor ax, ax
    mov cx, 32000
    rep stosw

    ; ---- move the square, bouncing off the edges ----
    mov ax, [vx]
    add [x], ax
    mov ax, [vy]
    add [y], ax
    cmp word [x], 0
    jg .x2
    neg word [vx]
.x2:
    cmp word [x], 320-16
    jl .y1
    neg word [vx]
.y1:
    cmp word [y], 0
    jg .y2
    neg word [vy]
.y2:
    cmp word [y], 200-16
    jl .draw
    neg word [vy]

    ; ---- draw a 16x16 block at (x, y) ----
.draw:
    mov di, [y]
    mov ax, di
    shl di, 8
    shl ax, 6
    add di, ax              ; y * 320
    add di, [x]
    mov al, 15
    mov dx, 16              ; 16 rows
.rows:
    mov cx, 16              ; 16 pixels each
    push di
    rep stosb
    pop di
    add di, 320             ; next screen row
    dec dx
    jnz .rows

    ; ---- wait for the vertical retrace ----
    mov dx, 3DAh
.v1:
    in al, dx
    test al, 8
    jnz .v1                 ; wait until any retrace in progress ends
.v2:
    in al, dx
    test al, 8
    jz .v2                  ; wait for the next one to begin

    ; ---- flip: copy the back buffer to video memory ----
    push ds
    mov ax, BACK
    mov ds, ax
    push VGA_SEG
    pop es
    xor si, si
    xor di, di
    mov cx, 32000
    rep movsw               ; 64000 bytes, DS:SI -> ES:DI
    pop ds
    mov ax, BACK
    mov es, ax              ; ES back to the buffer for the next frame

    ; ---- ESC quits ----
    mov ah, 01h
    int 16h
    jz frame
    xor ah, ah
    int 16h
    cmp al, 27
    jne frame

    mov ax, 0003h
    int 10h
    ret

x:  dw 100
y:  dw 80
vx: dw 2
vy: dw 3
```

This skeleton — clear, update, draw, wait for retrace, flip, check for ESC —
is the shape of every remaining program in this guide.

Notes:

- **`cld` matters.** `stosb`/`movsw` move forward when the direction flag is
  clear and backward when it is set. DOS hands you `DF=0`, but interrupt
  handlers and other people's code do not always leave it that way, so set
  it yourself once at startup.
- **`rep movsw` copies words**, so the count is 32000, not 64000. On a 386
  `rep movsd` with 16000 dwords is faster still.
- **Timing comes free.** Because the flip waits for the retrace, the demo
  runs at 70 frames per second — the mode 13h refresh rate — and you can use
  the frame counter as your clock instead of reading a timer.

### 9. Lookup tables and the sine table

Old CPUs are bad at maths and good at fetching bytes. The demoscene answer
is always the same: **precompute a table, then index it**. A 256-entry sine
table is the workhorse; because the index is a byte, angles wrap around at
256 for free.

`xlat` (also spelled `xlatb`) is the instruction built for this: it replaces
`AL` with the byte at `DS:BX + AL`. One byte of code, one table lookup, and
the `& 255` is implicit.

The FPU builds the table at startup so you do not have to paste 256 numbers
into your source:

```nasm
; lesson09.asm - build a sine table with the x87 and plot it
org 100h
bits 16

start:
    mov ax, 0013h
    int 10h
    call build_sine

    push 0A000h
    pop es
    mov bx, sine            ; XLAT reads DS:BX + AL
    xor cx, cx              ; x
.draw:
    mov al, cl              ; AL = x & 255 - the table wraps by itself
    xlatb                   ; AL = sine[AL], 0..254
    shr al, 1               ; 0..127
    add al, 36              ; centre it vertically
    movzx di, al
    mov ax, di
    shl di, 8
    shl ax, 6
    add di, ax              ; y * 320
    add di, cx              ; + x
    mov byte [es:di], 15
    inc cx
    cmp cx, 320
    jb .draw

    xor ah, ah
    int 16h
    mov ax, 0003h
    int 10h
    ret

; build_sine: sine[i] = round(sin(i * 2pi/256) * 127 + 127), i = 0..255
build_sine:
    finit
    fild word [c256]        ; ST0 = 256
    fldpi                   ; ST0 = pi,  ST1 = 256
    fld1
    fld1
    faddp                   ; ST0 = 2,   ST1 = pi,  ST2 = 256
    fmulp                   ; ST0 = 2pi, ST1 = 256
    fdivrp                  ; ST0 = step = 2pi/256
    fldz                    ; ST0 = angle, ST1 = step
    mov di, sine
    mov cx, 256
.gen:
    fld st0                 ; copy the angle
    fsin
    fild word [c127]
    fmulp                   ; sin * 127
    fild word [c127]
    faddp                   ; + 127  -> 0..254
    fistp word [tmp]        ; store as an integer and pop
    mov al, [tmp]
    mov [di], al
    inc di
    fadd st1                ; angle += step
    loop .gen
    finit                   ; leave the FPU stack empty
    ret

c256: dw 256
c127: dw 127
tmp:  dw 0
sine: times 256 db 0
```

Reading x87 code: the FPU is a stack of eight registers. `ST0` is the top;
`fld` pushes, `fistp` stores-and-pops, and instructions ending in `p` pop an
operand after using it. `fdivrp` is the *reversed* divide — `ST1 / ST0`
rather than `ST0 / ST1`. Keeping a running comment of what is on the stack,
as above, is not optional; everyone does it.

Tables you will want later:

| Table | Built with | Used for |
|-------|-----------|----------|
| `sine[256]` | `fsin` | plasma, scrollers, orbits, camera paths |
| `atan2` and `distance` per pixel | `fpatan`, `fsqrt` | tunnels ([examples/tunnel.asm](../examples/tunnel.asm)) |
| a 320-entry row offset table | `y*320` once | avoids the shift-add per row |
| a random table | an LCG or xorshift | fire, starfields, noise |

### 10. Fixed-point maths

There are no floats in an inner loop. Instead you keep fractions in
integers: **8.8 fixed point** stores a number multiplied by 256, so the high
byte is the integer part and the low byte is the fraction.

| Operation | Fixed point |
|-----------|-------------|
| the constant *k* | `k * 256` |
| add, subtract | plain `add`, `sub` |
| multiply | `imul`, then `sar` by 8 — the product has 16 fraction bits |
| divide | shift the numerator left by 8 first, then `idiv` |
| back to an integer | `sar x, 8` |

The sine table is already a fixed-point table of a sort: `sine[i] - 128` is
sin(θ)·128 as a signed byte. Multiply a radius by it, shift right by 7, and
you have rotated a point without touching the FPU:

```nasm
; lesson10.asm - a rotating spiral in fixed point
org 100h
bits 16

BACK equ 07000h

start:
    mov ax, 0013h
    int 10h
    cld
    call build_sine
    call build_palette

    mov ax, BACK
    mov es, ax              ; ES = back buffer

frame:
    xor di, di              ; clear it
    xor ax, ax
    mov cx, 32000
    rep stosw

    mov si, 199             ; SI = the dot counter
.dot:
    mov cx, si
    shr cx, 1               ; CX = radius, 0..99
    mov bx, sine            ; XLAT base
    mov ax, si
    add ax, si
    add ax, si              ; radius * 3
    add ax, [time]          ; ... plus the frame counter = rotation
    mov bp, ax              ; keep the angle: IMUL clobbers DX

    add al, 64              ; cos(a) = sin(a + 90 degrees)
    xlatb                   ; AL = sine[(a+64) & 255]
    sub al, 128             ; -128..126
    cbw                     ; sign-extend AL into AX
    imul cx                 ; DX:AX = radius * cos * 128
    sar ax, 7               ; ... / 128
    add ax, 160             ; centre of the screen
    mov di, ax              ; DI = x

    mov ax, bp
    xlatb                   ; AL = sine[a & 255]
    sub al, 128
    cbw
    imul cx
    sar ax, 7
    add ax, 100             ; AX = y

    mov bx, ax              ; offset = y*320 + x
    shl ax, 6
    shl bx, 8
    add bx, ax
    add bx, di
    mov ax, si              ; colour follows the radius
    mov [es:bx], al

    dec si
    jnz .dot

    call flip
    inc word [time]

    mov ah, 01h             ; ESC quits
    int 16h
    jz frame
    xor ah, ah
    int 16h
    cmp al, 27
    jne frame

    mov ax, 0003h
    int 10h
    ret

; flip: wait for the vertical retrace, then copy BACK to video memory.
flip:
    mov dx, 3DAh
.v1:
    in al, dx
    test al, 8
    jnz .v1
.v2:
    in al, dx
    test al, 8
    jz .v2
    push ds
    mov ax, BACK
    mov ds, ax
    push 0A000h
    pop es
    xor si, si
    xor di, di
    mov cx, 32000
    rep movsw
    pop ds
    mov ax, BACK
    mov es, ax
    ret

; build_palette: colour i = a black-to-amber ramp (colour 0 stays black)
build_palette:
    mov dx, 3C8h
    xor al, al
    out dx, al
    inc dx
    xor cx, cx
.p:
    mov al, cl
    shr al, 2
    out dx, al              ; R
    mov al, cl
    shr al, 3
    out dx, al              ; G
    mov al, cl
    shr al, 5
    out dx, al              ; B
    inc cl
    jnz .p
    ret

; build_sine: sine[i] = round(sin(i * 2pi/256) * 127 + 127)
build_sine:
    finit
    fild word [c256]
    fldpi
    fld1
    fld1
    faddp
    fmulp                   ; 2pi
    fdivrp                  ; step = 2pi/256
    fldz                    ; angle
    mov di, sine
    mov cx, 256
.gen:
    fld st0
    fsin
    fild word [c127]
    fmulp
    fild word [c127]
    faddp
    fistp word [tmp]
    mov al, [tmp]
    mov [di], al
    inc di
    fadd st1
    loop .gen
    finit
    ret

time: dw 0
c256: dw 256
c127: dw 127
tmp:  dw 0
sine: times 256 db 0
```

`cbw` ("convert byte to word") is what makes the signed byte from the table
usable: it copies bit 7 of `AL` across all of `AH`, so `-2` as a byte
becomes `-2` as a word instead of `254`. Its siblings are `cwd` (word to
`DX:AX`, needed before `idiv`) and `movsx`/`movzx` on the 386.

Notice also that this program has grown a shape: `build_*` routines run
once, `flip` is shared, and `frame` is the loop. From here on, effects are
variations of the body of `frame`.

---

## Part 3 — Effects

Everything from here on is the same program: build tables, then per frame
fill 64000 bytes and flip. What changes is the function that turns *(x, y,
time)* into a colour. Two rules of thumb carry you a long way:

1. **Move work out of the inner loop.** Anything that depends only on the
   row, or only on the frame, is computed once per row or once per frame.
2. **Let bytes wrap.** An 8-bit index into a 256-entry table wraps for free,
   which is why demo maths is full of `add al, something` with no masking.

### 11. Plasma

Plasma is the sum of a few sine waves, used as a palette index. The classic
version is `sin(x + t) + sin(y*2 - t)`; because both terms come from the
same 256-byte table and the sum is taken in a byte, it costs about five
instructions per pixel.

```nasm
; lesson11.asm - plasma
org 100h
bits 16

BACK equ 07000h

start:
    mov ax, 0013h
    int 10h
    cld
    call build_sine
    call build_palette
    mov ax, BACK
    mov es, ax

frame:
    xor di, di
    mov bx, sine            ; XLAT base, constant for the whole frame
    mov dl, [ytime]         ; DL = the phase of the vertical wave
    mov cx, 200             ; rows
.row:
    mov al, dl
    xlatb
    mov dh, al              ; DH = this row's wave value - computed ONCE
    mov ah, [xtime]         ; AH = the horizontal phase, walks along the row
    mov si, 320
.col:
    mov al, ah
    xlatb                   ; AL = sine[x phase]
    add al, dh              ; + the row's value; the byte wraps by itself
    stosb                   ; that is the pixel
    inc ah
    dec si
    jnz .col
    add dl, 2               ; the vertical wave runs twice as fast
    dec cx
    jnz .row

    call flip
    inc byte [xtime]        ; scroll the two waves in opposite directions
    sub byte [ytime], 3

    mov ah, 01h
    int 16h
    jz frame
    xor ah, ah
    int 16h
    cmp al, 27
    jne frame

    mov ax, 0003h
    int 10h
    ret

; build_palette: a smooth rainbow - R, G and B are the same sine curve
; sampled 120 degrees apart.
build_palette:
    mov dx, 3C8h
    xor al, al
    out dx, al
    inc dx
    mov bx, sine
    xor cx, cx
.p:
    mov al, cl
    xlatb
    shr al, 2
    out dx, al              ; R = sine[i]
    mov al, cl
    add al, 85
    xlatb
    shr al, 2
    out dx, al              ; G = sine[i + 85]
    mov al, cl
    add al, 170
    xlatb
    shr al, 2
    out dx, al              ; B = sine[i + 170]
    inc cl
    jnz .p
    ret

; flip: wait for the vertical retrace, then copy BACK to video memory.
flip:
    mov dx, 3DAh
.v1:
    in al, dx
    test al, 8
    jnz .v1
.v2:
    in al, dx
    test al, 8
    jz .v2
    push ds
    mov ax, BACK
    mov ds, ax
    push 0A000h
    pop es
    xor si, si
    xor di, di
    mov cx, 32000
    rep movsw
    pop ds
    mov ax, BACK
    mov es, ax
    ret

; build_sine: sine[i] = round(sin(i * 2pi/256) * 127 + 127)
build_sine:
    finit
    fild word [c256]
    fldpi
    fld1
    fld1
    faddp
    fmulp
    fdivrp                  ; step = 2pi/256
    fldz
    mov di, sine
    mov cx, 256
.gen:
    fld st0
    fsin
    fild word [c127]
    fmulp
    fild word [c127]
    faddp
    fistp word [tmp]
    mov al, [tmp]
    mov [di], al
    inc di
    fadd st1
    loop .gen
    finit
    ret

xtime: db 0
ytime: db 0
c256:  dw 256
c127:  dw 127
tmp:   dw 0
sine:  times 256 db 0
```

Things to try: change `add dl, 2` to `add dl, 5` (tighter horizontal
bands), add a third wave indexed by `x+y`, or animate the palette instead of
the pixels — rewriting 768 DAC bytes per frame is far cheaper than 64000
pixels.

### 12. Fire

Fire is a cellular automaton. The bottom rows are filled with random heat;
every other pixel becomes the average of the four pixels below it, minus a
little, so heat rises and fades. The palette turns the resulting heat value
into black → red → orange → yellow → white.

```nasm
; lesson12.asm - the classic demoscene fire
org 100h
bits 16

BACK equ 07000h

; RAMP: emit one DAC component that ramps from 0 at index %1 to 63 at %1+63
%macro RAMP 1
    mov al, cl
    sub al, %1
    jnc %%not_negative
    xor al, al
%%not_negative:
    cmp al, 63
    jbe %%send
    mov al, 63
%%send:
    out dx, al
%endmacro

start:
    mov ax, 0013h
    int 10h
    cld

    ; --- palette: red rises first, then green, then blue ---
    mov dx, 3C8h
    xor al, al
    out dx, al
    inc dx
    xor cx, cx
.pal:
    RAMP 0                  ; R
    RAMP 64                 ; G
    RAMP 160                ; B
    inc cl
    jnz .pal

    mov ax, BACK
    mov es, ax
    xor di, di              ; start with a cold screen
    xor ax, ax
    mov cx, 32000
    rep stosw

frame:
    ; --- seed the bottom two rows ---
    mov di, 198*320
    mov cx, 640
.seed:
    call rnd
    test ah, 40h
    jz .cold
    or al, 0C0h             ; white hot: 192..255
    jmp .put
.cold:
    xor al, al              ; a black speck: this is what makes it flicker
.put:
    stosb
    loop .seed

    ; --- propagate upward ---
    xor di, di
    xor dx, dx              ; DH = 0 so DL can widen bytes to words
    mov cx, 198*320
.prop:
    movzx ax, byte [es:di+319]  ; below left
    mov dl, [es:di+320]         ; below
    add ax, dx
    mov dl, [es:di+321]         ; below right
    add ax, dx
    mov dl, [es:di+640]         ; two rows below
    add ax, dx
    shr ax, 2                   ; average of the four
    jz .store
    dec ax                      ; cool down
.store:
    stosb                       ; writes [ES:DI], DI = DI + 1
    loop .prop

    call flip

    mov ah, 01h
    int 16h
    jz frame
    xor ah, ah
    int 16h
    cmp al, 27
    jne frame

    mov ax, 0003h
    int 10h
    ret

; rnd: AX = the next value of a 16-bit xorshift generator
rnd:
    mov ax, [seed]
    mov dx, ax
    shl ax, 7
    xor ax, dx
    mov dx, ax
    shr dx, 9
    xor ax, dx
    mov dx, ax
    shl dx, 8
    xor ax, dx
    mov [seed], ax
    ret

; flip: wait for the retrace, then copy BACK to video memory.
flip:
    mov dx, 3DAh
.v1:
    in al, dx
    test al, 8
    jnz .v1
.v2:
    in al, dx
    test al, 8
    jz .v2
    push ds
    mov ax, BACK
    mov ds, ax
    push 0A000h
    pop es
    xor si, si
    xor di, di
    mov cx, 32000
    rep movsw
    pop ds
    mov ax, BACK
    mov es, ax
    ret

seed: dw 1
```

Details that matter:

- **The average is read from rows that have not been written yet.** The loop
  walks `DI` upward and reads `DI+320`, so every source pixel is still last
  frame's value. Get the direction wrong and the fire smears.
- **`movzx` clears the high byte** so the four additions cannot pick up
  garbage. On an 8086 you would write `xor ah, ah` / `mov al, ...`.
- **The black specks are the effect.** Seeding uniformly hot gives a boring
  orange wall; the randomly extinguished pixels are what makes flames.
- The propagation is 63360 pixels of about ten instructions — the most
  expensive thing in this guide. Halving the vertical resolution (render
  160×100 and double the pixels on the way out) is the traditional cure.

### 13. Copper bars

On the Amiga, the copper could change a colour register mid-scanline. On a
PC you fake it: give every screen row its own palette entry, then rewrite
those entries once a frame. Not a single pixel is touched after startup —
this is the cheapest effect in the book, and it composites over anything.

```nasm
; lesson13.asm - copper bars by palette animation
org 100h
bits 16

ROW0 equ 56                 ; row y uses colour ROW0 + y

; ADDC: add AL to the byte at %1, clamped to 63
%macro ADDC 1
    add [%1], al
    cmp byte [%1], 63
    jbe %%ok
    mov byte [%1], 63
%%ok:
%endmacro

start:
    mov ax, 0013h
    int 10h
    cld
    call build_sine

    ; --- paint row y in colour ROW0 + y, once ---
    push 0A000h
    pop es
    xor di, di
    mov al, ROW0
    mov dx, 200
.fill:
    mov cx, 320
    rep stosb
    inc al
    dec dx
    jnz .fill
    push ds
    pop es                  ; from now on STOSB fills palbuf, not the screen

frame:
    ; --- where are the three bars this frame? ---
    mov bx, sine
    mov al, [t]
    xlatb
    shr al, 1
    add al, 30
    mov [bar0], al          ; 30..157
    mov al, [t]
    add al, 85              ; the second bar trails by 120 degrees
    xlatb
    shr al, 1
    add al, 30
    mov [bar1], al
    mov al, [t]
    add al, 170
    xlatb
    shr al, 1
    add al, 30
    mov [bar2], al

    ; --- build 200 RGB triples ---
    mov di, palbuf
    xor bx, bx              ; BX = row
.row:
    mov word [r], 0
    mov byte [b], 0

    movzx ax, byte [bar0]   ; bar 0 is red
    call bar_intensity
    ADDC r
    shr al, 2
    ADDC g

    movzx ax, byte [bar1]   ; bar 1 is green
    call bar_intensity
    ADDC g
    shr al, 2
    ADDC b

    movzx ax, byte [bar2]   ; bar 2 is blue
    call bar_intensity
    ADDC b
    shr al, 2
    ADDC r

    mov al, [r]             ; store the triple
    stosb
    mov al, [g]
    stosb
    mov al, [b]
    stosb
    inc bx
    cmp bx, 200
    jb .row

    ; --- upload during the vertical retrace, where it is invisible ---
    mov dx, 3DAh
.v1:
    in al, dx
    test al, 8
    jnz .v1
.v2:
    in al, dx
    test al, 8
    jz .v2

    mov dx, 3C8h
    mov al, ROW0
    out dx, al
    inc dx                  ; DX = 3C9h
    mov si, palbuf
    mov cx, 200*3
    rep outsb               ; OUT DX, [DS:SI] - 600 bytes, one instruction

    inc byte [t]

    mov ah, 01h
    int 16h
    jz frame
    xor ah, ah
    int 16h
    cmp al, 27
    jne frame

    mov ax, 0003h
    int 10h
    ret

; bar_intensity: AX = the bar's centre row, BX = this row
;                -> AL = 0..60, brightest at the centre
bar_intensity:
    sub ax, bx
    jns .abs
    neg ax
.abs:
    cmp ax, 16
    jae .dark
    mov dx, 16
    sub dx, ax
    mov ax, dx
    shl ax, 2
    ret
.dark:
    xor ax, ax
    ret

; build_sine: sine[i] = round(sin(i * 2pi/256) * 127 + 127)
build_sine:
    finit
    fild word [c256]
    fldpi
    fld1
    fld1
    faddp
    fmulp
    fdivrp
    fldz
    mov di, sine
    mov cx, 256
.gen:
    fld st0
    fsin
    fild word [c127]
    fmulp
    fild word [c127]
    faddp
    fistp word [tmp]
    mov al, [tmp]
    mov [di], al
    inc di
    fadd st1
    loop .gen
    finit
    ret

t:      db 0
bar0:   db 0
bar1:   db 0
bar2:   db 0
r:      db 0
g:      db 0
b:      db 0
c256:   dw 256
c127:   dw 127
tmp:    dw 0
sine:   times 256 db 0
palbuf: times 200*3 db 0
```

`mov word [r], 0` clears `r` and `g` in one instruction because they are
adjacent — the kind of shortcut that is fine when the data layout is right
next to the code that relies on it, and a bug waiting to happen otherwise.

The 600 `outsb` writes are deliberately inside the vertical blank: change a
DAC entry while the beam is drawing and you get a visible glitch on that
scanline. That is a real hardware behaviour the emulator reproduces.

### 14. Starfield

A 3D starfield needs one division per star: project (x, y, z) to
(x/z, y/z). With a few hundred stars that is affordable even on a 286, and
the depth gives you the brightness for free.

```nasm
; lesson14.asm - a perspective starfield
org 100h
bits 16

BACK   equ 07000h
NSTARS equ 256
SPEED  equ 6

start:
    mov ax, 0013h
    int 10h
    cld

    ; --- greyscale palette: colour = brightness ---
    mov dx, 3C8h
    xor al, al
    out dx, al
    inc dx
    xor cx, cx
.pal:
    mov al, cl
    shr al, 2
    out dx, al
    out dx, al
    out dx, al
    inc cl
    jnz .pal

    ; --- scatter the stars ---
    xor si, si
.init:
    call respawn
    add si, 2
    cmp si, NSTARS*2
    jb .init

    mov ax, BACK
    mov es, ax

frame:
    xor di, di              ; clear the back buffer
    xor ax, ax
    mov cx, 32000
    rep stosw

    xor si, si              ; SI = star index * 2
.star:
    sub word [zs+si], SPEED ; fly toward the viewer
    cmp word [zs+si], 16
    jb .new

    mov ax, [xs+si]         ; screen x = x/z + 160
    cwd
    idiv word [zs+si]
    add ax, 160
    cmp ax, 320
    jae .new                ; unsigned: catches negatives too
    mov di, ax

    mov ax, [ys+si]         ; screen y = y/z + 100
    cwd
    idiv word [zs+si]
    add ax, 100
    cmp ax, 200
    jae .new

    mov bx, ax              ; offset = y*320 + x
    shl ax, 6
    shl bx, 8
    add bx, ax
    add bx, di

    mov ax, 271             ; brightness: near stars are white
    sub ax, [zs+si]
    mov [es:bx], al
    jmp .next
.new:
    call respawn
.next:
    add si, 2
    cmp si, NSTARS*2
    jb .star

    call flip

    mov ah, 01h
    int 16h
    jz frame
    xor ah, ah
    int 16h
    cmp al, 27
    jne frame

    mov ax, 0003h
    int 10h
    ret

; respawn: give star SI/2 a new random position far away
respawn:
    call rnd
    mov [xs+si], ax
    call rnd
    mov [ys+si], ax
    call rnd
    and ax, 0FFh
    add ax, 16
    mov [zs+si], ax
    ret

; rnd: AX = the next value of a 16-bit xorshift generator
rnd:
    mov ax, [seed]
    mov dx, ax
    shl ax, 7
    xor ax, dx
    mov dx, ax
    shr dx, 9
    xor ax, dx
    mov dx, ax
    shl dx, 8
    xor ax, dx
    mov [seed], ax
    ret

; flip: wait for the retrace, then copy BACK to video memory.
flip:
    mov dx, 3DAh
.v1:
    in al, dx
    test al, 8
    jnz .v1
.v2:
    in al, dx
    test al, 8
    jz .v2
    push ds
    mov ax, BACK
    mov ds, ax
    push 0A000h
    pop es
    xor si, si
    xor di, di
    mov cx, 32000
    rep movsw
    pop ds
    mov ax, BACK
    mov es, ax
    ret

seed: dw 12345
xs:   times NSTARS dw 0
ys:   times NSTARS dw 0
zs:   times NSTARS dw 0
```

`cwd` before `idiv` is mandatory: `idiv` divides the signed 32-bit `DX:AX`,
so `DX` must hold the sign extension of `AX`, not leftovers. A star that
projects off-screen is recycled rather than clamped, which keeps the
density even as stars approach.

To turn this into the parallax starfield of a cracktro, drop the division:
give each star a fixed layer (three or four speeds), move it horizontally,
and give each layer its own colour.

### 15. A sine scroller

No intro is complete without text sliding along a wave. The characters come
from the BIOS ROM font at `F000:A000` — 16 bytes per character, one byte per
row, bit 7 leftmost — so you get a full 8×16 typeface without shipping any
font data.

```nasm
; lesson15.asm - a sine scroller in the BIOS ROM font
org 100h
bits 16

BACK     equ 07000h
FONT_SEG equ 0F000h
FONT_OFF equ 0A000h         ; the 8x16 font
STEP     equ 2              ; scroll speed in pixels per frame

start:
    mov ax, 0013h
    int 10h
    cld
    call build_sine
    call build_palette
    mov ax, BACK
    mov es, ax
    mov ax, FONT_SEG
    mov fs, ax              ; FS -> the ROM, so ES stays on the buffer

frame:
    xor di, di              ; clear the back buffer
    xor ax, ax
    mov cx, 32000
    rep stosw

    mov si, [scroll]        ; SI = the text pixel column shown at x = 0
    xor bp, bp              ; BP = screen x
.col:
    ; --- which character is this column in? ---
    mov ax, si
    shr ax, 3               ; 8 pixels per character
    xor dx, dx
    div word [msglen]       ; DX = index into the message (it repeats)
    mov bx, dx
    mov al, [msg+bx]
    movzx di, al
    shl di, 4               ; 16 bytes per character
    add di, FONT_OFF        ; DI -> this character's bitmap in the ROM

    ; --- how high is this column? ---
    mov bx, sine
    mov ax, bp
    add al, [t]
    xlatb                   ; AL = sine[(x + t) & 255]
    shr al, 2               ; 0..63 pixels of wobble
    add al, 80
    movzx bx, al
    mov ax, bx
    shl bx, 8
    shl ax, 6
    add bx, ax              ; y * 320
    add bx, bp              ; + x

    ; --- which of its eight columns? (last, so nothing clobbers AH) ---
    mov cx, si
    and cx, 7
    mov ah, 80h
    shr ah, cl              ; AH = the bit mask for this column

    ; --- draw the 16 pixels of the column ---
    mov dl, 32              ; colour 32..47 = the vertical gradient
    mov cx, 16
.rows:
    mov al, [fs:di]         ; one row of the glyph
    test al, ah             ; is our column set?
    jz .skip
    mov [es:bx], dl
.skip:
    inc di
    inc dl
    add bx, 320
    loop .rows

    inc si
    inc bp
    cmp bp, 320
    jb .col

    call flip

    mov ax, [scroll]        ; advance, wrapping at the end of the message
    add ax, STEP
    cmp ax, [scrollmax]
    jb .keep
    sub ax, [scrollmax]
.keep:
    mov [scroll], ax
    inc byte [t]

    mov ah, 01h
    int 16h
    jz frame
    xor ah, ah
    int 16h
    cmp al, 27
    jne frame

    mov ax, 0003h
    int 10h
    ret

; build_palette: colours 32..47, white at the top of a glyph to blue at the
; bottom.
build_palette:
    mov dx, 3C8h
    mov al, 32
    out dx, al
    inc dx
    xor bx, bx
    mov cx, 16
.p:
    mov al, bl
    shl al, 2
    mov ah, 63
    sub ah, al              ; 63 down to 3
    mov al, ah
    out dx, al              ; R
    out dx, al              ; G
    mov al, 63
    out dx, al              ; B
    inc bx
    loop .p
    ret

; flip: wait for the retrace, then copy BACK to video memory.
flip:
    mov dx, 3DAh
.v1:
    in al, dx
    test al, 8
    jnz .v1
.v2:
    in al, dx
    test al, 8
    jz .v2
    push ds
    mov ax, BACK
    mov ds, ax
    push 0A000h
    pop es
    xor si, si
    xor di, di
    mov cx, 32000
    rep movsw
    pop ds
    mov ax, BACK
    mov es, ax
    ret

; build_sine: sine[i] = round(sin(i * 2pi/256) * 127 + 127)
build_sine:
    finit
    fild word [c256]
    fldpi
    fld1
    fld1
    faddp
    fmulp
    fdivrp
    fldz
    mov di, sine
    mov cx, 256
.gen:
    fld st0
    fsin
    fild word [c127]
    fmulp
    fild word [c127]
    faddp
    fistp word [tmp]
    mov al, [tmp]
    mov [di], al
    inc di
    fadd st1
    loop .gen
    finit
    ret

t:         db 0
scroll:    dw 0
c256:      dw 256
c127:      dw 127
tmp:       dw 0
sine:      times 256 db 0
msg:       db '   GREETINGS FROM ASM-EMU !   THIS SCROLLER USES THE BIOS ROM FONT '
           db 'AND A 256-BYTE SINE TABLE.   PRESS ESC WHEN YOU HAVE SEEN ENOUGH ...   '
MSGLEN     equ $-msg
msglen:    dw MSGLEN
scrollmax: dw MSGLEN*8
```

The wave is `sine[(x + t) & 255]`, one lookup per column: scale `x` before
the lookup (`shl ax, 1`) for a tighter wave, or add a second sine at a
different rate for a wobblier one. Rendering each glyph twice — once in a
dark colour, one pixel down and to the right — gives it a drop shadow.

---

## Part 4 — Sound

A PC of this era has two sound sources you can rely on: the built-in speaker
(a square wave from a timer channel) and, if the user bought a card, an
AdLib — a Yamaha OPL2 FM synthesiser on ports `388h`/`389h`. The emulator
implements both, mixed at 48 kHz. Headless runs can capture the result:

```
./asm-emu run -wav out.wav -headless -max-insns 200000000 lesson17.asm
```

### 16. The PC speaker

Timer channel 2 of the 8254 is wired to the speaker. Program it with a
divisor of the 1.193182 MHz PIT clock and open the gate:

- port `43h` ← `0B6h`: "channel 2, write low byte then high byte, mode 3
  (square wave)"
- port `42h` ← divisor low, then divisor high, where
  divisor = 1193182 / frequency
- port `61h` bits 0 and 1: bit 0 gates the timer, bit 1 connects it to the
  speaker. Set both for sound, clear both for silence.

```nasm
; lesson16.asm - a tune on the PC speaker
org 100h
bits 16

start:
    mov ah, 09h
    mov dx, banner
    int 21h
    sti                     ; the BIOS wait service needs interrupts

    mov si, tune
.next:
    lodsw                   ; AX = the divisor for this note, 0 ends the tune
    test ax, ax
    jz .done
    call tone_on
    call delay
    jmp .next
.done:
    call tone_off
    ret

; tone_on: AX = the PIT divisor
tone_on:
    push ax
    mov al, 0B6h            ; channel 2, lo/hi byte, mode 3 = square wave
    out 43h, al
    pop ax
    out 42h, al             ; divisor, low byte
    mov al, ah
    out 42h, al             ; divisor, high byte
    in al, 61h
    or al, 3                ; bit 0 = gate on, bit 1 = speaker on
    out 61h, al
    ret

tone_off:
    in al, 61h
    and al, 0FCh
    out 61h, al
    ret

; delay: about 150 ms, via the BIOS "wait CX:DX microseconds" service
delay:
    mov ah, 86h
    mov cx, 0002h
    mov dx, 49F0h           ; 0002_49F0h = 150000 us
    int 15h
    ret

; 1193182 / frequency, for C D E G A C
tune:   dw 4561, 4063, 3619, 3044, 2712, 2280, 0
banner: db 'Beeping the PC speaker...', 13, 10, '$'
```

Because the speaker is a bare square wave, the classic trick for "better"
sound is to abuse it: switch it on and off at high speed (pulse-width
modulation) to fake sample playback. That is how DOS games got digitised
speech out of a one-bit output.

### 17. AdLib (OPL2) FM

The OPL2 has nine melodic channels, each built from two operators — a
*modulator* that shapes and a *carrier* that you hear. You talk to it
through two ports: write a register number to `388h`, then its value to
`389h`, with a short delay after each because the chip is slow.

Registers you need to start:

| Register | Meaning |
|----------|---------|
| `20h+op` | tremolo/vibrato/sustain flags and frequency multiplier |
| `40h+op` | output level (0 = loudest, 63 = silent) |
| `60h+op` | attack rate (high nibble), decay rate (low nibble) |
| `80h+op` | sustain level (high), release rate (low) |
| `A0h+ch` | low 8 bits of the F-number (pitch) |
| `B0h+ch` | key-on (bit 5), octave/"block" (bits 2-4), F-number bits 8-9 |
| `C0h+ch` | feedback and the modulator/carrier connection |

Operator offsets for channel 0 are `00h` (modulator) and `03h` (carrier);
channel *n* uses the *n*-th entry of the usual `00 01 02 08 09 0A 10 11 12`
table.

```nasm
; lesson17.asm - an AdLib arpeggio
org 100h
bits 16

OPL_IDX equ 388h
OPL_DAT equ 389h

start:
    mov ah, 09h
    mov dx, banner
    int 21h
    sti

    call opl_reset
    mov si, patch
    call opl_program

    mov si, tune
.play:
    lodsb
    cmp al, 0FFh
    je .done
    mov bl, al              ; BL = semitone, 0 = C
    lodsb
    mov bh, al              ; BH = block (octave)
    call note_on
    call delay
    call note_off
    jmp .play
.done:
    call opl_reset
    ret

; opl_write: AH = register, AL = value.  The delays are the ones real
; AdLib code uses; the emulator charges about 1 us per port access, so
; "read the index port N times" takes the intended time here too.
opl_write:
    push ax
    push cx
    push dx
    mov dx, OPL_IDX
    xchg al, ah
    out dx, al              ; select the register
    mov cx, 6
.d1:
    in al, dx
    loop .d1
    mov dx, OPL_DAT
    mov al, ah
    out dx, al              ; write the value
    mov dx, OPL_IDX
    mov cx, 35
.d2:
    in al, dx
    loop .d2
    pop dx
    pop cx
    pop ax
    ret

; opl_reset: silence every register, then enable the waveform select
opl_reset:
    mov ah, 1
.z:
    xor al, al
    call opl_write
    inc ah
    cmp ah, 0F6h
    jne .z
    mov ax, 0120h           ; register 01h = 20h
    call opl_write
    ret

; opl_program: SI -> pairs of (register, value), terminated by register 0
opl_program:
    lodsb
    test al, al
    jz .done
    mov ah, al
    lodsb
    call opl_write
    jmp opl_program
.done:
    ret

; note_on: BL = semitone (0..11), BH = block (octave)
note_on:
    push si
    movzx si, bl
    shl si, 1
    mov cx, [fnum+si]       ; CX = the 10-bit F-number
    mov ah, 0A0h
    mov al, cl
    call opl_write          ; F-number, low 8 bits
    mov ah, 0B0h
    mov al, ch
    and al, 3               ; F-number, high 2 bits
    mov dl, bh
    shl dl, 2
    or al, dl               ; block
    or al, 20h              ; key on
    call opl_write
    pop si
    ret

note_off:
    mov ax, 0B000h          ; key off, same channel
    call opl_write
    ret

delay:
    mov ah, 86h
    mov cx, 0002h
    mov dx, 49F0h           ; 150 ms
    int 15h
    ret

; A bright plucked patch on channel 0.
patch:
    db 20h, 01h             ; modulator: multiplier 1
    db 40h, 10h             ; modulator: level
    db 60h, 0F0h            ; modulator: fast attack, no decay
    db 80h, 77h             ; modulator: sustain/release
    db 23h, 01h             ; carrier: multiplier 1
    db 43h, 00h             ; carrier: full volume
    db 63h, 0F1h            ; carrier: fast attack, slight decay
    db 83h, 76h             ; carrier: sustain/release
    db 0C0h, 0Eh            ; feedback 7, FM connection
    db 0                    ; end of the patch

; F-numbers for C, C#, D ... B (the block selects the octave)
fnum:   dw 157h, 16Bh, 181h, 198h, 1B0h, 1CAh, 1E5h, 202h, 220h, 241h, 263h, 287h

; semitone, block pairs
tune:   db 0,4, 4,4, 7,4, 0,5, 7,4, 4,4, 0,4, 0,5, 0FFh

banner: db 'AdLib arpeggio - use -wav out.wav to capture it', 13, 10, '$'
```

Real AdLib code starts with a detection sequence (mask the timers, start
timer 1 with a preset of `FFh`, read the status port twice and check for
`00h` then `C0h`); [examples/cracktro.asm](../examples/cracktro.asm) has it,
and the emulator answers exactly as the hardware does.

Playing music, rather than a scale, means a small player: a table of rows
(one per note step), a counter that fires a row every N frames, and a
per-channel state machine for effects such as arpeggio or vibrato. The
cracktro example does all four in about a hundred lines.

---

## Part 5 — Making a demo

### 18. Structure and a timeline

A demo is a sequence of parts with a clock. The clock is the frame counter
you already have, because the flip waits for the retrace: 70 frames is one
second. Everything else is bookkeeping — a table of parts, each with a
duration, a setup routine and a per-frame routine, and a transition so parts
do not just snap into place.

The transition here is the classic one: keep the palette in a 768-byte
buffer, and upload it scaled by a brightness that ramps up at the start of a
part and down at the end. Fading costs nothing per pixel, works with every
effect, and is why demos feel produced rather than assembled.

```nasm
; lesson18.asm - a two-part demo with a timeline and palette fades
org 100h
bits 16

BACK   equ 07000h
NSTARS equ 256
SPEED  equ 6
FADE   equ 16               ; frames of fade at each end of a part

start:
    mov ax, 0013h
    int 10h
    cld
    call build_sine
    mov ax, BACK
    mov es, ax
    call part_enter

main:
    mov bx, [part]          ; run this part's renderer
    imul bx, 6              ; six bytes per timeline entry
    mov bx, [parts+bx+2]
    call bx

    call wait_vsync
    call upload             ; palette changes belong in the blank
    call blit

    inc word [frames]       ; advance the clock
    mov ax, [dur]
    cmp [frames], ax
    jb .keys
    mov word [frames], 0
    inc word [part]
    mov bx, [part]
    imul bx, 6
    cmp word [parts+bx], 0  ; a zero duration ends the demo
    je .quit
    call part_enter

.keys:
    mov ah, 01h
    int 16h
    jz main
    xor ah, ah
    int 16h
    cmp al, 27
    jne main
.quit:
    mov ax, 0003h
    int 10h
    ret

; ---------------------------------------------------------------- timeline
; each entry: duration in frames, render routine, setup routine
parts:
    dw 400, render_stars,  setup_stars
    dw 500, render_plasma, setup_plasma
    dw 0,   0,             0

; part_enter: latch the new part's duration and run its setup
part_enter:
    mov bx, [part]
    imul bx, 6
    mov ax, [parts+bx]
    mov [dur], ax
    mov byte [lastfade], 0FFh   ; force the next upload
    mov bx, [parts+bx+4]
    call bx
    ret

; ---------------------------------------------------------------- fading
; fade_level: AL = 0..16, ramping in at the start of a part and out at its end
fade_level:
    mov ax, [frames]
    mov bx, [dur]
    sub bx, [frames]
    cmp bx, ax
    jae .keep
    mov ax, bx
.keep:
    cmp ax, FADE
    jbe .done
    mov ax, FADE
.done:
    ret

; upload: DAC <- pal, scaled by the fade level.  Skipped entirely when the
; level has not changed, so a part at full brightness costs nothing.
upload:
    call fade_level
    cmp al, [lastfade]
    je .skip
    mov [lastfade], al
    mov bl, al
    mov dx, 3C8h
    xor al, al
    out dx, al
    inc dx
    mov si, pal
    mov cx, 768
.u:
    lodsb
    mul bl                  ; AX = component * level
    shr ax, 4               ; / 16
    out dx, al
    loop .u
.skip:
    ret

; ---------------------------------------------------------------- part 1
setup_stars:
    mov di, pal             ; a grey ramp: colour = brightness
    xor cx, cx
.p:
    mov al, cl
    shr al, 2
    mov [di], al
    mov [di+1], al
    mov [di+2], al
    add di, 3
    inc cl
    jnz .p
    xor si, si              ; scatter the stars
.s:
    call respawn
    add si, 2
    cmp si, NSTARS*2
    jb .s
    ret

render_stars:
    xor di, di
    xor ax, ax
    mov cx, 32000
    rep stosw
    xor si, si
.star:
    sub word [zs+si], SPEED
    cmp word [zs+si], 16
    jb .new
    mov ax, [xs+si]
    cwd
    idiv word [zs+si]
    add ax, 160
    cmp ax, 320
    jae .new
    mov di, ax
    mov ax, [ys+si]
    cwd
    idiv word [zs+si]
    add ax, 100
    cmp ax, 200
    jae .new
    mov bx, ax
    shl ax, 6
    shl bx, 8
    add bx, ax
    add bx, di
    mov ax, 271
    sub ax, [zs+si]
    mov [es:bx], al
    jmp .next
.new:
    call respawn
.next:
    add si, 2
    cmp si, NSTARS*2
    jb .star
    ret

respawn:
    call rnd
    mov [xs+si], ax
    call rnd
    mov [ys+si], ax
    call rnd
    and ax, 0FFh
    add ax, 16
    mov [zs+si], ax
    ret

rnd:
    mov ax, [seed]
    mov dx, ax
    shl ax, 7
    xor ax, dx
    mov dx, ax
    shr dx, 9
    xor ax, dx
    mov dx, ax
    shl dx, 8
    xor ax, dx
    mov [seed], ax
    ret

; ---------------------------------------------------------------- part 2
setup_plasma:
    mov di, pal             ; a rainbow, straight out of the sine table
    mov bx, sine
    xor cx, cx
.p:
    mov al, cl
    xlatb
    shr al, 2
    mov [di], al
    mov al, cl
    add al, 85
    xlatb
    shr al, 2
    mov [di+1], al
    mov al, cl
    add al, 170
    xlatb
    shr al, 2
    mov [di+2], al
    add di, 3
    inc cl
    jnz .p
    ret

render_plasma:
    xor di, di
    mov bx, sine
    mov dl, [ytime]
    mov cx, 200
.row:
    mov al, dl
    xlatb
    mov dh, al
    mov ah, [xtime]
    mov si, 320
.col:
    mov al, ah
    xlatb
    add al, dh
    stosb
    inc ah
    dec si
    jnz .col
    add dl, 2
    dec cx
    jnz .row
    inc byte [xtime]
    sub byte [ytime], 3
    ret

; ---------------------------------------------------------------- plumbing
wait_vsync:
    mov dx, 3DAh
.a:
    in al, dx
    test al, 8
    jnz .a
.b:
    in al, dx
    test al, 8
    jz .b
    ret

blit:
    push ds
    mov ax, BACK
    mov ds, ax
    push 0A000h
    pop es
    xor si, si
    xor di, di
    mov cx, 32000
    rep movsw
    pop ds
    mov ax, BACK
    mov es, ax
    ret

build_sine:
    finit
    fild word [c256]
    fldpi
    fld1
    fld1
    faddp
    fmulp
    fdivrp
    fldz
    mov di, sine
    mov cx, 256
.gen:
    fld st0
    fsin
    fild word [c127]
    fmulp
    fild word [c127]
    faddp
    fistp word [tmp]
    mov al, [tmp]
    mov [di], al
    inc di
    fadd st1
    loop .gen
    finit
    ret

part:     dw 0
frames:   dw 0
dur:      dw 0
lastfade: db 0
xtime:    db 0
ytime:    db 0
seed:     dw 12345
c256:     dw 256
c127:     dw 127
tmp:      dw 0
sine:     times 256 db 0
pal:      times 768 db 0
xs:       times NSTARS dw 0
ys:       times NSTARS dw 0
zs:       times NSTARS dw 0
```

That is a demo: parts, a clock, transitions, a clean exit. Adding a third
part means writing two routines and one line in `parts`. Adding music means
calling a player routine once per frame from `main`.

The last thing a real intro does is synchronise to the music instead of to
the frame counter — the player knows which row of the song it is on, and the
parts read that. [examples/cracktro.asm](../examples/cracktro.asm) is this
same skeleton with five effects and an OPL2 player running from the same
frame tick.

### 19. Going faster

When a frame takes too long, work in this order:

1. **Do less per pixel.** Six instructions per pixel × 64000 pixels is
   384000 instructions a frame; at 70 fps that is 27 million instructions a
   second. The budget is real. Look at the inner loop first, always.
2. **Move loop-invariant work outward.** Anything depending only on `y` goes
   in the row loop; anything depending only on the frame goes above it.
   (`examples/plasma.asm` builds a 320-entry table of the x wave once per
   frame and then adds one byte per pixel.)
3. **Replace maths with tables.** Multiplies, divides, square roots and
   trigonometry all become one `xlat` or one `mov al, [bx+si]`.
4. **Use the string instructions.** `stosb`/`movsw` combine a store, a
   pointer increment and (with `rep`) the loop; `lodsb` does the same for
   reads. `rep movsw` is the fastest memory copy available on a 286, and
   `rep movsd` on a 386.
5. **Unroll.** `%rep 4` around the body of an inner loop removes three
   quarters of the loop overhead. Unroll a whole 320-pixel row and the
   overhead vanishes at the cost of code size.
6. **Halve the resolution.** Render 160×100 and double it on the way out, or
   render every second row and let the copy duplicate it. Most classic
   effects (fire, plasma, tunnels) were done this way; nobody noticed.
7. **Only touch what changed.** Starfields erase and redraw individual
   pixels rather than clearing 64000 bytes; copper bars change nothing at
   all.

Measure rather than guess:

```
./asm-emu run -headless -max-insns 20000000 -stats lesson11.asm
./asm-emu run -speed 8 lesson11.asm      # would it have run on a 286?
```

### 20. Size coding

Intros are often size-limited: 4096 bytes, 1024, 256. A `.COM` file has no
header, so the file *is* the code, and every byte you save is real. The
tricks, roughly in order of how much they save:

- **Exit with `ret`** (1 byte) instead of `mov ax,4C00h` / `int 21h` (5).
- **`int 20h`** (2 bytes) is the other short exit.
- **Set mode 13h in three bytes**: `mov al, 13h` / `int 10h` — `AH` is
  already 0 when DOS starts you.
- **`push 0A000h` / `pop es`** (4 bytes) beats `mov ax, 0A000h` / `mov es,
  ax` (5). `push cs` / `pop ds` (2) resets `DS`.
- **Prefer `AL`/`AX`.** Many instructions have a short accumulator-only
  form: `xchg ax, reg` and `inc reg` (16-bit) are one byte each.
- **`xor ax, ax`** (2) instead of `mov ax, 0` (3); `cwd` after it to zero
  `DX` too.
- **Use the flags you already have.** `jcxz`, `loop`, `salc`, `sbb al, al`
  (turn `CF` into 0 or −1) replace whole comparisons.
- **`xlat`** is one byte for a table lookup that would otherwise be three.
- **Let bytes wrap** instead of masking: `inc al` costs nothing at 255.
- **Reuse memory you did not allocate.** The PSP at offset 80h is 128 bytes
  of scratch; unused BIOS data and the whole segment above your code are
  free.
- **Generate data instead of storing it.** A 256-byte sine table costs 256
  bytes as `db`, or about 40 bytes of FPU code — and the FPU code can make
  256 entries of anything.

The 256-byte intro `memories`, in the corpus this emulator is tested
against, is a full multi-effect production in one quarter of a kilobyte. It
is worth disassembling once you can read this guide's listings.

---

## Debugging and common mistakes

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

---

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

- [MANUAL.md](../MANUAL.md) — the full instruction set, every BIOS and DOS
  call, every emulated port.
- [examples/](../examples) — plasma, fire, tunnel, rotozoom, cube,
  starfield, mandelbrot, sine scroller and a full cracktro, all runnable
  with `asm-emu run`.
- `asm-emu asm file.asm -l file.lst` — read what your source became.
- The corpus in `docs/ACCURACY.md` and the `tests/` directory show how the
  emulator is validated against real hardware traces, if you want to know
  how far you can trust it. (Answer: far enough that programs written here
  run on a real 386.)
