; Ported to NASM syntax: assemble with `asm-emu asm` or run directly.
org 100h
bits 16

; Cracktro - a classic crack-intro screen in the 1990 style:
;   * copper bars done the PC way (the DAC entries of 100 row colours are
;     rewritten every frame during vertical blank),
;   * a three-layer parallax starfield drawn incrementally,
;   * a typewriter that writes pages of text with the BIOS ROM 8x16 font,
;     holds them and wipes them,
;   * a sine scroller at the bottom in the same font,
;   * an AdLib (OPL2, ports 388h/389h) tune in the Amiga cracktro style:
;     octave bass, a chord channel arpeggiated at 70 Hz, a vibrato lead
;     with a delayed echo, and FM drums.
; ESC returns to DOS.

; ---------------------------------------------------------------------------
VGA_SEG     equ 0A000h
FONT_SEG    equ 0F000h      ; BIOS 8x16 font at F000:A000
FONT_OFF    equ 0A000h
BAND_SEG    equ 07000h      ; back buffer for the scroller band
BAND_Y      equ 168         ; first screen row of the scroller band
BAND_H      equ 32
BAND_VRAM   equ BAND_Y*320

TEXT_X0     equ 16          ; typewriter text box
TEXT_Y0     equ 24
TEXT_H      equ 7*16+2      ; seven lines plus the shadow row

COL_SHADOW  equ 8
COL_CURSOR  equ 14
COL_TEXT    equ 15
COL_STAR0   equ 16          ; 16-18: star layers
COL_SCROLL0 equ 32          ; 32-47: scroller gradient, one per glyph row
COL_ROW0    equ 56          ; 56-155: row colour y>>1 (the copper bars)

NSTARS      equ 96
TYPE_DELAY  equ 3           ; frames per typed character
HOLD_FRAMES equ 210         ; 3 s at 70 Hz
WIPE_ROWS   equ 4           ; rows wiped per frame
SCROLL_STEP equ 2           ; pixels per frame

SPEED       equ 7           ; frames per song row (~150 BPM)
OPL_INDEX   equ 388h
OPL_DATA    equ 389h

; ---------------------------------------------------------------------------
section .text
start:
    mov ax, 13h
    int 10h

    mov ax, FONT_SEG
    mov fs, ax
    mov ax, VGA_SEG
    mov gs, ax
    mov ax, BAND_SEG
    mov es, ax

    call setup_palette
    call draw_background
    call init_stars
    call opl_detect
    call opl_init
    sti

main_loop:
    call wait_vsync
    call upload_palette         ; both of these must finish inside vblank
    call blit_band
    call music_tick
    call copper_update
    call stars_update
    call typewriter_update
    call scroller_update
    inc word [frame]

    mov ah, 01h
    int 16h
    jz main_loop
    xor ah, ah
    int 16h
    cmp al, 27
    jne main_loop

exit:
    call opl_silence
    mov ax, 03h
    int 10h
    mov ax, 4C00h
    int 21h

; ---------------------------------------------------------------------------
; Video helpers
; ---------------------------------------------------------------------------
wait_vsync:
    mov dx, 3DAh
.a: in al, dx
    test al, 8
    jnz .a                      ; wait until the current retrace ends
.b: in al, dx
    test al, 8
    jz .b                       ; wait for the next one to start
    ret

; DAC entries 56..155 <- pal_buf (300 bytes)
upload_palette:
    mov dx, 3C8h
    mov al, COL_ROW0
    out dx, al
    inc dx
    mov si, pal_buf
    mov cx, 300
    rep outsb
    ret

; Copy the 320x32 scroller band from BAND_SEG to the bottom of the screen.
blit_band:
    push ds
    push es
    mov ax, BAND_SEG
    mov ds, ax
    mov ax, VGA_SEG
    mov es, ax
    xor si, si
    mov di, BAND_VRAM
    mov cx, (320*BAND_H)/4
    rep movsd
    pop es
    pop ds
    ret

