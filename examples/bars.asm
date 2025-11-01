; Color Bars
; Displays vertical color bars across the screen

.code
    ; Set VGA Mode 13h
    MOV AX, 13h
    INT 10h

    ; Set DS to VGA segment
    MOV AX, 0xA000
    MOV DS, AX

    ; Draw 16 vertical bars (20 pixels wide each = 320 total)
    XOR DI, DI         ; Row counter (0-199)

row_loop:
    ; Calculate row start offset: row * 320
    MOV AX, DI
    MOV BX, 320
    MUL BX             ; AX = row * 320
    MOV SI, AX         ; SI = row start offset

    ; For each row, draw 16 bars of 20 pixels each
    MOV DL, 0          ; Color counter (0-15) - use DL instead of BL

bar_loop:
    ; Draw 20 pixels of current color (in DL)
    MOV CX, 20
    pixel_loop:
        MOV BX, SI      ; BX = offset
        MOV AL, DL      ; AL = color
        MOV [BX], AL    ; Write pixel
        INC SI          ; Next pixel
        LOOP pixel_loop

    INC DL             ; Next color
    CMP DL, 16
    JB bar_loop

    INC DI
    CMP DI, 200
    JB row_loop

    HLT
