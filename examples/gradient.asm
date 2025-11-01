; Smooth Gradient Demo
; Creates a full 256-color palette and displays a smooth gradient

; Set video mode 13h
MOV AX, 0x0013
INT 0x10

; --- Create 256-color gradient palette from black to white ---
MOV DX, 0x03C8   ; DAC Write Index
XOR AL, AL       ; Start at palette index 0
OUT DX, AL

MOV DX, 0x03C9   ; DAC Data port
XOR BX, BX       ; BX = color counter (0-255)

palette_loop:
    ; Calculate RGB value: BX >> 2 (divide by 4 to get 0-63 range)
    MOV AX, BX
    SHR AX, 1
    SHR AX, 1    ; AX = BX / 4

    OUT DX, AL   ; Red
    OUT DX, AL   ; Green
    OUT DX, AL   ; Blue

    INC BX
    CMP BX, 256
    JNE palette_loop

; --- Fill screen with smooth horizontal gradient ---
; Each row uses incrementing palette colors
MOV DI, 0
MOV BX, 0        ; Color index

row_loop:
    ; Fill one row (320 pixels) with current color
    MOV AL, BL
    MOV CX, 320

fill_row:
    MOV [DI], AL
    INC DI
    LOOP fill_row

    ; Next row uses next color
    INC BL
    CMP BL, 200  ; Only 200 rows
    JNE row_loop

HLT