; Fixed palette entries; the row colours are uploaded every frame.
setup_palette:
    mov dx, 3C8h
    mov al, COL_SHADOW
    out dx, al
    inc dx
    mov al, 12
    out dx, al
    out dx, al
    mov al, 20
    out dx, al

    mov dx, 3C8h
    mov al, COL_CURSOR
    out dx, al
    inc dx
    mov al, 63
    out dx, al
    out dx, al
    mov al, 24
    out dx, al
    mov al, 63                  ; 15 = white
    out dx, al
    out dx, al
    out dx, al
    mov al, 20                  ; 16 = far stars
    out dx, al
    out dx, al
    mov al, 30
    out dx, al
    mov al, 38                  ; 17
    out dx, al
    out dx, al
    mov al, 48
    out dx, al
    mov al, 63                  ; 18 = near stars
    out dx, al
    out dx, al
    out dx, al

    mov dx, 3C8h                ; scroller gradient: white -> orange
    mov al, COL_SCROLL0
    out dx, al
    inc dx
    xor cx, cx
.grad:
    mov al, 63
    out dx, al
    mov al, 63
    sub al, cl
    sub al, cl
    out dx, al
    mov al, 63
    mov bl, cl
    shl bl, 2
    sub al, bl
    out dx, al
    inc cx
    cmp cx, 16
    jb .grad
    ret

; Fill the screen so that row y has colour COL_ROW0 + y/2.
draw_background:
    push es
    mov ax, VGA_SEG
    mov es, ax
    xor di, di
    xor bx, bx                  ; y
.row:
    mov al, bl
    shr al, 1
    add al, COL_ROW0
    mov ah, al
    mov cx, 160
    rep stosw
    inc bx
    cmp bx, 200
    jb .row
    pop es
    ret

; Returns AL = background colour of screen row AL.
rowcol:
    shr al, 1
    add al, COL_ROW0
    ret

; ---------------------------------------------------------------------------
; Copper bars: three sine-driven bars added into the 100 row colours.
; ---------------------------------------------------------------------------
copper_update:
    mov di, pal_buf
    mov cx, 150
.clr:
    mov word [di], 0
    add di, 2
    loop .clr

    mov si, bar_ramp
    xor bx, bx                  ; bar index
.bar:
    mov ax, [frame]
    imul ax, [bar_speed+bx]
    add ax, [bar_phase+bx]
    and ax, 0FFh
    mov di, ax
    mov al, [sine_table+di]
    mov ah, 84
    mul ah                      ; AX = sine * 84
    mov al, ah                  ; AL = 0..83 (half-rows)
    xor ah, ah
    mov di, ax
    imul di, 3
    add di, pal_buf             ; bar occupies entries center-8 .. center+7
    mov cx, 48
.add:
    mov al, [si]
    add al, [di]
    cmp al, 63
    jbe .ok
    mov al, 63
.ok:
    mov [di], al
    inc si
    inc di
    loop .add
    add bx, 2
    cmp bx, 6
    jb .bar
    ret

; ---------------------------------------------------------------------------
; Starfield
; ---------------------------------------------------------------------------
init_stars:
    mov si, stars
    mov cx, NSTARS
    xor bx, bx
.s:
    push cx
    call rand16
    xor dx, dx
    mov cx, 320
    div cx
    mov [si], dx                ; x
    call rand16
    xor dx, dx
    mov cx, BAND_Y
    div cx
    mov [si+2], dl              ; y
    mov ax, bx
    xor dx, dx
    mov cx, 3
    div cx
    mov [si+3], dl              ; layer 0..2
    inc bx
    add si, 4
    pop cx
    loop .s
    ret

; 16-bit xorshift; returns AX.
rand16:
    mov ax, [seed]
    mov bx, ax
    shl bx, 7
    xor ax, bx
    mov bx, ax
    shr bx, 9
    xor ax, bx
    mov bx, ax
    shl bx, 8
    xor ax, bx
    mov [seed], ax
    ret

stars_update:
    mov si, stars
    mov cx, NSTARS
.star:
    push cx
    movzx bx, byte [si+3]       ; layer
    movzx di, byte [si+2]       ; y
    imul di, 320
    add di, [si]
    mov dl, bl
    add dl, COL_STAR0
    cmp [gs:di], dl             ; erase only if nobody drew over us
    jne .moved0
    mov al, [si+2]
    call rowcol
    mov [gs:di], al
.moved0:
    mov ax, [si]
    sub ax, bx
    dec ax                      ; speed = layer + 1
    jns .store
    push bx
    call rand16                 ; respawn on the right edge at a new row
    pop bx
    xor dx, dx
    mov cx, BAND_Y
    div cx
    mov [si+2], dl
    mov ax, 319
