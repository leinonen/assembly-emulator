; Starfield with Perspective Projection and Rotation
; 100 stars, depth-based brightness and size, rotating around Z-axis
;
; Memory layout in segment 0x7000:
; Offset 0-599: Star data (100 stars × 6 bytes: X, Y, Z as words)

.data
; Sine lookup table (256 entries, values 0-255, centered at 127)
sine_table: db 127,130,133,136,139,142,145,148,151,154,157,160,163,166,169,172,175,178,181,184,187,190,193,196,199,202,205,208,211,214,217,219,222,225,228,231,233,236,239,241,244,247,249,252,254,255,255,255,255,255,255,255,254,252,249,247,244,241,239,236,233,231,228,225,222,219,217,214,211,208,205,202,199,196,193,190,187,184,181,178,175,172,169,166,163,160,157,154,151,148,145,142,139,136,133,130,127,124,121,118,115,112,109,106,103,100,97,94,91,88,85,82,79,76,73,70,67,64,61,58,55,52,49,46,43,40,37,35,32,29,26,23,21,18,15,13,10,7,5,2,0,0,0,0,0,0,0,0,2,5,7,10,13,15,18,21,23,26,29,32,35,37,40,43,46,49,52,55,58,61,64,67,70,73,76,79,82,85,88,91,94,97,100,103,106,109,112,115,118,121,124

; PRNG seed (persisted across function calls)
prng_seed: dw 12345

.code
start:
    ; Initialize PRNG seed in memory
    push ax
    push ds
    push si
    xor ax, ax
    mov ds, ax
    mov si, 0x0500
    mov ax, 12345
    mov [si], ax            ; Initial seed value
    pop si
    pop ds
    pop ax

    ; Set VGA Mode 13h (320x200, 256 colors)
    mov ax, 0x13
    int 0x10

    ; Set up grayscale palette (17 shades from black to white)
    call setup_palette

    ; Initialize stars with random positions
    call init_stars

    ; Initialize angle in BP register
    xor bp, bp

main_loop:
    ; Clear video memory to black
    call clear_buffer

    ; Increment rotation angle (stored in BP)
    inc bp

    ; Render all stars
    call render_stars

    ; Move stars forward (decrease Z)
    call move_stars

    ; Add delay to slow down animation
    call delay

    ; Check for ESC key
    mov ah, 1
    int 0x16
    jz main_loop

    xor ah, ah
    int 0x16
    cmp al, 27  ; ESC key
    jne main_loop

    ; Return to text mode
    mov ax, 0x03
    int 0x10

    ; Exit
    mov ax, 0x4C00
    int 0x21

; Set up grayscale palette (16 shades + black)
setup_palette:
    push ax
    push bx
    push cx
    push dx

    ; Start at color 0 (black background)
    mov dx, 0x3C8
    xor al, al
    out dx, al

    ; Set color 0 to black
    mov dx, 0x3C9
    xor al, al
    out dx, al  ; R
    out dx, al  ; G
    out dx, al  ; B

    ; Set colors 1-16 as grayscale (dim to bright)
    mov cx, 16
    mov bl, 0

palette_loop:
    mov dx, 0x3C9
    mov al, bl
    shr al, 2   ; Scale to 0-63 for VGA DAC
    out dx, al  ; R
    out dx, al  ; G
    out dx, AL  ; B

    add bl, 4   ; Increment brightness
    loop palette_loop

    pop dx
    pop cx
    pop bx
    pop ax
    ret

; Initialize 100 stars with pseudo-random positions
; Stars stored at segment 0x7000, offset 0
init_stars:
    push ax
    push bx
    push cx
    push di
    push es

    mov ax, 0x7000
    mov es, ax
    mov cx, 200         ; 200 stars for better coverage
    xor di, di          ; Start at offset 0

init_loop:
    push cx
    push di

    ; Generate random X (0-255)
    call prng
    xor ah, ah
    pop di
    stosw               ; Store X, DI += 2

    ; Generate random Y (0-255)
    push di
    call prng
    xor ah, ah
    pop di
    stosw               ; Store Y, DI += 2

    ; Generate Z coordinate (128 to 384)
    push di
    call prng
    xor ah, ah
    add ax, 128
    pop di
    stosw               ; Store Z, DI += 2

    pop cx
    loop init_loop

    pop es
    pop di
    pop cx
    pop bx
    pop ax
    ret

; Simple PRNG (Linear Congruential Generator)
; Seed stored in memory at 0x0500 (safe DOS memory area)
; Output: AL = random byte
prng:
    push bx
    push dx
    push ds
    push si

    ; Point DS to segment 0
    xor ax, ax
    mov ds, ax
    mov si, 0x0500      ; Seed stored at 0x0000:0x0500

    ; Load seed from memory
    mov dx, [si]
    mov ax, dx

    ; AX = (AX * 25173 + 13849) & 0xFFFF
    mov bx, 25173
    mul bx              ; DX:AX = AX * 25173
    add ax, 13849

    ; Save new seed back to memory
    mov [si], ax

    pop si
    pop ds
    pop dx
    pop bx
    ret

