// Package disasm decodes 386 real-mode machine code (the integer set plus
// x87) into NASM syntax, in the style of ndisasm: hexadecimal numbers with
// a 0x prefix, no space after commas, and explicit size keywords only where
// the operands would otherwise be ambiguous.
package disasm

import (
	"fmt"
	"strings"
)

// Insn is one decoded instruction.
type Insn struct {
	Addr  uint32 // offset of the first byte within the code segment
	Bytes []byte // the encoded instruction (prefixes included)
	Text  string // NASM syntax, e.g. "mov ax,[bx+si+0x10]"

	// Control-flow hints for debuggers.
	Call      bool   // CALL near or far
	Int       bool   // INT n, INT3, INTO, INT1
	Loop      bool   // LOOP, LOOPE, LOOPNE
	RepString bool   // string instruction with a REP/REPE/REPNE prefix
	HasTarget bool   // Target holds the destination of a direct near branch
	Target    uint32 // offset of the branch destination
}

// Len returns the instruction length in bytes.
func (i Insn) Len() int { return len(i.Bytes) }

// MaxLen is the longest instruction the decoder will consume.
const MaxLen = 15

// Decode disassembles the instruction at offset ip. fetch returns the code
// byte at an offset within the code segment; the caller applies CS.
// Undefined opcodes decode as a one-byte "db".
func Decode(fetch func(off uint32) byte, ip uint32) Insn {
	d := &decoder{fetch: fetch, start: ip, ip: ip, opsize: 2, addrsize: 2}
	d.insn.Addr = ip
	ok := d.run()
	if !ok {
		// Undefined: emit the first byte as data.
		b := fetch(ip)
		return Insn{Addr: ip, Bytes: []byte{b}, Text: fmt.Sprintf("db 0x%02x", b)}
	}
	d.insn.Bytes = d.bytes
	return d.insn
}

type decoder struct {
	fetch    func(uint32) byte
	start    uint32
	ip       uint32
	bytes    []byte
	opsize   int // 2 or 4
	addrsize int // 2 or 4
	seg      string
	segUsed  bool
	rep      byte // 0, 0xF2, 0xF3
	repUsed  bool
	lock     bool

	mod, reg, rm byte
	mem          string // memory operand "[...]" when mod != 3

	insn Insn
}

var (
	r8   = [8]string{"al", "cl", "dl", "bl", "ah", "ch", "dh", "bh"}
	r16  = [8]string{"ax", "cx", "dx", "bx", "sp", "bp", "si", "di"}
	r32  = [8]string{"eax", "ecx", "edx", "ebx", "esp", "ebp", "esi", "edi"}
	sreg = [8]string{"es", "cs", "ss", "ds", "fs", "gs", "?", "?"}
	cc   = [16]string{"o", "no", "c", "nc", "z", "nz", "na", "a", "s", "ns", "pe", "po", "l", "nl", "ng", "g"}
	alu  = [8]string{"add", "or", "adc", "sbb", "and", "sub", "xor", "cmp"}
	shft = [8]string{"rol", "ror", "rcl", "rcr", "shl", "shr", "sal", "sar"}
)

func regName(w int, i byte) string {
	switch w {
	case 1:
		return r8[i]
	case 2:
		return r16[i]
	}
	return r32[i]
}

func sizeName(w int) string {
	switch w {
	case 1:
		return "byte"
	case 2:
		return "word"
	case 4:
		return "dword"
	case 8:
		return "qword"
	case 10:
		return "tword"
	}
	return "?"
}

// ---- byte fetching -------------------------------------------------------

func (d *decoder) next() byte {
	b := d.fetch(d.ip)
	d.ip++
	d.bytes = append(d.bytes, b)
	return b
}

func (d *decoder) next16() uint16 {
	lo := uint16(d.next())
	return lo | uint16(d.next())<<8
}

func (d *decoder) next32() uint32 {
	lo := uint32(d.next16())
	return lo | uint32(d.next16())<<16
}

// nextv fetches an operand-size immediate.
func (d *decoder) nextv() uint32 {
	if d.opsize == 4 {
		return d.next32()
	}
	return uint32(d.next16())
}

// nexta fetches an address-size displacement.
func (d *decoder) nexta() uint32 {
	if d.addrsize == 4 {
		return d.next32()
	}
	return uint32(d.next16())
}

// ---- formatting ----------------------------------------------------------

func hexs(v uint64) string { return fmt.Sprintf("0x%x", v) }

// signed formats a displacement as +0xN / -0xN.
func signed(v int64) string {
	if v < 0 {
		return fmt.Sprintf("-0x%x", -v)
	}
	return fmt.Sprintf("+0x%x", v)
}

// sbyte formats a sign-extended imm8 the way ndisasm does.
func sbyte(v int8) string { return "byte " + signed(int64(v)) }