.store:
    mov [si], ax
    movzx di, byte [si+2]
    imul di, 320
    add di, ax
    mov al, [si+2]
    call rowcol
    cmp [gs:di], al             ; draw only over plain background
    jne .next
    mov al, bl
    add al, COL_STAR0
    mov [gs:di], al
.next:
    add si, 4
    pop cx
    loop .star
    ret

; ---------------------------------------------------------------------------
; Typewriter
; ---------------------------------------------------------------------------
TW_TYPE     equ 0
TW_HOLD     equ 1
TW_WIPE     equ 2

typewriter_update:
    mov al, [tw_state]
    cmp al, TW_HOLD
    je .hold
    cmp al, TW_WIPE
    je .wipe

    ; ---- typing ----
    dec byte [tw_timer]
    jnz .cursor
    mov byte [tw_timer], TYPE_DELAY
    mov si, [tw_ptr]
    mov al, [si]
    test al, al
    jz .endpage
    cmp al, 10
    jne .char
    inc word [tw_ptr]
    call cursor_hide
    mov word [tw_x], TEXT_X0
    add word [tw_y], 16
    jmp .cursor
.char:
    inc word [tw_ptr]
    push ax
    call cursor_hide
    pop ax
    cmp al, ' '
    je .advance
    push ax
    mov di, [tw_y]
    imul di, 320
    add di, [tw_x]
    add di, 321                 ; shadow one pixel down and right
    mov bl, COL_SHADOW
    call draw_glyph
    pop ax
    mov di, [tw_y]
    imul di, 320
    add di, [tw_x]
    mov bl, COL_TEXT
    call draw_glyph
.advance:
    add word [tw_x], 8
.cursor:
    ; blink the block cursor at the current position
    mov ax, [frame]
    test ax, 16
    jnz cursor_hide
    jmp cursor_show

.endpage:
    call cursor_hide
    mov byte [tw_state], TW_HOLD
    mov word [tw_timer16], HOLD_FRAMES
    ret

.hold:
    dec word [tw_timer16]
    jnz .ret
    mov byte [tw_state], TW_WIPE
    mov word [wipe_y], TEXT_Y0
.ret:
    ret

.wipe:
    mov cx, WIPE_ROWS
.wrow:
    push cx
    mov ax, [wipe_y]
    mov di, ax
    imul di, 320
    call rowcol
    mov ah, al
    push es
    push gs
    pop es
    mov cx, 160
    rep stosw
    pop es
    inc word [wipe_y]
    pop cx
    loop .wrow
    cmp word [wipe_y], TEXT_Y0+TEXT_H
    jb .ret
    ; next page (the page list ends with an empty page)
    mov si, [tw_ptr]
    inc si
    cmp byte [si], 0
    jne .setpage
    mov si, pages
.setpage:
    mov [tw_ptr], si
    mov word [tw_x], TEXT_X0
    mov word [tw_y], TEXT_Y0
    mov byte [tw_state], TW_TYPE
    mov byte [tw_timer], TYPE_DELAY
    ret

; draw_glyph: AL = character, DI = screen offset of the top-left pixel,
; BL = colour. Reads the 8x16 glyph from the BIOS ROM font.
draw_glyph:
    push si
    push cx
    push dx
    movzx si, al
    shl si, 4
    add si, FONT_OFF
    mov cx, 16
.row:
    mov ah, [fs:si]
    inc si
    push di
    mov dl, 8
.px:
    shl ah, 1
    jnc .skip
    mov [gs:di], bl
.skip:
    inc di
    dec dl
    jnz .px
    pop di
    add di, 320
    loop .row
    pop dx
    pop cx
    pop si
    ret

; The cursor is an 8x16 block at (tw_x, tw_y); cur_on remembers whether it
; is drawn so it can be restored to the background colours.
cursor_show:
    cmp byte [cur_on], 0
    jne .ret
    mov byte [cur_on], 1
    mov di, [tw_y]
    imul di, 320
    add di, [tw_x]
    mov cx, 16
.row:
    push cx
    push di
    mov al, COL_CURSOR
    mov cx, 8
.px:
    mov [gs:di], al
    inc di
    loop .px
    pop di
    add di, 320
    pop cx
    loop .row
