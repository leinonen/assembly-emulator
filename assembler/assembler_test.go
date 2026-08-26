package assembler

import (
	"bytes"
	"fmt"
	"testing"
)

func asm(t *testing.T, src string) []byte {
	t.Helper()
	b, err := Assemble([]byte(src), Options{Filename: "test.asm"})
	if err != nil {
		t.Fatalf("assemble %q: %v", src, err)
	}
	return b
}

func hexs(b []byte) string {
	s := ""
	for i, x := range b {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("%02X", x)
	}
	return s
}

func TestEncodings(t *testing.T) {
	cases := []struct{ src, want string }{
		{"mov ax,13h", "B8 13 00"},
		{"mov al,3", "B0 03"},
		{"int 10h", "CD 10"},
		{"mov ax,0A000h\nmov es,ax", "B8 00 A0 8E C0"},
		{"xor di,di", "31 FF"},
		{"mov cx,64000", "B9 00 FA"},
		{"rep stosb", "F3 AA"},
		{"mov [es:di],al", "26 88 05"},
		{"mov byte [bx+si+4],7", "C6 40 04 07"},
		{"mov word [bx],1", "C7 07 01 00"},
		{"add ax,1", "83 C0 01"},
		{"add ax,300", "05 2C 01"},
		{"add al,1", "04 01"},
		{"cmp byte [di],0", "80 3D 00"},
		{"inc dx", "42"},
		{"dec byte [bx]", "FE 0F"},
		{"push es\npop ds", "06 1F"},
		{"push 0A000h\npop es", "68 00 A0 07"},
		{"push 5", "6A 05"},
		{"shl ax,1", "D1 E0"},
		{"shr al,cl", "D2 E8"},
		{"shl bx,4", "C1 E3 04"},
		{"ror eax,8", "66 C1 C8 08"},
		{"mov eax,12345678h", "66 B8 78 56 34 12"},
		{"mov [0x1234],ax", "A3 34 12"},
		{"mov al,[0x60]", "A0 60 00"},
		{"mov ax,[bx]", "8B 07"},
		{"mov ax,[bp]", "8B 46 00"},
		{"mov ax,[bp+di-2]", "8B 43 FE"},
		{"mov ax,[si+100h]", "8B 84 00 01"},
		{"mov [di],ax", "89 05"},
		{"lea si,[bx+2]", "8D 77 02"},
		{"in al,dx", "EC"},
		{"in al,60h", "E4 60"},
		{"out dx,al", "EE"},
		{"out 20h,al", "E6 20"},
		{"loop $", "E2 FE"},
		{"jmp $", "EB FE"},
		{"jmp short $", "EB FE"},
		{"jmp near $", "E9 FD FF"},
		{"jz $", "74 FE"},
		{"x: nop\ntimes 200 nop\njnz x", "90 " + repeat("90 ", 200) + "0F 85 33 FF"},
		{"call $", "E8 FD FF"},
		{"ret", "C3"},
		{"ret 4", "C2 04 00"},
		{"retf", "CB"},
		{"iret", "CF"},
		{"jmp 0F000h:0FFF0h", "EA F0 FF 00 F0"},
		{"jmp far [bx]", "FF 2F"},
		{"jmp [bx]", "FF 27"},
		{"call bx", "FF D3"},
		{"enter 20,0", "C8 14 00 00"},
		{"leave", "C9"},
		{"imul ax,181", "69 C0 B5 00"},
		{"imul ax,bx,3", "6B C3 03"},
		{"imul bx", "F7 EB"},
		{"mul cl", "F6 E1"},
		{"div word [bx]", "F7 37"},
		{"idiv bl", "F6 FB"},
		{"movzx ax,al", "0F B6 C0"},
		{"movzx eax,word [si]", "66 0F B7 04"},
		{"movsx bx,byte [di]", "0F BE 1D"},
		{"xchg ax,bx", "93"},
		{"xchg bx,ax", "93"},
		{"xchg bl,cl", "86 D9"},
		{"test al,80h", "A8 80"},
		{"test byte [bx],1", "F6 07 01"},
		{"cbw\ncwd\ncwde\ncdq", "98 99 66 98 66 99"},
		{"pushad\npopad\npusha\npopa", "66 60 66 61 60 61"},
		{"stc\nclc\ncld\nstd\ncli\nsti\nhlt\nnop", "F9 F8 FC FD FA FB F4 90"},
		{"xlat\nxlatb\nsalc\nlahf\nsahf", "D7 D7 D6 9F 9E"},
		{"aam\naad\naam 16\ndaa\ndas", "D4 0A D5 0A D4 10 27 2F"},
		{"int3\ninto\nint 3", "CC CE CD 03"},
		{"bt ax,3\nbts word [bx],cx\nbsr ax,bx", "0F BA E0 03 0F AB 0F 0F BD C3"},
		{"shld ax,bx,4\nshrd cx,dx,cl", "0F A4 D8 04 0F AD D1"},
		{"setz al\nsetnz byte [bx]", "0F 94 C0 0F 95 07"},
		{"bswap eax", "66 0F C8"},
		{"rdtsc\ncpuid", "0F 31 0F A2"},
		{"lds si,[bx]\nles di,[bx+2]\nlss sp,[bx]", "C5 37 C4 7F 02 0F B2 27"},
		{"mov [ebx+ecx*4],eax", "66 67 89 04 8B"},
		{"mov al,[ebx+ecx*4+8]", "67 8A 44 8B 08"},
		{"mov eax,[esi]", "66 67 8B 06"},
		{"add [ebp],ax", "67 01 45 00"},
		{"mov ax,[esp]", "67 8B 04 24"},
		{"mov ax,[ecx*2+eax]", "67 8B 04 48"},
		{"mov eax,[ebx*4]", "66 67 8B 04 9D 00 00 00 00"},
		{"jecxz $", "67 E3 FD"},
		{"jcxz $", "E3 FE"},
		{"a32 rep movsb", "F3 67 A4"},
		{"movsd\nstosd\nlodsd", "66 A5 66 AB 66 AD"},
		{"mov ds,ax\nmov ax,ds\nmov [bx],es\nmov cs:[bx],ax", "8E D8 8C D8 8C 07 2E 89 07"},
		{"finit\nfninit\nfld1\nfldz\nfldpi", "9B DB E3 DB E3 D9 E8 D9 EE D9 EB"},
		{"fld dword [bx]\nfld qword [bx]\nfld tword [bx]\nfld st1", "D9 07 DD 07 DB 2F D9 C1"},
		{"fstp dword [bx]\nfstp st0\nfst st2", "D9 1F DD D8 DD D2"},
		{"fild word [bx]\nfild dword [bx]\nfild qword [bx]", "DF 07 DB 07 DF 2F"},
		{"fistp word [bx]\nfistp dword [bx]\nfist word [bx]", "DF 1F DB 1F DF 17"},
		{"fadd st0,st1\nfadd st1,st0\nfaddp st1,st0\nfaddp\nfadd st2", "D8 C1 DC C1 DE C1 DE C1 D8 C2"},
		{"fsub st1,st0\nfsubr st1,st0\nfsubp st1\nfsubrp st1\nfdivp st1,st0\nfdivrp", "DC E9 DC E1 DE E9 DE E1 DE F9 DE F1"},
		{"fmulp st1,st0\nfmul dword [si]\nfdiv qword [di]", "DE C9 D8 0C DC 35"},
		{"fiadd word [bx]\nfimul dword [bx]\nfidiv word [bx]", "DE 07 DA 0F DE 37"},
		{"fsin\nfcos\nfsqrt\nfabs\nfchs\nfptan\nfpatan\nfxch\nfxch st2", "D9 FE D9 FF D9 FA D9 E1 D9 E0 D9 F2 D9 F3 D9 C9 D9 CA"},
		{"fcompp\nfcomp st1\nfcom qword [bx]\nfnstsw ax\nfstsw ax\nfldcw [bx]\nfnstcw [bx]", "DE D9 D8 D9 DC 17 DF E0 9B DF E0 D9 2F D9 3F"},
		{"fprem\nfrndint\nfscale\nf2xm1\nfyl2x\nfsincos", "D9 F8 D9 FC D9 FD D9 F0 D9 F1 D9 FB"},
		{"fucomi st1\nfcomip st2\nffree st3\nfincstp\nfdecstp", "DB E9 DF F2 DD C3 D9 F7 D9 F6"},
		{"db 1,2,3\ndw 1234h\ndd 12345678h", "01 02 03 34 12 78 56 34 12"},
		{"db 'AB',0\ndb \"C\"", "41 42 00 43"},
		{"dw 'ab'", "61 62"},
		{"dd 1.0\ndq 1.0", "00 00 80 3F 00 00 00 00 00 00 F0 3F"},
		{"dq -2.5\ndd +1.5", "00 00 00 00 00 00 04 C0 00 00 C0 3F"},
		{"msg db 'hi$'\nlen equ $-msg\ndb len", "68 69 24 03"},
		{"org 100h\nstart: mov dx,msg\nmsg db 'x'", "BA 03 01 78"},
		{"times 4 db 0", "00 00 00 00"},
		{"times 3 nop", "90 90 90"},
		{"db 1\nalign 4\ndb 2", "01 90 90 90 02"},
		{"db 1\nalign 4,db 0\ndb 2", "01 00 00 00 02"},
		{"resb 2\ndb 5", "00 00 05"},
		{"%define W 5\nmov ax,W", "B8 05 00"},
		{"%define SQ(x) ((x)*(x))\nmov ax,SQ(3)", "B8 09 00"},
		{"%assign n 1+2\ndb n", "03"},
		{"%macro two 2\ndb %1,%2\n%endmacro\ntwo 4,5", "04 05"},
		{"%macro lab 0\n%%l: db 1\njmp %%l\n%endmacro\nlab\nlab", "01 EB FD 01 EB FD"},
		{"%rep 3\ndb 7\n%endrep", "07 07 07"},
		{"%if 1+1 == 2\ndb 1\n%else\ndb 2\n%endif", "01"},
		{"%ifdef X\ndb 1\n%endif\ndb 2", "02"},
		{"%define X\n%ifdef X\ndb 1\n%endif", "01"},
		{"a: db 0\n.b: db 1\njmp .b\njmp a.b", "00 01 EB FD EB FB"},
		{"mov al,'A'\nmov ax,'AB'", "B0 41 B8 41 42"},
		{"mov ax,(1<<4)|3\nmov bx,-1\nmov cx,~0", "B8 13 00 BB FF FF B9 FF FF"},
		{"mov ax,0x1F\nmov bx,1Fh\nmov cx,$1F\nmov dx,0b101\nmov si,101b\nmov di,17o", "B8 1F 00 BB 1F 00 B9 1F 00 BA 05 00 BE 05 00 BF 0F 00"},
		{"mov al,[cs:bx]\nmov ax,[ss:bp+2]\nmov [fs:di],al", "2E 8A 07 36 8B 46 02 64 88 05"},
		{"section .text\ndb 1\nsection .data\ndb 2\nsection .bss\nbuf resb 10\nsection .text\ndw buf", "01 04 00 02"},
		{"lock inc word [bx]\nrepne scasb\nrepe cmpsb", "F0 FF 07 F2 AE F3 A6"},
		{"cmp al,'a'\njb $\nja $", "3C 61 72 FE 77 FE"},
		{"mov word [bx],0FFFFh\nmov byte [bx],-1", "C7 07 FF FF C6 07 FF"},
		{"sub sp,2\nsbb ax,ax\nadc bx,1\nneg cx\nnot dl", "83 EC 02 19 C0 83 D3 01 F7 D9 F6 D2"},
		{"mov al,byte [bx]\nmov byte al,[bx]", "8A 07 8A 07"},
		{"pushf\npopf\npushfd\npopfd\niretd", "9C 9D 66 9C 66 9D 66 CF"},
		{"mov eax,cr0\nmov cr0,eax", "0F 20 C0 0F 22 C0"},
		{"loopnz $\nloope $", "E0 FE E1 FE"},
		{"xor eax,eax\nadd eax,[bx]\ncmp dword [bx],1", "66 31 C0 66 03 07 66 83 3F 01"},
	}
	for _, tc := range cases {
		got := asm(t, tc.src)
		if hexs(got) != tc.want {
			t.Errorf("%q:\n got %s\nwant %s", tc.src, hexs(got), tc.want)
		}
	}
}

