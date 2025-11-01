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

    ; Set DS and ES to back buffer segment for rendering (0x7000)
    ; This is our off-screen buffer in regular RAM (64000 bytes)
    MOV AX, 0x7000
    MOV DS, AX
    MOV ES, AX

    ; Initialize PRNG seeds (BP for seed1, SI for seed2)
    MOV BP, 0x7FFF      ; seed1
    MOV SI, 0xACE1      ; seed2

    ; Main animation loop
frame_loop:
    XOR DI, DI          ; Start at offset 0
    MOV CX, 64000       ; 64000 pixels

pixel_loop:
    ; Linear Congruential Generator (LCG)
    ; seed1 = (seed1 * 1103515245 + 12345) & 0xFFFF
    MOV AX, BP
    MOV DX, 0x4E35      ; Low word of 1103515245
    MUL DX
    ADD AX, 0x3039      ; 12345
    MOV BP, AX          ; Update seed1

    ; Xorshift for seed2
    ; seed2 ^= seed2 << 7
    MOV AX, SI
    MOV BX, AX
    SHL AX, 7
    XOR SI, AX

    ; seed2 ^= seed2 >> 9
    MOV AX, SI
    SHR AX, 9
    XOR SI, AX

    ; seed2 ^= seed2 << 8
    MOV AX, SI
    SHL AX, 8
    XOR SI, AX

    ; Combine both PRNGs
    MOV AX, BP
    XOR AX, SI

    ; Extract low byte for 0-255 range
    AND AL, 0xFF

    ; Write single pixel to back buffer (DS:DI)
    MOV [DI], AL
    INC DI

    LOOP pixel_loop

    ; Advance seeds for next frame to ensure different pattern
    ADD BP, 0x1234
    ADD SI, 0x5678

    ; Wait for VBlank - this will block until next frame
    ; This prevents screen tearing by synchronizing with the display
    MOV DX, 0x3DA
    IN AL, DX        ; Reading 0x3DA waits for VBlank via channel

    ; DOUBLE BUFFER FLIP: Copy complete back buffer to VGA memory
    ; Source: back buffer at 0x7000 (DS is already set to this)
    XOR SI, SI

    ; Dest: VGA memory
    MOV AX, 0xA000
    MOV ES, AX
    XOR DI, DI

    ; Copy 64000 bytes (320x200)
    MOV CX, 32000      ; 64000 / 2 = 32000 words
    REP MOVSW

    ; Restore ES to back buffer for next frame
    MOV AX, 0x7000
    MOV ES, AX

    ; Check for ESC
    MOV AH, 0x01
    INT 0x16
    JZ frame_loop

    MOV AH, 0x00
    INT 0x16
    CMP AL, 0x1B
    JNE frame_loop

    HLT