.ret:
    ret

cursor_hide:
    cmp byte [cur_on], 0
    je .ret
    mov byte [cur_on], 0
    mov di, [tw_y]
    imul di, 320
    add di, [tw_x]
    mov bx, [tw_y]
    mov cx, 16
.row:
    push cx
    push di
    mov al, bl
    call rowcol
    mov cx, 8
.px:
    mov [gs:di], al
    inc di
    loop .px
    pop di
    add di, 320
    inc bx
    pop cx
    loop .row
.ret:
    ret

; ---------------------------------------------------------------------------
; Sine scroller (drawn into the band buffer at ES = BAND_SEG)
; ---------------------------------------------------------------------------
scroller_update:
    ; background: the band rows keep their copper colours
    xor di, di
    mov bx, BAND_Y
    mov dx, BAND_H
.fill:
    mov al, bl
    call rowcol
    mov ah, al
    mov cx, 160
    rep stosw
    inc bx
    dec dx
    jnz .fill

    ; glyphs
    mov bx, [scroll_x]
    mov si, [scroll_ptr]
.glyph:
    mov al, [si]
    test al, al
    jz .done
    cmp bx, 320
    jge .done
    cmp al, ' '
    je .space
    call scroll_glyph
.space:
    add bx, 8
    inc si
    jmp .glyph
.done:
    add word [wave_t], 3
    mov ax, [scroll_x]
    sub ax, SCROLL_STEP
    cmp ax, -8
    jg .keep
    add ax, 8                   ; the first character left the screen
    mov si, [scroll_ptr]
    inc si
    cmp byte [si], 0
    jne .ptr
    mov si, scroll_msg
.ptr:
    mov [scroll_ptr], si
.keep:
    mov [scroll_x], ax
    ret

; scroll_glyph: AL = character, BX = x of its first column (may be negative).
; Every column gets its own y offset from the sine table.
scroll_glyph:
    push bx
    push si
    movzx si, al
    shl si, 4
    add si, FONT_OFF
    mov dh, 80h                 ; column mask
.col:
    cmp bx, 320
    jae .next                   ; unsigned: also skips negative x
    mov ax, bx
    add ax, ax
    add ax, [wave_t]
    and ax, 0FFh
    mov di, ax
    mov al, [sine_table+di]
    shr al, 4                   ; 0..15
    movzx di, al
    imul di, 320
    add di, bx
    mov cl, COL_SCROLL0
    push si
.row:
    test byte [fs:si], dh
    jz .nopx
    mov [es:di], cl
.nopx:
    inc si
    inc cl
    add di, 320
    cmp cl, COL_SCROLL0+16
    jne .row
    pop si
.next:
    inc bx
    shr dh, 1
    jnz .col
    pop si
    pop bx
    ret

; ---------------------------------------------------------------------------
; AdLib
; ---------------------------------------------------------------------------
; opl_write: AH = register, AL = value (all registers preserved). The reads
; give the chip the 3.3 us and 23 us it needs after the index and data writes.
opl_write:
    push dx
    push cx
    push ax
    mov dx, OPL_INDEX
    xchg al, ah
    out dx, al
    mov cx, 6
.d1:
    in al, dx
    loop .d1
    inc dx
    mov al, ah
    out dx, al
    dec dx
    mov cx, 35
.d2:
    in al, dx
    loop .d2
    pop ax
    pop cx
    pop dx
    ret

; The standard AdLib detection: timer 1 must overflow within 80 us.
opl_detect:
    mov ax, 0460h               ; mask both timers
    call opl_write
    mov ax, 0480h               ; reset IRQ flags
    call opl_write
    mov dx, OPL_INDEX
    in al, dx
    mov bl, al
    mov ax, 02FFh               ; timer 1 preset FF: one tick to overflow
    call opl_write
    mov ax, 0421h               ; unmask + start timer 1
    call opl_write
    mov cx, 100
.wait:
    in al, dx
    loop .wait
    in al, dx
    mov bh, al
    push bx
    mov ax, 0460h
    call opl_write
    mov ax, 0480h
    call opl_write
    pop bx
    and bx, 0E0E0h
    cmp bx, 0C000h              ; first read 00, second C0
    jne .none
    mov byte [have_opl], 1
.none:
    ret

