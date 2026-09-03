# Part 3 — Effects

Everything from here on is the same program: build tables, then per frame
fill 64000 bytes and flip. What changes is the function that turns *(x, y,
time)* into a colour. Two rules of thumb carry you a long way:

1. **Move work out of the inner loop.** Anything that depends only on the
   row, or only on the frame, is computed once per row or once per frame.
2. **Let bytes wrap.** An 8-bit index into a 256-entry table wraps for free,
   which is why demo maths is full of `add al, something` with no masking.

## 11. Plasma

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

## 12. Fire

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

## 13. Copper bars

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

## 14. Starfield

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

## 15. A sine scroller

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

[← Part 2 — Pixels](02-pixels.md) · [Contents](README.md) · [Part 4 — Sound →](04-sound.md)
