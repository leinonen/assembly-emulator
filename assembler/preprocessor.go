package assembler

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// srcLine is a preprocessed line of tokens with its origin.
type srcLine struct {
	toks []token
	file string
	line int
}

type smacro struct {
	name    string
	params  []string
	body    []token
	hasArgs bool
	caseIns bool
}

type mmacro struct {
	name    string
	nparams int
	greedy  bool
	body    []rawLine
}

type rawLine struct {
	text string
	file string
	line int
}

type condState struct {
	active   bool // this branch is active
	taken    bool // some branch already taken
	parentOK bool
}

// preprocessor expands %-directives and macros producing token lines.
type preprocessor struct {
	as      *Assembler
	smacros map[string]*smacro
	mmacros map[string]*mmacro
	out     []srcLine
	conds   []condState
	uniq    int
	incDirs []string
	depth   int
	// %rep handling
	repDepth int
}

func newPreprocessor(as *Assembler) *preprocessor {
	return &preprocessor{as: as, smacros: map[string]*smacro{}, mmacros: map[string]*mmacro{}, incDirs: as.opts.IncludeDirs}
}

func (p *preprocessor) errorf(l rawLine, format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", l.file, l.line, fmt.Sprintf(format, args...))
}

func (p *preprocessor) active() bool {
	for _, c := range p.conds {
		if !c.active {
			return false
		}
	}
	return true
}

// run processes the whole source file.
func (p *preprocessor) run(text, file string) error {
	lines := splitLines(text, file)
	return p.processLines(lines)
}

func splitLines(text, file string) []rawLine {
	var out []rawLine
	for i, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		out = append(out, rawLine{text: l, file: file, line: i + 1})
	}
	return out
}

// processLines handles a sequence of raw lines (top level, include, or
// macro expansion).
func (p *preprocessor) processLines(lines []rawLine) error {
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		trimmed := strings.TrimSpace(l.text)
		// Line continuation with backslash.
		for strings.HasSuffix(trimmed, "\\") && i+1 < len(lines) {
			i++
			trimmed = strings.TrimSuffix(trimmed, "\\") + " " + strings.TrimSpace(lines[i].text)
		}
		if strings.HasPrefix(trimmed, "%") {
			consumed, err := p.directive(trimmed, l, lines, i)
			if err != nil {
				return err
			}
			i += consumed
			continue
		}
		if !p.active() {
			continue
		}
		if err := p.emitLine(l.text, l); err != nil {
			return err
		}
	}
	return nil
}

// emitLine lexes, expands single-line macros and multi-line macro
// invocations, and appends to the output.
func (p *preprocessor) emitLine(text string, l rawLine) error {
	toks, err := lexLine(text, l.line)
	if err != nil {
		return fmt.Errorf("%s:%d: %s", l.file, l.line, err.(*lexError).msg)
	}
	toks, err = p.expandSingle(toks, l, 0)
	if err != nil {
		return err
	}
	if len(toks) == 0 {
		return nil
	}
	// Multi-line macro invocation? (optionally after a label)
	idx := 0
	if len(toks) >= 2 && toks[0].kind == tkIdent && toks[1].is(":") {
		idx = 2
	}
	if idx < len(toks) && toks[idx].kind == tkIdent {
		if m, ok := p.mmacros[strings.ToLower(toks[idx].text)]; ok {
			if idx > 0 {
				p.out = append(p.out, srcLine{toks: toks[:idx], file: l.file, line: l.line})
			}
			return p.expandMulti(m, toks[idx+1:], l)
		}
	}
	p.out = append(p.out, srcLine{toks: toks, file: l.file, line: l.line})
	return nil
}

// expandSingle substitutes single-line macros in a token list.
func (p *preprocessor) expandSingle(toks []token, l rawLine, depth int) ([]token, error) {
	if depth > 64 {
		return nil, p.errorf(l, "macro expansion too deep")
	}
	changed := false
	var out []token
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind != tkIdent {
			out = append(out, t)
			continue
		}
		m := p.lookupSmacro(t.text)
		if m == nil {
			out = append(out, t)
			continue
		}
		if m.hasArgs {
			// Expect ( args )
			if i+1 >= len(toks) || !toks[i+1].is("(") {
				out = append(out, t)
				continue
			}
			args, next, err := splitArgs(toks, i+2, l)
			if err != nil {
				return nil, err
			}
			if len(args) != len(m.params) {
				return nil, p.errorf(l, "macro %s expects %d arguments, got %d", m.name, len(m.params), len(args))
			}
			body := substituteParams(m.body, m.params, args)
			out = append(out, body...)
			i = next - 1
			changed = true
			continue
		}
		out = append(out, m.body...)
		changed = true
	}
	if changed {
		return p.expandSingle(out, l, depth+1)
	}
	return out, nil
}

