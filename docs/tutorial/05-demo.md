# Part 5 — Making a demo

Five effects do not make a demo. A timeline, transitions, a frame budget
and a size budget do.

## 18. Structure and a timeline

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
parts read that. [examples/cracktro.asm](../../examples/cracktro.asm) is this
same skeleton with five effects and an OPL2 player running from the same
frame tick.

## 19. Going faster

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

## 20. Size coding

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

[← Part 4 — Sound](04-sound.md) · [Contents](README.md) · [Debugging and reference →](06-reference.md)
