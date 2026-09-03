# Part 4 — Sound

A PC of this era has two sound sources you can rely on: the built-in speaker
(a square wave from a timer channel) and, if the user bought a card, an
AdLib — a Yamaha OPL2 FM synthesiser on ports `388h`/`389h`. The emulator
implements both, mixed at 48 kHz. Headless runs can capture the result:

```
./asm-emu run -wav out.wav -headless -max-insns 200000000 lesson17.asm
```

## 16. The PC speaker

Timer channel 2 of the 8254 is wired to the speaker. Program it with a
divisor of the 1.193182 MHz PIT clock and open the gate:

- port `43h` ← `0B6h`: "channel 2, write low byte then high byte, mode 3
  (square wave)"
- port `42h` ← divisor low, then divisor high, where
  divisor = 1193182 / frequency
- port `61h` bits 0 and 1: bit 0 gates the timer, bit 1 connects it to the
  speaker. Set both for sound, clear both for silence.

```nasm
; lesson16.asm - a tune on the PC speaker
org 100h
bits 16

start:
    mov ah, 09h
    mov dx, banner
    int 21h
    sti                     ; the BIOS wait service needs interrupts

    mov si, tune
.next:
    lodsw                   ; AX = the divisor for this note, 0 ends the tune
    test ax, ax
    jz .done
    call tone_on
    call delay
    jmp .next
.done:
    call tone_off
    ret

; tone_on: AX = the PIT divisor
tone_on:
    push ax
    mov al, 0B6h            ; channel 2, lo/hi byte, mode 3 = square wave
    out 43h, al
    pop ax
    out 42h, al             ; divisor, low byte
    mov al, ah
    out 42h, al             ; divisor, high byte
    in al, 61h
    or al, 3                ; bit 0 = gate on, bit 1 = speaker on
    out 61h, al
    ret

tone_off:
    in al, 61h
    and al, 0FCh
    out 61h, al
    ret

; delay: about 150 ms, via the BIOS "wait CX:DX microseconds" service
delay:
    mov ah, 86h
    mov cx, 0002h
    mov dx, 49F0h           ; 0002_49F0h = 150000 us
    int 15h
    ret

; 1193182 / frequency, for C D E G A C
tune:   dw 4561, 4063, 3619, 3044, 2712, 2280, 0
banner: db 'Beeping the PC speaker...', 13, 10, '$'
```

Because the speaker is a bare square wave, the classic trick for "better"
sound is to abuse it: switch it on and off at high speed (pulse-width
modulation) to fake sample playback. That is how DOS games got digitised
speech out of a one-bit output.

## 17. AdLib (OPL2) FM

The OPL2 has nine melodic channels, each built from two operators — a
*modulator* that shapes and a *carrier* that you hear. You talk to it
through two ports: write a register number to `388h`, then its value to
`389h`, with a short delay after each because the chip is slow.

Registers you need to start:

| Register | Meaning |
|----------|---------|
| `20h+op` | tremolo/vibrato/sustain flags and frequency multiplier |
| `40h+op` | output level (0 = loudest, 63 = silent) |
| `60h+op` | attack rate (high nibble), decay rate (low nibble) |
| `80h+op` | sustain level (high), release rate (low) |
| `A0h+ch` | low 8 bits of the F-number (pitch) |
| `B0h+ch` | key-on (bit 5), octave/"block" (bits 2-4), F-number bits 8-9 |
| `C0h+ch` | feedback and the modulator/carrier connection |

Operator offsets for channel 0 are `00h` (modulator) and `03h` (carrier);
channel *n* uses the *n*-th entry of the usual `00 01 02 08 09 0A 10 11 12`
table.

