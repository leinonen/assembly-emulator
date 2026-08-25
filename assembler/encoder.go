package assembler

import (
	"fmt"
	"strings"
)

// encoding context for one instruction
type encCtx struct {
	as    *Assembler
	it    *item
	in    *instruction
	frm   *form
	osz   int          // operand size 16/32 (or 8)
	asz   int          // address size 16/32
	vals  []exprResult // evaluated immediates per operand index
	unres bool
}

// sizeInsn matches the instruction against the table and encodes it.
func (as *Assembler) sizeInsn(it *item) (int64, error) {
	in := it.insn
	forms, ok := insnTable[in.mnemonic]
	if !ok {
		return 0, as.errorf(it.line, "unknown instruction %q", in.mnemonic)
	}
	// Evaluate immediates / displacements once.
	vals := make([]exprResult, len(in.ops))
	unres := false
	for i, op := range in.ops {
		if len(op.expr) > 0 {
			r, err := as.evalExpr(op.expr, it.line)
			if err != nil {
				return 0, err
			}
			vals[i] = r
			if r.unres {
				unres = true
			}
		}
	}
	var lastErr error
	for fi := range forms {
		frm := &forms[fi]
		ctx := &encCtx{as: as, it: it, in: in, frm: frm, vals: vals, unres: unres}
		if !ctx.match() {
			continue
		}
		b, err := ctx.encode()
		if err != nil {
			lastErr = err
			continue
		}
		it.bytes = b
		for _, p := range frm.ops {
			if p == pRELV && hasRel8Form(forms) {
				it.nearJump = true
			}
		}
		return int64(len(b)), nil
	}
	if lastErr != nil {
		return 0, lastErr
	}
	if hasRel8Form(forms) && !hasRelVForm(forms) && len(in.ops) == 1 && in.ops[0].kind == okImm {
		return 0, as.errorf(it.line, "short jump is out of range")
	}
	if as.sizeAmbiguous(in) {
		return 0, as.errorf(it.line, "operation size not specified")
	}
	return 0, as.errorf(it.line, "invalid combination of opcode and operands (%s)", in.mnemonic)
}

func hasRelVForm(forms []form) bool {
	for _, f := range forms {
		for _, p := range f.ops {
			if p == pRELV {
				return true
			}
		}
	}
	return false
}

func hasRel8Form(forms []form) bool {
	for _, f := range forms {
		for _, p := range f.ops {
			if p == pREL8 {
				return true
			}
		}
	}
	return false
}

// sizeAmbiguous reports whether all operands are unsized memory/immediates.
func (as *Assembler) sizeAmbiguous(in *instruction) bool {
	if len(in.ops) == 0 {
		return false
	}
	for _, op := range in.ops {
		if op.kind == okReg {
			return false
		}
		if op.size != 0 {
			return false
		}
	}
	return true
}

// operandSize returns the size implied by an operand (0 = unknown).
func opSize(op operand) int {
	switch op.kind {
	case okReg:
		switch op.reg.class {
		case rcReg8:
			return 1
		case rcReg16, rcSeg:
			return 2
		case rcReg32, rcCR, rcDR:
			return 4
		case rcST:
			return 10
		}
	case okMem, okImm:
		return op.size
	}
	return 0
}

// immFits reports whether the value fits the pattern.
func immFits(pat opPat, r exprResult, unresIsBig bool) bool {
	if r.unres {
		// Unknown values are assumed large so later passes can shrink.
		return !unresIsBig
	}
	v := r.val
	switch pat {
	case pI8:
		return v >= -128 && v <= 255
	case pIS8:
		return v >= -128 && v <= 127
	case pI16:
		return v >= -32768 && v <= 65535
	case pIV, pIMM:
		return true
	case pONE:
		return v == 1
	}
	return true
}

