; Simple pixel test for Mode 13h
; Writes a few colored pixels to verify VGA works

.code
    ; Set VGA Mode 13h (320x200, 256 colors)
    MOV AX, 13h
    INT 10h

    ; Set DS to VGA segment
    MOV AX, 0A000h
    MOV DS, AX

    ; Write red pixel at (10, 10)
    ; VGA offset = y * 320 + x = 10 * 320 + 10 = 3210
    MOV BX, 3210
    MOV AL, 4          ; Red color (index 4)
    MOV [BX], AL

    ; Write blue pixel at (20, 10)
    MOV BX, 3220
    MOV AL, 1          ; Blue color (index 1)
    MOV [BX], AL

    ; Write green pixel at (30, 10)
    MOV BX, 3230
    MOV AL, 2          ; Green color (index 2)
    MOV [BX], AL

    ; Write white pixel at (40, 10)
    MOV BX, 3240
    MOV AL, 15         ; White color (index 15)
    MOV [BX], AL

    HLT