func (d *decoder) imm8() string { return hexs(uint64(d.next())) }
func (d *decoder) immv() string { return hexs(uint64(d.nextv())) }

// ---- ModR/M --------------------------------------------------------------

func (d *decoder) modrm() {
	b := d.next()
	d.mod, d.reg, d.rm = b>>6, (b>>3)&7, b&7
	if d.mod == 3 {
		d.mem = ""
		return
	}
	var sb strings.Builder
	sb.WriteByte('[')
	if d.seg != "" {
		sb.WriteString(d.seg)
		sb.WriteByte(':')
		d.segUsed = true
	}
	if d.addrsize == 2 {
		bases := [8]string{"bx+si", "bx+di", "bp+si", "bp+di", "si", "di", "bp", "bx"}
		switch {
		case d.mod == 0 && d.rm == 6:
			sb.WriteString(hexs(uint64(d.next16())))
		case d.mod == 0:
			sb.WriteString(bases[d.rm])
		case d.mod == 1:
			sb.WriteString(bases[d.rm])
			sb.WriteString(signed(int64(int8(d.next()))))
		default:
			sb.WriteString(bases[d.rm])
			sb.WriteString("+" + hexs(uint64(d.next16())))
		}
	} else {
		var parts []string
		var disp int64
		haveDisp := false
		if d.rm == 4 {
			sib := d.next()
			scale, idx, base := sib>>6, (sib>>3)&7, sib&7
			if base == 5 && d.mod == 0 {
				disp = int64(int32(d.next32()))
				haveDisp = true
			} else {
				parts = append(parts, r32[base])
			}
			if idx != 4 {
				s := r32[idx]
				if scale > 0 {
					s += fmt.Sprintf("*%d", 1<<scale)
				}
				parts = append(parts, s)
			}
		} else if d.rm == 5 && d.mod == 0 {
			disp = int64(int32(d.next32()))
			haveDisp = true
		} else {
			parts = append(parts, r32[d.rm])
		}
		switch d.mod {
		case 1:
			disp = int64(int8(d.next()))
			haveDisp = true
		case 2:
			disp = int64(int32(d.next32()))
			haveDisp = true
		}
		sb.WriteString(strings.Join(parts, "+"))
		if haveDisp {
			if len(parts) == 0 {
				sb.WriteString(hexs(uint64(uint32(disp))))
			} else {
				sb.WriteString(signed(disp))
			}
		}
	}
	sb.WriteByte(']')
	d.mem = sb.String()
}

// rmOp returns the r/m operand without a size keyword.
func (d *decoder) rmOp(w int) string {
	if d.mod == 3 {
		return regName(w, d.rm)
	}
	return d.mem
}

// rmSized returns the r/m operand with a size keyword for memory.
func (d *decoder) rmSized(w int) string {
	if d.mod == 3 {
		return regName(w, d.rm)
	}
	return sizeName(w) + " " + d.mem
}

// memSized returns a memory-only operand (register forms are invalid but
// printed anyway).
func (d *decoder) memSized(w int) string {
	if d.mod == 3 {
		return regName(2, d.rm)
	}
	if w == 0 {
		return d.mem
	}
	return sizeName(w) + " " + d.mem
}

func (d *decoder) regOp(w int) string { return regName(w, d.reg) }

// ---- output helpers ------------------------------------------------------

func (d *decoder) emit(mn string, ops ...string) {
	var sb strings.Builder
	if d.lock {
		sb.WriteString("lock ")
	}
	if d.rep != 0 && !d.repUsed {
		if d.rep == 0xF2 {
			sb.WriteString("repne ")
		} else {
			sb.WriteString("rep ")
		}
	}
	if d.seg != "" && !d.segUsed {
		sb.WriteString(d.seg)
		sb.WriteByte(' ')
	}
	sb.WriteString(mn)
	if len(ops) > 0 {
		sb.WriteByte(' ')
		sb.WriteString(strings.Join(ops, ","))
	}
	d.insn.Text = sb.String()
}

// branch records a direct near branch target.
func (d *decoder) branch(disp int32) string {
	t := d.ip + uint32(disp)
	if d.opsize == 2 {
		t &= 0xFFFF
		d.insn.Target, d.insn.HasTarget = t, true
		return fmt.Sprintf("0x%04x", t)
	}
	d.insn.Target, d.insn.HasTarget = t, true
	return fmt.Sprintf("0x%08x", t)
}

func (d *decoder) rel8() string { return d.branch(int32(int8(d.next()))) }

func (d *decoder) relv() string {
	if d.opsize == 4 {
		return d.branch(int32(d.next32()))
	}
	return d.branch(int32(int16(d.next16())))
}

