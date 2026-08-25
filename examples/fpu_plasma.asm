; Ported to NASM syntax: assemble with `asm-emu asm` or run directly.
org 100h
bits 16

; FPU Plasma - sine table generated at runtime via x87 FPU
; Demonstrates: FSIN, FILD, FISTP, FLDPI, FADDP, FMULP, FDIVRP, FLD ST(i)

section .data
    val_127    DW 127
    val_256    DW 256
    temp_w     DW 0        ; FPU fistp scratch during init; pixel accumulator during render
    frame      DW 0        ; frame counter (incremented by 3 each frame)

    ; 256-entry byte sine table (populated by FPU at startup)
    sine_table:
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        DB 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0

section .text
start:
    mov ax, 0x13
    int 0x10

    ; ==================================================
    ; Build sine_table[i] = round(sin(i*2pi/256)*127+127)
    ; ==================================================
    finit

    fild word [val_256]     ; ST0=256
    fldpi                   ; ST0=pi,   ST1=256
    fld1                    ; ST0=1,    ST1=pi,   ST2=256
    fld1                    ; ST0=1,    ST1=1,    ST2=pi,  ST3=256
    faddp                   ; ST0=2,    ST1=pi,   ST2=256
    fmulp                   ; ST0=2pi,  ST1=256
    fdivrp                  ; ST0=step=2pi/256

    fldz                    ; ST0=0(angle), ST1=step

    mov di, sine_table
    mov si, temp_w
    mov cx, 256

gen_loop:
    fld st0                 ; ST0=angle, ST1=angle, ST2=step
    fsin                    ; ST0=sin,   ST1=angle, ST2=step
    fild word [val_127]     ; ST0=127,   ST1=sin,   ST2=angle, ST3=step
    fmulp                   ; ST0=sin*127, ST1=angle, ST2=step
    fild word [val_127]     ; ST0=127,   ST1=sin*127, ST2=angle, ST3=step
    faddp                   ; ST0=result, ST1=angle, ST2=step
    fistp word [si]         ; store to temp_w, pop
    mov al, [si]
    mov [es:di], al
    inc di

    fadd st1                ; angle += step
    loop gen_loop

    finit                   ; clear FPU stack

    ; ==================================================
    ; Rainbow palette (R/G/B from sine table with offsets)
    ; ==================================================
    mov dx, 0x3C8
    xor al, al
    out dx, al
    mov dx, 0x3C9
    xor cx, cx
pal_loop:
    mov si, sine_table
    mov ax, cx
    and ax, 0xFF
    add si, ax
    mov al, [si]
    shr al, 2
    out dx, al

    mov si, sine_table
    mov ax, cx
    add ax, 85
    and ax, 0xFF
    add si, ax
    mov al, [si]
    shr al, 2
    out dx, al

    mov si, sine_table
    mov ax, cx
    add ax, 170
    and ax, 0xFF
    add si, ax
    mov al, [si]
    shr al, 2
    out dx, al

    inc cx
    cmp cx, 256
    jl pal_loop

main_loop:
    push ds
    mov ax, 0x7000
    mov es, ax
    xor di, di
    xor bx, bx              ; BX = y (0..199)

y_loop:
    xor cx, cx              ; CX = x (0..319)

x_loop:
    ; wave1 = sine[(x + x/2 + frame*2) & 0xFF]
    mov ax, cx
    mov dx, ax
    shr dx, 1
    add ax, dx              ; ax = x + x/2
    mov si, frame
    mov dx, [si]            ; dx = frame
    add dx, dx              ; dx = frame*2
    add ax, dx
    and ax, 0xFF
    mov si, sine_table
    add si, ax
    mov al, [si]
    ; save wave1 to temp_w (byte write keeps high byte=0)
    mov si, temp_w
    mov [si], al

    ; wave2 = sine[(y + y/2 + frame*3) & 0xFF]
    mov ax, bx              ; ax = y
    mov dx, ax
    shr dx, 1
    add ax, dx              ; ax = y + y/2
    mov si, frame
    mov dx, [si]            ; dx = frame
    add dx, dx
    add dx, [si]            ; dx = frame*3
    add ax, dx
    and ax, 0xFF
    mov si, sine_table
    add si, ax
    mov al, [si]
    ; accumulate: al = wave2 + wave1
    mov si, temp_w
    add al, [si]
    mov [si], al

    ; wave3 = sine[(x + y + frame) & 0xFF]
    mov ax, cx
    add ax, bx
    mov si, frame
    add ax, [si]            ; ax = x + y + frame
    and ax, 0xFF
    mov si, sine_table
    add si, ax
    mov al, [si]
    ; final: al = wave3 + wave1 + wave2
    mov si, temp_w
    add al, [si]
    stosb

    inc cx
    cmp cx, 320
    jl x_loop

    inc bx
    cmp bx, 200
    jl y_loop

    ; Advance frame counter while DS still points to data segment
    mov si, frame
    mov ax, [si]
    add ax, 3
    mov [si], ax

    ; VBlank sync, then blit backbuffer → VGA
    mov dx, 0x3DA
    ; wait for the current vertical retrace to end, then for the next one to start
.vs184a:
    in al, dx
    test al, 8
    jnz .vs184a
.vs184b:
    in al, dx
    test al, 8
    jz .vs184b

    mov ax, 0x7000
    mov ds, ax
    xor si, si
    mov ax, 0xA000
    mov es, ax
    xor di, di
    mov cx, 32000
    rep movsw

    ; Key check
    xor ax, ax
    mov ds, ax
    mov ah, 0x01
    int 0x16
    pop ds
    jz main_loop

    mov ah, 0x00
    int 0x16
    cmp al, 27
    jne main_loop

    mov ax, 4C00h
    int 21h
