; TV Static Noise
; Fills the screen with pseudo-random greyscale pixels

.code
    ; Set VGA Mode 13h
    MOV AX, 13h
    INT 10h

    ; Set up 256-level greyscale palette
    MOV DX, 0x03C8      ; DAC Write Index
    XOR AL, AL          ; Start at palette index 0
    OUT DX, AL

    MOV DX, 0x03C9      ; DAC Data port
    XOR BX, BX          ; BX = color counter (0-255)

greyscale_palette:
    ; Calculate grey value with better scaling
    ; Map 0-255 to 0-63 using: (BX * 63) / 255
    ; Approximation: BX / 4 is too dark, so use (BX >> 2) + (BX >> 4)
    ; Or simpler: just use BX >> 2 but add some brightness
    MOV AX, BX
    SHR AX, 1
    SHR AX, 1           ; AX = BX / 4

    ; Add extra brightness for better contrast
    ADD AL, 16          ; Shift range up
    CMP AL, 63
    JBE grey_ok
    MOV AL, 63          ; Cap at max
grey_ok:
    OUT DX, AL          ; Red = grey
    OUT DX, AL          ; Green = grey
    OUT DX, AL          ; Blue = grey

    INC BX
    CMP BX, 256
    JNE greyscale_palette

    ; Initialize random seed once before main loop
    MOV BX, 137         ; Initial random seed

    ; Main loop - continuously generate noise until ESC is pressed
main_loop:
    ; Fill entire screen with random noise
    ; Use simple LCG: next = (current * 29 + 17) & 0xFF
    ; Map result to greyscale colors (0-255)
    ; BX carries the seed from previous frame for variety

    MOV CX, 200         ; 200 rows
    XOR SI, SI          ; Start at offset 0

row_loop:
    MOV DX, 160         ; 160 words per row (320 pixels)

pixel_loop:
    ; Simple pseudo-random using BX and SI
    ; XOR with position gives different values per pixel
    MOV AX, BX
    XOR AX, SI          ; XOR with position for variation
    ADD BX, 127         ; Add to seed for next iteration

    ; Use full 0-255 range for both bytes (no masking)
    ; AX contains two random bytes (AL and AH)

    ; Calculate VGA address and write
    MOV DI, AX          ; Save pixel data (both bytes)
    MOV AX, 0xA000
    ADD AX, SI
    MOV [AX], DI        ; Write word to VGA (2 pixels)

    ; Next word (2 pixels)
    INC SI
    INC SI

    ; Continue pixel loop
    DEC DX
    CMP DX, 0
    JNE pixel_loop

    ; Continue row loop
    DEC CX
    CMP CX, 0
    JNE row_loop

    ; Screen filled - now check for ESC key
    MOV AH, 0x01        ; INT 16h function 01h - check for keystroke
    INT 0x16
    JZ main_loop        ; ZF=1 means no key, continue looping

    ; Key is available - read it
    MOV AH, 0x00        ; INT 16h function 00h - read keystroke
    INT 0x16
    CMP AL, 0x1B        ; Is it ESC? (ASCII 27 = 0x1B)
    JNE main_loop       ; Not ESC, continue

    ; ESC pressed - exit
    HLT