opl_init:
    mov ah, 01h                 ; clear every register
.clr:
    xor al, al
    call opl_write
    inc ah
    cmp ah, 0F6h
    jne .clr
    mov ax, 0120h               ; enable waveform select
    call opl_write
    mov ax, 0BD00h              ; melodic mode, shallow AM/vibrato
    call opl_write
    xor bx, bx                  ; channel
.instr:
    mov si, bx
    add si, si
    mov si, [instr_list+si]
    call set_instrument
    inc bx
    cmp bx, NCHANNELS
    jb .instr
    ret

; set_instrument: BL = channel, SI -> 11 bytes (modulator 20/40/60/80/E0,
; carrier 23/43/63/83/E3, C0) as in an SBI file.
set_instrument:
    movzx di, bl
    mov dl, [op_offset+di]
    mov di, reg_bases
    mov cx, 5
.mod:
    mov ah, [di]
    add ah, dl
    lodsb
    call opl_write
    inc di
    loop .mod
    mov di, reg_bases
    mov cx, 5
.car:
    mov ah, [di]
    add ah, dl
    add ah, 3
    lodsb
    call opl_write
    inc di
    loop .car
    mov ah, 0C0h
    add ah, bl
    lodsb
    call opl_write
    ret

; note_on: BL = channel, AL = note (octave*12 + semitone).
note_on:
    push bx
    push si
    push dx
    xor ah, ah
    push bx
    mov bl, 12
    div bl                      ; AL = block, AH = semitone
    pop bx
    movzx si, ah
    add si, si
    mov dx, [fnum_table+si]     ; DX = F-number
    mov ah, al                  ; AH = block
    shl ah, 2
    or ah, dh                   ; block<<2 | F-number bits 8-9
    or ah, 20h                  ; key on
    movzx si, bl
    mov [ch_b0+si], ah
    push ax
    mov ah, 0A0h
    add ah, bl
    mov al, dl
    call opl_write              ; A0+ch = F-number low byte
    pop ax
    mov ah, 0B0h
    add ah, bl
    mov al, [ch_b0+si]
    call opl_write
    pop dx
    pop si
    pop bx
    ret

; note_off: BL = channel.
note_off:
    push si
    movzx si, bl
    mov al, [ch_b0+si]
    and al, 0DFh
    mov [ch_b0+si], al
    mov ah, 0B0h
    add ah, bl
    call opl_write
    pop si
    ret

opl_silence:
    xor bl, bl
.ch:
    mov ah, 0B0h
    add ah, bl
    xor al, al
    call opl_write
    inc bl
    cmp bl, 9
    jb .ch
    ret

; ---------------------------------------------------------------------------
; Music player. Every track is a list of (value, rows) pairs ending in FF
; (loop). Melodic tracks carry notes (0 = rest), the chord track carries
; chord numbers that the arpeggiator cycles through every frame, the drum
; tracks carry drum numbers. Tracks loop independently; their lengths are
; multiples of a bar so they stay in step.
; ---------------------------------------------------------------------------
KIND_NOTE   equ 0
KIND_CHORD  equ 1
KIND_DRUM   equ 2

music_tick:
    call arp_step
    dec byte [row_timer]
    jnz .ret
    mov byte [row_timer], SPEED
    xor bx, bx                  ; track index
.trk:
    cmp byte [trk_wait+bx], 0
    jne .wait
    mov si, bx
    add si, si
    mov di, [trk_ptr+si]
    mov al, [di]
    cmp al, 0FFh
    jne .event
    mov di, [trk_start+si]
    mov al, [di]
.event:
    mov ah, [di+1]
    add di, 2
    mov [trk_ptr+si], di
    mov [trk_wait+bx], ah
    mov cl, [trk_kind+bx]
    push bx
    mov bl, [trk_chan+bx]
    cmp cl, KIND_CHORD
    je .chord
    cmp cl, KIND_DRUM
    je .drum
    push ax
    call note_off
    pop ax
    test al, al
    jz .done
    call note_on
    jmp .done
.chord:
    test al, al
    jnz .chordon
    mov byte [arp_on], 0
    call note_off
    jmp .done
