package assembler

import (
	"fmt"
	"strings"
)

// Expression evaluation with NASM precedence. Symbols are resolved against
// the assembler's symbol table; undefined symbols make the result
// "unresolved" (value 0) so that earlier passes can proceed.

type exprParser struct {
	as        *Assembler
	toks      []token
	pos       int
	line      rawLine
	unres     bool // an undefined symbol was referenced
	usedLabel bool
}

type exprResult struct {
	val       int64
	unres     bool
	usedLabel bool
}

func (as *Assembler) evalExpr(toks []token, l rawLine) (exprResult, error) {
	p := &exprParser{as: as, toks: toks, line: l}
	v, err := p.parseLogicalOr()
	if err != nil {
		return exprResult{}, err
	}
	if p.pos < len(p.toks) {
		return exprResult{}, p.errorf("unexpected %q in expression", p.toks[p.pos].String())
	}
	return exprResult{val: v, unres: p.unres, usedLabel: p.usedLabel}, nil
}

// evalConst evaluates an expression that must be fully known now.
func (as *Assembler) evalConst(toks []token, l rawLine) (int64, error) {
	r, err := as.evalExpr(toks, l)
	if err != nil {
		return 0, err
	}
	if r.unres {
		return 0, fmt.Errorf("%s:%d: expression must be constant", l.file, l.line)
	}
	return r.val, nil
}

func (p *exprParser) errorf(format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", p.line.file, p.line.line, fmt.Sprintf(format, args...))
}

func (p *exprParser) peek() token {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return token{kind: tkEOF}
}

func (p *exprParser) next() token {
	t := p.peek()
	p.pos++
	return t
}

func (p *exprParser) accept(puncts ...string) (string, bool) {
	t := p.peek()
	if t.kind != tkPunct {
		return "", false
	}
	for _, s := range puncts {
		if t.text == s {
			p.pos++
			return s, true
		}
	}
	return "", false
}

func b2i(b bool) int64 {
	if b {
		return -1 // NASM: true is -1 for || && etc.? NASM uses 1 for comparisons
	}
	return 0
}

func (p *exprParser) parseLogicalOr() (int64, error) {
	// ternary: cond ? a : b
	v, err := p.parseLogicalXor()
	if err != nil {
		return 0, err
	}
	for {
		if _, ok := p.accept("||"); ok {
			r, err := p.parseLogicalXor()
			if err != nil {
				return 0, err
			}
			if v != 0 || r != 0 {
				v = 1
			} else {
				v = 0
			}
			continue
		}
		if _, ok := p.accept("?"); ok {
			a, err := p.parseLogicalOr()
			if err != nil {
				return 0, err
			}
			if _, ok := p.accept(":"); !ok {
				return 0, p.errorf("expected ':' in conditional expression")
			}
			b, err := p.parseLogicalOr()
			if err != nil {
				return 0, err
			}
			if v != 0 {
				v = a
			} else {
				v = b
			}
			continue
		}
		return v, nil
	}
}

func (p *exprParser) parseLogicalXor() (int64, error) {
	v, err := p.parseLogicalAnd()
	if err != nil {
		return 0, err
	}
	for {
		if _, ok := p.accept("^^"); !ok {
			return v, nil
		}
		r, err := p.parseLogicalAnd()
		if err != nil {
			return 0, err
		}
		if (v != 0) != (r != 0) {
			v = 1
		} else {
			v = 0
		}
	}
}

func (p *exprParser) parseLogicalAnd() (int64, error) {
	v, err := p.parseCompare()
	if err != nil {
		return 0, err
	}
	for {
		if _, ok := p.accept("&&"); !ok {
			return v, nil
		}
		r, err := p.parseCompare()
		if err != nil {
			return 0, err
		}
		if v != 0 && r != 0 {
			v = 1
		} else {
			v = 0
		}
	}
}

func (p *exprParser) parseCompare() (int64, error) {
	v, err := p.parseBitOr()
	if err != nil {
		return 0, err
	}
	for {
		op, ok := p.accept("==", "=", "!=", "<>", "<=", ">=", "<", ">", "<=>")
		if !ok {
			return v, nil
		}
		r, err := p.parseBitOr()
		if err != nil {
			return 0, err
		}
		var res bool
		switch op {
		case "==", "=":
			res = v == r
		case "!=", "<>":
			res = v != r
		case "<":
			res = v < r
		case ">":
			res = v > r
		case "<=":
			res = v <= r
		case ">=":
			res = v >= r
		case "<=>":
			switch {
			case v < r:
				v = -1
			case v > r:
				v = 1
			default:
				v = 0
			}
			continue
		}
		if res {
			v = -1
		} else {
			v = 0
		}
	}
}

