; TV Static Noise
; Fills the screen with pseudo-random greyscale pixels

.code
    ; Set VGA Mode 13h
    MOV AX, 13h
    INT 10h

    ; Fill entire screen with random noise
    ; Use simple LCG: next = (current * 29 + 17) & 0xFF
    ; Map result to greyscale colors (7-15 for varying grays)

    MOV BX, 137         ; Random seed
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

    ; Mask to 0-15 range for both bytes
    AND AX, 3855        ; 0x0F0F mask

    ; Calculate VGA address and write
    MOV DI, AX          ; Save pixel data
    MOV AX, 0xA000
    ADD AX, SI
    MOV [AX], DI        ; Write to VGA

    ; Next word
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

    HLT