.chordon:
    dec al
    movzx si, al
    imul si, 3
    mov cx, [chords+si]
    mov [arp_notes], cx
    mov cl, [chords+si+2]
    mov [arp_notes+2], cl
    mov byte [arp_on], 1
    mov byte [arp_pos], 0
    mov al, [arp_notes]
    push ax
    call note_off               ; retrigger on a chord change
    pop ax
    call note_on
    jmp .done
.drum:
    test al, al
    jz .done
    movzx si, al
    mov bl, [drum_chan+si]
    mov al, [drum_note+si]
    push ax
    call note_off
    pop ax
    call note_on
.done:
    pop bx
.wait:
    dec byte [trk_wait+bx]
    inc bx
    cmp bx, NTRACKS
    jb .trk
.ret:
    ret

; Cycle the chord channel through its three notes, one per frame, without
; retriggering (the key-on bit stays set).
arp_step:
    cmp byte [arp_on], 0
    je .ret
    mov al, [arp_pos]
    inc al
    cmp al, 3
    jb .ok
    xor al, al
.ok:
    mov [arp_pos], al
    movzx si, al
    mov al, [arp_notes+si]
    mov bl, CH_CHORD
    call note_on
.ret:
    ret

; ---------------------------------------------------------------------------
section .data
; ---------------------------------------------------------------------------
frame:      dw 0
seed:       dw 0ACE1h

; copper
bar_speed:  dw 2, 3, 5
bar_phase:  dw 0, 85, 170
bar_ramp:
    db 7, 2, 2, 15, 3, 3, 23, 5, 5, 31, 7, 7, 39, 9, 9, 47, 10, 10, 55, 12, 12, 63, 14, 14
    db 63, 14, 14, 55, 12, 12, 47, 10, 10, 39, 9, 9, 31, 7, 7, 23, 5, 5, 15, 3, 3, 7, 2, 2
    db 2, 7, 2, 3, 15, 3, 5, 23, 5, 7, 31, 7, 9, 39, 9, 10, 47, 10, 12, 55, 12, 14, 63, 14
    db 14, 63, 14, 12, 55, 12, 10, 47, 10, 9, 39, 9, 7, 31, 7, 5, 23, 5, 3, 15, 3, 2, 7, 2
    db 2, 2, 7, 5, 5, 15, 8, 8, 23, 11, 11, 31, 14, 14, 39, 16, 16, 47, 19, 19, 55, 22, 22, 63
    db 22, 22, 63, 19, 19, 55, 16, 16, 47, 14, 14, 39, 11, 11, 31, 8, 8, 23, 5, 5, 15, 2, 2, 7

; typewriter
tw_state:   db TW_TYPE
tw_timer:   db TYPE_DELAY
tw_timer16: dw 0
tw_ptr:     dw pages
tw_x:       dw TEXT_X0
tw_y:       dw TEXT_Y0
wipe_y:     dw 0
cur_on:     db 0

pages:
    db "         ASM-EMU  PRESENTS", 10
    db 10
    db "       CRACKTRO  -  386 INTRO", 10
    db 10
    db "     CODE, MUSIC AND FX: FABLE 5", 0
    db "        GREETINGS FLY OUT TO", 10
    db 10
    db "   FUTURE CREW      RAZOR 1911", 10
    db "   TRITON           FAIRLIGHT", 10
    db "   THE HUMBLE GUYS  INC", 10
    db "   BITBENDAZ", 10
    db "        AND EVERYONE ELSE!", 0
    db "      THIS INTRO RUNS ON A REAL", 10
    db "     386 EMULATOR WITH AN EMULATED", 10
    db "         ADLIB OPL2 SOUND CHIP", 10
    db 10
    db "     PRESS ESC TO RETURN TO DOS", 0
    db 0                        ; end of the page list

; scroller
scroll_x:   dw 320
scroll_ptr: dw scroll_msg
wave_t:     dw 0
scroll_msg:
    db "        WELCOME TO THE CRACKTRO EXAMPLE...   "
    db "COPPER BARS, STARS, A TYPEWRITER AND THIS SINE SCROLLER ARE ALL "
    db "DRAWN WITH THE BIOS FONT IN MODE 13H, WHILE THE MUSIC COMES FROM "
    db "AN EMULATED YAMAHA YM3812 ON PORTS 388H AND 389H...   "
    db "WRAPPING AROUND NOW!        ", 0