// farImm formats a seg:off immediate.
func (d *decoder) farImm() string {
	off := d.nextv()
	seg := d.next16()
	if d.opsize == 4 {
		return fmt.Sprintf("0x%04x:0x%08x", seg, off)
	}
	return fmt.Sprintf("0x%04x:0x%04x", seg, off)
}

// moffs formats a direct memory operand (A0-A3).
func (d *decoder) moffs() string {
	off := d.nexta()
	d.segUsed = true
	if d.seg != "" {
		return fmt.Sprintf("[%s:0x%x]", d.seg, off)
	}
	return fmt.Sprintf("[0x%x]", off)
}

// strOp names a string instruction at the operand size, with the REP
// prefix spelt as NASM expects.
func (d *decoder) strOp(base string, sized bool) {
	mn := base
	if sized {
		switch d.opsize {
		case 2:
			mn += "w"
		default:
			mn += "d"
		}
	} else {
		mn += "b"
	}
	if d.rep != 0 {
		d.repUsed = true
		d.insn.RepString = true
		switch {
		case d.rep == 0xF2:
			mn = "repne " + mn
		case base == "cmps" || base == "scas":
			mn = "repe " + mn
		default:
			mn = "rep " + mn
		}
	}
	d.emit(mn)
}

// ---- main decode ---------------------------------------------------------

func (d *decoder) run() bool {
	for {
		if len(d.bytes) >= MaxLen {
			return false
		}
		b := d.next()
		switch b {
		case 0x26:
			d.seg = "es"
		case 0x2E:
			d.seg = "cs"
		case 0x36:
			d.seg = "ss"
		case 0x3E:
			d.seg = "ds"
		case 0x64:
			d.seg = "fs"
		case 0x65:
			d.seg = "gs"
		case 0x66:
			d.opsize = 4
		case 0x67:
			d.addrsize = 4
		case 0xF0:
			d.lock = true
		case 0xF2, 0xF3:
			d.rep = b
		default:
			return d.op1(b)
		}
	}
}

