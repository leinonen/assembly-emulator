package assembler

import (
	"fmt"
	"strings"
)

// Register classes.
type regClass int

const (
	rcNone regClass = iota
	rcReg8
	rcReg16
	rcReg32
	rcSeg
	rcST
	rcCR
	rcDR
)

type regInfo struct {
	class regClass
	num   int
}

var registers = map[string]regInfo{
	"al": {rcReg8, 0}, "cl": {rcReg8, 1}, "dl": {rcReg8, 2}, "bl": {rcReg8, 3},
	"ah": {rcReg8, 4}, "ch": {rcReg8, 5}, "dh": {rcReg8, 6}, "bh": {rcReg8, 7},
	"ax": {rcReg16, 0}, "cx": {rcReg16, 1}, "dx": {rcReg16, 2}, "bx": {rcReg16, 3},
	"sp": {rcReg16, 4}, "bp": {rcReg16, 5}, "si": {rcReg16, 6}, "di": {rcReg16, 7},
	"eax": {rcReg32, 0}, "ecx": {rcReg32, 1}, "edx": {rcReg32, 2}, "ebx": {rcReg32, 3},
	"esp": {rcReg32, 4}, "ebp": {rcReg32, 5}, "esi": {rcReg32, 6}, "edi": {rcReg32, 7},
	"es": {rcSeg, 0}, "cs": {rcSeg, 1}, "ss": {rcSeg, 2}, "ds": {rcSeg, 3}, "fs": {rcSeg, 4}, "gs": {rcSeg, 5},
	"st0": {rcST, 0}, "st1": {rcST, 1}, "st2": {rcST, 2}, "st3": {rcST, 3},
	"st4": {rcST, 4}, "st5": {rcST, 5}, "st6": {rcST, 6}, "st7": {rcST, 7}, "st": {rcST, 0},
	"cr0": {rcCR, 0}, "cr2": {rcCR, 2}, "cr3": {rcCR, 3}, "cr4": {rcCR, 4},
	"dr0": {rcDR, 0}, "dr1": {rcDR, 1}, "dr2": {rcDR, 2}, "dr3": {rcDR, 3}, "dr6": {rcDR, 6}, "dr7": {rcDR, 7},
}

type operandKind int

const (
	okNone operandKind = iota
	okReg
	okImm
	okMem
	okFarImm // seg:off
)

// operand is a parsed instruction operand.
type operand struct {
	kind operandKind
	reg  regInfo
	size int // explicit size in bytes (byte/word/dword/qword/tword), 0 = none
	// immediate / displacement expression
	expr []token
	// far immediate
	segExpr []token
	// memory
	seg      int // segment override register number or -1
	base     regInfo
	index    regInfo
	scale    int
	hasBase  bool
	hasIndex bool
	addrSize int // 2 or 4, 0 = infer
	// jump distance hints
	short, near, far bool
	strict           bool
	dispSize         int // forced displacement size inside [] (byte/word/dword)
}

type instruction struct {
	mnemonic  string
	prefixes  []string // rep, repe, repne, lock, o16, o32, a16, a32
	segPrefix int
	ops       []operand
}

var sizeWords = map[string]int{"byte": 1, "word": 2, "dword": 4, "qword": 8, "tword": 10, "tbyte": 10, "oword": 16}

var prefixWords = map[string]bool{"rep": true, "repe": true, "repz": true, "repne": true, "repnz": true, "lock": true, "o16": true, "o32": true, "a16": true, "a32": true, "wait": false}

// parseInstruction parses "prefixes mnemonic operands".
func (as *Assembler) parseInstruction(toks []token, l rawLine) (*instruction, error) {
	in := &instruction{segPrefix: -1}
	i := 0
	for i < len(toks) && toks[i].kind == tkIdent {
		w := strings.ToLower(toks[i].text)
		if prefixWords[w] {
			in.prefixes = append(in.prefixes, w)
			i++
			continue
		}
		if r, ok := registers[w]; ok && r.class == rcSeg && i+1 < len(toks) && toks[i+1].kind == tkIdent && isMnemonic(toks[i+1].text) {
			in.segPrefix = r.num
			i++
			continue
		}
		break
	}
	if i >= len(toks) || toks[i].kind != tkIdent {
		return nil, as.errorf(l, "instruction expected")
	}
	in.mnemonic = strings.ToLower(toks[i].text)
	if !isMnemonic(in.mnemonic) {
		return nil, as.errorf(l, "parser: instruction expected (got %q)", toks[i].text)
	}
	i++
	for _, part := range splitCommas(toks[i:]) {
		op, err := as.parseOperand(part, l)
		if err != nil {
			return nil, err
		}
		in.ops = append(in.ops, op)
	}
	return in, nil
}

