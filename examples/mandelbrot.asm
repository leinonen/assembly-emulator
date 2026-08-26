; Mandelbrot set rendered with the x87 (FCOMP / FSTSW / SAHF for the
; escape test). Draws straight to video memory line by line; ESC quits
; at any time, even mid-render.
org 100h
bits 16

MAXITER equ 32

section .data
    re0   dq -2.5               ; left edge
    im0   dq -1.25              ; top edge
    dre   dq 0.0109375          ; 3.5 / 320
    dim   dq 0.0125             ; 2.5 / 200
    four  dq 4.0
    cre   dq 0.0                ; current c
    cim   dq 0.0

section .text
start:
    mov ax, 0x13
    int 0x10

    ; palette: 0 = inside (black), 1..32 = blue -> white
    mov dx, 0x3C8
    xor al, al
    out dx, al
    inc dx
    xor al, al
    out dx, al
    out dx, al
    out dx, al
    mov cx, 1
.pal:
    mov al, cl
    dec al
    add al, al
    out dx, al                  ; R = (i-1)*2
    out dx, al                  ; G
    mov al, cl
    add al, 31
    out dx, al                  ; B = 31+i
    inc cx
    cmp cx, MAXITER
    jbe .pal

    mov ax, 0xA000
    mov es, ax
    xor di, di
    finit
    fld qword [im0]
    fstp qword [cim]
    mov dx, 200                 ; rows left
row_loop:
    fld qword [re0]
    fstp qword [cre]
    mov bx, 320                 ; columns left
pix_loop:
    fldz                        ; zi
    fldz                        ; zr
    mov cx, MAXITER
iter:
    ; ST0=zr ST1=zi
    fld st0
    fmul st0, st0               ; zr^2      ST0=zr2 ST1=zr ST2=zi
    fld st2
    fmul st0, st0               ; zi^2      ST0=zi2 ST1=zr2 ST2=zr ST3=zi
    fld st1
    fadd st0, st1               ; |z|^2     ST0=mag ST1=zi2 ST2=zr2 ST3=zr ST4=zi
    fcomp qword [four]          ; pop mag
    fstsw ax
    sahf
    ja escaped                  ; |z|^2 > 4
    ; zi' = 2*zr*zi + cim ; zr' = zr^2 - zi^2 + cre
    fsubp st1, st0              ; ST0=zr2-zi2 ST1=zr ST2=zi
    fld st1                     ; zr
    fmul st0, st3               ; zr*zi
    fadd st0, st0               ; 2*zr*zi
    fadd qword [cim]            ; ST0=zi' ST1=zr2-zi2 ST2=zr ST3=zi
    fxch st3
    fstp st0                    ; drop old zi: ST0=zr2-zi2 ST1=zr ST2=zi'
    fadd qword [cre]            ; ST0=zr' ST1=zr ST2=zi'
    fxch st1
    fstp st0                    ; drop old zr: ST0=zr' ST1=zi'
    loop iter
    ; never escaped: inside the set
    fstp st0
    fstp st0
    xor al, al
    jmp plot
escaped:
    fstp st0                    ; zi2
    fstp st0                    ; zr2
    fstp st0                    ; zr
    fstp st0                    ; zi
    mov al, MAXITER + 1
    sub al, cl                  ; iteration count 1..MAXITER
plot:
    stosb
    fld qword [cre]
    fadd qword [dre]
    fstp qword [cre]
    dec bx
    jnz pix_loop

    fld qword [cim]
    fadd qword [dim]
    fstp qword [cim]

    ; allow ESC mid-render
    mov ah, 0x01
    int 0x16
    jz .next_row
    mov ah, 0x00
    int 0x16
    cmp al, 27
    je done
.next_row:
    dec dx
    jnz row_loop

wait_key:
    mov ah, 0x01
    int 0x16
    jz wait_key
    mov ah, 0x00
    int 0x16
    cmp al, 27
    jne wait_key
done:
    mov ax, 0x03
    int 0x10
    mov ax, 0x4C00
    int 0x21