func (d *decoder) op1(op byte) bool {
	w := d.opsize
	switch {
	case op < 0x40 && op&7 < 6:
		mn := alu[op>>3]
		switch op & 7 {
		case 0:
			d.modrm()
			d.emit(mn, d.rmOp(1), d.regOp(1))
		case 1:
			d.modrm()
			d.emit(mn, d.rmOp(w), d.regOp(w))
		case 2:
			d.modrm()
			d.emit(mn, d.regOp(1), d.rmOp(1))
		case 3:
			d.modrm()
			d.emit(mn, d.regOp(w), d.rmOp(w))
		case 4:
			d.emit(mn, "al", d.imm8())
		case 5:
			d.emit(mn, regName(w, 0), d.immv())
		}
	case op == 0x06 || op == 0x0E || op == 0x16 || op == 0x1E:
		d.emit("push", sreg[op>>3])
	case op == 0x07 || op == 0x17 || op == 0x1F:
		d.emit("pop", sreg[op>>3])
	case op == 0x0F:
		return d.op0F()
	case op == 0x27:
		d.emit("daa")
	case op == 0x2F:
		d.emit("das")
	case op == 0x37:
		d.emit("aaa")
	case op == 0x3F:
		d.emit("aas")
	case op < 0x48:
		d.emit("inc", regName(w, op&7))
	case op < 0x50:
		d.emit("dec", regName(w, op&7))
	case op < 0x58:
		d.emit("push", regName(w, op&7))
	case op < 0x60:
		d.emit("pop", regName(w, op&7))
	case op == 0x60:
		if w == 4 {
			d.emit("pushad")
		} else {
			d.emit("pusha")
		}
	case op == 0x61:
		if w == 4 {
			d.emit("popad")
		} else {
			d.emit("popa")
		}
	case op == 0x62:
		d.modrm()
		d.emit("bound", d.regOp(w), d.memSized(0))
	case op == 0x63:
		d.modrm()
		d.emit("arpl", d.rmOp(2), d.regOp(2))
	case op == 0x68:
		d.emit("push", sizeName(w)+" "+d.immv())
	case op == 0x69:
		d.modrm()
		d.emit("imul", d.regOp(w), d.rmOp(w), d.immv())
	case op == 0x6A:
		d.emit("push", sbyte(int8(d.next())))
	case op == 0x6B:
		d.modrm()
		d.emit("imul", d.regOp(w), d.rmOp(w), sbyte(int8(d.next())))
	case op == 0x6C:
		d.strOp("ins", false)
	case op == 0x6D:
		d.strOp("ins", true)
	case op == 0x6E:
		d.strOp("outs", false)
	case op == 0x6F:
		d.strOp("outs", true)
	case op >= 0x70 && op < 0x80:
		d.emit("j"+cc[op&0xF], "short "+d.rel8())
	case op == 0x80 || op == 0x82:
		d.modrm()
		d.emit(alu[d.reg], d.rmSized(1), d.imm8())
	case op == 0x81:
		d.modrm()
		d.emit(alu[d.reg], d.rmSized(w), d.immv())
	case op == 0x83:
		d.modrm()
		d.emit(alu[d.reg], d.rmSized(w), sbyte(int8(d.next())))
	case op == 0x84:
		d.modrm()
		d.emit("test", d.rmOp(1), d.regOp(1))
	case op == 0x85:
		d.modrm()
		d.emit("test", d.rmOp(w), d.regOp(w))
	case op == 0x86:
		d.modrm()
		d.emit("xchg", d.regOp(1), d.rmOp(1))
	case op == 0x87:
		d.modrm()
		d.emit("xchg", d.regOp(w), d.rmOp(w))
	case op == 0x88:
		d.modrm()
		d.emit("mov", d.rmOp(1), d.regOp(1))
	case op == 0x89:
		d.modrm()
		d.emit("mov", d.rmOp(w), d.regOp(w))
	case op == 0x8A:
		d.modrm()
		d.emit("mov", d.regOp(1), d.rmOp(1))
	case op == 0x8B:
		d.modrm()
		d.emit("mov", d.regOp(w), d.rmOp(w))
	case op == 0x8C:
		d.modrm()
		if d.reg >= 6 {
			return false
		}
		d.emit("mov", d.rmOp(w), sreg[d.reg])
	case op == 0x8D:
		d.modrm()
		d.emit("lea", d.regOp(w), d.memSized(0))
	case op == 0x8E:
		d.modrm()
		if d.reg >= 6 || d.reg == 1 {
			return false
		}
		d.emit("mov", sreg[d.reg], d.rmOp(2))
	case op == 0x8F:
		d.modrm()
		if d.reg != 0 {
			return false
		}
		d.emit("pop", d.rmSized(w))
	case op == 0x90:
		if d.rep == 0xF3 {
			d.repUsed = true
			d.emit("pause")
		} else {
			d.emit("nop")
		}
	case op < 0x98:
		d.emit("xchg", regName(w, 0), regName(w, op&7))
	case op == 0x98:
		if w == 4 {
			d.emit("cwde")
		} else {
			d.emit("cbw")
		}
	case op == 0x99:
		if w == 4 {
			d.emit("cdq")
		} else {
			d.emit("cwd")
		}
	case op == 0x9A:
		d.insn.Call = true
		d.emit("call", d.farImm())
	case op == 0x9B:
		d.fwait()
	case op == 0x9C:
		if w == 4 {
			d.emit("pushfd")
		} else {
			d.emit("pushf")
		}
	case op == 0x9D:
		if w == 4 {
			d.emit("popfd")
		} else {
			d.emit("popf")
		}
	case op == 0x9E:
		d.emit("sahf")
	case op == 0x9F:
		d.emit("lahf")
	case op == 0xA0:
		d.emit("mov", "al", d.moffs())
	case op == 0xA1:
		d.emit("mov", regName(w, 0), d.moffs())
	case op == 0xA2:
		d.emit("mov", d.moffs(), "al")
	case op == 0xA3:
		d.emit("mov", d.moffs(), regName(w, 0))
	case op == 0xA4:
		d.strOp("movs", false)
	case op == 0xA5:
		d.strOp("movs", true)
	case op == 0xA6:
		d.strOp("cmps", false)
	case op == 0xA7:
		d.strOp("cmps", true)
	case op == 0xA8:
		d.emit("test", "al", d.imm8())
	case op == 0xA9:
		d.emit("test", regName(w, 0), d.immv())
	case op == 0xAA:
		d.strOp("stos", false)
	case op == 0xAB:
		d.strOp("stos", true)
	case op == 0xAC:
		d.strOp("lods", false)
	case op == 0xAD:
		d.strOp("lods", true)
	case op == 0xAE:
		d.strOp("scas", false)
	case op == 0xAF:
		d.strOp("scas", true)
	case op < 0xB8:
		d.emit("mov", r8[op&7], d.imm8())
	case op < 0xC0:
		d.emit("mov", regName(w, op&7), d.immv())
	case op == 0xC0:
		d.modrm()
		d.emit(shft[d.reg], d.rmSized(1), d.imm8())
	case op == 0xC1:
		d.modrm()
		d.emit(shft[d.reg], d.rmSized(w), d.imm8())
	case op == 0xC2:
		d.emit("ret", hexs(uint64(d.next16())))
	case op == 0xC3:
		d.emit("ret")
	case op == 0xC4:
		d.modrm()
		d.emit("les", d.regOp(w), d.memSized(0))
	case op == 0xC5:
		d.modrm()
		d.emit("lds", d.regOp(w), d.memSized(0))
	case op == 0xC6:
		d.modrm()
		if d.reg != 0 {
			return false
		}
		d.emit("mov", d.rmSized(1), d.imm8())
	case op == 0xC7:
		d.modrm()
		if d.reg != 0 {
			return false
		}
		d.emit("mov", d.rmSized(w), d.immv())
	case op == 0xC8:
		size := d.next16()
		d.emit("enter", hexs(uint64(size)), d.imm8())
	case op == 0xC9:
		d.emit("leave")
	case op == 0xCA:
		d.emit("retf", hexs(uint64(d.next16())))
	case op == 0xCB:
		d.emit("retf")
	case op == 0xCC:
		d.insn.Int = true
		d.emit("int3")
	case op == 0xCD:
		d.insn.Int = true
		d.emit("int", d.imm8())
	case op == 0xCE:
		d.insn.Int = true
		d.emit("into")
	case op == 0xCF:
		if w == 4 {
			d.emit("iretd")
		} else {
			d.emit("iret")
		}
	case op == 0xD0:
		d.modrm()
		d.emit(shft[d.reg], d.rmSized(1), "1")
	case op == 0xD1:
		d.modrm()
		d.emit(shft[d.reg], d.rmSized(w), "1")
	case op == 0xD2:
		d.modrm()
		d.emit(shft[d.reg], d.rmSized(1), "cl")
	case op == 0xD3:
		d.modrm()
		d.emit(shft[d.reg], d.rmSized(w), "cl")
	case op == 0xD4:
		if b := d.next(); b == 10 {
			d.emit("aam")
		} else {
			d.emit("aam", hexs(uint64(b)))
		}
	case op == 0xD5:
		if b := d.next(); b == 10 {
			d.emit("aad")
		} else {
			d.emit("aad", hexs(uint64(b)))
		}
	case op == 0xD6:
		d.emit("salc")
	case op == 0xD7:
		d.emit("xlatb")
	case op >= 0xD8 && op <= 0xDF:
		return d.fpu(op)
	case op == 0xE0:
		d.insn.Loop = true
		d.emit("loopne", d.rel8())
	case op == 0xE1:
		d.insn.Loop = true
		d.emit("loope", d.rel8())
	case op == 0xE2:
		d.insn.Loop = true
		d.emit("loop", d.rel8())
	case op == 0xE3:
		if d.addrsize == 4 {
			d.emit("jecxz", d.rel8())
		} else {
			d.emit("jcxz", d.rel8())
		}
	case op == 0xE4:
		d.emit("in", "al", d.imm8())
	case op == 0xE5:
		d.emit("in", regName(w, 0), d.imm8())
	case op == 0xE6:
		d.emit("out", d.imm8(), "al")
	case op == 0xE7:
		d.emit("out", d.imm8(), regName(w, 0))
	case op == 0xE8:
		d.insn.Call = true
		d.emit("call", d.relv())
	case op == 0xE9:
		d.emit("jmp", "near "+d.relv())
	case op == 0xEA:
		d.emit("jmp", d.farImm())
	case op == 0xEB:
		d.emit("jmp", "short "+d.rel8())
	case op == 0xEC:
		d.emit("in", "al", "dx")
	case op == 0xED:
		d.emit("in", regName(w, 0), "dx")
	case op == 0xEE:
		d.emit("out", "dx", "al")
	case op == 0xEF:
		d.emit("out", "dx", regName(w, 0))
	case op == 0xF1:
		d.insn.Int = true
		d.emit("int1")
	case op == 0xF4:
		d.emit("hlt")
	case op == 0xF5:
		d.emit("cmc")
	case op == 0xF6 || op == 0xF7:
		ww := w
		if op == 0xF6 {
			ww = 1
		}
		d.modrm()
		switch d.reg {
		case 0, 1:
			if ww == 1 {
				d.emit("test", d.rmSized(1), d.imm8())
			} else {
				d.emit("test", d.rmSized(ww), d.immv())
			}
		case 2:
			d.emit("not", d.rmSized(ww))
		case 3:
			d.emit("neg", d.rmSized(ww))
		case 4:
			d.emit("mul", d.rmSized(ww))
		case 5:
			d.emit("imul", d.rmSized(ww))
		case 6:
			d.emit("div", d.rmSized(ww))
		case 7:
			d.emit("idiv", d.rmSized(ww))
		}
	case op == 0xF8:
		d.emit("clc")
	case op == 0xF9:
		d.emit("stc")
	case op == 0xFA:
		d.emit("cli")
	case op == 0xFB:
		d.emit("sti")
	case op == 0xFC:
		d.emit("cld")
	case op == 0xFD:
		d.emit("std")
	case op == 0xFE:
		d.modrm()
		switch d.reg {
		case 0:
			d.emit("inc", d.rmSized(1))
		case 1:
			d.emit("dec", d.rmSized(1))
		default:
			return false
		}
	case op == 0xFF:
		d.modrm()
		switch d.reg {
		case 0:
			d.emit("inc", d.rmSized(w))
		case 1:
			d.emit("dec", d.rmSized(w))
		case 2:
			d.insn.Call = true
			d.emit("call", d.rmSized(w))
		case 3:
			d.insn.Call = true
			d.emit("call", "far "+d.memSized(0))
		case 4:
			d.emit("jmp", d.rmSized(w))
		case 5:
			d.emit("jmp", "far "+d.memSized(0))
		case 6:
			d.emit("push", d.rmSized(w))
		default:
			return false
		}
	default:
		return false
	}
	return true
}