```nasm
; lesson17.asm - an AdLib arpeggio
org 100h
bits 16

OPL_IDX equ 388h
OPL_DAT equ 389h

start:
    mov ah, 09h
    mov dx, banner
    int 21h
    sti

    call opl_reset
    mov si, patch
    call opl_program

    mov si, tune
.play:
    lodsb
    cmp al, 0FFh
    je .done
    mov bl, al              ; BL = semitone, 0 = C
    lodsb
    mov bh, al              ; BH = block (octave)
    call note_on
    call delay
    call note_off
    jmp .play
.done:
    call opl_reset
    ret

; opl_write: AH = register, AL = value.  The delays are the ones real
; AdLib code uses; the emulator charges about 1 us per port access, so
; "read the index port N times" takes the intended time here too.
opl_write:
    push ax
    push cx
    push dx
    mov dx, OPL_IDX
    xchg al, ah
    out dx, al              ; select the register
    mov cx, 6
.d1:
    in al, dx
    loop .d1
    mov dx, OPL_DAT
    mov al, ah
    out dx, al              ; write the value
    mov dx, OPL_IDX
    mov cx, 35
.d2:
    in al, dx
    loop .d2
    pop dx
    pop cx
    pop ax
    ret

; opl_reset: silence every register, then enable the waveform select
opl_reset:
    mov ah, 1
.z:
    xor al, al
    call opl_write
    inc ah
    cmp ah, 0F6h
    jne .z
    mov ax, 0120h           ; register 01h = 20h
    call opl_write
    ret

; opl_program: SI -> pairs of (register, value), terminated by register 0
opl_program:
    lodsb
    test al, al
    jz .done
    mov ah, al
    lodsb
    call opl_write
    jmp opl_program
.done:
    ret

; note_on: BL = semitone (0..11), BH = block (octave)
note_on:
    push si
    movzx si, bl
    shl si, 1
    mov cx, [fnum+si]       ; CX = the 10-bit F-number
    mov ah, 0A0h
    mov al, cl
    call opl_write          ; F-number, low 8 bits
    mov ah, 0B0h
    mov al, ch
    and al, 3               ; F-number, high 2 bits
    mov dl, bh
    shl dl, 2
    or al, dl               ; block
    or al, 20h              ; key on
    call opl_write
    pop si
    ret

note_off:
    mov ax, 0B000h          ; key off, same channel
    call opl_write
    ret

delay:
    mov ah, 86h
    mov cx, 0002h
    mov dx, 49F0h           ; 150 ms
    int 15h
    ret

; A bright plucked patch on channel 0.
patch:
    db 20h, 01h             ; modulator: multiplier 1
    db 40h, 10h             ; modulator: level
    db 60h, 0F0h            ; modulator: fast attack, no decay
    db 80h, 77h             ; modulator: sustain/release
    db 23h, 01h             ; carrier: multiplier 1
    db 43h, 00h             ; carrier: full volume
    db 63h, 0F1h            ; carrier: fast attack, slight decay
    db 83h, 76h             ; carrier: sustain/release
    db 0C0h, 0Eh            ; feedback 7, FM connection
    db 0                    ; end of the patch

; F-numbers for C, C#, D ... B (the block selects the octave)
fnum:   dw 157h, 16Bh, 181h, 198h, 1B0h, 1CAh, 1E5h, 202h, 220h, 241h, 263h, 287h

; semitone, block pairs
tune:   db 0,4, 4,4, 7,4, 0,5, 7,4, 4,4, 0,4, 0,5, 0FFh

banner: db 'AdLib arpeggio - use -wav out.wav to capture it', 13, 10, '$'
```

Real AdLib code starts with a detection sequence (mask the timers, start
timer 1 with a preset of `FFh`, read the status port twice and check for
`00h` then `C0h`); [examples/cracktro.asm](../../examples/cracktro.asm) has it,
and the emulator answers exactly as the hardware does.

Playing music, rather than a scale, means a small player: a table of rows
(one per note step), a counter that fires a row every N frames, and a
per-channel state machine for effects such as arpeggio or vibrato. The
cracktro example does all four in about a hundred lines.

---

[← Part 3 — Effects](03-effects.md) · [Contents](README.md) · [Part 5 — Making a demo →](05-demo.md)
