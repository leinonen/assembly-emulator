; Rainbow Lines - Draw 200 horizontal lines with different colors
; Demonstrates segment-based VGA programming

.code
    ; Set VGA Mode 13h
    MOV AX, 13h
    INT 10h

    ; Set ES to VGA segment (0xA000)
    MOV AX, 0xA000
    MOV ES, AX

    ; Initialize
    XOR DI, DI          ; Start at offset 0 (top-left of screen)
    MOV BL, 0           ; BL = color counter (0-255)
    MOV DX, 200         ; DX = number of rows

draw_row:
    ; Draw one row (320 pixels) with current color
    MOV CX, 320         ; 320 pixels per row
    MOV AL, BL          ; Use current color

draw_pixel:
    MOV [DI], AL        ; Write pixel to ES:DI
    INC DI              ; Move to next pixel
    LOOP draw_pixel

    ; Move to next color
    INC BL              ; Increment color (will wrap at 256)

    ; Next row
    DEC DX
    CMP DX, 0
    JNE draw_row

    ; Busy-wait for keypress
wait_loop:
    MOV AH, 0x01        ; Check for keystroke
    INT 0x16
    JZ wait_loop        ; No key, keep waiting

    ; Key pressed - read and exit
    MOV AH, 0x00
    INT 0x16

    HLT
