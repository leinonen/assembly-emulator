; VGA Palette Manipulation Demo
; This program sets up a custom grayscale palette and displays gradient bars

; Set video mode 13h (320x200, 256 colors)
MOV AX, 0x0013
INT 0x10

; --- Program a custom grayscale palette (0..255) ---
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

; Set DS to VGA segment
MOV AX, 0xA000
MOV DS, AX

; Draw horizontal gradient bars
MOV DI, 0        ; DI = row counter

draw_row_loop:
    ; Calculate row offset: row * 320
    MOV AX, DI
    MOV BX, 320
    MUL BX       ; DX:AX = row * 320
    MOV SI, AX   ; SI = start offset for this row

    ; Use row number as color
    MOV BL, DL   ; BL = color (low byte of row)

    ; Draw 320 pixels
    MOV CX, 320
    pixel_loop:
        MOV AL, BL
        MOV BX, SI
        MOV [BX], AL
        INC SI
        LOOP pixel_loop

    INC DI
    CMP DI, 200
    JL draw_row_loop

HLT