func (p *exprParser) parseBitOr() (int64, error) {
	v, err := p.parseBitXor()
	if err != nil {
		return 0, err
	}
	for {
		if _, ok := p.accept("|"); !ok {
			return v, nil
		}
		r, err := p.parseBitXor()
		if err != nil {
			return 0, err
		}
		v |= r
	}
}

func (p *exprParser) parseBitXor() (int64, error) {
	v, err := p.parseBitAnd()
	if err != nil {
		return 0, err
	}
	for {
		if _, ok := p.accept("^"); !ok {
			return v, nil
		}
		r, err := p.parseBitAnd()
		if err != nil {
			return 0, err
		}
		v ^= r
	}
}

func (p *exprParser) parseBitAnd() (int64, error) {
	v, err := p.parseShift()
	if err != nil {
		return 0, err
	}
	for {
		if _, ok := p.accept("&"); !ok {
			return v, nil
		}
		r, err := p.parseShift()
		if err != nil {
			return 0, err
		}
		v &= r
	}
}

func (p *exprParser) parseShift() (int64, error) {
	v, err := p.parseAdd()
	if err != nil {
		return 0, err
	}
	for {
		op, ok := p.accept("<<", ">>")
		if !ok {
			return v, nil
		}
		r, err := p.parseAdd()
		if err != nil {
			return 0, err
		}
		if r < 0 || r > 63 {
			v = 0
			continue
		}
		if op == "<<" {
			v <<= uint(r)
		} else {
			v = int64(uint64(v) >> uint(r))
		}
	}
}

func (p *exprParser) parseAdd() (int64, error) {
	v, err := p.parseMul()
	if err != nil {
		return 0, err
	}
	for {
		op, ok := p.accept("+", "-")
		if !ok {
			return v, nil
		}
		r, err := p.parseMul()
		if err != nil {
			return 0, err
		}
		if op == "+" {
			v += r
		} else {
			v -= r
		}
	}
}

func (p *exprParser) parseMul() (int64, error) {
	v, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		op, ok := p.accept("*", "/", "//", "%", "%%")
		if !ok {
			return v, nil
		}
		r, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		switch op {
		case "*":
			v *= r
		case "/":
			if r == 0 {
				return 0, p.errorf("division by zero")
			}
			v = int64(uint64(v) / uint64(r))
		case "//":
			if r == 0 {
				return 0, p.errorf("division by zero")
			}
			v /= r
		case "%":
			if r == 0 {
				return 0, p.errorf("division by zero")
			}
			v = int64(uint64(v) % uint64(r))
		case "%%":
			if r == 0 {
				return 0, p.errorf("division by zero")
			}
			v %= r
		}
	}
}

func (p *exprParser) parseUnary() (int64, error) {
	t := p.peek()
	if t.kind == tkPunct {
		switch t.text {
		case "-":
			p.pos++
			v, err := p.parseUnary()
			return -v, err
		case "+":
			p.pos++
			return p.parseUnary()
		case "~":
			p.pos++
			v, err := p.parseUnary()
			return ^v, err
		case "!":
			p.pos++
			v, err := p.parseUnary()
			if v == 0 {
				return 1, err
			}
			return 0, err
		}
	}
	if t.kind == tkIdent && (strings.EqualFold(t.text, "seg")) {
		p.pos++
		_, err := p.parseUnary()
		// Flat binary: segment of everything is the load segment; NASM
		// gives 0 for bin format relative to org.
		return 0, err
	}
	return p.parsePrimary()
}

func (p *exprParser) parsePrimary() (int64, error) {
	t := p.next()
	switch t.kind {
	case tkNumber:
		v, err := parseNumber(t.text)
		if err != nil {
			return 0, p.errorf("%s", err)
		}
		return v, nil
	case tkString:
		// Character constant: little-endian packing of up to 8 bytes.
		b, err := p.as.stringBytes(t)
		if err != nil {
			return 0, p.errorf("%s", err)
		}
		if len(b) > 8 {
			return 0, p.errorf("character constant too long")
		}
		var v int64
		for i := len(b) - 1; i >= 0; i-- {
			v = v<<8 | int64(b[i])
		}
		return v, nil
	case tkDollar:
		p.usedLabel = true
		return p.as.here(), nil
	case tkDDollar:
		p.usedLabel = true
		return p.as.sectionStart(), nil
	case tkIdent:
		name := p.as.qualify(t.text)
		if v, ok := p.as.lookup(name); ok {
			p.usedLabel = true
			return v, nil
		}
		p.unres = true
		p.as.noteUndefined(name, p.line)
		return 0, nil
	case tkPunct:
		if t.text == "(" {
			v, err := p.parseLogicalOr()
			if err != nil {
				return 0, err
			}
			if _, ok := p.accept(")"); !ok {
				return 0, p.errorf("expected ')'")
			}
			return v, nil
		}
	case tkFloat:
		return 0, p.errorf("floating-point value not allowed here")
	}
	return 0, p.errorf("unexpected %q in expression", t.String())
}
