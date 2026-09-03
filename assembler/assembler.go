package assembler

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"assembly-emulator/cp437"
)

// Options configure assembly.
type Options struct {
	Filename    string
	IncludeDirs []string
	// Listing, if non-nil, receives "address bytes source" lines.
	Listing func(addr int64, code []byte, file string, line int)
}

// Assemble assembles NASM-syntax source into a flat binary.
func Assemble(src []byte, opts Options) ([]byte, error) {
	as := &Assembler{opts: opts, symbols: map[string]*symbol{}}
	if opts.Filename == "" {
		as.opts.Filename = "<input>"
	}
	pp := newPreprocessor(as)
	as.pp = pp
	if err := pp.run(string(src), as.opts.Filename); err != nil {
		return nil, err
	}
	if err := as.parse(pp.out); err != nil {
		return nil, err
	}
	return as.assemble()
}

type symbol struct {
	value   int64
	defined bool
	isEqu   bool
	equToks []token
	equLine rawLine
	section int
}

type section struct {
	name   string
	nobits bool
	data   []byte
	size   int64 // size after the current pass
	start  int64 // address of the section start (after layout)
}

// item is one assembled unit (instruction, data, directive).
type item struct {
	line  rawLine
	label string // label defined at this item (already qualified)
	kind  itemKind
	insn  *instruction
	data  *dataDirective
	toks  []token // for directives like times/align/org
	times []token // times prefix expression
	sect  int

	size  int64 // size decided in the last pass
	addr  int64 // address in the last pass
	bytes []byte
	// jump relaxation state: once a jump needed a near form it stays near
	// (only-grow relaxation guarantees convergence)
	nearJump bool
	// times count computed
	timesCount int64
	inner      *item
}

type itemKind int

const (
	itNone itemKind = iota
	itInsn
	itData
	itRes
	itAlign
	itOrg
	itIncbin
	itEqu
	itTimes
	itSection
)

// Assembler state.
type Assembler struct {
	opts      Options
	pp        *preprocessor
	symbols   map[string]*symbol
	items     []*item
	sections  []*section
	curSect   int
	org       int64
	bits      int
	cpu       string
	lastLabel string // for local labels
	pass      int
	final     bool
	changed   bool
	undefined map[string]rawLine
	cur       *item // item being sized/encoded (for $)
	curAddr   int64
	warnings  []string
}

func (as *Assembler) warnf(l rawLine, format string, args ...any) {
	as.warnings = append(as.warnings, fmt.Sprintf("%s:%d: warning: %s", l.file, l.line, fmt.Sprintf(format, args...)))
}

func (as *Assembler) errorf(l rawLine, format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", l.file, l.line, fmt.Sprintf(format, args...))
}

// qualify resolves local label names (.name -> parent.name).
func (as *Assembler) qualify(name string) string {
	if strings.HasPrefix(name, "..@") {
		return name
	}
	if strings.HasPrefix(name, ".") && as.lastLabel != "" {
		return as.lastLabel + name
	}
	return name
}

func (as *Assembler) lookup(name string) (int64, bool) {
	s, ok := as.symbols[name]
	if !ok {
		s, ok = as.symbols[strings.ToLower(name)]
	}
	if !ok || !s.defined {
		return 0, false
	}
	return s.value, true
}

func (as *Assembler) noteUndefined(name string, l rawLine) {
	if as.undefined == nil {
		as.undefined = map[string]rawLine{}
	}
	if _, ok := as.undefined[name]; !ok {
		as.undefined[name] = l
	}
}

// here returns the current address ($).
func (as *Assembler) here() int64 { return as.curAddr }

func (as *Assembler) sectionStart() int64 {
	if as.cur != nil {
		return as.sections[as.cur.sect].start
	}
	return as.org
}

// stringBytes converts a string token to bytes (CP437 for non-ASCII,
// escapes for backquoted strings).
func (as *Assembler) stringBytes(t token) ([]byte, error) {
	s := t.text
	if t.quote == '`' {
		s = unescape(s)
	}
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 0x80 {
			out = append(out, byte(r))
			continue
		}
		b, ok := cp437.RuneToCP437(r)
		if !ok {
			return nil, fmt.Errorf("character %q has no CP437 encoding", r)
		}
		out = append(out, b)
	}
	return out, nil
}

