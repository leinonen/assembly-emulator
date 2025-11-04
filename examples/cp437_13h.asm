; CP437 Text Rendering Demo in VGA Mode 13h
; This program demonstrates text rendering with CP437 characters
; including box-drawing characters

.code
    JMP start

; Demo text using CP437 box-drawing chars (now with readable string literals!)
msg:
    db "Mode 13h CP437 demo:", 13, 10
    db "╔══════════════╗", 13, 10
    db "║ Hello world! ║", 13, 10
    db "╚══════════════╝", 0

start:
    ; Switch to 320x200x256 (mode 13h)
    mov ax, 0x13
    int 0x10

    ; Teletype function setup
    xor bh, bh        ; display page 0
    mov bl, 15        ; color = white

    mov si, msg
print:
    lodsb             ; AL = [SI], SI++
    test al, al
    jz waitkey        ; zero = end of string
    mov ah, 0x0E      ; BIOS teletype function
    int 0x10
    jmp print

waitkey:
    xor ax, ax
    int 0x16          ; wait for key press

    ; Back to text mode
    mov ax, 0x03
    int 0x10
    hlt
