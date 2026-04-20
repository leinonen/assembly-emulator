; Fill entire screen with color
.code
    ; Set VGA Mode 13h
    MOV AX, 13h
    INT 10h

    ; Set ES to VGA segment (0xA000)
    MOV AX, 0xA000
    MOV ES, AX

    ; Fill entire screen (320x200 = 64000 pixels)
    XOR DI, DI          ; Start at offset 0
    MOV CX, 32000       ; 32000 words = 64000 bytes
    MOV AX, 0x0F0F      ; Color 15 (white) in both bytes
    REP STOSW            ; Fill screen

    ; Busy-wait for keypress
wait_loop:
    MOV AH, 0x01
    INT 0x16
    JZ wait_loop

    MOV AH, 0x00
    INT 0x16

    HLT
