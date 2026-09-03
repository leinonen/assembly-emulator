package disasm

import (
	"bytes"
	"strings"
	"testing"

	"assembly-emulator/assembler"
)

func assemble(t *testing.T, line string) []byte {
	t.Helper()
	src := "bits 16\norg 0x100\n" + line + "\n"
	out, err := assembler.Assemble([]byte(src), assembler.Options{Filename: "t.asm"})
	if err != nil {
		t.Fatalf("assemble %q: %v", line, err)
	}
	return out
}

func decode(code []byte, ip uint32) Insn {
	return Decode(func(off uint32) byte {
		i := int(off) - 0x100
		if i < 0 || i >= len(code) {
			return 0
		}
		return code[i]
	}, ip)
}

// TestRoundTrip assembles NASM source with the project assembler,
// disassembles the result, and checks that the text re-assembles to the
// same bytes (and, where given, matches the expected spelling).
func TestRoundTrip(t *testing.T) {
	cases := []struct{ src, want string }{
		{"mov ax,0x1234", "mov ax,0x1234"},
		{"mov al,5", "mov al,0x5"},
		{"mov ah,9", "mov ah,0x9"},
		{"mov ax,[bx+si+0x10]", "mov ax,[bx+si+0x10]"},
		{"mov [bp-2],ax", "mov [bp-0x2],ax"},
		{"mov word [bx],5", "mov word [bx],0x5"},
		{"mov byte [0x1234],0xff", "mov byte [0x1234],0xff"},
		{"mov al,[0x1234]", "mov al,[0x1234]"},
		{"mov [es:di],al", "mov [es:di],al"},
		{"mov ax,[es:0x1234]", "mov ax,[es:0x1234]"},
		{"mov eax,0x12345678", "mov eax,0x12345678"},
		{"mov eax,[bx]", "mov eax,[bx]"},
		{"mov dword [bx],1", "mov dword [bx],0x1"},
		{"mov eax,[ebx+esi*4+0x10]", "mov eax,[ebx+esi*4+0x10]"},
		{"mov ax,[eax]", "mov ax,[eax]"},
		{"mov ax,[esp]", "mov ax,[esp]"},
		{"mov ax,[ebp]", "mov ax,[ebp+0x0]"},
		{"mov ax,[esi*2+0x100]", "mov ax,[esi*2+0x100]"},
		{"mov ds,ax", "mov ds,ax"},
		{"mov ax,es", "mov ax,es"},
		{"add ax,bx", "add ax,bx"},
		{"add ax,5", "add ax,byte +0x5"},
		{"add ax,0x1234", "add ax,0x1234"},
		{"add al,5", "add al,0x5"},
		{"sub word [bx],5", "sub word [bx],byte +0x5"},
		{"cmp byte [si],0x41", "cmp byte [si],0x41"},
		{"xor eax,eax", "xor eax,eax"},
		{"inc word [bx]", "inc word [bx]"},
		{"dec cx", "dec cx"},
		{"push ax", "push ax"},
		{"push es", "push es"},
		{"push fs", "push fs"},
		{"push word 0x1234", "push word 0x1234"},
		{"push byte 5", "push byte +0x5"},
		{"pusha", "pusha"},
		{"pushad", "pushad"},
		{"pop word [bx]", "pop word [bx]"},
		{"shl ax,1", "shl ax,1"},
		{"shr ax,cl", "shr ax,cl"},
		{"sar word [bx],4", "sar word [bx],0x4"},
		{"rol al,3", "rol al,0x3"},
		{"shld ax,bx,4", "shld ax,bx,0x4"},
		{"imul ax,bx,10", "imul ax,bx,byte +0xa"},
		{"imul ax,bx,0x1234", "imul ax,bx,0x1234"},
		{"imul ax,bx", "imul ax,bx"},
		{"mul byte [bx]", "mul byte [bx]"},
		{"div cx", "div cx"},
		{"neg ax", "neg ax"},
		{"not byte [si]", "not byte [si]"},
		{"test al,1", "test al,0x1"},
		{"test word [bx],0x8000", "test word [bx],0x8000"},
		{"xchg ax,bx", "xchg ax,bx"},
		{"xchg bl,[si]", "xchg bl,[si]"},
		{"lea ax,[bx+si]", "lea ax,[bx+si]"},
		{"les di,[bx]", "les di,[bx]"},
		{"lds si,[0x1234]", "lds si,[0x1234]"},
		{"lss sp,[bx]", "lss sp,[bx]"},
		{"movzx ax,byte [bx]", "movzx ax,byte [bx]"},
		{"movzx eax,bx", "movzx eax,bx"},
		{"movsx ax,cl", "movsx ax,cl"},
		{"bt ax,3", "bt ax,0x3"},
		{"bts word [bx],cx", "bts [bx],cx"},
		{"bsf ax,bx", "bsf ax,bx"},
		{"bswap eax", "bswap eax"},
		{"cmpxchg [bx],ax", "cmpxchg [bx],ax"},
		{"xadd [bx],al", "xadd [bx],al"},
		{"setz al", "setz al"},
		{"setnz byte [bx]", "setnz byte [bx]"},
		{"cmovz ax,bx", "cmovz ax,bx"},
		{"jmp short $+5", "jmp short 0x0105"},
		{"jmp near $+0x200", "jmp near 0x0300"},
		{"jmp 0x800:0x100", "jmp 0x0800:0x0100"},
		{"jmp far [bx]", "jmp far [bx]"},
		{"jmp ax", "jmp ax"},
		{"jmp word [bx]", "jmp word [bx]"},
		{"call $+0x100", "call 0x0200"},
		{"call 0x800:0x100", "call 0x0800:0x0100"},
		{"call far [bx]", "call far [bx]"},
		{"call word [bx]", "call word [bx]"},
		{"jz $+3", "jz short 0x0103"},
		{"jnz $-0x10", "jnz short 0x00f0"},
		{"jz near $+0x300", "jz near 0x0400"},
		{"loop $-3", "loop 0x00fd"},
		{"loopne $-3", "loopne 0x00fd"},
		{"jcxz $+2", "jcxz 0x0102"},
		{"jecxz $+3", "jecxz 0x0103"},
		{"ret", "ret"},
		{"ret 4", "ret 0x4"},
		{"retf", "retf"},
		{"iret", "iret"},
		{"int 0x21", "int 0x21"},
		{"int3", "int3"},
		{"into", "into"},
		{"hlt", "hlt"},
		{"nop", "nop"},
		{"cli", "cli"},
		{"sti", "sti"},
		{"cld", "cld"},
		{"cbw", "cbw"},
		{"cwd", "cwd"},
		{"cwde", "cwde"},
		{"cdq", "cdq"},
		{"xlatb", "xlatb"},
		{"in al,0x60", "in al,0x60"},
		{"in al,dx", "in al,dx"},
		{"out 0x20,al", "out 0x20,al"},
		{"out dx,ax", "out dx,ax"},
		{"rep movsb", "rep movsb"},
		{"rep stosw", "rep stosw"},
		{"rep stosd", "rep stosd"},
		{"repe cmpsb", "repe cmpsb"},
		{"repne scasb", "repne scasb"},
		{"lodsb", "lodsb"},
		{"movsw", "movsw"},
		{"es lodsb", "es lodsb"},
		{"lock add [bx],ax", "lock add [bx],ax"},
		{"enter 8,0", "enter 0x8,0x0"},
		{"leave", "leave"},
		{"aam", "aam"},
		{"aad", "aad"},
		{"aam 0x10", "aam 0x10"},
		{"pushf", "pushf"},
		{"popf", "popf"},
		{"sahf", "sahf"},
		{"rdtsc", "rdtsc"},
		{"cpuid", "cpuid"},
		{"bound ax,[bx]", "bound ax,[bx]"},
		{"mov eax,cr0", "mov eax,cr0"},
		{"mov cr0,eax", "mov cr0,eax"},
		// x87
		{"fld dword [bx]", "fld dword [bx]"},
		{"fld qword [si+8]", "fld qword [si+0x8]"},
		{"fld tword [bx]", "fld tword [bx]"},
		{"fld st1", "fld st1"},
		{"fstp st0", "fstp st0"},
		{"fstp qword [bx]", "fstp qword [bx]"},
		{"fild word [bx]", "fild word [bx]"},
		{"fild dword [bx]", "fild dword [bx]"},
		{"fild qword [bx]", "fild qword [bx]"},
		{"fistp word [bx]", "fistp word [bx]"},
		{"fistp dword [bx]", "fistp dword [bx]"},
		{"fistp qword [bx]", "fistp qword [bx]"},
		{"fadd dword [bx]", "fadd dword [bx]"},
		{"fadd qword [bx]", "fadd qword [bx]"},
		{"fadd st0,st1", "fadd st0,st1"},
		{"fadd st1,st0", "fadd st1,st0"},
		{"faddp st1,st0", "faddp st1,st0"},
		{"faddp", "faddp st1,st0"},
		{"fmul st0,st2", "fmul st0,st2"},
		{"fmulp st1,st0", "fmulp st1,st0"},
		{"fsub st0,st1", "fsub st0,st1"},
		{"fsub st1,st0", "fsub st1,st0"},
		{"fsubr st1,st0", "fsubr st1,st0"},
		{"fsubp st1,st0", "fsubp st1,st0"},
		{"fsubrp st1,st0", "fsubrp st1,st0"},
		{"fdiv st0,st1", "fdiv st0,st1"},
		{"fdiv st1,st0", "fdiv st1,st0"},
		{"fdivr st1,st0", "fdivr st1,st0"},
		{"fdivp st1,st0", "fdivp st1,st0"},
		{"fdivrp st1,st0", "fdivrp st1,st0"},
		{"fiadd word [bx]", "fiadd word [bx]"},
		{"fidiv dword [bx]", "fidiv dword [bx]"},
		{"fcom st1", "fcom st1"},
		{"fcomp st1", "fcomp st1"},
		{"fcompp", "fcompp"},
		{"fucompp", "fucompp"},
		{"fucom st1", "fucom st1"},
		{"fcomi st0,st1", "fcomi st0,st1"},
		{"fcomip st0,st1", "fcomip st0,st1"},
		{"fucomi st0,st1", "fucomi st0,st1"},
		{"fucomip st0,st1", "fucomip st0,st1"},
		{"fcmovb st0,st1", "fcmovb st0,st1"},
		{"fcmovne st0,st2", "fcmovne st0,st2"},
		{"fxch st1", "fxch st1"},
		{"fxch", "fxch st1"},
		{"ffree st1", "ffree st1"},
		{"fchs", "fchs"},
		{"fabs", "fabs"},
		{"ftst", "ftst"},
		{"fxam", "fxam"},
		{"fld1", "fld1"},
		{"fldz", "fldz"},
		{"fldpi", "fldpi"},
		{"fsin", "fsin"},
		{"fcos", "fcos"},
		{"fsincos", "fsincos"},
		{"fsqrt", "fsqrt"},
		{"fscale", "fscale"},
		{"fprem", "fprem"},
		{"frndint", "frndint"},
		{"fpatan", "fpatan"},
		{"f2xm1", "f2xm1"},
		{"fyl2x", "fyl2x"},
		{"fldcw [bx]", "fldcw word [bx]"},
		{"fnstcw [bx]", "fnstcw word [bx]"},
		{"fstcw [bx]", "fstcw word [bx]"},
		{"fnstsw ax", "fnstsw ax"},
		{"fstsw ax", "fstsw ax"},
		{"fnstsw [bx]", "fnstsw word [bx]"},
		{"fninit", "fninit"},
		{"finit", "finit"},
		{"fnclex", "fnclex"},
		{"fclex", "fclex"},
		{"fnop", "fnop"},
		{"fwait", "wait"},
		{"fnsave [bx]", "fnsave [bx]"},
		{"frstor [bx]", "frstor [bx]"},
		{"fnstenv [bx]", "fnstenv [bx]"},
		{"fldenv [bx]", "fldenv [bx]"},
	}
	for _, c := range cases {
		code := assemble(t, c.src)
		in := decode(code, 0x100)
		if in.Len() != len(code) {
			t.Errorf("%q: decoded %d bytes of %d (% X) -> %q", c.src, in.Len(), len(code), code, in.Text)
			continue
		}
		if c.want != "" && in.Text != c.want {
			t.Errorf("%q (% X): got %q, want %q", c.src, code, in.Text, c.want)
		}
		again := assemble(t, in.Text)
		if !bytes.Equal(again, code) {
			t.Errorf("%q -> %q re-assembles to % X, want % X", c.src, in.Text, again, code)
		}
	}
}

