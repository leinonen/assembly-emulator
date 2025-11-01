; Grayscale Palette Demo
; Creates a 256-color grayscale palette and displays gradient bars

; Set video mode 13h
MOV AX, 0x0013
INT 0x10

; Create grayscale palette (0-255)
; Each entry goes from black (0,0,0) to white (63,63,63)
MOV DX, 0x03C8   ; DAC Write Index port
XOR AL, AL       ; Start at palette index 0
OUT DX, AL

INC DX           ; DX = 0x03C9 (DAC Data port)
XOR BX, BX       ; BX = palette counter (0-255)

create_palette:
    ; Calculate grayscale value: BL / 4 (to scale 0-255 to 0-63)
    MOV AL, BL
    SHR AL, 1
    SHR AL, 1    ; Divide by 4
    OUT DX, AL   ; Red
    OUT DX, AL   ; Green
    OUT DX, AL   ; Blue
    INC BL
    JNZ create_palette

; Fill screen with horizontal gradient bars
; Each row will use a different shade of gray
MOV CX, 200      ; 200 rows
MOV DI, 0        ; Start at pixel 0

draw_rows:
    PUSH CX

    ; Calculate color for this row (map row 0-199 to palette 0-255)
    MOV AX, 200
    SUB AX, CX   ; AX = current row number (0-199)

    ; We'll just use row number as approximate color
    ; For simplicity: use (row * 256 / 200) but we'll approximate
    ; by using the row number directly (0-199 maps reasonably to 0-255)
    MOV BL, AL

    ; Draw 320 pixels in this row with color BL
    PUSH CX
    MOV CX, 320

draw_pixels:
    MOV AL, BL
    MOV [DI], AL
    INC DI
    LOOP draw_pixels

    POP CX
    POP CX
    LOOP draw_rows

HLT