; AdLib
have_opl:   db 0
op_offset:  db 0, 1, 2, 8, 9, 0Ah, 10h, 11h, 12h
reg_bases:  db 20h, 40h, 60h, 80h, 0E0h
ch_b0:      times 9 db 0

CH_BASS     equ 0
CH_LEAD     equ 1
CH_CHORD    equ 2
CH_KICK     equ 3
CH_SNARE    equ 4
CH_HAT      equ 5
CH_ECHO     equ 6
NCHANNELS   equ 7

instr_list: dw instr_bass, instr_lead, instr_chord, instr_kick, instr_snare, instr_hat, instr_echo

;              mod: 20    40    60    80    E0   car: 23    43    60    80    E0   C0
instr_bass:  db 11h,  12h,  0F5h, 5Fh,  00h,      01h,  02h,  0F8h, 4Ah,  00h,  04h
instr_lead:  db 21h,  1Ah,  0F2h, 24h,  00h,      61h,  00h,  0F2h, 35h,  00h,  06h
instr_echo:  db 21h,  1Ah,  0F2h, 24h,  00h,      21h,  14h,  0F2h, 35h,  00h,  06h
instr_chord: db 21h,  20h,  0F0h, 10h,  00h,      21h,  0Ch,  0F0h, 0Ah,  00h,  02h
instr_kick:  db 01h,  0Bh,  0F8h, 06h,  00h,      01h,  00h,  0F8h, 48h,  00h,  0Ah
instr_snare: db 07h,  00h,  0F8h, 08h,  00h,      02h,  00h,  0F7h, 07h,  00h,  0Eh
instr_hat:   db 0Ch,  00h,  0FAh, 0Ah,  03h,      04h,  1Ah,  0FBh, 0Bh,  03h,  0Eh

; F-numbers for C..B (block = octave)
fnum_table: dw 157h, 16Bh, 181h, 198h, 1B0h, 1CAh, 1E5h, 202h, 220h, 241h, 263h, 287h

; note = octave*12 + semitone (C=0 ... B=11)
C2 equ 24
A2 equ 33
C3 equ 36
D3 equ 38
E3 equ 40
F3 equ 41
G3 equ 43
A3 equ 45
B3 equ 47
C4 equ 48
D4 equ 50
E4 equ 52
F4 equ 53
G4 equ 55
GS4 equ 56
A4 equ 57
B4 equ 59
C5 equ 60
D5 equ 62
E5 equ 64
F5 equ 65
G5 equ 67
GS5 equ 68
A5 equ 69
C6 equ 72

; drums: number -> channel and pitch
drum_chan:  db 0, CH_KICK, CH_SNARE, CH_HAT
drum_note:  db 0, C2, 70, 96
KICK equ 1
SNARE equ 2
HAT equ 3

; chords: number -> three notes for the arpeggiator
chords:     db A3, C4, E4         ; 1 Am
            db F3, A3, C4         ; 2 F
            db C4, E4, G4         ; 3 C
            db G3, B3, D4         ; 4 G
            db D4, F4, A4         ; 5 Dm
            db E4, GS4, B4        ; 6 E
AM equ 1
FM equ 2
CM equ 3
GM equ 4
DM equ 5
EM equ 6

NTRACKS     equ 6
trk_start:  dw song_bass, song_lead, song_lead, song_chords, song_drums, song_hats
trk_ptr:    dw song_bass, song_lead, song_lead, song_chords, song_drums, song_hats
trk_chan:   db CH_BASS, CH_LEAD, CH_ECHO, CH_CHORD, 0, 0
trk_kind:   db KIND_NOTE, KIND_NOTE, KIND_NOTE, KIND_CHORD, KIND_DRUM, KIND_DRUM
trk_wait:   db 0, 0, 3, 0, 0, 0   ; the echo plays the lead three rows late
row_timer:  db 1
arp_on:     db 0
arp_pos:    db 0
arp_notes:  db 0, 0, 0

; One bar = 16 rows. Progression: Am F C G Am F Dm E.
%macro BASSBAR 1
    %rep 7
        db %1, 1, %1+12, 1
    %endrep
    db %1, 1, %1+7, 1
%endmacro

song_bass:
    BASSBAR A2
    BASSBAR F3-12
    BASSBAR C3
    BASSBAR G3-12
    BASSBAR A2
    BASSBAR F3-12
    BASSBAR D3
    BASSBAR E3
    db 0FFh

