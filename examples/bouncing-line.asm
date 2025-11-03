; Bouncing Line with Bresenham Algorithm
; Two endpoints bounce on screen edges, connected by a line
; Uses double buffering and VSync for smooth animation

.code
    ; Set VGA mode 13h (320x200, 256 colors)
    MOV AX, 0x13
    INT 0x10

    ; Set DS and ES to back buffer segment (0x7000)
    ; This is our off-screen buffer in regular RAM (64000 bytes)
    MOV AX, 0x7000
    MOV DS, AX
    MOV ES, AX

    ; Clear back buffer
    XOR DI, DI
    MOV CX, 32000
    XOR AX, AX
    REP STOSW

    ; Initialize endpoint 1: (x1, y1) = (50, 50) at offset 64100
    MOV AX, 50
    MOV [64100], AX        ; x1
    MOV [64102], AX        ; y1 (also 50)
    MOV AX, 2
    MOV [64104], AX        ; dx1 (velocity)
    MOV AX, 3
    MOV [64106], AX        ; dy1 (velocity)
    MOV AX, 1
    MOV [64108], AX        ; dir_x1 (1 = right)
    MOV [64110], AX        ; dir_y1 (1 = down)

    ; Initialize endpoint 2: (x2, y2) = (270, 150) at offset 64120
    MOV AX, 270
    MOV [64120], AX        ; x2
    MOV AX, 150
    MOV [64122], AX        ; y2
    MOV AX, 3
    MOV [64124], AX        ; dx2 (velocity)
    MOV AX, 2
    MOV [64126], AX        ; dy2 (velocity)
    MOV AX, 0
    MOV [64128], AX        ; dir_x2 (0 = left)
    MOV [64130], AX        ; dir_y2 (0 = up)

main_loop:
    ; Clear back buffer (black)
    XOR DI, DI
    MOV CX, 32000
    XOR AX, AX
    REP STOSW

    ; Update endpoint 1 position (x1)
    MOV AX, [64100]        ; x1
    MOV BX, [64108]        ; dir_x1
    CMP BX, 1
    JE move_x1_right
    SUB AX, [64104]        ; move left
    JMP check_x1_bounds
move_x1_right:
    ADD AX, [64104]        ; move right

check_x1_bounds:
    CMP AX, 0
    JLE bounce_x1_left
    CMP AX, 319
    JGE bounce_x1_right
    MOV [64100], AX
    JMP update_y1

bounce_x1_left:
    MOV AX, 0
    MOV [64100], AX
    MOV AX, 1
    MOV [64108], AX        ; change direction to right
    JMP update_y1

bounce_x1_right:
    MOV AX, 319
    MOV [64100], AX
    MOV AX, 0
    MOV [64108], AX        ; change direction to left

update_y1:
    MOV AX, [64102]        ; y1
    MOV BX, [64110]        ; dir_y1
    CMP BX, 1
    JE move_y1_down
    SUB AX, [64106]        ; move up
    JMP check_y1_bounds
move_y1_down:
    ADD AX, [64106]        ; move down

check_y1_bounds:
    CMP AX, 0
    JLE bounce_y1_top
    CMP AX, 199
    JGE bounce_y1_bottom
    MOV [64102], AX
    JMP update_point2

bounce_y1_top:
    MOV AX, 0
    MOV [64102], AX
    MOV AX, 1
    MOV [64110], AX        ; change direction to down
    JMP update_point2

bounce_y1_bottom:
    MOV AX, 199
    MOV [64102], AX
    MOV AX, 0
    MOV [64110], AX        ; change direction to up

update_point2:
    ; Update endpoint 2 position (x2)
    MOV AX, [64120]        ; x2
    MOV BX, [64128]        ; dir_x2
    CMP BX, 1
    JE move_x2_right
    SUB AX, [64124]        ; move left
    JMP check_x2_bounds
move_x2_right:
    ADD AX, [64124]        ; move right

check_x2_bounds:
    CMP AX, 0
    JLE bounce_x2_left
    CMP AX, 319
    JGE bounce_x2_right
    MOV [64120], AX
    JMP update_y2

bounce_x2_left:
    MOV AX, 0
    MOV [64120], AX
    MOV AX, 1
    MOV [64128], AX        ; change direction to right
    JMP update_y2

bounce_x2_right:
    MOV AX, 319
    MOV [64120], AX
    MOV AX, 0
    MOV [64128], AX        ; change direction to left

update_y2:
    MOV AX, [64122]        ; y2
    MOV BX, [64130]        ; dir_y2
    CMP BX, 1
    JE move_y2_down
    SUB AX, [64126]        ; move up
    JMP check_y2_bounds
move_y2_down:
    ADD AX, [64126]        ; move down

check_y2_bounds:
    CMP AX, 0
    JLE bounce_y2_top
    CMP AX, 199
    JGE bounce_y2_bottom
    MOV [64122], AX
    JMP draw_line

bounce_y2_top:
    MOV AX, 0
    MOV [64122], AX
    MOV AX, 1
    MOV [64130], AX        ; change direction to down
    JMP draw_line

bounce_y2_bottom:
    MOV AX, 199
    MOV [64122], AX
    MOV AX, 0
    MOV [64130], AX        ; change direction to up

