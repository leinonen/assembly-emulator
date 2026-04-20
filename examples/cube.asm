; Rotating 3D Wireframe Cube
; 8 vertices, 12 edges, X and Y axis rotation, perspective projection
; Double-buffered animation with VSync

.data
    ; 8 cube vertices: (vx, vy, vz) as signed words, range -64..64
    cube_verts:
        dw -64, -64, -64
        dw  64, -64, -64
        dw  64,  64, -64
        dw -64,  64, -64
        dw -64, -64,  64
        dw  64, -64,  64
        dw  64,  64,  64
        dw -64,  64,  64

    ; 12 edges: pairs of vertex indices (bytes)
    cube_edges:
        db 0, 1,  1, 2,  2, 3,  3, 0
        db 4, 5,  5, 6,  6, 7,  7, 4
        db 0, 4,  1, 5,  2, 6,  3, 7

    ; Constants
    val_127:   dw 127
    val_256:   dw 256
    focal_200: dw 200
    temp_w:    dw 0

    ; Rotation angles (0..255)
    angle_x:   dw 0
    angle_y:   dw 0

    ; Per-frame trig values (signed, -127..127)
    sin_x_val: dw 0
    cos_x_val: dw 0
    sin_y_val: dw 0
    cos_y_val: dw 0

    ; Rotation scratch
    rvx_val:   dw 0
    rvz_val:   dw 0
    rvy_val:   dw 0

    ; Projected 2D screen coordinates: 8 * (sx, sy) words = 32 bytes
    screen_verts:
        dw 0, 0,  0, 0,  0, 0,  0, 0
        dw 0, 0,  0, 0,  0, 0,  0, 0

    ; Sine lookup table (256 bytes: value = sin(i*2pi/256)*127+127)
    sine_table:
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
        db 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0

    ; Bresenham line drawing state
    bres_x0:  dw 0
    bres_y0:  dw 0
    bres_x1:  dw 0
    bres_y1:  dw 0
    bres_dx:  dw 0
    bres_dy:  dw 0
    bres_sx:  dw 0
    bres_sy:  dw 0
    bres_err: dw 0
    bres_e2:  dw 0

.code
start:
    mov ax, 0x13
    int 0x10

    call build_sine_table

    mov ax, 0x7000
    mov es, ax

main_loop:
    ; Clear back buffer
    xor di, di
    xor ax, ax
    mov cx, 32000
    rep stosw

    ; Advance rotation angles
    mov ax, [angle_y]
    add ax, 2
    and ax, 0xFF
    mov [angle_y], ax

    mov ax, [angle_x]
    inc ax
    and ax, 0xFF
    mov [angle_x], ax

    call transform_vertices
    call draw_edges

    ; VBlank sync
    mov dx, 0x3DA
    in al, dx

    ; Blit back buffer to VGA
    push ds
    mov ax, 0x7000
    mov ds, ax
    xor si, si
    mov ax, 0xA000
    mov es, ax
    xor di, di
    mov cx, 32000
    rep movsw
    pop ds

    mov ax, 0x7000
    mov es, ax

    ; Check ESC
    mov ah, 1
    int 0x16
    jz main_loop

    xor ah, ah
    int 0x16
    cmp al, 27
    jne main_loop

    mov ax, 0x03
    int 0x10
    hlt


; Build sine_table[i] = round(sin(i*2pi/256)*127+127) using x87 FPU
build_sine_table:
    push ax
    push cx
    push di
    push si

    finit
    fild word [val_256]
    fldpi
    fld1
    fld1
    faddp
    fmulp
    fdivrp
    fldz                    ; ST0=0 (angle), ST1=step

    mov di, sine_table
    mov si, temp_w
    mov cx, 256

bst_loop:
    fld st0                 ; ST0=angle, ST1=angle, ST2=step
    fsin
    fild word [val_127]
    fmulp
    fild word [val_127]
    faddp
    fistp word [si]
    mov al, [si]
    mov [di], al
    inc di
    fadd st1                ; angle += step
    loop bst_loop

    finit

    pop si
    pop di
    pop cx
    pop ax
    ret


