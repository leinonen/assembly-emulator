; Sine Scroller Demo
; Text scrolling along a sine wave with actual character rendering
; Uses CP437 font data for readable text

.data
; Sine lookup table (256 entries, values 0-255)
sine_table: 
    db 127, 130, 133, 136, 139, 143, 146, 149, 152, 155, 158, 161, 164, 167, 170, 173,
    db 176, 179, 182, 184, 187, 190, 193, 195, 198, 200, 203, 205, 208, 210, 213, 215,
    db 217, 219, 221, 224, 226, 228, 229, 231, 233, 235, 236, 238, 239, 241, 242, 244,
    db 245, 246, 247, 248, 249, 250, 251, 251, 252, 253, 253, 254, 254, 254, 254, 254,
    db 255, 254, 254, 254, 254, 254, 253, 253, 252, 251, 251, 250, 249, 248, 247, 246,
    db 245, 244, 242, 241, 239, 238, 236, 235, 233, 231, 229, 228, 226, 224, 221, 219,
    db 217, 215, 213, 210, 208, 205, 203, 200, 198, 195, 193, 190, 187, 184, 182, 179,
    db 176, 173, 170, 167, 164, 161, 158, 155, 152, 149, 146, 143, 139, 136, 133, 130,
    db 127, 124, 121, 118, 115, 111, 108, 105, 102, 99, 96, 93, 90, 87, 84, 81,
    db 78, 75, 72, 70, 67, 64, 61, 59, 56, 54, 51, 49, 46, 44, 41, 39,
    db 37, 35, 33, 30, 28, 26, 25, 23, 21, 19, 18, 16, 15, 13, 12, 10,
    db 9, 8, 7, 6, 5, 4, 3, 3, 2, 1, 1, 0, 0, 0, 0, 0,
    db 0, 0, 0, 0, 0, 0, 1, 1, 2, 3, 3, 4, 5, 6, 7, 8,
    db 9, 10, 12, 13, 15, 16, 18, 19, 21, 23, 25, 26, 28, 30, 33, 35,
    db 37, 39, 41, 44, 46, 49, 51, 54, 56, 59, 61, 64, 67, 70, 72, 75,
    db 78, 81, 84, 87, 90, 93, 96, 99, 102, 105, 108, 111, 115, 118, 121, 124

; Scroll message
scroll_msg:
    db "  HELLO WORLD!  THIS IS A SINE SCROLLER...  SCROLLING TEXT ON A WAVE...  USING BIOS ROM FONT! ", 0

.code
; BIOS font is at F000:A000 (linear address 0xFA000)
FONT_SEG equ 0xF000
FONT_OFF equ 0xA000

; Scroll reset threshold (supports up to 128 character messages)
; -1024 pixels = 128 chars * 8 pixels/char = 0xFC00
SCROLL_RESET equ 0xFC00

start:
    ; Save data segment at fixed location (segment 0, offset 0xFFFE)
    ; so we can always access it regardless of current segment
    push ds
    xor ax, ax
    mov ds, ax
    mov si, 0xFFFE
    pop ax
    mov [si], ax
    ; Restore DS to data segment
    mov ds, ax

    ; Switch to VGA mode 13h
    mov ax, 0x13
    int 0x10

    ; Set ES to back buffer segment (0x7000)
    ; Keep DS at 0 (code segment) for data access
    mov ax, 0x7000
    mov es, ax

    ; Clear back buffer initially (blue background)
    xor di, di
    mov cx, 32000
    mov ax, 0x0101          ; Blue color (palette index 1)
    rep stosw

    ; BP = scroll_x position
    mov bp, 320
    ; DI = wave_time offset
    xor di, di

main_loop:
    push ds                 ; Save data segment for later restoration
    ; Clear back buffer (blue background)
    push es
    mov ax, 0x7000
    mov es, ax
    xor di, di
    mov ax, 0x0101          ; Blue color (palette index 1)
    mov cx, 32000
    rep stosw
    pop es

    ; Draw scrolling text characters
    mov si, scroll_msg
    xor bx, bx              ; BX = character index