draw_line:
    ; Bresenham's line algorithm
    ; Copy coordinates to working variables (offset 64200+)
    MOV AX, [64100]        ; x1
    MOV [64200], AX        ; bres_x0
    MOV AX, [64102]        ; y1
    MOV [64202], AX        ; bres_y0
    MOV AX, [64120]        ; x2
    MOV [64204], AX        ; bres_x1
    MOV AX, [64122]        ; y2
    MOV [64206], AX        ; bres_y1

    ; Calculate dx = abs(x1 - x0)
    MOV AX, [64204]
    SUB AX, [64200]
    JGE dx_positive
    NEG AX
dx_positive:
    MOV [64208], AX        ; bres_dx

    ; Calculate sx = x0 < x1 ? 1 : 0 (using 0 to mean -1)
    MOV AX, [64200]
    CMP AX, [64204]
    JL sx_positive
    MOV AX, 0
    MOV [64210], AX        ; bres_sx = 0 (will subtract)
    JMP calc_dy
sx_positive:
    MOV AX, 1
    MOV [64210], AX        ; bres_sx = 1 (will add)

calc_dy:
    ; Calculate dy = abs(y1 - y0)
    MOV AX, [64206]
    SUB AX, [64202]
    JGE dy_positive
    NEG AX
dy_positive:
    MOV [64212], AX        ; bres_dy

    ; Calculate sy = y0 < y1 ? 1 : 0 (using 0 to mean -1)
    MOV AX, [64202]
    CMP AX, [64206]
    JL sy_positive
    MOV AX, 0
    MOV [64214], AX        ; bres_sy = 0 (will subtract)
    JMP calc_err
sy_positive:
    MOV AX, 1
    MOV [64214], AX        ; bres_sy = 1 (will add)

calc_err:
    ; err = dx - dy
    MOV AX, [64208]
    SUB AX, [64212]
    MOV [64216], AX        ; bres_err

bresenham_loop:
    ; Plot pixel at (x0, y0)
    ; Calculate offset: y0 * 320 + x0
    MOV AX, [64202]        ; bres_y0
    MOV BX, 320
    MUL BX
    ADD AX, [64200]        ; bres_x0
    MOV DI, AX

    ; Write to back buffer
    MOV AL, 15
    MOV [DI], AL           ; Write white pixel

    ; Check if we reached the end point
    MOV AX, [64200]
    CMP AX, [64204]
    JNE continue_line
    MOV AX, [64202]
    CMP AX, [64206]
    JE line_done

continue_line:
    ; e2 = 2 * err
    MOV AX, [64216]
    SHL AX, 1
    MOV [64218], AX        ; bres_e2

    ; if (e2 > -dy)
    MOV BX, [64212]
    NEG BX
    CMP AX, BX
    JLE check_e2_dx

    ; err -= dy
    MOV AX, [64216]
    SUB AX, [64212]
    MOV [64216], AX

    ; x0 += sx (or -= 1 if sx=0)
    MOV BX, [64210]
    CMP BX, 1
    JE add_sx
    MOV AX, [64200]
    DEC AX
    MOV [64200], AX
    JMP check_e2_dx
add_sx:
    MOV AX, [64200]
    INC AX
    MOV [64200], AX

check_e2_dx:
    ; if (e2 < dx)
    MOV AX, [64218]
    CMP AX, [64208]
    JGE bresenham_loop

    ; err += dx
    MOV AX, [64216]
    ADD AX, [64208]
    MOV [64216], AX

    ; y0 += sy (or -= 1 if sy=0)
    MOV BX, [64214]
    CMP BX, 1
    JE add_sy
    MOV AX, [64202]
    DEC AX
    MOV [64202], AX
    JMP bresenham_loop
add_sy:
    MOV AX, [64202]
    INC AX
    MOV [64202], AX
    JMP bresenham_loop

line_done:
    ; Check for keypress (non-blocking)
    MOV AH, 0x01
    INT 0x16
    JZ do_flip             ; No key pressed, continue

    ; Key pressed - read it
    MOV AH, 0x00
    INT 0x16
    ; Exit on any key
    JMP exit_program

do_flip:
    ; Wait for VBlank - ensures smooth 60 FPS without tearing
    MOV DX, 0x3DA
    IN AL, DX              ; Reading 0x3DA waits for VBlank

    ; DOUBLE BUFFER FLIP: Copy back buffer to VGA memory
    PUSH DS
    PUSH ES

    ; Source: back buffer at 0x7000
    MOV AX, 0x7000
    MOV DS, AX
    XOR SI, SI

    ; Dest: VGA memory
    MOV AX, 0xA000
    MOV ES, AX
    XOR DI, DI

    ; Copy 64000 bytes (320x200)
    MOV CX, 32000          ; 64000 / 2 = 32000 words
    REP MOVSW

    POP ES
    POP DS

    ; Restore DS and ES to back buffer for next frame
    MOV AX, 0x7000
    MOV DS, AX
    MOV ES, AX

    JMP main_loop

exit_program:
    ; Restore text mode
    MOV AX, 0x03
    INT 0x10

    HLT
