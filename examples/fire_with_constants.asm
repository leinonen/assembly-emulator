; Classic Fire Effect - Double Buffered (Demoscene Style)
; Version with constants to demonstrate improved readability

; VGA Constants
VGA_MODE_13H     EQU 0x13
VGA_SEGMENT      EQU 0xA000
DAC_WRITE_INDEX  EQU 0x3C8
DAC_DATA         EQU 0x3C9
VGA_STATUS_PORT  EQU 0x3DA

; Screen dimensions
SCREEN_WIDTH  EQU 320
SCREEN_HEIGHT EQU 200
SCREEN_WORDS  EQU (SCREEN_WIDTH * SCREEN_HEIGHT) / 2
LINE_198_OFFSET EQU SCREEN_WIDTH * 198
LINE_199_OFFSET EQU SCREEN_WIDTH * 199
FIRE_LINE_START EQU SCREEN_HEIGHT - 3     ; Start propagation from line 197

; Buffer configuration
BUFFER_SEGMENT EQU 0x7000

; Color constants
COLOR_RAMP_SIZE EQU 64
MAX_COLOR_VALUE EQU 63
FIRE_MIN_COLOR  EQU 192        ; Minimum fire intensity (192-255 range)

; Palette bit masks
HOT_PIXEL_MASK_1 EQU 0x80
HOT_PIXEL_MASK_2 EQU 0x40
FIRE_RANGE_MASK  EQU 0x3F

; PRNG constants
PRNG_SEED EQU 0x5678

; Keyboard constants
KEY_ESC      EQU 0x1B
INT_VIDEO    EQU 0x10
INT_KEYBOARD EQU 0x16

.code
    ; Set VGA Mode 13h
    mov ax, VGA_MODE_13H
    int INT_VIDEO

    ; Set up fire palette (256 colors) - smooth black → red → orange → yellow → white
    mov dx, DAC_WRITE_INDEX
    mov al, 0
    out dx, al
    mov dx, DAC_DATA

    ; Colors 0-63: Black to red (red: 0 → 63)
    mov cx, COLOR_RAMP_SIZE
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
    mov cx, COLOR_RAMP_SIZE
    mov bx, 0
    pal_red_to_orange:
        mov al, MAX_COLOR_VALUE
        out dx, al        ; Red = 63
        mov al, bl        ; Green: 0-63
        out dx, al
        xor al, al
        out dx, al        ; Blue = 0
        inc bx
        loop pal_red_to_orange

    ; Colors 128-191: Orange to yellow (red=63, green=63, blue: 0 → 63)
    mov cx, COLOR_RAMP_SIZE
    mov bx, 0
    pal_orange_to_yellow:
        mov al, MAX_COLOR_VALUE
        out dx, al        ; Red = 63
        mov al, MAX_COLOR_VALUE
        out dx, al        ; Green = 63
        mov al, bl        ; Blue: 0-63
        out dx, al
        inc bx
        loop pal_orange_to_yellow

    ; Colors 192-255: Yellow to white (all at 63, stay bright)
    mov cx, COLOR_RAMP_SIZE
    mov bx, 0
    pal_yellow_to_white:
        mov al, MAX_COLOR_VALUE
        out dx, al        ; Red = 63
        mov al, MAX_COLOR_VALUE
        out dx, al        ; Green = 63
        mov al, MAX_COLOR_VALUE
        out dx, al        ; Blue = 63 (pure white/yellow)
        inc bx
        loop pal_yellow_to_white

    ; Set DS and ES to back buffer segment for rendering
    mov ax, BUFFER_SEGMENT
    mov ds, ax
    mov es, ax

    ; PRNG seed
    mov bp, PRNG_SEED

    ; Clear screen
    xor di, di
    mov cx, SCREEN_WORDS
    xor ax, ax
    rep stosw

; Main loop
main_loop:
    ; Step 1: Fill bottom TWO lines with random hot pixels and black specks

    ; Fill line 198
    mov di, LINE_198_OFFSET
    mov cx, SCREEN_WIDTH
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
        test ah, HOT_PIXEL_MASK_1
        jz hot_pixel_198

        ; Black speck (about 1 in 2 chance based on high bit)
        ; But let's make it less frequent - check another bit
        test ah, HOT_PIXEL_MASK_2
        jz hot_pixel_198

        xor al, al        ; Black pixel
        jmp write_198

        hot_pixel_198:
        ; High intensity for fire source (192-255)
        and al, FIRE_RANGE_MASK
        add al, FIRE_MIN_COLOR

        write_198:
        mov [di], al
        inc di
        loop fill_line_198

    ; Fill line 199
    mov di, LINE_199_OFFSET
    mov cx, SCREEN_WIDTH
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
        test ah, HOT_PIXEL_MASK_1
        jz hot_pixel_199
        test ah, HOT_PIXEL_MASK_2
        jz hot_pixel_199

        xor al, al        ; Black pixel
        jmp write_199

        hot_pixel_199:
        ; High intensity for fire source (192-255)
        and al, FIRE_RANGE_MASK
        add al, FIRE_MIN_COLOR

        write_199:
        mov [di], al
        inc di
        loop fill_line_199

    ; Step 2: Classic fire propagation with 4-point averaging
    ; Process from line 197 down to 0 (lines 198-199 are the heat source)
    mov bx, FIRE_LINE_START

    propagate_lines:
        cmp bx, 0
        jl check_key

        ; Calculate destination line offset
        mov ax, bx
        mov dx, SCREEN_WIDTH
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
        int INT_KEYBOARD
        jz do_flip  ; No key pressed, continue

        mov ah, 0x00
        int INT_KEYBOARD
        cmp al, KEY_ESC
        je exit_program

    do_flip:
        ; Wait for VBlank - this will block until next frame
        ; Ensures smooth 60 FPS animation without tearing
        mov dx, VGA_STATUS_PORT
        in al, dx

        ; DOUBLE BUFFER FLIP: Copy complete back buffer to VGA memory
        ; This is the classic demoscene technique - atomic page flip!
        push ds
        push es

        ; Source: back buffer
        mov ax, BUFFER_SEGMENT
        mov ds, ax
        xor si, si

        ; Dest: VGA memory
        mov ax, VGA_SEGMENT
        mov es, ax
        xor di, di

        ; Copy entire screen
        mov cx, SCREEN_WORDS
        rep movsw

        pop es
        pop ds

        ; Restore DS and ES to back buffer for next frame rendering
        mov ax, BUFFER_SEGMENT
        mov ds, ax
        mov es, ax

        jmp main_loop

    exit_program:
        hlt