song_chords:
    db AM,16, FM,16, CM,16, GM,16, AM,16, FM,16, DM,16, EM,16
    db 0FFh

song_drums:
    db KICK,4, SNARE,4, KICK,2, KICK,2, SNARE,3, KICK,1
    db 0FFh

song_hats:
    db HAT,2, HAT,2, HAT,2, HAT,2, HAT,2, HAT,2, HAT,2, HAT,1, HAT,1
    db 0FFh

song_lead:
    ; phrase 1: Am F C G
    db E5,2, A5,2, E5,2, C5,2, D5,2, E5,4, C5,2
    db A4,2, C5,2, F5,4, E5,2, C5,2, A4,4
    db G4,2, C5,2, E5,2, G5,4, E5,2, D5,2, C5,2
    db B4,2, D5,2, G5,4, D5,2, B4,2, G4,2, A4,2
    ; phrase 2: Am F Dm E
    db A4,2, C5,2, E5,2, A5,4, G5,2, E5,2, C5,2
    db F5,2, E5,2, C5,2, A4,4, C5,2, D5,2, E5,2
    db F5,2, D5,2, A4,2, D5,2, F5,2, A5,4, G5,2
    db GS5,4, E5,4, B4,4, 0,4
    ; phrase 1 again
    db E5,2, A5,2, E5,2, C5,2, D5,2, E5,4, C5,2
    db A4,2, C5,2, F5,4, E5,2, C5,2, A4,4
    db G4,2, C5,2, E5,2, G5,4, E5,2, D5,2, C5,2
    db B4,2, D5,2, G5,4, D5,2, B4,2, G4,2, A4,2
    ; phrase 3: Am F Dm E, climbing
    db A5,1, 0,1, A5,1, 0,1, A5,2, G5,2, E5,2, C5,2, D5,2, E5,2
    db F5,2, A5,2, C6,4, A5,2, F5,2, E5,4
    db D5,2, F5,2, A5,4, F5,2, D5,2, C5,2, D5,2
    db E5,4, B4,4, E5,8
    db 0FFh

sine_table:
    db 128, 131, 134, 137, 140, 143, 146, 149, 152, 155, 158, 162, 165, 167, 170, 173
    db 176, 179, 182, 185, 188, 190, 193, 196, 198, 201, 203, 206, 208, 211, 213, 215
    db 218, 220, 222, 224, 226, 228, 230, 232, 234, 235, 237, 238, 240, 241, 243, 244
    db 245, 246, 248, 249, 250, 250, 251, 252, 253, 253, 254, 254, 254, 255, 255, 255
    db 255, 255, 255, 255, 254, 254, 254, 253, 253, 252, 251, 250, 250, 249, 248, 246
    db 245, 244, 243, 241, 240, 238, 237, 235, 234, 232, 230, 228, 226, 224, 222, 220
    db 218, 215, 213, 211, 208, 206, 203, 201, 198, 196, 193, 190, 188, 185, 182, 179
    db 176, 173, 170, 167, 165, 162, 158, 155, 152, 149, 146, 143, 140, 137, 134, 131
    db 128, 124, 121, 118, 115, 112, 109, 106, 103, 100, 97, 93, 90, 88, 85, 82
    db 79, 76, 73, 70, 67, 65, 62, 59, 57, 54, 52, 49, 47, 44, 42, 40
    db 37, 35, 33, 31, 29, 27, 25, 23, 21, 20, 18, 17, 15, 14, 12, 11
    db 10, 9, 7, 6, 5, 5, 4, 3, 2, 2, 1, 1, 1, 0, 0, 0
    db 0, 0, 0, 0, 1, 1, 1, 2, 2, 3, 4, 5, 5, 6, 7, 9
    db 10, 11, 12, 14, 15, 17, 18, 20, 21, 23, 25, 27, 29, 31, 33, 35
    db 37, 40, 42, 44, 47, 49, 52, 54, 57, 59, 62, 65, 67, 70, 73, 76
    db 79, 82, 85, 88, 90, 93, 97, 100, 103, 106, 109, 112, 115, 118, 121, 124

section .bss
pal_buf:    resb 300
stars:      resb NSTARS*4        ; x dw, y db, layer db