// match checks operands against the form and sets osz/asz.
func (c *encCtx) match() bool {
	in := c.in
	frm := c.frm
	if len(frm.ops) != len(in.ops) {
		return false
	}
	// Determine operand size from registers and explicit sizes.
	osz := 0
	sizeFromReg := false
	for i, op := range in.ops {
		p := frm.ops[i]
		s := opSize(op)
		if s == 0 {
			continue
		}
		switch p {
		case pRMV, pRV, pAX, pIV, pIS8, pI8, pI16, pIMM:
			// contributes to operand size (imm sizes don't decide)
			if p == pRMV || p == pRV || p == pAX {
				if op.kind == okReg || op.kind == okMem {
					if osz != 0 && osz != s && op.kind == okReg {
						return false
					}
					if osz == 0 || op.kind == okReg {
						osz = s
					}
					if op.kind == okReg {
						sizeFromReg = true
					}
				}
			}
		}
	}
	// Fixed-size register patterns (movzx/movsx/bswap/...) decide the
	// operand size from the destination operand.
	if len(in.ops) > 0 {
		switch frm.ops[0] {
		case pR32, pRM32:
			osz = 4
		case pR16, pRM16:
			if osz == 0 {
				osz = 2
			}
		}
	}
	// Explicit prefixes o16/o32.
	for _, p := range in.prefixes {
		if p == "o32" {
			osz = 4
		} else if p == "o16" {
			osz = 2
		}
	}
	asz := 0
	for i, op := range in.ops {
		p := frm.ops[i]
		if !c.matchOp(p, op, i, &osz) {
			return false
		}
		if op.kind == okMem && op.addrSize != 0 {
			asz = op.addrSize
		}
	}
	// An unsized memory operand in an r/m slot needs another operand to
	// fix the size (NASM: "operation size not specified").
	for i, op := range in.ops {
		p := frm.ops[i]
		if op.kind == okMem && op.size == 0 && (p == pRM8 || p == pRM16 || p == pRM32 || p == pRMV) {
			ok := false
			for j, o2 := range in.ops {
				if j != i && (o2.kind == okReg || o2.size != 0) {
					ok = true
				}
			}
			if !ok {
				switch in.mnemonic {
				case "jmp", "call", "push", "pop":
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
	}
	// Forms that need an operand size must know it.
	needsSize := false
	for _, p := range frm.ops {
		if p == pRMV || p == pRV || p == pAX || p == pIV || p == pIS8 {
			needsSize = true
		}
	}
	if needsSize && osz == 0 {
		// jmp/call [mem] default to word; push imm defaults to word.
		switch in.mnemonic {
		case "jmp", "call", "push":
			osz = 2
		default:
			hasMemUnsized := false
			for _, op := range in.ops {
				if (op.kind == okMem) && op.size == 0 {
					hasMemUnsized = true
				}
			}
			if hasMemUnsized {
				return false
			}
			osz = 2
		}
	}
	if osz == 0 {
		osz = 2
	}
	_ = sizeFromReg
	c.osz = osz
	if asz == 0 {
		asz = 2
	}
	for _, p := range in.prefixes {
		if p == "a32" {
			asz = 4
		}
	}
	c.asz = asz
	return true
}

func (c *encCtx) matchOp(p opPat, op operand, idx int, osz *int) bool {
	sz := opSize(op)
	isReg := op.kind == okReg
	isMem := op.kind == okMem
	switch p {
	case pRM8:
		if isReg {
			return op.reg.class == rcReg8
		}
		return isMem && (sz == 0 || sz == 1)
	case pRM16:
		if isReg {
			return op.reg.class == rcReg16
		}
		return isMem && (sz == 0 || sz == 2)
	case pRM32:
		if isReg {
			return op.reg.class == rcReg32
		}
		return isMem && (sz == 0 || sz == 4)
	case pRMV:
		if isReg {
			return op.reg.class == rcReg16 || op.reg.class == rcReg32
		}
		if !isMem {
			return false
		}
		if sz == 0 {
			return true
		}
		if sz == 2 || sz == 4 {
			if *osz == 0 {
				*osz = sz
			}
			return *osz == sz
		}
		return false
	case pR8:
		return isReg && op.reg.class == rcReg8
	case pR16:
		return isReg && op.reg.class == rcReg16
	case pR32:
		return isReg && op.reg.class == rcReg32
	case pRV:
		return isReg && (op.reg.class == rcReg16 || op.reg.class == rcReg32)
	case pM:
		return isMem
	case pM8:
		return isMem && (sz == 0 || sz == 1)
	case pM16:
		return isMem && (sz == 0 || sz == 2)
	case pM32:
		return isMem && (sz == 0 || sz == 4)
	case pM64:
		return isMem && (sz == 0 || sz == 8)
	case pM80:
		return isMem && (sz == 0 || sz == 10)
	case pMFar:
		return isMem && op.far
	case pMOFFS:
		return isMem && !op.hasBase && !op.hasIndex && !op.far
	case pI8, pIS8, pI16, pIV, pIMM, pONE:
		if op.kind != okImm {
			return false
		}
		if op.size != 0 {
			// explicit "byte 1" / "word 1"
			switch p {
			case pI8, pIS8:
				if op.size != 1 {
					return false
				}
			case pI16:
				if op.size != 2 {
					return false
				}
			case pIV:
				if op.size != 2 && op.size != 4 {
					return false
				}
				if *osz == 0 {
					*osz = op.size
				}
			}
		}
		if p == pONE && op.size != 0 {
			return false
		}
		// Unknown (forward) values are assumed to fit the smallest form;
		// later passes grow the instruction if needed (only-grow
		// relaxation converges).
		unresBig := false
		return immFits(p, c.vals[idx], unresBig)
	case pAL:
		return isReg && op.reg.class == rcReg8 && op.reg.num == 0
	case pAX:
		return isReg && (op.reg.class == rcReg16 || op.reg.class == rcReg32) && op.reg.num == 0
	case pCL:
		return isReg && op.reg.class == rcReg8 && op.reg.num == 1
	case pDX:
		return isReg && op.reg.class == rcReg16 && op.reg.num == 2
	case pSREG:
		return isReg && op.reg.class == rcSeg
	case pSEGES, pSEGCS, pSEGSS, pSEGDS, pSEGFS, pSEGGS:
		return isReg && op.reg.class == rcSeg && op.reg.num == int(p-pSEGES)
	case pREL8:
		if op.kind != okImm || op.near || op.far {
			return false
		}
		if op.short {
			return true
		}
		if op.size == 2 || op.size == 4 {
			return false
		}
		if c.it.nearJump {
			return false // grew to near in an earlier pass: stay near
		}
		if c.vals[idx].unres {
			return true // assume short until known
		}
		// Estimate: instruction length 2 (jcc/jmp short); jecxz with a32 prefix 3.
		length := int64(2)
		if strings.HasPrefix(c.frm.enc, "a32") {
			length = 3
		}
		disp := c.vals[idx].val - (c.it.addr + length)
		return disp >= -128 && disp <= 127
	case pRELV:
		return op.kind == okImm && !op.short && !op.far
	case pFARIMM:
		return op.kind == okFarImm
	case pST0:
		return isReg && op.reg.class == rcST && op.reg.num == 0
	case pSTI:
		return isReg && op.reg.class == rcST
	case pCR:
		return isReg && op.reg.class == rcCR
	case pDR:
		return isReg && op.reg.class == rcDR
	}
	return false
}

// encode produces the bytes for a matched form.
func (c *encCtx) encode() ([]byte, error) {
	in := c.in
	frm := c.frm
	var out []byte
	// Prefixes: lock/rep, segment, operand size, address size.
	for _, p := range in.prefixes {
		switch p {
		case "lock":
			out = append(out, 0xF0)
		case "rep", "repe", "repz":
			out = append(out, 0xF3)
		case "repne", "repnz":
			out = append(out, 0xF2)
		}
	}
	seg := in.segPrefix
	var memOp *operand
	memIdx := -1
	for i := range in.ops {
		if in.ops[i].kind == okMem {
			memOp = &in.ops[i]
			memIdx = i
			if in.ops[i].seg >= 0 {
				seg = in.ops[i].seg
			}
		}
	}
	if seg >= 0 {
		out = append(out, []byte{0x26, 0x2E, 0x36, 0x3E, 0x64, 0x65}[seg])
	}
	toks := strings.Fields(frm.enc)
	osz := c.osz
	needO32 := false
	for _, t := range toks {
		if t == "o32" {
			needO32 = true
		}
	}
	if osz == 4 {
		needO32 = true
	}
	// Byte-sized instructions and control/debug register moves never need 66.
	if osz == 1 {
		needO32 = false
	}
	for _, p := range frm.ops {
		if p == pCR || p == pDR {
			needO32 = false
		}
	}
	if needO32 {
		out = append(out, 0x66)
	}
	asz := c.asz
	for _, t := range toks {
		if t == "a32" {
			asz = 4
		}
	}
	if memOp != nil && memOp.addrSize == 4 {
		asz = 4
	}
	if asz == 4 {
		out = append(out, 0x67)
	}

	var modrmReg int = -1
	var immIdx = -1
	nextImm := 0
	pendingRel := -1
	opcodeStart := len(out)
	for _, t := range toks {
		switch {
		case t == "o32" || t == "a32":
		case t == "/r":
			// reg field from the register operand, r/m from the other.
			regIdx, rmIdx := -1, -1
			for i, p := range frm.ops {
				switch p {
				case pR8, pR16, pR32, pRV, pSREG, pCR, pDR:
					if regIdx < 0 {
						regIdx = i
					} else {
						rmIdx = i
					}
				case pRM8, pRM16, pRM32, pRMV, pM, pM8, pM16, pM32, pM64, pM80, pMFar:
					rmIdx = i
				}
			}
			if regIdx >= 0 && rmIdx < 0 {
				rmIdx = regIdx // e.g. imul r16, imm: r/m is the same register
			}
			if regIdx < 0 || rmIdx < 0 {
				return nil, c.as.errorf(c.it.line, "internal: bad /r form for %s", in.mnemonic)
			}
			// mov r/m, sreg: sreg is always the reg field
			modrmReg = in.ops[regIdx].reg.num
			b, err := c.modrm(modrmReg, in.ops[rmIdx], rmIdx)
			if err != nil {
				return nil, err
			}
			out = append(out, b...)
		case len(t) == 2 && t[0] == '/':
			d := int(t[1] - '0')
			// r/m operand is the first operand that is rm/mem
			rmIdx := -1
			for i, p := range frm.ops {
				switch p {
				case pRM8, pRM16, pRM32, pRMV, pM, pM8, pM16, pM32, pM64, pM80, pMFar:
					rmIdx = i
				}
				if rmIdx >= 0 {
					break
				}
			}
			if rmIdx < 0 {
				return nil, c.as.errorf(c.it.line, "internal: bad /n form for %s", in.mnemonic)
			}
			b, err := c.modrm(d, in.ops[rmIdx], rmIdx)
			if err != nil {
				return nil, err
			}
			out = append(out, b...)
		case t == "+r" || t == "+r2":
			idx := 0
			if t == "+r2" {
				idx = 1
			}
			// register operand with pRV/pR8/pR32 pattern at idx (or the non-acc one)
			for i, p := range frm.ops {
				if p == pRV || p == pR8 || p == pR32 || p == pR16 {
					idx = i
					break
				}
			}
			out[len(out)-1] += byte(in.ops[idx].reg.num)
		case t == "+i" || t == "+i2":
			idx := -1
			for i, p := range frm.ops {
				if p == pSTI {
					idx = i
				}
			}
			if idx < 0 {
				return nil, c.as.errorf(c.it.line, "internal: +i without st(i)")
			}
			out[len(out)-1] += byte(in.ops[idx].reg.num)
		case t == "ib" || t == "ibs" || t == "ibs2" || t == "iw" || t == "iw16" || t == "id" || t == "iv" || t == "iv2":
			// immediate operand: last imm operand (or the one before for iv2/ibs2)
			immIdx = -1
			for i := nextImm; i < len(in.ops); i++ {
				if in.ops[i].kind == okImm && frm.ops[i] != pREL8 && frm.ops[i] != pRELV {
					immIdx = i
					nextImm = i + 1
					break
				}
			}
			if immIdx < 0 {
				return nil, c.as.errorf(c.it.line, "internal: immediate expected")
			}
			v := c.vals[immIdx].val
			switch t {
			case "ib", "ibs", "ibs2":
				if c.as.final && !c.vals[immIdx].unres && (v < -128 || v > 255) {
					c.as.warnf(c.it.line, "byte value exceeds bounds")
				}
				out = append(out, byte(v))
			case "iw", "iw16":
				out = appendLE(out, uint64(v), 2)
			case "id":
				out = appendLE(out, uint64(v), 4)
			case "iv", "iv2":
				if osz == 4 {
					out = appendLE(out, uint64(v), 4)
				} else {
					if c.as.final && !c.vals[immIdx].unres && (v < -32768 || v > 65535) {
						c.as.warnf(c.it.line, "word value exceeds bounds")
					}
					out = appendLE(out, uint64(v), 2)
				}
			}
		case t == "rb":
			pendingRel = 1
		case t == "rv":
			pendingRel = osz
		case t == "fp":
			// far pointer: offset (operand size) then segment
			idx := 0
			for i, op := range in.ops {
				if op.kind == okFarImm {
					idx = i
				}
			}
			segR, err := c.as.evalExpr(in.ops[idx].segExpr, c.it.line)
			if err != nil {
				return nil, err
			}
			out = appendLE(out, uint64(c.vals[idx].val), osz)
			out = appendLE(out, uint64(segR.val), 2)
		case t == "moffs":
			idx := memIdx
			out = appendLE(out, uint64(c.vals[idx].val), asz)
		default:
			b, err := parseNumber("0x" + t)
			if err != nil {
				return nil, c.as.errorf(c.it.line, "internal: bad template %q", t)
			}
			out = append(out, byte(b))
		}
	}
	_ = opcodeStart
	if pendingRel > 0 {
		idx := -1
		for i, p := range frm.ops {
			if p == pREL8 || p == pRELV {
				idx = i
			}
		}
		target := c.vals[idx].val
		end := c.it.addr + int64(len(out)) + int64(pendingRel)
		disp := target - end
		if pendingRel == 1 {
			if !c.vals[idx].unres && (disp < -128 || disp > 127) {
				return nil, c.as.errorf(c.it.line, "short jump is out of range")
			}
			out = append(out, byte(disp))
		} else {
			out = appendLE(out, uint64(disp), pendingRel)
		}
	}
	return out, nil
}

// modrm encodes ModR/M (+SIB, displacement) for reg field r and operand op.
func (c *encCtx) modrm(r int, op operand, idx int) ([]byte, error) {
	if op.kind == okReg {
		return []byte{byte(0xC0 | r<<3 | op.reg.num)}, nil
	}
	if op.kind != okMem {
		return nil, c.as.errorf(c.it.line, "memory operand expected")
	}
	disp := c.vals[idx]
	dv := disp.val
	if op.addrSize == 4 || (c.asz == 4 && !op.hasBase && !op.hasIndex) {
		return c.modrm32(r, op, dv, disp.unres)
	}
	// 16-bit addressing.
	if !op.hasBase && !op.hasIndex {
		return append([]byte{byte(0x06 | r<<3)}, byte(dv), byte(dv>>8)), nil
	}
	base, index := -1, -1
	if op.hasBase {
		base = op.base.num
	}
	if op.hasIndex {
		if op.scale != 1 {
			return nil, c.as.errorf(c.it.line, "invalid effective address: scale in 16-bit addressing")
		}
		index = op.index.num
	}
	// Normalise: base ∈ {bx,bp}, index ∈ {si,di}.
	isBX := func(n int) bool { return n == 3 || n == 5 }
	isSI := func(n int) bool { return n == 6 || n == 7 }
	if base >= 0 && index >= 0 {
		if isSI(base) && isBX(index) {
			base, index = index, base
		}
		if !isBX(base) || !isSI(index) {
			return nil, c.as.errorf(c.it.line, "invalid effective address")
		}
	} else if base >= 0 && !isBX(base) {
		if isSI(base) {
			index, base = base, -1
		} else {
			return nil, c.as.errorf(c.it.line, "invalid effective address")
		}
	}
	var rm int
	switch {
	case base == 3 && index == 6:
		rm = 0
	case base == 3 && index == 7:
		rm = 1
	case base == 5 && index == 6:
		rm = 2
	case base == 5 && index == 7:
		rm = 3
	case base < 0 && index == 6:
		rm = 4
	case base < 0 && index == 7:
		rm = 5
	case base == 5 && index < 0:
		rm = 6
	case base == 3 && index < 0:
		rm = 7
	default:
		return nil, c.as.errorf(c.it.line, "invalid effective address")
	}
	if op.dispSize == 1 {
		return []byte{byte(0x40 | rm | r<<3), byte(dv)}, nil
	}
	if op.dispSize == 2 {
		return []byte{byte(0x80 | rm | r<<3), byte(dv), byte(dv >> 8)}, nil
	}
	if dv == 0 && !disp.unres && rm != 6 && len(op.expr) == 0 {
		return []byte{byte(rm | r<<3)}, nil
	}
	if dv == 0 && !disp.unres && rm != 6 && !disp.usedLabel {
		return []byte{byte(rm | r<<3)}, nil
	}
	if !disp.unres && dv >= -128 && dv <= 127 {
		return []byte{byte(0x40 | rm | r<<3), byte(dv)}, nil
	}
	return []byte{byte(0x80 | rm | r<<3), byte(dv), byte(dv >> 8)}, nil
}

func (c *encCtx) modrm32(r int, op operand, dv int64, unres bool) ([]byte, error) {
	// [disp32] only
	if !op.hasBase && !op.hasIndex {
		return append([]byte{byte(0x05 | r<<3)}, le32(dv)...), nil
	}
	base := -1
	if op.hasBase {
		base = op.base.num
	}
	index := -1
	scale := 1
	if op.hasIndex {
		index = op.index.num
		scale = op.scale
	}
	// reg*3/5/9 -> base=reg, index=reg*(n-1)
	if index >= 0 && (scale == 3 || scale == 5 || scale == 9) {
		if base >= 0 {
			return nil, c.as.errorf(c.it.line, "invalid effective address")
		}
		base = index
		scale--
	}
	if index == 4 {
		if scale == 1 && base != 4 {
			base, index = index, base
			scale = 1
			if index < 0 {
				index = -1
			}
		} else {
			return nil, c.as.errorf(c.it.line, "invalid effective address: esp cannot be an index")
		}
	}
	scaleBits := map[int]int{1: 0, 2: 1, 4: 2, 8: 3}[scale]
	needSIB := index >= 0 || base == 4
	var mod int
	var dispBytes []byte
	switch {
	case op.dispSize == 1:
		mod = 1
		dispBytes = []byte{byte(dv)}
	case op.dispSize == 4:
		mod = 2
		dispBytes = le32(dv)
	case dv == 0 && !unres && base != 5:
		mod = 0
	case !unres && dv >= -128 && dv <= 127:
		mod = 1
		dispBytes = []byte{byte(dv)}
	default:
		mod = 2
		dispBytes = le32(dv)
	}
	if base < 0 {
		// index only: mod=00 base=101 disp32
		sib := byte(scaleBits<<6 | index<<3 | 5)
		return append([]byte{byte(0x04 | r<<3), sib}, le32(dv)...), nil
	}
	if !needSIB {
		return append([]byte{byte(mod<<6 | r<<3 | base)}, dispBytes...), nil
	}
	idxBits := 4
	if index >= 0 {
		idxBits = index
	}
	sib := byte(scaleBits<<6 | idxBits<<3 | base)
	return append([]byte{byte(mod<<6 | r<<3 | 4), sib}, dispBytes...), nil
}

func le32(v int64) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

func (c *encCtx) String() string {
	return fmt.Sprintf("%s %v", c.in.mnemonic, c.frm.enc)
}