func unescape(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			sb.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			sb.WriteByte('\n')
		case 'r':
			sb.WriteByte('\r')
		case 't':
			sb.WriteByte('\t')
		case 'a':
			sb.WriteByte(7)
		case 'b':
			sb.WriteByte(8)
		case 'e':
			sb.WriteByte(27)
		case '0':
			sb.WriteByte(0)
		case 'x':
			v := 0
			n := 0
			for n < 2 && i+1 < len(s) && isHexDigit(s[i+1]) {
				i++
				v = v*16 + hexVal(s[i])
				n++
			}
			sb.WriteByte(byte(v))
		default:
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return int(c-'A') + 10
	}
}

// ---- passes -----------------------------------------------------------------

func (as *Assembler) sectionIndex(name string) int {
	for i, s := range as.sections {
		if s.name == name {
			return i
		}
	}
	nobits := name == ".bss"
	as.sections = append(as.sections, &section{name: name, nobits: nobits})
	return len(as.sections) - 1
}

// assemble runs sizing passes until addresses converge, then encodes.
func (as *Assembler) assemble() ([]byte, error) {
	if len(as.sections) == 0 {
		as.sectionIndex(".text")
	}
	const maxPasses = 40
	for as.pass = 1; as.pass <= maxPasses; as.pass++ {
		as.final = false
		as.changed = false
		as.undefined = nil
		if err := as.runPass(); err != nil {
			return nil, err
		}
		if !as.changed && as.pass > 1 {
			break
		}
	}
	if as.changed {
		return nil, fmt.Errorf("%s: assembly did not converge after %d passes", as.opts.Filename, maxPasses)
	}
	// Final pass: encode with all symbols known.
	as.final = true
	as.undefined = nil
	if err := as.runPass(); err != nil {
		return nil, err
	}
	if len(as.undefined) > 0 {
		names := make([]string, 0, len(as.undefined))
		for n := range as.undefined {
			names = append(names, n)
		}
		sort.Strings(names)
		l := as.undefined[names[0]]
		return nil, as.errorf(l, "symbol `%s' undefined", names[0])
	}
	for _, w := range as.warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	if as.opts.Listing != nil {
		for _, it := range as.items {
			if it.kind == itNone && it.label == "" {
				continue
			}
			as.opts.Listing(it.addr, it.bytes, it.line.file, it.line.line)
		}
	}
	// Layout: progbits sections in order, then nobits (not emitted).
	var out []byte
	for _, s := range as.sections {
		if s.nobits {
			continue
		}
		out = append(out, s.data...)
	}
	return out, nil
}

// runPass sizes (and in the final pass encodes) every item.
func (as *Assembler) runPass() error {
	// Section layout from the previous pass' sizes.
	addr := as.org
	for _, s := range as.sections {
		if s.nobits {
			continue
		}
		s.start = addr
		addr += s.size
		s.data = s.data[:0]
	}
	for _, s := range as.sections {
		if !s.nobits {
			continue
		}
		s.start = addr
		addr += s.size
	}
	sizes := make([]int64, len(as.sections))
	as.lastLabel = ""
	for _, it := range as.items {
		s := as.sections[it.sect]
		as.cur = it
		as.curAddr = s.start + sizes[it.sect]
		it.addr = as.curAddr
		if it.label != "" && it.kind != itEqu {
			as.define(it.label, it.addr, it.sect, it.line)
			// Macro-local labels (..@n.name) never become the parent of
			// the local labels around them, as in NASM.
			if !strings.HasPrefix(it.label, "..@") {
				if !strings.Contains(it.label, ".") || !strings.HasPrefix(it.label, as.lastLabel+".") {
					as.lastLabel = it.label
				}
			}
		}
		n, err := as.sizeItem(it)
		if err != nil {
			return err
		}
		if n != it.size {
			if os.Getenv("ASMDEBUG") != "" {
				fmt.Fprintf(os.Stderr, "pass %d: %s:%d size %d -> %d\n", as.pass, it.line.file, it.line.line, it.size, n)
			}
			as.changed = true
			it.size = n
		}
		if as.final && !s.nobits {
			s.data = append(s.data, it.bytes...)
			if int64(len(it.bytes)) != n {
				return as.errorf(it.line, "internal error: size mismatch (%d vs %d)", len(it.bytes), n)
			}
		}
		sizes[it.sect] += n
	}
	for i, s := range as.sections {
		if s.size != sizes[i] {
			as.changed = true
			s.size = sizes[i]
		}
	}
	// EQUs that reference forward labels are re-evaluated each pass.
	return nil
}

func (as *Assembler) define(name string, v int64, sect int, l rawLine) {
	s, ok := as.symbols[name]
	if !ok {
		s = &symbol{}
		as.symbols[name] = s
	}
	if s.defined && s.value != v {
		if os.Getenv("ASMDEBUG") != "" {
			fmt.Fprintf(os.Stderr, "pass %d: symbol %s %d -> %d\n", as.pass, name, s.value, v)
		}
		as.changed = true
	}
	s.value = v
	s.defined = true
	s.section = sect
}

