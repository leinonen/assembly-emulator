package assembler

import (
	"fmt"
	"math"
	"strings"
)

type dataDirective struct {
	width int       // 1,2,4,8,10
	args  [][]token // comma-separated operands
}

var dataWidths = map[string]int{"db": 1, "dw": 2, "dd": 4, "dq": 8, "dt": 10}
var resWidths = map[string]int{"resb": 1, "resw": 2, "resd": 4, "resq": 8, "rest": 10}

// parse converts preprocessed lines to items.
func (as *Assembler) parse(lines []srcLine) error {
	as.bits = 16
	as.curSect = as.sectionIndex(".text")
	for _, sl := range lines {
		l := rawLine{file: sl.file, line: sl.line}
		if err := as.parseLine(sl.toks, l); err != nil {
			return err
		}
	}
	return nil
}

func splitCommas(toks []token) [][]token {
	var out [][]token
	var cur []token
	depth := 0
	for _, t := range toks {
		if t.is("(") || t.is("[") {
			depth++
		}
		if t.is(")") || t.is("]") {
			depth--
		}
		if t.is(",") && depth == 0 {
			out = append(out, cur)
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 || len(out) > 0 {
		out = append(out, cur)
	}
	return out
}

func (as *Assembler) parseLine(toks []token, l rawLine) error {
	if len(toks) == 0 {
		return nil
	}
	label := ""
	// Label: ident ':' or ident followed by something that is not an
	// instruction keyword (NASM allows labels without colons).
	if toks[0].kind == tkIdent && !isReservedWord(toks[0].text) {
		if len(toks) >= 2 && toks[1].is(":") {
			label = toks[0].text
			toks = toks[2:]
		} else if len(toks) >= 2 && toks[1].kind == tkIdent && (isDirectiveWord(toks[1].text) || isMnemonic(toks[1].text) || strings.EqualFold(toks[1].text, "equ")) {
			label = toks[0].text
			toks = toks[1:]
		} else if len(toks) == 1 {
			label = toks[0].text
			toks = nil
		} else if !isMnemonic(toks[0].text) && !isDirectiveWord(toks[0].text) {
			return as.errorf(l, "parser: instruction expected (got %q)", toks[0].text)
		}
	}
	if label != "" {
		if strings.HasPrefix(label, ".") && !strings.HasPrefix(label, "..@") {
			label = as.lastLabel + label
		} else if !strings.HasPrefix(label, "..@") {
			as.lastLabel = label
		}
		if s, ok := as.symbols[label]; ok && s.defined && !s.isEqu {
			return as.errorf(l, "symbol `%s' redefined", label)
		}
		as.symbols[label] = &symbol{}
	}
	it := &item{line: l, label: label, sect: as.curSect}
	if len(toks) == 0 {
		as.items = append(as.items, it)
		return nil
	}
	if toks[0].kind != tkIdent {
		return as.errorf(l, "parser: instruction expected")
	}
	word := strings.ToLower(toks[0].text)
	rest := toks[1:]

	switch word {
	case "equ":
		if label == "" {
			return as.errorf(l, "EQU needs a label")
		}
		it.kind = itEqu
		it.toks = rest
		as.symbols[label].isEqu = true
		as.items = append(as.items, it)
		return nil
	case "org":
		v, err := as.evalConst(rest, l)
		if err != nil {
			return err
		}
		as.org = v
		it.kind = itOrg
		as.items = append(as.items, it)
		return nil
	case "bits", "use16", "use32":
		if word == "bits" {
			v, err := as.evalConst(rest, l)
			if err != nil {
				return err
			}
			if v != 16 {
				return as.errorf(l, "only BITS 16 is supported")
			}
		}
		if word == "use32" {
			return as.errorf(l, "only 16-bit code is supported")
		}
		as.items = append(as.items, it)
		return nil
	case "cpu", "default", "global", "extern", "absolute", "common", "float", "warning":
		as.items = append(as.items, it)
		return nil
	case "section", "segment":
		name := ".text"
		if len(rest) > 0 {
			name = strings.ToLower(rest[0].text)
		}
		if name != ".text" && name != ".data" && name != ".bss" && name != ".rodata" && name != "code" && name != "data" && name != "text" {
			// treat unknown sections as progbits
		}
		if name == "code" || name == "text" {
			name = ".text"
		}
		if name == "data" || name == ".rodata" {
			name = ".data"
		}
		as.curSect = as.sectionIndex(name)
		it.sect = as.curSect
		it.kind = itSection
		as.items = append(as.items, it)
		return nil
	case "align", "alignb":
		it.kind = itAlign
		parts := splitCommas(rest)
		if len(parts) == 0 {
			return as.errorf(l, "ALIGN needs a value")
		}
		it.toks = parts[0]
		if len(parts) > 1 {
			inner := &item{line: l, sect: as.curSect}
			if err := as.parseDataOrInsn(inner, parts[1], l); err != nil {
				return err
			}
			it.inner = inner
		}
		as.items = append(as.items, it)
		return nil
	case "times":
		// times <expr> <item>: the expression ends where a mnemonic or
		// data directive begins.
		split := -1
		for i, t := range rest {
			if t.kind == tkIdent && (isMnemonic(t.text) || isDirectiveWord(t.text)) {
				split = i
				break
			}
		}
		if split <= 0 {
			return as.errorf(l, "TIMES needs a count and an instruction")
		}
		it.kind = itTimes
		it.times = rest[:split]
		inner := &item{line: l, sect: as.curSect}
		if err := as.parseDataOrInsn(inner, rest[split:], l); err != nil {
			return err
		}
		it.inner = inner
		as.items = append(as.items, it)
		return nil
	case "incbin":
		it.kind = itIncbin
		it.toks = rest
		as.items = append(as.items, it)
		return nil
	}
	if err := as.parseDataOrInsn(it, toks, l); err != nil {
		return err
	}
	as.items = append(as.items, it)
	return nil
}

// parseDataOrInsn fills an item from a data directive or instruction.
func (as *Assembler) parseDataOrInsn(it *item, toks []token, l rawLine) error {
	word := strings.ToLower(toks[0].text)
	rest := toks[1:]
	if w, ok := dataWidths[word]; ok {
		it.kind = itData
		it.data = &dataDirective{width: w, args: splitCommas(rest)}
		return nil
	}
	if w, ok := resWidths[word]; ok {
		it.kind = itRes
		it.data = &dataDirective{width: w}
		it.toks = rest
		return nil
	}
	insn, err := as.parseInstruction(toks, l)
	if err != nil {
		return err
	}
	it.kind = itInsn
	it.insn = insn
	return nil
}

func isDirectiveWord(s string) bool {
	switch strings.ToLower(s) {
	case "db", "dw", "dd", "dq", "dt", "resb", "resw", "resd", "resq", "rest", "times", "align", "alignb", "incbin", "equ", "org", "bits", "section", "segment", "cpu", "global", "extern", "default", "use16", "use32", "absolute":
		return true
	}
	return false
}

func isReservedWord(s string) bool {
	if isDirectiveWord(s) || isMnemonic(s) || prefixWords[strings.ToLower(s)] {
		return true
	}
	_, isReg := registers[strings.ToLower(s)]
	return isReg
}

// ---- data directives ----------------------------------------------------------

func (as *Assembler) sizeData(it *item) (int64, error) {
	d := it.data
	var out []byte
	for _, arg := range d.args {
		if len(arg) == 0 {
			return 0, as.errorf(it.line, "empty data operand")
		}
		if len(arg) == 1 && arg[0].kind == tkString && (d.width == 1 || len(arg[0].text) > 1) {
			b, err := as.stringBytes(arg[0])
			if err != nil {
				return 0, as.errorf(it.line, "%s", err)
			}
			if d.width == 1 {
				out = append(out, b...)
			} else {
				// Pad string to a multiple of the width (NASM packs strings).
				for len(b)%d.width != 0 {
					b = append(b, 0)
				}
				out = append(out, b...)
			}
			continue
		}
		if len(arg) == 1 && arg[0].kind == tkFloat {
			var f float64
			if _, err := fmt.Sscanf(arg[0].text, "%g", &f); err != nil {
				return 0, as.errorf(it.line, "bad float %q", arg[0].text)
			}
			switch d.width {
			case 4:
				out = appendLE(out, uint64(math.Float32bits(float32(f))), 4)
			case 8:
				out = appendLE(out, math.Float64bits(f), 8)
			case 10:
				m, se := encodeF80(f)
				out = appendLE(out, m, 8)
				out = appendLE(out, uint64(se), 2)
			default:
				return 0, as.errorf(it.line, "floating-point constant in a %d-byte data directive", d.width)
			}
			continue
		}
		if len(arg) == 1 && arg[0].kind == tkPunct && arg[0].text == "?" {
			out = append(out, make([]byte, d.width)...)
			continue
		}
		r, err := as.evalExpr(arg, it.line)
		if err != nil {
			return 0, err
		}
		if as.final && !r.unres {
			if err := checkRange(r.val, d.width); err != nil {
				as.warnings = append(as.warnings, fmt.Sprintf("%s:%d: warning: %s", it.line.file, it.line.line, err))
			}
		}
		if d.width == 10 {
			out = appendLE(out, uint64(r.val), 8)
			out = appendLE(out, 0, 2)
		} else {
			out = appendLE(out, uint64(r.val), d.width)
		}
	}
	it.bytes = out
	return int64(len(out)), nil
}

func checkRange(v int64, width int) error {
	switch width {
	case 1:
		if v < -128 || v > 255 {
			return fmt.Errorf("byte data exceeds bounds")
		}
	case 2:
		if v < -32768 || v > 65535 {
			return fmt.Errorf("word data exceeds bounds")
		}
	case 4:
		if v < -2147483648 || v > 4294967295 {
			return fmt.Errorf("dword data exceeds bounds")
		}
	}
	return nil
}

func appendLE(b []byte, v uint64, n int) []byte {
	for i := 0; i < n; i++ {
		b = append(b, byte(v>>(8*uint(i))))
	}
	return b
}

// encodeF80 converts a float64 to the x87 extended format.
func encodeF80(v float64) (mant uint64, se uint16) {
	bits := math.Float64bits(v)
	sign := uint16(bits>>63) << 15
	exp := int((bits >> 52) & 0x7FF)
	frac := bits & (1<<52 - 1)
	switch {
	case exp == 0x7FF:
		if frac == 0 {
			return 1 << 63, sign | 0x7FFF
		}
		return 1<<63 | frac<<11 | 1<<62, sign | 0x7FFF
	case exp == 0 && frac == 0:
		return 0, sign
	case exp == 0:
		e := -1022
		for frac&(1<<52) == 0 {
			frac <<= 1
			e--
		}
		return frac << 11, sign | uint16(e+16383)
	default:
		return 1<<63 | frac<<11, sign | uint16(exp-1023+16383)
	}
}
