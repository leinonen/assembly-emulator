; Simple Color Test - Set bright colors and display them

; Set video mode 13h
MOV AX, 0x0013
INT 0x10

; Set palette color 1 to BRIGHT RED (63, 0, 0)
MOV DX, 0x03C8
MOV AL, 1
OUT DX, AL
MOV DX, 0x03C9
MOV AL, 63
OUT DX, AL
MOV AL, 0
OUT DX, AL
MOV AL, 0
OUT DX, AL

; Set palette color 2 to BRIGHT GREEN (0, 63, 0)
MOV DX, 0x03C8
MOV AL, 2
OUT DX, AL
MOV DX, 0x03C9
MOV AL, 0
OUT DX, AL
MOV AL, 63
OUT DX, AL
MOV AL, 0
OUT DX, AL

; Set palette color 3 to BRIGHT BLUE (0, 0, 63)
MOV DX, 0x03C8
MOV AL, 3
OUT DX, AL
MOV DX, 0x03C9
MOV AL, 0
OUT DX, AL
MOV AL, 0
OUT DX, AL
MOV AL, 63
OUT DX, AL

; Set palette color 4 to WHITE (63, 63, 63)
MOV DX, 0x03C8
MOV AL, 4
OUT DX, AL
MOV DX, 0x03C9
MOV AL, 63
OUT DX, AL
MOV AL, 63
OUT DX, AL
MOV AL, 63
OUT DX, AL

; Fill screen with color stripes
; Use DI as pixel pointer
MOV DI, 0

; Fill 50 rows with color 1 (red)
MOV BL, 1
MOV CX, 16000
fill1:
    MOV AL, BL
    MOV [DI], AL
    INC DI
    LOOP fill1

; Fill 50 rows with color 2 (green)
MOV BL, 2
MOV CX, 16000
fill2:
    MOV AL, BL
    MOV [DI], AL
    INC DI
    LOOP fill2

; Fill 50 rows with color 3 (blue)
MOV BL, 3
MOV CX, 16000
fill3:
    MOV AL, BL
    MOV [DI], AL
    INC DI
    LOOP fill3

; Fill 50 rows with color 4 (white)
MOV BL, 4
MOV CX, 16000
fill4:
    MOV AL, BL
    MOV [DI], AL
    INC DI
    LOOP fill4

HLT
