; Simple pixel test for Mode 13h
; Writes a few colored pixels to verify VGA works

.code
    ; Set VGA Mode 13h (320x200, 256 colors)
    MOV AX, 13h
    INT 10h

    ; Write red pixel at (10, 10)
    ; VGA address = 0xA000 + (y * 320 + x)
    ; = 0xA000 + (10 * 320 + 10)
    ; = 0xA000 + 3210
    ; = 0xA000 + 0x0C8A
    MOV AX, 0A000h
    ADD AX, 3210
    MOV BX, 4          ; Red color (index 4)
    MOV [AX], BX

    ; Write blue pixel at (20, 10)
    MOV AX, 0A000h
    ADD AX, 3220
    MOV BX, 1          ; Blue color (index 1)
    MOV [AX], BX

    ; Write green pixel at (30, 10)
    MOV AX, 0A000h
    ADD AX, 3230
    MOV BX, 2          ; Green color (index 2)
    MOV [AX], BX

    ; Write white pixel at (40, 10)
    MOV AX, 0A000h
    ADD AX, 3240
    MOV BX, 15         ; White color (index 15)
    MOV [AX], BX

    HLT
