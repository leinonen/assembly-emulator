; Rainbow Palette Demo
; Creates a colorful palette and displays it

; Set video mode 13h
MOV AX, 0x0013
INT 0x10

; Set ES to VGA segment
MOV AX, 0xA000
MOV ES, AX

; Create a simple rainbow palette
; Colors 0-63: Red gradient
; Colors 64-127: Green gradient
; Colors 128-191: Blue gradient
; Colors 192-255: White gradient

; Start with red gradient (0-63)
MOV DX, 0x03C8
MOV AL, 0
OUT DX, AL
MOV DX, 0x03C9
MOV CX, 64
MOV BL, 0

red_loop:
    MOV AL, BL
    OUT DX, AL      ; Red increases
    MOV AL, 0
    OUT DX, AL      ; Green = 0
    OUT DX, AL      ; Blue = 0
    INC BL
    LOOP red_loop

; Green gradient (64-127)
MOV DX, 0x03C8
MOV AL, 64
OUT DX, AL
MOV DX, 0x03C9
MOV CX, 64
MOV BL, 0

green_loop:
    MOV AL, 0
    OUT DX, AL      ; Red = 0
    MOV AL, BL
    OUT DX, AL      ; Green increases
    MOV AL, 0
    OUT DX, AL      ; Blue = 0
    INC BL
    LOOP green_loop

; Blue gradient (128-191)
MOV DX, 0x03C8
MOV AL, 128
OUT DX, AL
MOV DX, 0x03C9
MOV CX, 64
MOV BL, 0

blue_loop:
    MOV AL, 0
    OUT DX, AL      ; Red = 0
    OUT DX, AL      ; Green = 0
    MOV AL, BL
    OUT DX, AL      ; Blue increases
    INC BL
    LOOP blue_loop

; White gradient (192-255)
MOV DX, 0x03C8
MOV AL, 192
OUT DX, AL
MOV DX, 0x03C9
MOV CX, 64
MOV BL, 0

white_loop:
    MOV AL, BL
    OUT DX, AL      ; Red increases
    OUT DX, AL      ; Green increases
    OUT DX, AL      ; Blue increases
    INC BL
    LOOP white_loop

; Now fill the screen with these colors
; We'll draw horizontal stripes
MOV DI, 0
MOV CX, 320
MOV BL, 0

fill_screen:
    ; Fill one row (320 pixels) with incrementing colors
    PUSH CX
    MOV CX, 320

fill_row:
    MOV AL, BL
    MOV [DI], AL
    INC DI
    INC BL
    LOOP fill_row

    POP CX
    LOOP fill_screen

HLT
