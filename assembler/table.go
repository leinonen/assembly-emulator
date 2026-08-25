package assembler

import "strings"

// Operand patterns for instruction forms.
type opPat int

const (
	pNone  opPat = iota
	pRM8         // r/m8
	pRM16        // r/m16
	pRM32        // r/m32
	pRMV         // r/m16 or r/m32 (operand size)
	pR8          // r8
	pR16         // r16
	pR32         // r32
	pRV          // r16/r32
	pM           // memory operand of any size
	pM8          // memory, byte-sized (fpu/movzx)
	pM16         // memory, word
	pM32         // memory, dword
	pM64         // memory, qword
	pM80         // memory, tword
	pMFar        // memory for far jmp/call: "far [x]" or dword [x]
	pMOFFS       // memory with no registers (moffs)
	pI8          // imm8 (any value fitting in 8 bits)
	pIS8         // imm8 sign-extended to operand size (value in -128..127)
	pI16         // imm16 (fixed)
	pIV          // imm16/imm32
	pIMM         // any immediate (size decided elsewhere)
	pONE         // the immediate 1
	pAL
	pAX // ax/eax (accumulator, operand size)
	pCL
	pDX
	pSREG // any segment register
	pSEGES
	pSEGCS
	pSEGSS
	pSEGDS
	pSEGFS
	pSEGGS
	pREL8   // rel8 branch target
	pRELV   // rel16/32 branch target
	pFARIMM // seg:off immediate
	pST0    // st0
	pSTI    // st(i)
	pCR
	pDR
)

// form is one encoding of a mnemonic.
type form struct {
	ops []opPat
	enc string // template tokens
}

func f(enc string, ops ...opPat) form { return form{ops: ops, enc: enc} }

var insnTable = map[string][]form{}

func add(mn string, forms ...form) {
	insnTable[mn] = append(insnTable[mn], forms...)
}

func alias(newName, existing string) { insnTable[newName] = insnTable[existing] }

func isMnemonic(s string) bool {
	_, ok := insnTable[strings.ToLower(s)]
	return ok
}

