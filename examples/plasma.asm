; Classic Plasma - Simple and effective

.code
    JMP start

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
    push ds
    push cs
    pop ds

    ; Red component: sin(i)
    mov si, cx
    and si, 0xFF        ; mask to 0-255 first
    add si, 4           ; offset to sine_table
    cmp si, 260
    jl pal_red_ok
    sub si, 256
pal_red_ok:
    mov al, [si]
    shr al, 2           ; scale to 0-63
    out dx, al

    ; Green component: sin(i + 85)
    mov si, cx
    add si, 85
    and si, 0xFF
    add si, 4
    cmp si, 260
    jl pal_green_ok
    sub si, 256
pal_green_ok:
    mov al, [si]
    shr al, 2
    out dx, al

    ; Blue component: sin(i + 170)
    mov si, cx
    add si, 170
    and si, 0xFF
    add si, 4
    cmp si, 260
    jl pal_blue_ok
    sub si, 256
pal_blue_ok:
    mov al, [si]
    shr al, 2
    out dx, al

    pop ds

    inc cx
    cmp cx, 256
    jl pal

    xor bp, bp

main_loop:
    ; Render to backbuffer at 0x7000:0
    mov ax, 0x7000
    mov es, ax
    xor di, di
    xor bx, bx

y_loop:
    xor cx, cx

x_loop:
    push ds
    push cs
    pop ds

    ; Wave 1: sin(x + x/2 + time*2)
    mov ax, cx
    mov dx, ax
    shr dx, 1           ; X/2 in DX
    add ax, dx          ; X + X/2
    mov dx, bp
    add dx, bp          ; time * 2 in DX
    add ax, dx
    and ax, 0xFF        ; Mask first to get 0-255
    add ax, 4           ; Then add offset to get 4-259, but we need to wrap!
    cmp ax, 260         ; If >= 260 (beyond sine table)
    jl wave1_ok
    sub ax, 256         ; Wrap: subtract 256 to get back to 4-7
wave1_ok:
    mov si, ax
    mov al, [si]
    mov dh, al

    ; Wave 2: sin(y + y/2 + time*3)
    mov ax, bx
    push dx
    mov dx, ax
    shr dx, 1           ; Y/2 in DX
    add ax, dx          ; Y + Y/2
    pop dx
    push dx
    mov dx, bp
    add dx, bp
    add dx, bp          ; time * 3 in DX
    add ax, dx
    pop dx
    and ax, 0xFF        ; Mask first to get 0-255
    add ax, 4           ; Then add offset
    cmp ax, 260
    jl wave2_ok
    sub ax, 256
wave2_ok:
    mov si, ax
    mov al, [si]
    add dh, al

    ; Wave 3: sin(x+y + time)
    mov ax, cx
    add ax, bx
    add ax, bp
    and ax, 0xFF        ; Mask first to get 0-255
    add ax, 4           ; Then add offset
    cmp ax, 260
    jl wave3_ok
    sub ax, 256
wave3_ok:
    mov si, ax
    mov al, [si]

    ; Combine waves using addition (classic plasma effect)
    add al, dh

    pop ds

    ; mov [di], al
    ; inc di
    stosb

    inc cx
    cmp cx, 320
    jl x_loop

    inc bx
    cmp bx, 200
    jl y_loop

    ; Wait for VBlank
    mov dx, 0x3DA
    in al, dx

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
    jz main_loop

    mov ah, 0x00
    int 0x16
    cmp al, 27
    jne main_loop

    mov ax, 0x03
    int 0x10
    hlt