func (d *decoder) op0F() bool {
	w := d.opsize
	op := d.next()
	switch {
	case op == 0x00:
		d.modrm()
		names := [8]string{"sldt", "str", "lldt", "ltr", "verr", "verw", "", ""}
		if names[d.reg] == "" {
			return false
		}
		d.emit(names[d.reg], d.rmSized(2))
	case op == 0x01:
		d.modrm()
		switch d.reg {
		case 0:
			d.emit("sgdt", d.memSized(0))
		case 1:
			d.emit("sidt", d.memSized(0))
		case 2:
			d.emit("lgdt", d.memSized(0))
		case 3:
			d.emit("lidt", d.memSized(0))
		case 4:
			d.emit("smsw", d.rmSized(2))
		case 6:
			d.emit("lmsw", d.rmSized(2))
		case 7:
			d.emit("invlpg", d.memSized(0))
		default:
			return false
		}
	case op == 0x02:
		d.modrm()
		d.emit("lar", d.regOp(w), d.rmOp(w))
	case op == 0x03:
		d.modrm()
		d.emit("lsl", d.regOp(w), d.rmOp(w))
	case op == 0x06:
		d.emit("clts")
	case op == 0x08:
		d.emit("invd")
	case op == 0x09:
		d.emit("wbinvd")
	case op == 0x0B:
		d.emit("ud2")
	case op == 0x1F:
		d.modrm()
		d.emit("nop", d.rmSized(w))
	case op == 0x20:
		d.modrm()
		d.emit("mov", r32[d.rm], fmt.Sprintf("cr%d", d.reg))
	case op == 0x21:
		d.modrm()
		d.emit("mov", r32[d.rm], fmt.Sprintf("dr%d", d.reg))
	case op == 0x22:
		d.modrm()
		d.emit("mov", fmt.Sprintf("cr%d", d.reg), r32[d.rm])
	case op == 0x23:
		d.modrm()
		d.emit("mov", fmt.Sprintf("dr%d", d.reg), r32[d.rm])
	case op == 0x31:
		d.emit("rdtsc")
	case op >= 0x40 && op < 0x50:
		d.modrm()
		d.emit("cmov"+cc[op&0xF], d.regOp(w), d.rmOp(w))
	case op >= 0x80 && op < 0x90:
		d.emit("j"+cc[op&0xF], "near "+d.relv())
	case op >= 0x90 && op < 0xA0:
		d.modrm()
		d.emit("set"+cc[op&0xF], d.rmSized(1))
	case op == 0xA0:
		d.emit("push", "fs")
	case op == 0xA1:
		d.emit("pop", "fs")
	case op == 0xA2:
		d.emit("cpuid")
	case op == 0xA3:
		d.modrm()
		d.emit("bt", d.rmOp(w), d.regOp(w))
	case op == 0xA4:
		d.modrm()
		d.emit("shld", d.rmOp(w), d.regOp(w), d.imm8())
	case op == 0xA5:
		d.modrm()
		d.emit("shld", d.rmOp(w), d.regOp(w), "cl")
	case op == 0xA8:
		d.emit("push", "gs")
	case op == 0xA9:
		d.emit("pop", "gs")
	case op == 0xAB:
		d.modrm()
		d.emit("bts", d.rmOp(w), d.regOp(w))
	case op == 0xAC:
		d.modrm()
		d.emit("shrd", d.rmOp(w), d.regOp(w), d.imm8())
	case op == 0xAD:
		d.modrm()
		d.emit("shrd", d.rmOp(w), d.regOp(w), "cl")
	case op == 0xAF:
		d.modrm()
		d.emit("imul", d.regOp(w), d.rmOp(w))
	case op == 0xB0:
		d.modrm()
		d.emit("cmpxchg", d.rmOp(1), d.regOp(1))
	case op == 0xB1:
		d.modrm()
		d.emit("cmpxchg", d.rmOp(w), d.regOp(w))
	case op == 0xB2:
		d.modrm()
		d.emit("lss", d.regOp(w), d.memSized(0))
	case op == 0xB3:
		d.modrm()
		d.emit("btr", d.rmOp(w), d.regOp(w))
	case op == 0xB4:
		d.modrm()
		d.emit("lfs", d.regOp(w), d.memSized(0))
	case op == 0xB5:
		d.modrm()
		d.emit("lgs", d.regOp(w), d.memSized(0))
	case op == 0xB6:
		d.modrm()
		d.emit("movzx", d.regOp(w), d.rmSized(1))
	case op == 0xB7:
		d.modrm()
		d.emit("movzx", d.regOp(w), d.rmSized(2))
	case op == 0xBA:
		d.modrm()
		names := [8]string{"", "", "", "", "bt", "bts", "btr", "btc"}
		if names[d.reg] == "" {
			return false
		}
		d.emit(names[d.reg], d.rmSized(w), d.imm8())
	case op == 0xBB:
		d.modrm()
		d.emit("btc", d.rmOp(w), d.regOp(w))
	case op == 0xBC:
		d.modrm()
		d.emit("bsf", d.regOp(w), d.rmOp(w))
	case op == 0xBD:
		d.modrm()
		d.emit("bsr", d.regOp(w), d.rmOp(w))
	case op == 0xBE:
		d.modrm()
		d.emit("movsx", d.regOp(w), d.rmSized(1))
	case op == 0xBF:
		d.modrm()
		d.emit("movsx", d.regOp(w), d.rmSized(2))
	case op == 0xC0:
		d.modrm()
		d.emit("xadd", d.rmOp(1), d.regOp(1))
	case op == 0xC1:
		d.modrm()
		d.emit("xadd", d.rmOp(w), d.regOp(w))
	case op >= 0xC8:
		d.emit("bswap", r32[op&7])
	default:
		return false
	}
	return true
}

