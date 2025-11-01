; Classic Fire Effect - Double Buffered (Demoscene Style)
.code
    ; Set VGA Mode 13h
    mov ax, 0x13
    int 0x10

    ; Set up fire palette (256 colors) - smooth black → red → orange → yellow → white
    mov dx, 0x3C8
    mov al, 0
    out dx, al
    mov dx, 0x3C9

    ; Colors 0-63: Black to red (red: 0 → 63)
    mov cx, 64
    mov bx, 0
    pal_black_to_red:
        mov al, bl        ; Red: 0-63
        out dx, al
        xor al, al
        out dx, al        ; Green = 0
        out dx, al        ; Blue = 0
        inc bx
        loop pal_black_to_red

    ; Colors 64-127: Red to orange (red=63, green: 0 → 63)
    mov cx, 64
    mov bx, 0
    pal_red_to_orange:
        mov al, 63
        out dx, al        ; Red = 63
        mov al, bl        ; Green: 0-63
        out dx, al
        xor al, al
        out dx, al        ; Blue = 0
        inc bx
        loop pal_red_to_orange

    ; Colors 128-191: Orange to yellow (red=63, green=63, blue: 0 → 63)
    mov cx, 64
    mov bx, 0
    pal_orange_to_yellow:
        mov al, 63
        out dx, al        ; Red = 63
        mov al, 63
        out dx, al        ; Green = 63
        mov al, bl        ; Blue: 0-63
        out dx, al
        inc bx
        loop pal_orange_to_yellow

    ; Colors 192-255: Yellow to white (all at 63, stay bright)
    mov cx, 64
    mov bx, 0
    pal_yellow_to_white:
        mov al, 63
        out dx, al        ; Red = 63
        mov al, 63
        out dx, al        ; Green = 63
        mov al, 63
        out dx, al        ; Blue = 63 (pure white/yellow)
        inc bx
        loop pal_yellow_to_white

    ; Set DS and ES to back buffer segment for rendering (0x7000)
    ; This is our off-screen buffer in regular RAM (64000 bytes)
    mov ax, 0x7000
    mov ds, ax
    mov es, ax

    ; PRNG seed
    mov bp, 0x5678

    ; Clear screen
    xor di, di
    mov cx, 32000
    xor ax, ax
    rep stosw

; Main loop
main_loop:
    ; Step 1: Fill bottom TWO lines with random hot pixels and black specks
    ; Line 198 (offset 63360) and Line 199 (offset 63680)

    ; Fill line 198
    mov di, 63360
    mov cx, 320
    fill_line_198:
        ; Better PRNG
        mov ax, bp
        mov bx, ax
        shl ax, 7
        xor ax, bx
        mov bx, ax
        shr bx, 9
        xor ax, bx
        mov bp, ax

        ; Use high bit to determine if this should be a black speck
        test ah, 0x80     ; Test bit 15 of random value
        jz hot_pixel_198

        ; Black speck (about 1 in 2 chance based on high bit)
        ; But let's make it less frequent - check another bit
        test ah, 0x40
        jz hot_pixel_198

        xor al, al        ; Black pixel
        jmp write_198

        hot_pixel_198:
        ; High intensity for fire source (192-255)
        and al, 0x3F      ; 0-63
        add al, 192       ; 192-255

        write_198:
        mov [di], al
        inc di
        loop fill_line_198

    ; Fill line 199
    mov di, 63680
    mov cx, 320
    fill_line_199:
        ; Better PRNG
        mov ax, bp
        mov bx, ax
        shl ax, 7
        xor ax, bx
        mov bx, ax
        shr bx, 9
        xor ax, bx
        mov bp, ax

        ; Use high bits to determine if this should be a black speck
        test ah, 0x80
        jz hot_pixel_199
        test ah, 0x40
        jz hot_pixel_199

        xor al, al        ; Black pixel
        jmp write_199

        hot_pixel_199:
        ; High intensity for fire source (192-255)
        and al, 0x3F      ; 0-63
        add al, 192       ; 192-255

        write_199:
        mov [di], al
        inc di
        loop fill_line_199

    ; Step 2: Classic fire propagation with 4-point averaging
    ; Process from line 197 down to 0 (lines 198-199 are the heat source)
    mov bx, 197

    propagate_lines:
        cmp bx, 0
        jl check_key

        ; Calculate destination line offset
        mov ax, bx
        mov dx, 320
        mul dx
        mov di, ax

        ; Skip first pixel (left edge)
        inc di

        ; Process middle pixels (x = 1 to 318)
        mov cx, 318

        propagate_pixels:
            ; Average 4 pixels: [x-1,y+1], [x,y+1], [x+1,y+1], [x,y+2]
            xor ax, ax

            ; Pixel [x-1, y+1] = di + 319
            mov al, [di + 319]

            ; Pixel [x, y+1] = di + 320
            xor dx, dx
            mov dl, [di + 320]
            add ax, dx

            ; Pixel [x+1, y+1] = di + 321
            mov dl, [di + 321]
            add ax, dx

            ; Pixel [x, y+2] = di + 640
            mov dl, [di + 640]
            add ax, dx

            ; Divide by 4
            shr ax, 2

            ; Apply decay (cool down by 1)
            cmp ax, 0
            je write_pixel
            dec ax

            write_pixel:
            mov [di], al
            inc di
            loop propagate_pixels

        dec bx
        jmp propagate_lines

    check_key:
        ; Check for ESC key (non-blocking check)
        mov ah, 0x01
        int 0x16
        jz do_flip  ; No key pressed, continue

        mov ah, 0x00
        int 0x16
        cmp al, 0x1B
        je exit_program

    do_flip:
        ; Wait for VBlank - this will block until next frame
        ; Ensures smooth 60 FPS animation without tearing
        mov dx, 0x3DA
        in al, dx        ; Reading 0x3DA waits for VBlank via channel

        ; DOUBLE BUFFER FLIP: Copy complete back buffer to VGA memory
        ; This is the classic demoscene technique - atomic page flip!
        push ds
        push es

        ; Source: back buffer at 0x7000
        mov ax, 0x7000
        mov ds, ax
        xor si, si

        ; Dest: VGA memory
        mov ax, 0xA000
        mov es, ax
        xor di, di

        ; Copy 64000 bytes (320x200)
        mov cx, 32000      ; 64000 / 2 = 32000 words
        rep movsw

        pop es
        pop ds

        ; Restore DS and ES to back buffer for next frame rendering
        mov ax, 0x7000
        mov ds, ax
        mov es, ax

        jmp main_loop

    exit_program:
        hlt
