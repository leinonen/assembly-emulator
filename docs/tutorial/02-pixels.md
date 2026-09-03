# Part 2 — Pixels

Mode 13h, the palette and the frame loop, plus the two techniques every
effect in Part 3 is built from: lookup tables and fixed-point arithmetic.

## 6. Mode 13h and your first pixel

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

## 7. The palette

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
That trick is [lesson 13](03-effects.md#13-copper-bars).

The BIOS has palette calls too (`int 10h`, `AX=1010h` for one entry,
`AX=1012h` for a block from `ES:DX`), and they are perfectly fine outside
the inner loop. Direct port writes are what demos use because they are
faster and can be timed against the retrace.

## 8. The frame loop: back buffer and vsync

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

## 9. Lookup tables and the sine table

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
| `atan2` and `distance` per pixel | `fpatan`, `fsqrt` | tunnels ([examples/tunnel.asm](../../examples/tunnel.asm)) |
| a 320-entry row offset table | `y*320` once | avoids the shift-add per row |
| a random table | an LCG or xorshift | fire, starfields, noise |

## 10. Fixed-point maths

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

[← Part 1 — The basics](01-basics.md) · [Contents](README.md) · [Part 3 — Effects →](03-effects.md)