// fwait decodes 9B, folding it into the waiting forms of the x87 control
// instructions (finit, fstsw, ...) when one follows.
func (d *decoder) fwait() {
	save := len(d.bytes)
	rewind := func() {
		d.bytes = d.bytes[:save]
		d.ip = d.start + uint32(save)
	}
	b := d.next()
	switch b {
	case 0xD9, 0xDB, 0xDD, 0xDF:
		d.modrm()
		switch {
		case b == 0xDB && d.mod == 3 && d.rm == 3 && d.reg == 4:
			d.emit("finit")
			return
		case b == 0xDB && d.mod == 3 && d.rm == 2 && d.reg == 4:
			d.emit("fclex")
			return
		case b == 0xD9 && d.mod != 3 && d.reg == 7:
			d.emit("fstcw", d.memSized(2))
			return
		case b == 0xD9 && d.mod != 3 && d.reg == 6:
			d.emit("fstenv", d.memSized(0))
			return
		case b == 0xDD && d.mod != 3 && d.reg == 7:
			d.emit("fstsw", d.memSized(2))
			return
		case b == 0xDD && d.mod != 3 && d.reg == 6:
			d.emit("fsave", d.memSized(0))
			return
		case b == 0xDF && d.mod == 3 && d.rm == 0 && d.reg == 4:
			d.emit("fstsw", "ax")
			return
		}
	}
	rewind()
	d.emit("wait")
}