; Transform all 8 vertices: Y rotation, X rotation, perspective projection
; Output fills screen_verts with (sx, sy) pairs
transform_vertices:
    push ax
    push bx
    push cx
    push dx
    push si
    push di
    push bp

    ; sin_y = sine_table[angle_y] - 127
    mov ax, [angle_y]
    mov si, sine_table
    add si, ax
    mov al, [si]
    xor ah, ah
    sub ax, 127
    mov [sin_y_val], ax

    ; cos_y = sine_table[(angle_y+64)&0xFF] - 127
    mov ax, [angle_y]
    add ax, 64
    and ax, 0xFF
    mov si, sine_table
    add si, ax
    mov al, [si]
    xor ah, ah
    sub ax, 127
    mov [cos_y_val], ax

    ; sin_x = sine_table[angle_x] - 127
    mov ax, [angle_x]
    mov si, sine_table
    add si, ax
    mov al, [si]
    xor ah, ah
    sub ax, 127
    mov [sin_x_val], ax

    ; cos_x = sine_table[(angle_x+64)&0xFF] - 127
    mov ax, [angle_x]
    add ax, 64
    and ax, 0xFF
    mov si, sine_table
    add si, ax
    mov al, [si]
    xor ah, ah
    sub ax, 127
    mov [cos_x_val], ax

    mov si, cube_verts
    mov di, screen_verts
    mov bp, 8

tv_loop:
    ; Y rotation: rvx = (vx*cos_y - vz*sin_y) >> 7
    mov ax, [si]
    mov bx, [cos_y_val]
    imul bx
    mov cx, ax

    mov ax, [si+4]
    mov bx, [sin_y_val]
    imul bx
    sub cx, ax
    sar cx, 7
    mov [rvx_val], cx

    ; Y rotation: rvz = (vx*sin_y + vz*cos_y) >> 7
    mov ax, [si]
    mov bx, [sin_y_val]
    imul bx
    mov cx, ax

    mov ax, [si+4]
    mov bx, [cos_y_val]
    imul bx
    add cx, ax
    sar cx, 7
    mov [rvz_val], cx

    ; X rotation: rvy = (vy*cos_x - rvz*sin_x) >> 7
    mov ax, [si+2]
    mov bx, [cos_x_val]
    imul bx
    mov cx, ax

    mov ax, [rvz_val]
    mov bx, [sin_x_val]
    imul bx
    sub cx, ax
    sar cx, 7
    mov [rvy_val], cx

    ; X rotation: rvz2 = (vy*sin_x + rvz*cos_x) >> 7
    mov ax, [si+2]
    mov bx, [sin_x_val]
    imul bx
    mov cx, ax

    mov ax, [rvz_val]
    mov bx, [cos_x_val]
    imul bx
    add cx, ax
    sar cx, 7               ; cx = rvz2

    ; Perspective: depth = rvz2 + 400 (keeps all projections within screen bounds)
    add cx, 400
    mov bx, cx              ; bx = depth (preserved across both projections)

    ; sx = 160 + rvx*200/depth
    mov ax, [rvx_val]
    mov cx, [focal_200]
    imul cx
    idiv bx
    add ax, 160
    mov [ds:di], ax

    ; sy = 100 + rvy*200/depth
    mov ax, [rvy_val]
    mov cx, [focal_200]
    imul cx
    idiv bx
    add ax, 100
    mov [ds:di+2], ax

    add si, 6
    add di, 4
    dec bp
    jnz tv_loop

    pop bp
    pop di
    pop si
    pop dx
    pop cx
    pop bx
    pop ax
    ret


; Draw all 12 edges by looking up screen_verts and calling Bresenham
draw_edges:
    push ax
    push bx
    push cx
    push si
    push di
    push bp

    mov si, cube_edges
    mov bp, 12