func (as *Assembler) parseOperand(toks []token, l rawLine) (operand, error) {
	op := operand{seg: -1}
	if len(toks) == 0 {
		return op, as.errorf(l, "empty operand")
	}
	// Leading keywords: sizes, short/near/far, strict, "ptr".
	for len(toks) > 0 && toks[0].kind == tkIdent {
		w := strings.ToLower(toks[0].text)
		if sz, ok := sizeWords[w]; ok {
			op.size = sz
			toks = toks[1:]
			continue
		}
		switch w {
		case "ptr":
			toks = toks[1:]
			continue
		case "short":
			op.short = true
			toks = toks[1:]
			continue
		case "near":
			op.near = true
			toks = toks[1:]
			continue
		case "far":
			op.far = true
			toks = toks[1:]
			continue
		case "strict":
			op.strict = true
			toks = toks[1:]
			continue
		}
		break
	}
	if len(toks) == 0 {
		return op, as.errorf(l, "operand expected")
	}
	// Register?
	if len(toks) == 1 && toks[0].kind == tkIdent {
		if r, ok := registers[strings.ToLower(toks[0].text)]; ok {
			op.kind = okReg
			op.reg = r
			return op, nil
		}
	}
	// st(i) form
	if len(toks) == 4 && toks[0].isIdent("st") && toks[1].is("(") && toks[2].kind == tkNumber && toks[3].is(")") {
		n, _ := parseNumber(toks[2].text)
		op.kind = okReg
		op.reg = regInfo{rcST, int(n)}
		return op, nil
	}
	// Memory: [ ... ] possibly with a segment override prefix "es:[..]"
	if toks[0].kind == tkIdent && len(toks) > 2 && toks[1].is(":") && toks[2].is("[") {
		if r, ok := registers[strings.ToLower(toks[0].text)]; ok && r.class == rcSeg {
			op.seg = r.num
			toks = toks[2:]
		}
	}
	if toks[0].is("[") {
		if !toks[len(toks)-1].is("]") {
			return op, as.errorf(l, "expected ']'")
		}
		return as.parseMemory(op, toks[1:len(toks)-1], l)
	}
	// Far immediate seg:off (two expressions separated by ':').
	depth := 0
	for i, t := range toks {
		if t.is("(") {
			depth++
		}
		if t.is(")") {
			depth--
		}
		if t.is(":") && depth == 0 {
			op.kind = okFarImm
			op.segExpr = toks[:i]
			op.expr = toks[i+1:]
			return op, nil
		}
	}
	op.kind = okImm
	op.expr = toks
	return op, nil
}