func (d *decoder) fpu(op byte) bool {
	d.modrm()
	st := func(i byte) string { return fmt.Sprintf("st%d", i) }
	if d.mod != 3 {
		var mn string
		var size int
		arith := [8]string{"fadd", "fmul", "fcom", "fcomp", "fsub", "fsubr", "fdiv", "fdivr"}
		iarith := [8]string{"fiadd", "fimul", "ficom", "ficomp", "fisub", "fisubr", "fidiv", "fidivr"}
		switch op {
		case 0xD8:
			mn, size = arith[d.reg], 4
		case 0xDC:
			mn, size = arith[d.reg], 8
		case 0xDA:
			mn, size = iarith[d.reg], 4
		case 0xDE:
			mn, size = iarith[d.reg], 2
		case 0xD9:
			switch d.reg {
			case 0:
				mn, size = "fld", 4
			case 2:
				mn, size = "fst", 4
			case 3:
				mn, size = "fstp", 4
			case 4:
				mn = "fldenv"
			case 5:
				mn, size = "fldcw", 2
			case 6:
				mn = "fnstenv"
			case 7:
				mn, size = "fnstcw", 2
			}
		case 0xDB:
			switch d.reg {
			case 0:
				mn, size = "fild", 4
			case 1:
				mn, size = "fisttp", 4
			case 2:
				mn, size = "fist", 4
			case 3:
				mn, size = "fistp", 4
			case 5:
				mn, size = "fld", 10
			case 7:
				mn, size = "fstp", 10
			}
		case 0xDD:
			switch d.reg {
			case 0:
				mn, size = "fld", 8
			case 1:
				mn, size = "fisttp", 8
			case 2:
				mn, size = "fst", 8
			case 3:
				mn, size = "fstp", 8
			case 4:
				mn = "frstor"
			case 6:
				mn = "fnsave"
			case 7:
				mn, size = "fnstsw", 2
			}
		case 0xDF:
			switch d.reg {
			case 0:
				mn, size = "fild", 2
			case 1:
				mn, size = "fisttp", 2
			case 2:
				mn, size = "fist", 2
			case 3:
				mn, size = "fistp", 2
			case 4:
				mn, size = "fbld", 10
			case 5:
				mn, size = "fild", 8
			case 6:
				mn, size = "fbstp", 10
			case 7:
				mn, size = "fistp", 8
			}
		}
		if mn == "" {
			return false
		}
		d.emit(mn, d.memSized(size))
		return true
	}
	i := d.rm
	switch op {
	case 0xD8:
		names := [8]string{"fadd", "fmul", "fcom", "fcomp", "fsub", "fsubr", "fdiv", "fdivr"}
		if d.reg == 2 || d.reg == 3 {
			d.emit(names[d.reg], st(i))
		} else {
			d.emit(names[d.reg], "st0", st(i))
		}
	case 0xD9:
		switch d.reg {
		case 0:
			d.emit("fld", st(i))
		case 1:
			d.emit("fxch", st(i))
		case 2:
			if i != 0 {
				return false
			}
			d.emit("fnop")
		case 4:
			names := [8]string{"fchs", "fabs", "", "", "ftst", "fxam", "", ""}
			if names[i] == "" {
				return false
			}
			d.emit(names[i])
		case 5:
			names := [8]string{"fld1", "fldl2t", "fldl2e", "fldpi", "fldlg2", "fldln2", "fldz", ""}
			if names[i] == "" {
				return false
			}
			d.emit(names[i])
		case 6:
			d.emit([8]string{"f2xm1", "fyl2x", "fptan", "fpatan", "fxtract", "fprem1", "fdecstp", "fincstp"}[i])
		case 7:
			d.emit([8]string{"fprem", "fyl2xp1", "fsqrt", "fsincos", "frndint", "fscale", "fsin", "fcos"}[i])
		default:
			return false
		}
	case 0xDA:
		switch d.reg {
		case 0, 1, 2, 3:
			d.emit([4]string{"fcmovb", "fcmove", "fcmovbe", "fcmovu"}[d.reg], "st0", st(i))
		case 5:
			if i != 1 {
				return false
			}
			d.emit("fucompp")
		default:
			return false
		}
	case 0xDB:
		switch d.reg {
		case 0, 1, 2, 3:
			d.emit([4]string{"fcmovnb", "fcmovne", "fcmovnbe", "fcmovnu"}[d.reg], "st0", st(i))
		case 4:
			names := [8]string{"fneni", "fndisi", "fnclex", "fninit", "fsetpm", "", "", ""}
			if names[i] == "" {
				return false
			}
			d.emit(names[i])
		case 5:
			d.emit("fucomi", "st0", st(i))
		case 6:
			d.emit("fcomi", "st0", st(i))
		default:
			return false
		}
	case 0xDC:
		names := [8]string{"fadd", "fmul", "fcom", "fcomp", "fsubr", "fsub", "fdivr", "fdiv"}
		if d.reg == 2 || d.reg == 3 {
			d.emit(names[d.reg], st(i))
		} else {
			d.emit(names[d.reg], st(i), "st0")
		}
	case 0xDD:
		switch d.reg {
		case 0:
			d.emit("ffree", st(i))
		case 2:
			d.emit("fst", st(i))
		case 3:
			d.emit("fstp", st(i))
		case 4:
			d.emit("fucom", st(i))
		case 5:
			d.emit("fucomp", st(i))
		default:
			return false
		}
	case 0xDE:
		names := [8]string{"faddp", "fmulp", "", "", "fsubrp", "fsubp", "fdivrp", "fdivp"}
		if d.reg == 3 {
			if i != 1 {
				return false
			}
			d.emit("fcompp")
			break
		}
		if names[d.reg] == "" {
			return false
		}
		d.emit(names[d.reg], st(i), "st0")
	case 0xDF:
		switch d.reg {
		case 0:
			d.emit("ffreep", st(i))
		case 4:
			if i != 0 {
				return false
			}
			d.emit("fnstsw", "ax")
		case 5:
			d.emit("fucomip", "st0", st(i))
		case 6:
			d.emit("fcomip", "st0", st(i))
		default:
			return false
		}
	}
	return true
}
