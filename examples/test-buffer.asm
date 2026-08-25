; Ported to NASM syntax: assemble with `asm-emu asm` or run directly.
org 100h
bits 16

; Test double buffer - fill back buffer with color and copy to VGA
section .text
    ; Set VGA Mode 13h
    mov ax, 0x13
    int 0x10

    ; Fill back buffer at 0x7000 with color 15 (white)
    mov ax, 0x7000
    mov es, ax
    xor di, di
    mov cx, 32000  ; 64000 bytes / 2 = 32000 words
    mov ax, 0x0F0F ; White color in both bytes
    rep stosw

    ; Now copy back buffer to VGA
    mov ax, 0x7000
    mov ds, ax
    xor si, si

    mov ax, 0xA000
    mov es, ax
    xor di, di

    mov cx, 32000
    rep movsw

    ; Wait for key
    mov ah, 0x00
    int 0x16

    mov ax, 4C00h
    int 21h