func init() {
	// ---- ALU: add or adc sbb and sub xor cmp -------------------------------
	alu := []string{"add", "or", "adc", "sbb", "and", "sub", "xor", "cmp"}
	for i, mn := range alu {
		b := byte(i * 8)
		add(mn,
			f(hex(b+0)+" /r", pRM8, pR8),
			f(hex(b+1)+" /r", pRMV, pRV),
			f(hex(b+2)+" /r", pR8, pRM8),
			f(hex(b+3)+" /r", pRV, pRMV),
			f(hex(b+4)+" ib", pAL, pI8),
			f("83 /"+digit(i)+" ibs", pRMV, pIS8),
			f(hex(b+5)+" iv", pAX, pIV),
			f("80 /"+digit(i)+" ib", pRM8, pI8),
			f("81 /"+digit(i)+" iv", pRMV, pIV),
		)
	}
	// ---- test / xchg / mov ----------------------------------------------------
	add("test",
		f("84 /r", pRM8, pR8), f("85 /r", pRMV, pRV),
		f("84 /r", pR8, pRM8), f("85 /r", pRV, pRMV),
		f("A8 ib", pAL, pI8), f("A9 iv", pAX, pIV),
		f("F6 /0 ib", pRM8, pI8), f("F7 /0 iv", pRMV, pIV),
	)
	add("xchg",
		f("90 +r", pAX, pRV), f("90 +r2", pRV, pAX),
		f("86 /r", pR8, pRM8), f("87 /r", pRV, pRMV),
		f("86 /r", pRM8, pR8), f("87 /r", pRMV, pRV),
	)
	add("mov",
		f("A0 moffs", pAL, pMOFFS), f("A1 moffs", pAX, pMOFFS),
		f("A2 moffs", pMOFFS, pAL), f("A3 moffs", pMOFFS, pAX),
		f("88 /r", pRM8, pR8), f("89 /r", pRMV, pRV),
		f("8A /r", pR8, pRM8), f("8B /r", pRV, pRMV),
		f("8C /r", pRM16, pSREG), f("8C /r", pRM32, pSREG), f("8E /r", pSREG, pRM16), f("8E /r", pSREG, pRM32),
		f("B0 +r ib", pR8, pIMM), f("B8 +r iv", pRV, pIMM),
		f("C6 /0 ib", pRM8, pI8), f("C7 /0 iv", pRMV, pIV),
		f("0F 22 /r", pCR, pR32), f("0F 20 /r", pR32, pCR),
		f("0F 23 /r", pDR, pR32), f("0F 21 /r", pR32, pDR),
	)
	// ---- inc/dec/not/neg/mul/div ----------------------------------------------
	add("inc", f("40 +r", pRV), f("FE /0", pRM8), f("FF /0", pRMV))
	add("dec", f("48 +r", pRV), f("FE /1", pRM8), f("FF /1", pRMV))
	add("not", f("F6 /2", pRM8), f("F7 /2", pRMV))
	add("neg", f("F6 /3", pRM8), f("F7 /3", pRMV))
	add("mul", f("F6 /4", pRM8), f("F7 /4", pRMV))
	add("imul",
		f("F6 /5", pRM8), f("F7 /5", pRMV),
		f("6B /r ibs", pRV, pRMV, pIS8), f("69 /r iv", pRV, pRMV, pIV),
		f("6B /r ibs2", pRV, pIS8), f("69 /r iv2", pRV, pIV),
		f("0F AF /r", pRV, pRMV),
	)
	add("div", f("F6 /6", pRM8), f("F7 /6", pRMV))
	add("idiv", f("F6 /7", pRM8), f("F7 /7", pRMV))
	// ---- shifts ---------------------------------------------------------------
	shifts := []string{"rol", "ror", "rcl", "rcr", "shl", "shr", "sal", "sar"}
	for i, mn := range shifts {
		d := digit(i)
		if mn == "sal" {
			d = "4"
		}
		add(mn,
			f("D0 /"+d, pRM8, pONE), f("D1 /"+d, pRMV, pONE),
			f("D2 /"+d, pRM8, pCL), f("D3 /"+d, pRMV, pCL),
			f("C0 /"+d+" ib", pRM8, pI8), f("C1 /"+d+" ib", pRMV, pI8),
		)
	}
	add("shld", f("0F A4 /r ib", pRMV, pRV, pI8), f("0F A5 /r", pRMV, pRV, pCL))
	add("shrd", f("0F AC /r ib", pRMV, pRV, pI8), f("0F AD /r", pRMV, pRV, pCL))
	// ---- push/pop --------------------------------------------------------------
	add("push",
		f("50 +r", pRV),
		f("06", pSEGES), f("0E", pSEGCS), f("16", pSEGSS), f("1E", pSEGDS), f("0F A0", pSEGFS), f("0F A8", pSEGGS),
		f("6A ibs", pIS8), f("68 iv", pIV),
		f("FF /6", pRMV),
	)
	add("pop",
		f("58 +r", pRV),
		f("07", pSEGES), f("17", pSEGSS), f("1F", pSEGDS), f("0F A1", pSEGFS), f("0F A9", pSEGGS),
		f("8F /0", pRMV),
	)
	add("pusha", f("60"))
	add("pushaw", f("60"))
	add("pushad", f("o32 60"))
	add("popa", f("61"))
	add("popaw", f("61"))
	add("popad", f("o32 61"))
	add("pushf", f("9C"))
	add("pushfw", f("9C"))
	add("pushfd", f("o32 9C"))
	add("popf", f("9D"))
	add("popfw", f("9D"))
	add("popfd", f("o32 9D"))
	// ---- flags / misc ------------------------------------------------------------
	single := map[string]string{
		"nop": "90", "hlt": "F4", "cmc": "F5", "clc": "F8", "stc": "F9", "cli": "FA", "sti": "FB", "cld": "FC", "std": "FD",
		"lahf": "9F", "sahf": "9E", "salc": "D6", "xlat": "D7", "xlatb": "D7", "daa": "27", "das": "2F", "aaa": "37", "aas": "3F",
		"cbw": "98", "cwd": "99", "cwde": "o32 98", "cdq": "o32 99", "wait": "9B", "fwait": "9B",
		"leave": "C9", "into": "CE", "int3": "CC", "int1": "F1", "icebp": "F1", "iret": "CF", "iretw": "CF", "iretd": "o32 CF",
		"rdtsc": "0F 31", "cpuid": "0F A2", "ud2": "0F 0B", "clts": "0F 06", "invd": "0F 08", "wbinvd": "0F 09",
		"movsb": "A4", "movsw": "A5", "movsd": "o32 A5", "cmpsb": "A6", "cmpsw": "A7", "cmpsd": "o32 A7",
		"stosb": "AA", "stosw": "AB", "stosd": "o32 AB", "lodsb": "AC", "lodsw": "AD", "lodsd": "o32 AD",
		"scasb": "AE", "scasw": "AF", "scasd": "o32 AF", "insb": "6C", "insw": "6D", "insd": "o32 6D",
		"outsb": "6E", "outsw": "6F", "outsd": "o32 6F",
		"pause": "F3 90", "fninit": "DB E3", "finit": "9B DB E3", "fnclex": "DB E2", "fclex": "9B DB E2",
		"fnop": "D9 D0", "fchs": "D9 E0", "fabs": "D9 E1", "ftst": "D9 E4", "fxam": "D9 E5",
		"fld1": "D9 E8", "fldl2t": "D9 E9", "fldl2e": "D9 EA", "fldpi": "D9 EB", "fldlg2": "D9 EC", "fldln2": "D9 ED", "fldz": "D9 EE",
		"f2xm1": "D9 F0", "fyl2x": "D9 F1", "fptan": "D9 F2", "fpatan": "D9 F3", "fxtract": "D9 F4", "fprem1": "D9 F5",
		"fdecstp": "D9 F6", "fincstp": "D9 F7", "fprem": "D9 F8", "fyl2xp1": "D9 F9", "fsqrt": "D9 FA", "fsincos": "D9 FB",
		"frndint": "D9 FC", "fscale": "D9 FD", "fsin": "D9 FE", "fcos": "D9 FF", "fcompp": "DE D9", "fucompp": "DA E9",
		"fnstsw_ax": "DF E0", "fneni": "DB E0", "fndisi": "DB E1", "fsetpm": "DB E4",
	}
	for mn, enc := range single {
		add(mn, f(enc))
	}
	add("aam", f("D4 0A"), f("D4 ib", pI8))
	add("aad", f("D5 0A"), f("D5 ib", pI8))
	add("int", f("CD ib", pI8))
	add("enter", f("C8 iw16 ib", pI16, pI8))
	add("bound", f("62 /r", pRV, pM))
	add("lea", f("8D /r", pRV, pM))
	add("lds", f("C5 /r", pRV, pM))
	add("les", f("C4 /r", pRV, pM))
	add("lss", f("0F B2 /r", pRV, pM))
	add("lfs", f("0F B4 /r", pRV, pM))
	add("lgs", f("0F B5 /r", pRV, pM))
	add("in", f("E4 ib", pAL, pI8), f("E5 ib", pAX, pI8), f("EC", pAL, pDX), f("ED", pAX, pDX))
	add("out", f("E6 ib", pI8, pAL), f("E7 ib", pI8, pAX), f("EE", pDX, pAL), f("EF", pDX, pAX))
	add("ret", f("C3"), f("C2 iw16", pI16))
	alias("retn", "ret")
	alias("retw", "ret")
	add("retf", f("CB"), f("CA iw16", pI16))
	add("movzx", f("0F B6 /r", pR16, pRM8), f("0F B6 /r", pR32, pRM8), f("0F B7 /r", pR32, pRM16))
	add("movsx", f("0F BE /r", pR16, pRM8), f("0F BE /r", pR32, pRM8), f("0F BF /r", pR32, pRM16))
	add("bt", f("0F A3 /r", pRMV, pRV), f("0F BA /4 ib", pRMV, pI8))
	add("bts", f("0F AB /r", pRMV, pRV), f("0F BA /5 ib", pRMV, pI8))
	add("btr", f("0F B3 /r", pRMV, pRV), f("0F BA /6 ib", pRMV, pI8))
	add("btc", f("0F BB /r", pRMV, pRV), f("0F BA /7 ib", pRMV, pI8))
	add("bsf", f("0F BC /r", pRV, pRMV))
	add("bsr", f("0F BD /r", pRV, pRMV))
	add("bswap", f("0F C8 +r", pR32))
	add("cmpxchg", f("0F B0 /r", pRM8, pR8), f("0F B1 /r", pRMV, pRV))
	add("xadd", f("0F C0 /r", pRM8, pR8), f("0F C1 /r", pRMV, pRV))
	// ---- control flow ----------------------------------------------------------
	add("jmp",
		f("EB rb", pREL8), f("E9 rv", pRELV),
		f("EA fp", pFARIMM),
		f("FF /5", pMFar),
		f("FF /4", pRMV),
	)
	add("call",
		f("E8 rv", pRELV),
		f("9A fp", pFARIMM),
		f("FF /3", pMFar),
		f("FF /2", pRMV),
	)
	ccNames := [][]string{
		{"o"}, {"no"}, {"b", "c", "nae"}, {"ae", "nb", "nc"}, {"e", "z"}, {"ne", "nz"}, {"be", "na"}, {"a", "nbe"},
		{"s"}, {"ns"}, {"p", "pe"}, {"np", "po"}, {"l", "nge"}, {"ge", "nl"}, {"le", "ng"}, {"g", "nle"},
	}
	for cc, names := range ccNames {
		for _, n := range names {
			add("j"+n, f(hex(byte(0x70+cc))+" rb", pREL8), f("0F "+hex(byte(0x80+cc))+" rv", pRELV))
			add("set"+n, f("0F "+hex(byte(0x90+cc))+" /0", pRM8))
			add("cmov"+n, f("0F "+hex(byte(0x40+cc))+" /r", pRV, pRMV))
		}
	}
	add("jcxz", f("E3 rb", pREL8))
	add("jecxz", f("a32 E3 rb", pREL8))
	add("loop", f("E2 rb", pREL8))
	add("loope", f("E1 rb", pREL8))
	add("loopz", f("E1 rb", pREL8))
	add("loopne", f("E0 rb", pREL8))
	add("loopnz", f("E0 rb", pREL8))
	// ---- x87 ------------------------------------------------------------------------
	fpArith := []struct {
		name string
		n    int
		rev  int // /n for the st(i),st0 and pop forms (sub/div are reversed)
	}{{"fadd", 0, 0}, {"fmul", 1, 1}, {"fsub", 4, 5}, {"fsubr", 5, 4}, {"fdiv", 6, 7}, {"fdivr", 7, 6}}
	for _, a := range fpArith {
		add(a.name,
			f("D8 /"+digit(a.n), pM32), f("DC /"+digit(a.n), pM64),
			f("D8 "+hex(byte(0xC0+a.n*8))+" +i", pST0, pSTI),
			f("DC "+hex(byte(0xC0+a.rev*8))+" +i", pSTI, pST0),
			f("D8 "+hex(byte(0xC0+a.n*8))+" +i", pSTI),
		)
		add(a.name+"p",
			f("DE "+hex(byte(0xC0+a.rev*8))+" +i", pSTI, pST0),
			f("DE "+hex(byte(0xC0+a.rev*8))+" +i", pSTI),
			f("DE "+hex(byte(0xC1+a.rev*8))),
		)
		// integer forms
		iname := "fi" + a.name[1:]
		add(iname, f("DA /"+digit(a.n), pM32), f("DE /"+digit(a.n), pM16))
	}
	add("fcom", f("D8 /2", pM32), f("DC /2", pM64), f("D8 D0 +i", pSTI), f("D8 D1"))
	add("fcomp", f("D8 /3", pM32), f("DC /3", pM64), f("D8 D8 +i", pSTI), f("D8 D9"))
	add("ficom", f("DA /2", pM32), f("DE /2", pM16))
	add("ficomp", f("DA /3", pM32), f("DE /3", pM16))
	add("fucom", f("DD E0 +i", pSTI), f("DD E1"))
	add("fucomp", f("DD E8 +i", pSTI), f("DD E9"))
	add("fcomi", f("DB F0 +i", pST0, pSTI), f("DB F0 +i", pSTI))
	add("fcomip", f("DF F0 +i", pST0, pSTI), f("DF F0 +i", pSTI))
	add("fucomi", f("DB E8 +i", pST0, pSTI), f("DB E8 +i", pSTI))
	add("fucomip", f("DF E8 +i", pST0, pSTI), f("DF E8 +i", pSTI))
	add("fld", f("D9 /0", pM32), f("DD /0", pM64), f("DB /5", pM80), f("D9 C0 +i", pSTI))
	add("fst", f("D9 /2", pM32), f("DD /2", pM64), f("DD D0 +i", pSTI))
	add("fstp", f("D9 /3", pM32), f("DD /3", pM64), f("DB /7", pM80), f("DD D8 +i", pSTI))
	add("fild", f("DF /0", pM16), f("DB /0", pM32), f("DF /5", pM64))
	add("fist", f("DF /2", pM16), f("DB /2", pM32))
	add("fistp", f("DF /3", pM16), f("DB /3", pM32), f("DF /7", pM64))
	add("fisttp", f("DF /1", pM16), f("DB /1", pM32), f("DD /1", pM64))
	add("fbld", f("DF /4", pM80))
	add("fbstp", f("DF /6", pM80))
	add("fldcw", f("D9 /5", pM16))
	add("fnstcw", f("D9 /7", pM16))
	add("fstcw", f("9B D9 /7", pM16))
	add("fnstsw", f("DF E0", pAX), f("DD /7", pM16))
	add("fstsw", f("9B DF E0", pAX), f("9B DD /7", pM16))
	add("fldenv", f("D9 /4", pM))
	add("fnstenv", f("D9 /6", pM))
	add("fstenv", f("9B D9 /6", pM))
	add("frstor", f("DD /4", pM))
	add("fnsave", f("DD /6", pM))
	add("fsave", f("9B DD /6", pM))
	add("fxch", f("D9 C8 +i", pSTI), f("D9 C8 +i", pST0, pSTI), f("D9 C8 +i2", pSTI, pST0), f("D9 C9"))
	add("ffree", f("DD C0 +i", pSTI))
	add("ffreep", f("DF C0 +i", pSTI))
	fcmov := map[string]string{"fcmovb": "DA C0", "fcmove": "DA C8", "fcmovbe": "DA D0", "fcmovu": "DA D8",
		"fcmovnb": "DB C0", "fcmovne": "DB C8", "fcmovnbe": "DB D0", "fcmovnu": "DB D8"}
	for mn, enc := range fcmov {
		add(mn, f(enc+" +i", pST0, pSTI), f(enc+" +i", pSTI))
	}
	delete(insnTable, "fnstsw_ax")
}

func hex(b byte) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{digits[b>>4], digits[b&15]})
}

func digit(i int) string { return string(rune('0' + i)) }