draw_char_loop:
    lodsb                   ; AL = current character
    test al, al
    jz frame_done

    ; Calculate X position
    mov cx, bp
    mov dx, bx
    shl dx, 3
    add cx, dx              ; CX = char_x

    ; Skip if completely off-screen
    ; First check if negative (high bit set)
    test cx, 0x8000         ; Check if high bit set (negative)
    jz check_right_edge     ; If not negative, check right edge
    ; It's negative - skip if char_x < -7 (all 8 pixels are left of screen)
    cmp cx, 0xFFF9          ; Compare with -7 (0xFFF9 in two's complement)
    jl skip_char            ; Skip if less than -7 (SIGNED comparison)
    jmp bounds_ok           ; Otherwise OK to draw
check_right_edge:
    ; Right edge: skip if char_x >= 320
    cmp cx, 320
    jae skip_char
bounds_ok:

draw_character:
    ; Skip spaces
    push si
    dec si
    mov al, [si]
    pop si
    cmp al, 32
    je skip_char

    ; Get font data for this character from BIOS ROM
    ; Font offset = char_code * 16 (BIOS font has all 256 chars)
    xor ah, ah
    push cx
    mov cl, 4
    shl ax, cl              ; AX = char * 16
    pop cx

    ; Load BIOS font pointer and add character offset
    push si
    push ds
    mov si, FONT_OFF
    add si, ax              ; SI = font_off + (char * 16)
    mov ax, FONT_SEG
    mov ds, ax              ; DS:SI now points to BIOS font data in ROM

    push bx
    push cx

    ; Draw 16 rows
    xor bh, bh              ; BH = row offset (0-15)

char_row_loop:
    lodsb                   ; AL = font row bitmap from DS (code segment)
    mov ah, al              ; Save original bitmap in AH
    push cx
    mov bl, 8

char_pixel_loop:
    test ah, 0x80           ; Test MSB (leftmost pixel)
    jz skip_pixel           ; If 0, skip this pixel

    ; Check if pixel X is in bounds (0-319)
    ; Check for negative (high bit set means >= 32768, which wraps to negative)
    test cx, 0x8000
    jnz skip_pixel          ; Skip if negative (CX >= 32768)
    cmp cx, 320
    jge skip_pixel

    ; Calculate Y position for this pixel column based on its X coordinate
    push ax
    push bx
    push cx
    push si

    ; Calculate sine index: (pixel_x / 2 + wave_time) & 0xFF
    mov ax, cx              ; AX = pixel X
    shr ax, 1               ; AX = pixel_x / 2
    add ax, di              ; Add wave_time (stored in DI)
    and ax, 0xFF            ; Wrap to 0-255

    ; Look up sine value (sine_table is in data segment, not BIOS ROM!)
    push ds                 ; Save FONT_SEG
    push bx
    push si

    ; Get data segment from fixed location (segment 0, offset 0xFFFE)
    xor bx, bx
    mov ds, bx              ; DS = 0
    mov si, 0xFFFE
    mov bx, [si]            ; BX = saved data segment value
    mov ds, bx              ; DS now points to data segment

    ; Access sine_table
    mov si, sine_table
    add si, ax
    mov al, [si]            ; AL = sine value (0-255) from data segment

    pop si                  ; Restore SI
    pop bx
    pop ds                  ; Restore DS to BIOS ROM segment

    ; Calculate Y = 84 + ((sine - 128) * 40 / 128)
    sub al, 128             ; AL = sine - 128 (now -128 to +127)
    mov ah, al
    shr ah, 7               ; AH = sign bit
    test ah, ah
    jz positive_sine_pix

    ; Negative sine
    neg al
    xor ah, ah
    mov cx, 40
    mul cx                  ; AX = abs(sine-128) * 40
    xor dx, dx
    mov cx, 128
    div cx                  ; AX = result
    mov dx, 84
    sub dx, ax              ; DX = 84 - result
    jmp apply_row_offset

positive_sine_pix:
    xor ah, ah
    mov cx, 40
    mul cx                  ; AX = (sine-128) * 40
    xor dx, dx
    mov cx, 128
    div cx                  ; AX = result
    add ax, 84              ; AX = 84 + result
    mov dx, ax              ; DX = Y position

apply_row_offset:
    ; Add row offset to Y
    xor ax, ax
    mov al, bh              ; AL = row offset (0-15)
    add dx, ax              ; DX = final Y position

    ; Check if Y is in bounds
    cmp dx, 200
    jge skip_pixel_pop

    ; Calculate VGA offset: Y * 320 + X
    mov ax, dx
    mov cx, 320
    mul cx                  ; DX:AX = Y * 320
    pop si
    pop cx
    push cx
    push si
    add ax, cx              ; AX = Y * 320 + X

    cmp ax, 64000
    jae skip_pixel_pop

    ; Draw pixel
    push di
    mov di, ax
    mov al, 15              ; White pixel
    mov [di], al            ; Writes to ES:DI (back buffer at 0x7000)
    pop di

skip_pixel_pop:
    pop si
    pop cx
    pop bx
    pop ax

skip_pixel:
    shl ah, 1               ; Shift to next bit
    inc cx                  ; Next X
    dec bl
    jnz char_pixel_loop

    pop cx                  ; Restore starting X
    inc bh                  ; Next row
    cmp bh, 16
    jge char_done
    jmp char_row_loop

char_done:
    pop cx
    pop bx
    pop ds              ; Restore DS (was pointing to BIOS ROM)
    pop si

skip_char:
    inc bx
    jmp draw_char_loop

frame_done:
    ; Update scroll position
    sub bp, 2

    ; Check for wrap (supports up to 256 character messages)
    cmp bp, SCROLL_RESET
    jge no_reset
    mov bp, 320
no_reset:

    ; Update wave animation
    add di, 3
    and di, 0xFF

    ; Wait for VBlank
    mov dx, 0x3DA
    in al, dx

    ; DOUBLE BUFFER FLIP: Copy back buffer to VGA memory
    push ds
    push es

    ; Source: back buffer at 0x7000
    mov ax, 0x7000
    mov ds, ax
    xor si, si

    ; Dest: VGA memory
    mov ax, 0xA000
    mov es, ax
    push di
    xor di, di

    ; Copy 64000 bytes (320x200)
    mov cx, 32000           ; 64000 / 2 = 32000 words
    rep movsw

    pop di
    pop es
    pop ds

    ; Restore ES to back buffer for next frame (DS restored at start of main_loop)
    mov ax, 0x7000
    mov es, ax

    ; Check for key press
    mov ah, 0x01
    int 0x16
    pop ds                  ; Restore data segment for next iteration
    jz main_loop

    ; Exit
    xor ax, ax
    int 0x16

    mov ax, 0x03
    int 0x10
    hlt
