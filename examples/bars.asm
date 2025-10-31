; Color Bars
; Displays vertical color bars across the screen

.code
    ; Set VGA Mode 13h
    MOV AX, 13h
    INT 10h

    ; Draw 16 vertical bars (20 pixels wide each = 320 total)
    MOV DI, 0          ; Column counter
    MOV SI, 0          ; Start offset

column_loop:
    ; Fill 20 pixels with current color
    MOV CX, 20         ; 20 pixels wide

pixel_column:
    ; Calculate address for each row (200 rows)
    MOV BX, 0          ; Row counter

row_loop:
    ; Calculate offset = row*320 + column
    MOV AX, BX
    MOV DX, 320
    MUL DX             ; AX = row * 320
    ADD AX, SI         ; AX = offset (0-63999)

    ; Map offset to VGA memory address
    CMP AX, 0x6000     ; 24576 in hex
    JB use_high_region  ; Use JB (unsigned) instead of JL (signed)

    ; Wrapped region: 0x0400 + (offset - 24576)
    SUB AX, 0x6000     ; 24576 in hex
    ADD AX, 0x0400
    JMP write_pixel

use_high_region:
    ; High region: 0xA000 + offset
    ADD AX, 0xA000

write_pixel:
    ; Write color (DI contains color index 0-15)
    MOV DX, DI
    MOV [AX], DL       ; Write byte

    INC BX
    CMP BX, 200
    JB row_loop         ; Use JB for unsigned comparison

    INC SI             ; Next column
    DEC CX
    JNZ pixel_column   ; Use JNZ instead of CMP+JNE

    ; Next color bar
    INC DI
    CMP DI, 16
    JB column_loop       ; Use JB for unsigned comparison

    HLT