de_loop:
    ; Load vertex A index, compute screen_verts[A] address
    xor ah, ah
    mov al, [si]
    inc si
    shl ax, 2
    mov di, screen_verts
    add di, ax
    mov ax, [ds:di]
    mov [bres_x0], ax
    mov ax, [ds:di+2]
    mov [bres_y0], ax

    ; Load vertex B index, compute screen_verts[B] address
    xor ah, ah
    mov al, [si]
    inc si
    shl ax, 2
    mov di, screen_verts
    add di, ax
    mov ax, [ds:di]
    mov [bres_x1], ax
    mov ax, [ds:di+2]
    mov [bres_y1], ax

    call bresenham

    dec bp
    jnz de_loop

    pop bp
    pop di
    pop si
    pop cx
    pop bx
    pop ax
    ret


; Bresenham line drawing
; Reads bres_x0/y0 (start) and bres_x1/y1 (end)
; Draws color 15 (white) pixels to ES:back buffer
bresenham:
    push ax
    push bx
    push cx
    push dx
    push di

    ; dx = abs(x1 - x0)
    mov ax, [bres_x1]
    sub ax, [bres_x0]
    jge bres_dx_pos
    neg ax
bres_dx_pos:
    mov [bres_dx], ax

    ; sx = x0 < x1 ? 1 : -1
    mov ax, [bres_x0]
    cmp ax, [bres_x1]
    jl bres_sx_right
    mov ax, -1
    jmp bres_sx_done
bres_sx_right:
    mov ax, 1
bres_sx_done:
    mov [bres_sx], ax

    ; dy = abs(y1 - y0)
    mov ax, [bres_y1]
    sub ax, [bres_y0]
    jge bres_dy_pos
    neg ax
bres_dy_pos:
    mov [bres_dy], ax

    ; sy = y0 < y1 ? 1 : -1
    mov ax, [bres_y0]
    cmp ax, [bres_y1]
    jl bres_sy_down
    mov ax, -1
    jmp bres_sy_done
bres_sy_down:
    mov ax, 1
bres_sy_done:
    mov [bres_sy], ax

    ; err = dx - dy
    mov ax, [bres_dx]
    sub ax, [bres_dy]
    mov [bres_err], ax

bres_loop:
    ; Bounds check before plotting
    mov ax, [bres_x0]
    cmp ax, 0
    jl bres_skip_plot
    cmp ax, 319
    jg bres_skip_plot
    mov bx, [bres_y0]
    cmp bx, 0
    jl bres_skip_plot
    cmp bx, 199
    jg bres_skip_plot

    ; Pixel offset = y*320 + x
    mov cx, 320
    mov ax, bx
    mul cx
    add ax, [bres_x0]
    mov di, ax
    mov al, 15
    mov [es:di], al

bres_skip_plot:
    ; Done when we reach endpoint
    mov ax, [bres_x0]
    cmp ax, [bres_x1]
    jne bres_continue
    mov ax, [bres_y0]
    cmp ax, [bres_y1]
    je bres_done

bres_continue:
    ; e2 = 2 * err
    mov ax, [bres_err]
    shl ax, 1
    mov [bres_e2], ax

    ; if e2 > -dy: err -= dy, x0 += sx
    mov bx, [bres_dy]
    neg bx
    cmp ax, bx
    jle bres_check_y

    mov ax, [bres_err]
    sub ax, [bres_dy]
    mov [bres_err], ax

    mov ax, [bres_x0]
    add ax, [bres_sx]
    mov [bres_x0], ax

bres_check_y:
    ; if e2 < dx: err += dx, y0 += sy
    mov ax, [bres_e2]
    cmp ax, [bres_dx]
    jge bres_loop

    mov ax, [bres_err]
    add ax, [bres_dx]
    mov [bres_err], ax

    mov ax, [bres_y0]
    add ax, [bres_sy]
    mov [bres_y0], ax

    jmp bres_loop

bres_done:
    pop di
    pop dx
    pop cx
    pop bx
    pop ax
    ret
