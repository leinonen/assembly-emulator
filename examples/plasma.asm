; Ported to NASM syntax: assemble with `asm-emu asm` or run directly.
org 100h
bits 16

; Classic Plasma - Simple and effective

section .data
sine_table:
    db 127, 130, 133, 136, 139, 143, 146, 149, 152, 155, 158, 161, 164, 167, 170, 173
    db 176, 179, 182, 184, 187, 190, 193, 195, 198, 200, 203, 205, 208, 210, 213, 215
    db 217, 219, 221, 224, 226, 228, 229, 231, 233, 235, 236, 238, 239, 241, 242, 244
    db 245, 246, 247, 248, 249, 250, 251, 251, 252, 253, 253, 254, 254, 254, 254, 254
    db 255, 254, 254, 254, 254, 254, 253, 253, 252, 251, 251, 250, 249, 248, 247, 246
    db 245, 244, 242, 241, 239, 238, 236, 235, 233, 231, 229, 228, 226, 224, 221, 219
    db 217, 215, 213, 210, 208, 205, 203, 200, 198, 195, 193, 190, 187, 184, 182, 179
    db 176, 173, 170, 167, 164, 161, 158, 155, 152, 149, 146, 143, 139, 136, 133, 130
    db 127, 124, 121, 118, 115, 111, 108, 105, 102, 99, 96, 93, 90, 87, 84, 81
    db 78, 75, 72, 70, 67, 64, 61, 59, 56, 54, 51, 49, 46, 44, 41, 39
    db 37, 35, 33, 30, 28, 26, 25, 23, 21, 19, 18, 16, 15, 13, 12, 10
    db 9, 8, 7, 6, 5, 4, 3, 3, 2, 1, 1, 0, 0, 0, 0, 0
    db 0, 0, 0, 0, 0, 0, 1, 1, 2, 3, 3, 4, 5, 6, 7, 8
    db 9, 10, 12, 13, 15, 16, 18, 19, 21, 23, 25, 26, 28, 30, 33, 35
    db 37, 39, 41, 44, 46, 49, 51, 54, 56, 59, 61, 64, 67, 70, 72, 75
    db 78, 81, 84, 87, 90, 93, 96, 99, 102, 105, 108, 111, 115, 118, 121, 124

wave1: times 320 db 0

section .text
start:
    mov ax, 0x13
    int 0x10

    ; Smooth rainbow palette using sine table
    mov dx, 0x3C8
    xor al, al
    out dx, al
    mov dx, 0x3C9
    xor cx, cx
pal:
    ; Red component: sin(i)
    mov si, sine_table
    mov ax, cx
    and ax, 0xFF        ; mask to 0-255
    add si, ax
    mov al, [si]
    shr al, 2           ; scale to 0-63
    out dx, al

    ; Green component: sin(i + 85)
    mov si, sine_table
    mov ax, cx
    add ax, 85
    and ax, 0xFF
    add si, ax
    mov al, [si]
    shr al, 2
    out dx, al

    ; Blue component: sin(i + 170)
    mov si, sine_table
    mov ax, cx
    add ax, 170
    and ax, 0xFF
    add si, ax
    mov al, [si]
    shr al, 2
    out dx, al

    inc cx
    cmp cx, 256
    jl pal

    xor bp, bp

main_loop:
    ; Render to backbuffer at 0x7000:0
    push ds                 ; Save data segment (for sine table access later)
    mov bx, sine_table      ; XLAT base (index in AL, so the &0xFF mask is free)

    ; Wave 1 depends only on X and time: build a 320-entry row table once
    ; per frame.  wave1[x] = sin(x + x/2 + time*2)
    mov di, wave1
    xor cx, cx
    mov dx, bp
    add dx, bp              ; time * 2
w1_loop:
    mov ax, cx
    shr ax, 1
    add ax, cx              ; X + X/2
    add ax, dx
    xlat
    mov [di], al
    inc di
    inc cx
    cmp cx, 320
    jl w1_loop

    mov ax, 0x7000
    mov es, ax
    xor di, di
    xor cx, cx              ; CX = Y

y_loop:
    push cx

    ; Wave 2 is constant along a row: sin(y + y/2 + time*3) -> DH
    mov ax, cx
    shr ax, 1
    add ax, cx              ; Y + Y/2
    add ax, bp
    add ax, bp
    add ax, bp              ; + time * 3
    xlat
    mov dh, al

    ; Wave 3: sin(x + y + time) -- index just increments with X, keep it in DL
    mov ax, cx
    add ax, bp
    mov dl, al

    mov si, wave1
    mov cx, 320
x_loop:
    lodsb                   ; wave 1 for this X
    mov ah, al
    mov al, dl
    xlat                    ; wave 3
    add al, ah
    add al, dh              ; + wave 2
    stosb
    inc dl
    loop x_loop

    pop cx
    inc cx
    cmp cx, 200
    jl y_loop

    ; Wait for VBlank
    mov dx, 0x3DA
    ; wait for the current vertical retrace to end, then for the next one to start
.vs138a:
    in al, dx
    test al, 8
    jnz .vs138a
.vs138b:
    in al, dx
    test al, 8
    jz .vs138b

    ; Copy backbuffer to VGA memory
    mov ax, 0x7000
    mov ds, ax
    xor si, si
    mov ax, 0xA000
    mov es, ax
    xor di, di
    mov cx, 32000       ; 64000 bytes / 2 = 32000 words
    rep movsw

    inc bp
    inc bp
    inc bp  ; Increment by 3 to make movement more visible

    ; Check for key
    xor ax, ax
    mov ds, ax
    mov ah, 0x01
    int 0x16
    pop ds                  ; Restore data segment for next iteration
    jz main_loop

    mov ah, 0x00
    int 0x16
    cmp al, 27
    jne main_loop

    mov ax, 0x03
    int 0x10
    mov ax, 4C00h
    int 21h