; Clear video memory (0xA000) to black
clear_buffer:
    push ax
    push cx
    push di
    push es

    mov ax, 0xA000
    mov es, ax
    xor di, di
    xor ax, ax
    mov cx, 32000   ; 64000 bytes / 2
    rep stosw

    pop es
    pop di
    pop cx
    pop ax
    ret

; Render all stars with rotation and perspective
; Uses BP for rotation angle
render_stars:
    push ax
    push bx
    push cx
    push dx
    push si
    push di
    push ds
    push es

    ; Set DS to star data segment
    mov ax, 0x7000
    mov ds, ax
    ; Set ES to video memory
    mov ax, 0xA000
    mov es, ax

    mov cx, 200
    xor si, si      ; Start at offset 0

star_loop:
    push cx

    ; Load star X, Y, Z
    lodsw               ; AX = X (0-255)
    mov di, ax          ; DI = X
    lodsw               ; AX = Y (0-255)
    push ax             ; Save Y on stack
    lodsw               ; AX = Z (SI auto-increments to next star)
    mov bx, ax          ; BX = Z

    ; Perspective projection from center
    ; Store Z for later
    push bx

    ; screen_x = 160 + (X - 128) * 160 / Z
    ; X range is 0-255, centered at 128
    ; Focal length 160: at Z=128, max projection = 127*160/128 = 159px (fits!)
    mov ax, di          ; AX = X
    sub ax, 128         ; AX = X - 128
    ; Check if result is negative (> 32767 means wrapped/negative)
    cmp ax, 32767
    jbe pos_x           ; If <= 32767, it's positive
    ; Was negative (wrapped around)
    neg ax
    mov cx, 160
    mul cx
    div bx
    mov cx, 160
    sub cx, ax          ; 160 - offset
    jmp got_screen_x
pos_x:
    mov cx, 160
    mul cx
    div bx
    add ax, 160
    mov cx, ax
got_screen_x:

    ; screen_y = 100 + (Y - 128) * 160 / Z
    ; Y range is 0-255, centered at 128
    pop bx              ; Get Z
    pop ax              ; AX = Y
    push bx             ; Save Z again
    sub ax, 128
    cmp ax, 32767
    jbe pos_y
    ; Negative
    neg ax
    mov di, 160
    mul di
    div bx
    mov dx, 100
    sub dx, ax
    jmp got_screen_y
pos_y:
    mov di, 160
    mul di
    div bx
    add ax, 100
    mov dx, ax
got_screen_y:
    pop bx              ; Restore Z

    ; Calculate brightness from Z (closer = brighter)
    ; Z range is 128-384, map to colors 1-16
    mov ax, bx
    shr ax, 4           ; Z / 16 (128→8, 384→24)
    mov di, 24
    sub di, ax          ; 24 - (Z/16): Z=128 → 16, Z=384 → 0
    cmp di, 1
    jge bright_ok
    mov di, 1
bright_ok:
    cmp di, 16
    jle bright_ok2
    mov di, 16
bright_ok2:
    mov bx, di          ; BX = brightness

    ; Draw pixel at CX, DX with color BL
    ; Check bounds
    cmp cx, 0
    jl next_star
    cmp cx, 319
    jg next_star
    cmp dx, 0
    jl next_star
    cmp dx, 199
    jg next_star

    ; Calculate offset: Y * 320 + X
    mov ax, dx
    mov dx, 320
    mul dx
    add ax, cx
    mov di, ax

    ; Write pixel (ES already points to 0xA000)
    mov al, bl
    stosb

next_star:
    pop cx
    loop star_loop

    pop es
    pop ds
    pop di
    pop si
    pop dx
    pop cx
    pop bx
    pop ax
    ret

; Move stars forward (decrease Z), wrap if behind camera
move_stars:
    push ax
    push bx
    push cx
    push dx
    push si
    push di
    push ds

    mov ax, 0x7000
    mov ds, ax

    mov cx, 200
    xor si, si          ; Start at offset 0 (X, Y, Z)

move_loop:
    push cx

    ; Load Z
    mov ax, [si+4]      ; Z is at offset +4 (after X and Y)

    ; Move forward (moderate speed)
    sub ax, 4           ; Movement speed

    ; Check if behind camera
    cmp ax, 128
    jge no_wrap

    ; Star wrapped - generate new random position
    ; Generate new X
    call prng
    xor ah, ah
    mov [si], ax        ; Store new X

    ; Generate new Y
    call prng
    xor ah, ah
    mov [si+2], ax      ; Store new Y

    ; Reset Z to far (match new Z range 128-384)
    mov ax, 350
    jmp store_z

no_wrap:
    ; Just update Z
store_z:
    mov [si+4], ax

    pop cx
    add si, 6           ; Next star (X=2, Y=2, Z=2 bytes)
    loop move_loop

    pop ds
    pop di
    pop si
    pop dx
    pop cx
    pop bx
    pop ax
    ret

; Delay to slow down animation
delay:
    push cx
    push dx

    ; Outer loop - increased for slower animation
    mov cx, 10
delay_outer:
    push cx
    ; Inner loop for delay
    mov cx, 0x8000
delay_inner:
    nop
    loop delay_inner
    pop cx
    loop delay_outer

    pop dx
    pop cx
    ret
