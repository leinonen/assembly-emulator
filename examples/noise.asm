; TV Static Noise - Segment-based version
; Fills the screen with pseudo-random greyscale pixels

.code
    ; Set VGA Mode 13h
    MOV AX, 13h
    INT 10h

    ; Set ES to VGA segment
    MOV AX, 0xA000
    MOV ES, AX

    ; Set up 256-level greyscale palette
    MOV DX, 0x03C8      ; DAC Write Index
    XOR AL, AL
    OUT DX, AL

    MOV DX, 0x03C9      ; DAC Data port
    MOV BX, 0           ; Color index

palette_loop:
    MOV AX, BX
    SHR AX, 1
    SHR AX, 1
    ADD AL, 16
    CMP AL, 63
    JBE pal_ok
    MOV AL, 63
pal_ok:
    OUT DX, AL
    OUT DX, AL
    OUT DX, AL
    INC BX
    CMP BX, 256
    JNE palette_loop

    ; Initialize seed
    MOV BX, 137

    ; Main animation loop
frame_loop:
    XOR DI, DI          ; Start at offset 0
    MOV CX, 32000       ; 64000 pixels = 32000 words

pixel_loop:
    ; Generate random value
    MOV AX, BX
    XOR AX, DI
    ADD BX, 127

    ; Write to VGA (ES:DI)
    MOV [DI], AX
    INC DI
    INC DI

    LOOP pixel_loop

    ; Check for ESC
    MOV AH, 0x01
    INT 0x16
    JZ frame_loop

    MOV AH, 0x00
    INT 0x16
    CMP AL, 0x1B
    JNE frame_loop

    HLT