func TestHints(t *testing.T) {
	check := func(src string, f func(Insn) bool) {
		t.Helper()
		in := decode(assemble(t, src), 0x100)
		if !f(in) {
			t.Errorf("%q: hints %+v", src, in)
		}
	}
	check("call $+0x20", func(i Insn) bool { return i.Call && i.HasTarget && i.Target == 0x120 })
	check("call word [bx]", func(i Insn) bool { return i.Call && !i.HasTarget })
	check("int 0x21", func(i Insn) bool { return i.Int })
	check("loop $-2", func(i Insn) bool { return i.Loop && i.Target == 0xFE })
	check("rep movsb", func(i Insn) bool { return i.RepString })
	check("movsb", func(i Insn) bool { return !i.RepString })
	check("jmp short $+4", func(i Insn) bool { return i.HasTarget && i.Target == 0x104 && !i.Call })
}

func TestUndefined(t *testing.T) {
	in := decode([]byte{0x0F, 0x0C}, 0x100)
	if in.Len() != 1 || !strings.HasPrefix(in.Text, "db ") {
		t.Errorf("undefined opcode: %+v", in)
	}
	// Prefix runs never exceed MaxLen.
	long := bytes.Repeat([]byte{0x66}, 20)
	in = decode(long, 0x100)
	if in.Len() != 1 {
		t.Errorf("prefix run decoded as %d bytes", in.Len())
	}
}
