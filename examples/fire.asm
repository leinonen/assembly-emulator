; Classic Fire Effect - Double Buffered (Demoscene Style)
.code
    ; Set VGA Mode 13h
    mov ax, 0x13
    int 0x10

    ; Set up fire palette (64 colors)
    mov dx, 0x3C8
    mov al, 0
    out dx, al
    mov dx, 0x3C9

    ; Colors 0-31: Black to red
    mov cx, 32
    pal_red:
        mov al, cl
        shl al, 1
        out dx, al
        xor al, al
        out dx, al
        out dx, al
        loop pal_red

    ; Colors 32-47: Red to orange/yellow
    mov cx, 16
    pal_orange:
        mov al, 63
        out dx, al
        mov al, cl
        shl al, 2
        out dx, al
        xor al, al
        out dx, al
        loop pal_orange

    ; Colors 48-63: Yellow to white
    mov cx, 16
    pal_white:
        mov al, 63
        out dx, al
        out dx, al
        mov al, cl
        shl al, 2
        out dx, al
        loop pal_white

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
    ; Step 1: Fill bottom line with varied random hot pixels
    mov di, 63680
    mov cx, 320
    fill_bottom:
        ; Better PRNG
        mov ax, bp
        mov bx, ax
        shl ax, 7
        xor ax, bx
        mov bx, ax
        shr bx, 9
        xor ax, bx
        mov bp, ax

        ; More varied random values (40-63) for better effect
        and al, 0x1F   ; 0-31
        add al, 40     ; 40-71
        cmp al, 63
        jbe heat_ok
        mov al, 63
        heat_ok:

        mov [di], al
        inc di
        loop fill_bottom

    ; Step 2: Propagate fire upward
    ; Copy each line from the line below
    ; Process lines 198 down to 0
    mov bx, 198

    copy_lines:
        cmp bx, 0
        jl check_key

        ; Source = (bx+1) * 320
        mov ax, bx
        inc ax
        mov dx, 320
        mul dx
        mov si, ax

        ; Dest = bx * 320
        mov ax, bx
        mov dx, 320
        mul dx
        mov di, ax

        ; Copy 320 pixels with horizontal averaging
        ; Skip first and last pixel (edges)
        inc si
        inc di
        mov cx, 318

        copy_pixels:
            ; Read 3 pixels: left, center, right
            mov al, [si]      ; Center
            mov ah, 0
            push ax

            dec si
            mov al, [si]      ; Left
            mov ah, 0
            mov dx, ax
            inc si
            pop ax
            add ax, dx        ; Sum = center + left

            inc si
            mov dl, [si]      ; Right
            mov dh, 0
            add ax, dx        ; Sum = center + left + right
            dec si

            ; Less cooling - divide by 3 approximately
            ; Shift by 2 would be /4, so shift by 1 for /2 which is closer
            shr ax, 1

            ; Very rare cooling - cool down 1 in 16 times
            mov dx, bp
            and dx, 15
            cmp dx, 0
            jne write_it

            cmp ax, 0
            je write_it
            dec ax

            write_it:
            ; Write to destination
            mov [di], al

            inc si
            inc di
            loop copy_pixels

        ; Handle last pixel
        inc si
        inc di

        dec bx
        jmp copy_lines

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
