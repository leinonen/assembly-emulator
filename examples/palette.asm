; VGA Palette Manipulation Demo
; This program sets up a custom grayscale palette and displays vertical bars

; Set video mode 13h (320x200, 256 colors)
MOV AX, 0x0013
INT 0x10

; --- Program a custom grayscale palette (0..255) ---
; VGA uses 6-bit color (0–63), so we scale 8-bit (0–255) to 6-bit by dividing by 4

MOV DX, 0x03C8   ; Port 0x3C8 = DAC Write Index
MOV AL, 0        ; Start at palette index 0
OUT DX, AL

INC DX           ; DX = 0x03C9 (DAC Data port)
MOV BL, 0        ; BL = counter for 256 entries

set_gray_loop:
    ; Calculate grayscale value (0-63)
    MOV AL, BL
    SHR AL, 2    ; Divide by 4 to get 6-bit value (0-63)

    OUT DX, AL   ; Red component
    OUT DX, AL   ; Green component
    OUT DX, AL   ; Blue component

    INC BL
    JNZ set_gray_loop  ; Loop while BL != 0 (will wrap from 255 to 0)

; --- Fill screen with gradient pattern ---
MOV AX, 0xA000
MOV BX, AX       ; Save segment in BX temporarily
MOV DI, 0        ; DI = pixel offset

; We'll draw horizontal bars, each with a different color from the palette
MOV CX, 200      ; 200 rows

fill_row:
    PUSH CX

    ; Calculate color for this row (0-199 maps to 0-255)
    MOV AX, 200
    SUB AX, CX   ; AX = row number (0-199)

    ; Scale row number (0-199) to palette index (0-255)
    ; We'll use a simple mapping: multiply by 256/200 ≈ 1.28
    ; For simplicity, we'll just use the row number modulo extended
    MOV BX, AX
    SHL BX, 8    ; BX = row * 256
    MOV DX, 200
    DIV DX       ; AX = (row * 256) / 200

    ; AL now contains our palette color for this row
    MOV BL, AL

    ; Fill 320 pixels with this color
    MOV CX, 320

fill_pixel:
    ; Write pixel to VGA memory at 0xA000:DI
    PUSH AX
    PUSH BX
    MOV AX, 0xA000
    MOV BX, AX
    POP BX
    MOV AL, BL
    MOV [DI], AL
    INC DI
    POP AX

    LOOP fill_pixel

    POP CX
    LOOP fill_row

; Halt the program
HLT
