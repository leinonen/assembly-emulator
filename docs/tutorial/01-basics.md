# Part 1 — The basics

Five short programs: printing text, looping, doing arithmetic, reading
keys — and the preprocessor that saves you from typing any of it twice.

## 1. Hello, DOS

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

## 2. Registers, flags and loops

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

## 3. Memory, the stack and procedures

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

## 4. Reading the keyboard

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

## 5. Macros and the preprocessor

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

[Contents](README.md) · [Part 2 — Pixels →](02-pixels.md)