func (p *preprocessor) lookupSmacro(name string) *smacro {
	if m, ok := p.smacros[name]; ok {
		return m
	}
	if m, ok := p.smacros[strings.ToLower(name)]; ok && m.caseIns {
		return m
	}
	return nil
}

// splitArgs parses "a, b, c)" starting at toks[i], returning the args and
// the index after the closing paren.
func splitArgs(toks []token, i int, l rawLine) ([][]token, int, error) {
	var args [][]token
	var cur []token
	depth := 0
	for ; i < len(toks); i++ {
		t := toks[i]
		if t.is("(") {
			depth++
		}
		if t.is(")") {
			if depth == 0 {
				args = append(args, cur)
				return args, i + 1, nil
			}
			depth--
		}
		if t.is(",") && depth == 0 {
			args = append(args, cur)
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	return nil, 0, fmt.Errorf("%s:%d: unterminated macro argument list", l.file, l.line)
}

func substituteParams(body []token, params []string, args [][]token) []token {
	var out []token
	for _, t := range body {
		if t.kind == tkIdent {
			found := false
			for i, pn := range params {
				if t.text == pn {
					out = append(out, args[i]...)
					found = true
					break
				}
			}
			if found {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// expandMulti expands a multi-line macro invocation.
func (p *preprocessor) expandMulti(m *mmacro, argToks []token, l rawLine) error {
	// Split arguments on commas at depth 0.
	var args []string
	var cur []string
	depth := 0
	flush := func() {
		args = append(args, strings.TrimSpace(strings.Join(cur, " ")))
		cur = nil
	}
	for _, t := range argToks {
		if t.is("(") || t.is("[") || t.is("{") {
			depth++
		}
		if t.is(")") || t.is("]") || t.is("}") {
			depth--
		}
		if t.is(",") && depth == 0 {
			flush()
			continue
		}
		cur = append(cur, t.String())
	}
	if len(cur) > 0 {
		flush()
	}
	if m.greedy && len(args) > m.nparams && m.nparams > 0 {
		rest := strings.Join(args[m.nparams-1:], ", ")
		args = append(args[:m.nparams-1], rest)
	}
	p.uniq++
	uniq := p.uniq
	p.depth++
	if p.depth > 100 {
		return p.errorf(l, "macro recursion too deep")
	}
	defer func() { p.depth-- }()
	var lines []rawLine
	for _, bl := range m.body {
		text := bl.text
		text = expandMacroParams(text, args, uniq)
		lines = append(lines, rawLine{text: text, file: l.file, line: l.line})
	}
	return p.processLines(lines)
}

// expandMacroParams replaces %0, %1.., %%name, %+ in macro body text.
func expandMacroParams(text string, args []string, uniq int) string {
	var sb strings.Builder
	i := 0
	for i < len(text) {
		c := text[i]
		if c != '%' || i+1 >= len(text) {
			sb.WriteByte(c)
			i++
			continue
		}
		nx := text[i+1]
		switch {
		case isDigit(nx):
			j := i + 1
			for j < len(text) && isDigit(text[j]) {
				j++
			}
			n, _ := strconv.Atoi(text[i+1 : j])
			if n == 0 {
				sb.WriteString(strconv.Itoa(len(args)))
			} else if n <= len(args) {
				sb.WriteString(args[n-1])
			}
			i = j
		case nx == '{':
			j := strings.IndexByte(text[i:], '}')
			if j < 0 {
				sb.WriteByte(c)
				i++
				continue
			}
			inner := text[i+2 : i+j]
			// %{1} or %{1:2} (range) or %{%name}
			if strings.HasPrefix(inner, "%") {
				sb.WriteString("..@" + strconv.Itoa(uniq) + "." + inner[1:])
			} else if strings.Contains(inner, ":") {
				parts := strings.SplitN(inner, ":", 2)
				a, _ := strconv.Atoi(parts[0])
				b, _ := strconv.Atoi(parts[1])
				if b < 0 {
					b = len(args) + 1 + b
				}
				var sel []string
				for k := a; k <= b && k <= len(args); k++ {
					if k >= 1 {
						sel = append(sel, args[k-1])
					}
				}
				sb.WriteString(strings.Join(sel, ", "))
			} else {
				n, _ := strconv.Atoi(inner)
				if n >= 1 && n <= len(args) {
					sb.WriteString(args[n-1])
				}
			}
			i += j + 1
		case nx == '%':
			j := i + 2
			for j < len(text) && (isAlnum(text[j]) || text[j] == '_' || text[j] == '.' || text[j] == '@') {
				j++
			}
			sb.WriteString("..@" + strconv.Itoa(uniq) + "." + text[i+2:j])
			i = j
		case nx == '+':
			// token pasting: just drop the marker
			i += 2
		case nx == '-':
			// %-1: condition negation, unsupported; keep literal
			sb.WriteByte(c)
			i++
		default:
			sb.WriteByte(c)
			i++
		}
	}
	return sb.String()
}

// directive handles a %-line. Returns extra lines consumed.
func (p *preprocessor) directive(trimmed string, l rawLine, lines []rawLine, idx int) (int, error) {
	fields := strings.Fields(trimmed)
	name := strings.ToLower(fields[0])
	rest := strings.TrimSpace(trimmed[len(fields[0]):])

	// Conditionals are processed even when inactive (to track nesting).
	switch name {
	case "%if", "%ifdef", "%ifndef", "%ifidn", "%ifidni", "%ifnidn", "%ifmacro", "%ifnum", "%ifstr", "%ifempty", "%ifnempty", "%ifctx", "%ifn", "%ifnmacro", "%ifid":
		parent := p.active()
		v := false
		if parent {
			var err error
			v, err = p.evalCond(name, rest, l)
			if err != nil {
				return 0, err
			}
		}
		p.conds = append(p.conds, condState{active: parent && v, taken: v, parentOK: parent})
		return 0, nil
	case "%elif", "%elifdef", "%elifndef", "%elifidn", "%elifidni", "%elifnidn", "%elifn":
		if len(p.conds) == 0 {
			return 0, p.errorf(l, "%s without %%if", name)
		}
		c := &p.conds[len(p.conds)-1]
		if !c.parentOK || c.taken {
			c.active = false
			return 0, nil
		}
		v, err := p.evalCond("%"+name[3:], rest, l)
		if err != nil {
			return 0, err
		}
		c.active = v
		if v {
			c.taken = true
		}
		return 0, nil
	case "%else":
		if len(p.conds) == 0 {
			return 0, p.errorf(l, "%%else without %%if")
		}
		c := &p.conds[len(p.conds)-1]
		c.active = c.parentOK && !c.taken
		c.taken = true
		return 0, nil
	case "%endif":
		if len(p.conds) == 0 {
			return 0, p.errorf(l, "%%endif without %%if")
		}
		p.conds = p.conds[:len(p.conds)-1]
		return 0, nil
	}
	// Macro definitions must be skipped as a block when inactive.
	if name == "%macro" || name == "%imacro" {
		end := findEnd(lines, idx+1, "%macro", "%endmacro")
		if end < 0 {
			return 0, p.errorf(l, "%%macro without %%endmacro")
		}
		if p.active() {
			if err := p.defineMacro(rest, lines[idx+1:end], l); err != nil {
				return 0, err
			}
		}
		return end - idx, nil
	}
	if name == "%rep" {
		end := findEnd(lines, idx+1, "%rep", "%endrep")
		if end < 0 {
			return 0, p.errorf(l, "%%rep without %%endrep")
		}
		if p.active() {
			if err := p.doRep(rest, lines[idx+1:end], l); err != nil {
				return 0, err
			}
		}
		return end - idx, nil
	}
	if !p.active() {
		return 0, nil
	}
	switch name {
	case "%define", "%idefine", "%xdefine", "%ixdefine":
		return 0, p.define(rest, l, strings.HasPrefix(name, "%i"), strings.Contains(name, "x"))
	case "%undef":
		delete(p.smacros, strings.TrimSpace(rest))
		delete(p.smacros, strings.ToLower(strings.TrimSpace(rest)))
	case "%assign", "%iassign":
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) != 2 {
			return 0, p.errorf(l, "%%assign needs a name and an expression")
		}
		nm := strings.TrimSpace(parts[0])
		toks, err := lexLine(parts[1], l.line)
		if err != nil {
			return 0, err
		}
		toks, err = p.expandSingle(toks, l, 0)
		if err != nil {
			return 0, err
		}
		v, err := p.as.evalConst(toks, l)
		if err != nil {
			return 0, err
		}
		p.smacros[nm] = &smacro{name: nm, body: []token{{kind: tkNumber, text: strconv.FormatInt(v, 10), line: l.line}}, caseIns: name == "%iassign"}
		if name == "%iassign" {
			p.smacros[strings.ToLower(nm)] = p.smacros[nm]
		}
	case "%strlen":
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) != 2 {
			return 0, p.errorf(l, "%%strlen needs a name and a string")
		}
		toks, err := lexLine(parts[1], l.line)
		if err != nil {
			return 0, err
		}
		toks, _ = p.expandSingle(toks, l, 0)
		if len(toks) != 1 || toks[0].kind != tkString {
			return 0, p.errorf(l, "%%strlen needs a string")
		}
		nm := strings.TrimSpace(parts[0])
		p.smacros[nm] = &smacro{name: nm, body: []token{{kind: tkNumber, text: strconv.Itoa(len(toks[0].text)), line: l.line}}}
	case "%include":
		fname := strings.Trim(strings.TrimSpace(rest), "\"'")
		data, path, err := p.findInclude(fname, l.file)
		if err != nil {
			return 0, p.errorf(l, "cannot open include file %q", fname)
		}
		if err := p.processLines(splitLines(string(data), path)); err != nil {
			return 0, err
		}
	case "%error", "%fatal":
		return 0, p.errorf(l, "%s", strings.Trim(rest, "\"'"))
	case "%warning":
		fmt.Fprintf(os.Stderr, "%s:%d: warning: %s\n", l.file, l.line, rest)
	case "%push", "%pop", "%line", "%pragma", "%use", "%stacksize", "%arg", "%local", "%clear", "%deftok", "%pathsearch", "%depend", "%rotate", "%exitrep", "%unmacro", "%defstr", "%substr":
		// Ignored or unsupported (rare in DOS sources).
	default:
		return 0, p.errorf(l, "unknown preprocessor directive %s", fields[0])
	}
	return 0, nil
}

func findEnd(lines []rawLine, start int, open, close string) int {
	depth := 1
	for i := start; i < len(lines); i++ {
		t := strings.ToLower(strings.TrimSpace(lines[i].text))
		f := strings.Fields(t)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case open, "%i" + open[1:]:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func (p *preprocessor) findInclude(name, from string) ([]byte, string, error) {
	dirs := []string{filepath.Dir(from)}
	dirs = append(dirs, p.incDirs...)
	dirs = append(dirs, ".")
	for _, d := range dirs {
		path := filepath.Join(d, name)
		if data, err := os.ReadFile(path); err == nil {
			return data, path, nil
		}
	}
	return nil, "", os.ErrNotExist
}

func (p *preprocessor) define(rest string, l rawLine, caseIns, expand bool) error {
	// name[(params)] body
	rest = strings.TrimSpace(rest)
	i := 0
	for i < len(rest) && (isAlnum(rest[i]) || rest[i] == '_' || rest[i] == '.' || rest[i] == '@' || rest[i] == '?') {
		i++
	}
	if i == 0 {
		return p.errorf(l, "%%define needs a name")
	}
	m := &smacro{name: rest[:i], caseIns: caseIns}
	rest = rest[i:]
	if strings.HasPrefix(rest, "(") {
		j := strings.IndexByte(rest, ')')
		if j < 0 {
			return p.errorf(l, "bad macro parameter list")
		}
		for _, pn := range strings.Split(rest[1:j], ",") {
			pn = strings.TrimSpace(pn)
			if pn != "" {
				m.params = append(m.params, pn)
			}
		}
		m.hasArgs = true
		rest = rest[j+1:]
	}
	toks, err := lexLine(rest, l.line)
	if err != nil {
		return err
	}
	if expand {
		toks, err = p.expandSingle(toks, l, 0)
		if err != nil {
			return err
		}
	}
	m.body = toks
	p.smacros[m.name] = m
	if caseIns {
		p.smacros[strings.ToLower(m.name)] = m
	}
	return nil
}

func (p *preprocessor) defineMacro(rest string, body []rawLine, l rawLine) error {
	f := strings.Fields(rest)
	if len(f) < 1 {
		return p.errorf(l, "%%macro needs a name")
	}
	m := &mmacro{name: strings.ToLower(f[0])}
	if len(f) >= 2 {
		spec := f[1]
		if strings.HasSuffix(spec, "+") {
			m.greedy = true
			spec = strings.TrimSuffix(spec, "+")
		}
		if strings.Contains(spec, "-") {
			spec = spec[strings.Index(spec, "-")+1:]
		}
		n, err := strconv.Atoi(spec)
		if err == nil {
			m.nparams = n
		}
	}
	m.body = body
	p.mmacros[m.name] = m
	return nil
}

func (p *preprocessor) doRep(rest string, body []rawLine, l rawLine) error {
	toks, err := lexLine(rest, l.line)
	if err != nil {
		return err
	}
	toks, err = p.expandSingle(toks, l, 0)
	if err != nil {
		return err
	}
	n, err := p.as.evalConst(toks, l)
	if err != nil {
		return err
	}
	if n < 0 {
		n = 0
	}
	if n > 1000000 {
		return p.errorf(l, "%%rep count too large")
	}
	p.repDepth++
	defer func() { p.repDepth-- }()
	for k := int64(0); k < n; k++ {
		// %exitrep support: scan for it in active lines.
		var chunk []rawLine
		exit := false
		for _, bl := range body {
			if strings.HasPrefix(strings.TrimSpace(strings.ToLower(bl.text)), "%exitrep") {
				exit = true
				break
			}
			chunk = append(chunk, bl)
		}
		if err := p.processLines(chunk); err != nil {
			return err
		}
		if exit {
			break
		}
	}
	return nil
}

func (p *preprocessor) evalCond(name, rest string, l rawLine) (bool, error) {
	switch name {
	case "%ifdef", "%ifndef", "%elifdef", "%elifndef":
		nm := strings.TrimSpace(rest)
		_, ok := p.smacros[nm]
		if !ok {
			_, ok = p.smacros[strings.ToLower(nm)]
		}
		if strings.Contains(name, "ndef") {
			return !ok, nil
		}
		return ok, nil
	case "%ifmacro", "%ifnmacro":
		f := strings.Fields(rest)
		ok := false
		if len(f) > 0 {
			_, ok = p.mmacros[strings.ToLower(f[0])]
		}
		if name == "%ifnmacro" {
			return !ok, nil
		}
		return ok, nil
	case "%ifidn", "%ifidni", "%ifnidn", "%elifidn", "%elifidni", "%elifnidn":
		parts := strings.SplitN(rest, ",", 2)
		if len(parts) != 2 {
			return false, p.errorf(l, "%s needs two arguments", name)
		}
		a := strings.TrimSpace(parts[0])
		b := strings.TrimSpace(parts[1])
		eq := a == b
		if strings.HasSuffix(name, "i") {
			eq = strings.EqualFold(a, b)
		}
		if strings.Contains(name, "nidn") {
			return !eq, nil
		}
		return eq, nil
	case "%ifempty", "%ifnempty":
		empty := strings.TrimSpace(rest) == ""
		if name == "%ifnempty" {
			return !empty, nil
		}
		return empty, nil
	case "%ifnum", "%ifstr", "%ifid", "%ifctx":
		toks, _ := lexLine(rest, l.line)
		if len(toks) == 0 {
			return false, nil
		}
		switch name {
		case "%ifnum":
			return toks[0].kind == tkNumber, nil
		case "%ifstr":
			return toks[0].kind == tkString, nil
		case "%ifid":
			return toks[0].kind == tkIdent, nil
		}
		return false, nil
	case "%if", "%elif", "%ifn", "%elifn":
		toks, err := lexLine(rest, l.line)
		if err != nil {
			return false, err
		}
		toks, err = p.expandSingle(toks, l, 0)
		if err != nil {
			return false, err
		}
		v, err := p.as.evalConst(toks, l)
		if err != nil {
			return false, err
		}
		if strings.HasSuffix(name, "n") {
			return v == 0, nil
		}
		return v != 0, nil
	}
	return false, p.errorf(l, "unsupported conditional %s", name)
}