// parseMemory parses the inside of [...]: registers, scales and a
// displacement expression.
func (as *Assembler) parseMemory(op operand, toks []token, l rawLine) (operand, error) {
	op.kind = okMem
	// [byte bx+disp] / [word ...] / [dword ...]: displacement size hint.
	for len(toks) > 0 && toks[0].kind == tkIdent {
		if sz, ok := sizeWords[strings.ToLower(toks[0].text)]; ok && sz <= 4 {
			op.dispSize = sz
			toks = toks[1:]
			continue
		}
		break
	}
	// segment override inside brackets: [es:di]
	if len(toks) >= 2 && toks[0].kind == tkIdent && toks[1].is(":") {
		if r, ok := registers[strings.ToLower(toks[0].text)]; ok && r.class == rcSeg {
			op.seg = r.num
			toks = toks[2:]
		}
	}
	// Split into terms at top-level + and -.
	var terms [][]token
	var signs []bool // true = negative
	var cur []token
	depth := 0
	neg := false
	for i, t := range toks {
		if t.is("(") {
			depth++
		}
		if t.is(")") {
			depth--
		}
		if depth == 0 && (t.is("+") || t.is("-")) && i > 0 && !(toks[i-1].kind == tkPunct && toks[i-1].text != ")" && toks[i-1].text != "]") {
			terms = append(terms, cur)
			signs = append(signs, neg)
			cur = nil
			neg = t.is("-")
			continue
		}
		if i == 0 && t.is("-") {
			neg = true
			continue
		}
		if i == 0 && t.is("+") {
			continue
		}
		cur = append(cur, t)
	}
	terms = append(terms, cur)
	signs = append(signs, neg)

	var dispToks []token
	for i, term := range terms {
		if len(term) == 0 {
			continue
		}
		// register term: reg, reg*n, n*reg
		reg, scale, ok := regTerm(term)
		if ok {
			if signs[i] {
				return op, as.errorf(l, "invalid effective address: negative register")
			}
			if scale == 1 && !op.hasBase && (reg.class == rcReg16 || reg.class == rcReg32) {
				op.base = reg
				op.hasBase = true
				continue
			}
			if op.hasIndex {
				// two registers: move base to index if this one has scale 1
				if scale == 1 && !op.hasBase {
					op.base = reg
					op.hasBase = true
					continue
				}
				// e.g. [eax+ebx*2+ecx]: invalid
				if scale == 1 && op.hasBase {
					return op, as.errorf(l, "invalid effective address: too many registers")
				}
				return op, as.errorf(l, "invalid effective address: two scaled registers")
			}
			op.index = reg
			op.scale = scale
			op.hasIndex = true
			continue
		}
		if len(dispToks) > 0 || signs[i] {
			if signs[i] {
				dispToks = append(dispToks, token{kind: tkPunct, text: "-", line: l.line})
			} else {
				dispToks = append(dispToks, token{kind: tkPunct, text: "+", line: l.line})
			}
		}
		dispToks = append(dispToks, token{kind: tkPunct, text: "(", line: l.line})
		dispToks = append(dispToks, term...)
		dispToks = append(dispToks, token{kind: tkPunct, text: ")", line: l.line})
	}
	op.expr = dispToks
	// Normalise 16-bit forms: [si+bx] -> base bx index si etc. handled in encoder.
	if op.hasIndex && !op.hasBase && op.scale == 1 && op.index.class == rcReg16 {
		op.base = op.index
		op.hasBase = true
		op.hasIndex = false
	}
	// Determine address size.
	if op.hasBase || op.hasIndex {
		c := op.base.class
		if !op.hasBase {
			c = op.index.class
		}
		if op.hasBase && op.hasIndex && op.base.class != op.index.class {
			return op, as.errorf(l, "invalid effective address: mixed register sizes")
		}
		if c == rcReg32 {
			op.addrSize = 4
		} else if c == rcReg16 {
			op.addrSize = 2
		} else {
			return op, as.errorf(l, "invalid effective address: bad register")
		}
	}
	return op, nil
}

// regTerm recognises reg, reg*n, n*reg.
func regTerm(term []token) (regInfo, int, bool) {
	if len(term) == 1 && term[0].kind == tkIdent {
		if r, ok := registers[strings.ToLower(term[0].text)]; ok && (r.class == rcReg16 || r.class == rcReg32) {
			return r, 1, true
		}
		return regInfo{}, 0, false
	}
	if len(term) == 3 && term[1].is("*") {
		var rt, nt token
		if term[0].kind == tkIdent && term[2].kind == tkNumber {
			rt, nt = term[0], term[2]
		} else if term[2].kind == tkIdent && term[0].kind == tkNumber {
			rt, nt = term[2], term[0]
		} else {
			return regInfo{}, 0, false
		}
		r, ok := registers[strings.ToLower(rt.text)]
		if !ok || r.class != rcReg32 {
			return regInfo{}, 0, false
		}
		n, err := parseNumber(nt.text)
		if err != nil {
			return regInfo{}, 0, false
		}
		switch n {
		case 1, 2, 4, 8:
			return r, int(n), true
		case 3, 5, 9:
			// reg*3 = reg + reg*2: represent as index with scale n-1 plus base
			return r, int(n), true
		}
	}
	return regInfo{}, 0, false
}

func (o operand) String() string {
	switch o.kind {
	case okReg:
		return fmt.Sprintf("reg(%d,%d)", o.reg.class, o.reg.num)
	case okImm:
		return "imm"
	case okMem:
		return "mem"
	case okFarImm:
		return "seg:off"
	}
	return "?"
}
