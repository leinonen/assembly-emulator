; Tunnel effect. The x87 builds two 320x200 lookup tables at startup
; (angle via FPATAN, depth via FSQRT/FDIVR); every frame is then a cheap
; per-pixel table walk with a scrolling XOR texture. Press ESC to quit.
org 100h
bits 16

ANGLE_SEG equ 0x7000            ; 64000-byte angle table
DEPTH_SEG equ 0x8000            ; 64000-byte depth table
BACK_SEG  equ 0x9000            ; double buffer

section .data
    scale   dq 40.74366543152521 ; 256 / (2*pi): radians -> 0..255
    kdepth  dq 6000.0            ; depth = kdepth / radius
    dxw     dw 0
    dyw     dw 0
    tmp     dw 0
    frame   dw 0

section .text
start:
    mov ax, 0x13
    int 0x10

    ; ------------------------------------------------------------
    ; Palette: warm cyclic gradient so the XOR texture wraps nicely
    ; ------------------------------------------------------------
    mov dx, 0x3C8
    xor al, al
    out dx, al
    inc dx
    xor cx, cx
pal_loop:
    mov al, cl
    shr al, 2                   ; R = i/4
    out dx, al
    mov al, cl
    and al, 0x7F
    shr al, 1                   ; G = (i mod 128)/2
    out dx, al
    mov al, cl
    shr al, 2
    xor al, 63                  ; B = 63 - i/4
    out dx, al
    inc cl
    jnz pal_loop

    ; ------------------------------------------------------------
    ; Build tables: for each pixel (dx,dy) relative to the centre
    ;   angle[i] = atan2(dy,dx) * 256/2pi        (byte, wraps)
    ;   depth[i] = min(255, kdepth / sqrt(dx^2+dy^2))
    ; ------------------------------------------------------------
    mov ax, ANGLE_SEG
    mov es, ax
    mov ax, DEPTH_SEG
    mov fs, ax
    finit
    xor di, di
    mov word [dyw], -100
gen_y:
    mov word [dxw], -160
gen_x:
    fild word [dyw]             ; ST0=dy
    fild word [dxw]             ; ST0=dx, ST1=dy
    fpatan                      ; ST0=atan2(dy,dx)
    fmul qword [scale]
    fistp word [tmp]
    mov al, [tmp]
    mov [es:di], al

    fild word [dxw]
    fmul st0, st0               ; dx^2
    fild word [dyw]
    fmul st0, st0               ; dy^2
    faddp
    fsqrt                       ; radius
    fdivr qword [kdepth]        ; kdepth / radius (inf at the centre)
    fistp word [tmp]
    mov ax, [tmp]
    cmp ax, 255
    jbe .clamped                ; 0x8000 (inf) and >255 both clamp
    mov ax, 255
.clamped:
    mov [fs:di], al

    inc di
    inc word [dxw]
    cmp word [dxw], 160
    jl gen_x
    inc word [dyw]
    cmp word [dyw], 100
    jl gen_y

    mov ax, BACK_SEG
    mov gs, ax

    ; ------------------------------------------------------------
    ; Main loop: colour = (depth + t) xor (angle + t/2)
    ; ------------------------------------------------------------
main_loop:
    mov bx, [frame]
    mov bh, bl
    shr bh, 1                   ; BL = t, BH = t/2
    xor di, di
    mov cx, 64000
pix_loop:
    mov al, [fs:di]             ; depth
    add al, bl
    mov ah, [es:di]             ; angle
    add ah, bh
    xor al, ah
    mov [gs:di], al
    inc di
    loop pix_loop
    add word [frame], 2

    ; wait for vertical retrace
    mov dx, 0x3DA
.vs_a:
    in al, dx
    test al, 8
    jnz .vs_a
.vs_b:
    in al, dx
    test al, 8
    jz .vs_b

    ; blit back buffer to VGA
    push ds
    push es
    mov ax, BACK_SEG
    mov ds, ax
    xor si, si
    mov ax, 0xA000
    mov es, ax
    xor di, di
    mov cx, 32000
    rep movsw
    pop es
    pop ds

    ; ESC to exit
    mov ah, 0x01
    int 0x16
    jz main_loop
    mov ah, 0x00
    int 0x16
    cmp al, 27
    jne main_loop

    mov ax, 0x03
    int 0x10
    mov ax, 0x4C00
    int 0x21