// sizeItem computes the size of an item for this pass; in the final pass
// it also produces the bytes.
func (as *Assembler) sizeItem(it *item) (int64, error) {
	switch it.kind {
	case itNone:
		it.bytes = nil
		return 0, nil
	case itEqu:
		r, err := as.evalExpr(it.toks, it.line)
		if err != nil {
			return 0, err
		}
		if !r.unres {
			as.define(it.label, r.val, it.sect, it.line)
		}
		it.bytes = nil
		return 0, nil
	case itOrg:
		// handled at parse time
		it.bytes = nil
		return 0, nil
	case itTimes:
		r, err := as.evalExpr(it.times, it.line)
		if err != nil {
			return 0, err
		}
		if r.unres {
			if as.final {
				return 0, as.errorf(it.line, "TIMES count must be resolvable")
			}
			r.val = 0
		}
		if r.val < 0 {
			return 0, as.errorf(it.line, "TIMES value %d is negative", r.val)
		}
		it.timesCount = r.val
		if r.val > 16<<20 {
			return 0, as.errorf(it.line, "TIMES value too large")
		}
		one, err := as.sizeItem(it.inner)
		if err != nil {
			return 0, err
		}
		total := one * r.val
		if as.final {
			it.bytes = make([]byte, 0, total)
			for i := int64(0); i < r.val; i++ {
				// The inner item may depend on $ (rare); re-encode each.
				as.curAddr = it.addr + int64(len(it.bytes))
				if _, err := as.sizeItem(it.inner); err != nil {
					return 0, err
				}
				it.bytes = append(it.bytes, it.inner.bytes...)
			}
		}
		return total, nil
	case itAlign:
		r, err := as.evalExpr(it.toks, it.line)
		if err != nil {
			return 0, err
		}
		n := r.val
		if n <= 0 || r.unres {
			n = 1
		}
		off := as.curAddr - as.sectionStart()
		pad := (n - off%n) % n
		if as.final {
			it.bytes = make([]byte, pad)
			fill := byte(0x90)
			if as.sections[it.sect].name != ".text" && it.inner == nil {
				fill = 0
			}
			if it.inner != nil {
				// align n, <data directive>: use its first byte pattern
				if _, err := as.sizeItem(it.inner); err != nil {
					return 0, err
				}
				if len(it.inner.bytes) > 0 {
					for i := range it.bytes {
						it.bytes[i] = it.inner.bytes[i%len(it.inner.bytes)]
					}
					return pad, nil
				}
			}
			for i := range it.bytes {
				it.bytes[i] = fill
			}
		}
		return pad, nil
	case itRes:
		r, err := as.evalExpr(it.toks, it.line)
		if err != nil {
			return 0, err
		}
		n := r.val * int64(it.data.width)
		if as.final {
			it.bytes = make([]byte, n)
		}
		return n, nil
	case itIncbin:
		return as.sizeIncbin(it)
	case itData:
		return as.sizeData(it)
	case itInsn:
		return as.sizeInsn(it)
	case itSection:
		it.bytes = nil
		return 0, nil
	}
	return 0, as.errorf(it.line, "internal: unknown item kind")
}

func (as *Assembler) sizeIncbin(it *item) (int64, error) {
	if it.bytes == nil || !as.final {
		var data []byte
		name := ""
		if len(it.toks) > 0 && it.toks[0].kind == tkString {
			name = it.toks[0].text
		}
		d, _, err := as.pp.findInclude(name, it.line.file)
		if err != nil {
			return 0, as.errorf(it.line, "cannot open binary file %q", name)
		}
		data = d
		// optional skip, length
		var args [][]token
		var cur []token
		for _, t := range it.toks[1:] {
			if t.is(",") {
				args = append(args, cur)
				cur = nil
				continue
			}
			cur = append(cur, t)
		}
		if len(cur) > 0 {
			args = append(args, cur)
		}
		if len(args) >= 1 && len(args[0]) > 0 {
			v, err := as.evalConst(args[0], it.line)
			if err != nil {
				return 0, err
			}
			if v > int64(len(data)) {
				v = int64(len(data))
			}
			data = data[v:]
		}
		if len(args) >= 2 && len(args[1]) > 0 {
			v, err := as.evalConst(args[1], it.line)
			if err != nil {
				return 0, err
			}
			if v < int64(len(data)) {
				data = data[:v]
			}
		}
		it.bytes = data
	}
	return int64(len(it.bytes)), nil
}