func repeat(s string, n int) string {
	var b bytes.Buffer
	for i := 0; i < n; i++ {
		b.WriteString(s)
	}
	return b.String()
}

func TestErrors(t *testing.T) {
	bad := []string{
		"mov [bx],1",
		"mov ax,bl",
		"jmp short far_away\ntimes 200 nop\nfar_away:",
		"undefined_sym: mov ax,nothere",
		"foo bar",
		"%macro x 1\n",
		"mov ax,[bx+bx]",
		"db 'unterminated",
	}
	for _, src := range bad {
		if _, err := Assemble([]byte(src), Options{}); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestForwardReferenceRelaxation(t *testing.T) {
	// Forward short jump over small block must be 2 bytes; over big block near.
	got := asm(t, "jmp x\ndb 1\nx:")
	if hexs(got) != "EB 01 01" {
		t.Errorf("short forward jmp: %s", hexs(got))
	}
	got = asm(t, "jz x\ntimes 130 nop\nx: nop")
	if got[0] != 0x0F || got[1] != 0x84 {
		t.Errorf("near forward jz: % X", got[:4])
	}
	// add with forward-referenced small constant should shrink to 83 form.
	got = asm(t, "add ax,val\nval equ 3")
	if hexs(got) != "83 C0 03" {
		t.Errorf("forward equ: %s", hexs(got))
	}
}
