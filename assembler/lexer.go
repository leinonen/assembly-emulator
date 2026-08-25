// Package assembler is a NASM-compatible x86 assembler producing flat
// binaries (the subset needed for DOS .COM programs: 16-bit code with
// 386 instructions and x87).
package assembler

import (
	"fmt"
	"strings"
)

type tokKind int

const (
	tkEOF tokKind = iota
	tkIdent
	tkNumber     // integer literal (text kept for later parsing)
	tkFloat      // floating-point literal
	tkString     // quoted string; Text holds the raw contents, Quote the delimiter
	tkPunct      // operators and punctuation
	tkDollar     // $
	tkDDollar    // $$
	tkMacroParam // %1, %0, %+, %%name, %{...}
)

type token struct {
	kind  tokKind
	text  string
	quote byte
	line  int
	col   int
	// number value cache
	num   int64
	numOK bool
	fval  float64
}

func (t token) String() string {
	switch t.kind {
	case tkString:
		return string(t.quote) + t.text + string(t.quote)
	case tkEOF:
		return "<eof>"
	}
	return t.text
}

func (t token) is(p string) bool { return t.kind == tkPunct && t.text == p }
func (t token) isIdent(s string) bool {
	return t.kind == tkIdent && strings.EqualFold(t.text, s)
}

type lexError struct {
	line int
	msg  string
}

func (e *lexError) Error() string { return fmt.Sprintf("line %d: %s", e.line, e.msg) }

var punct3 = []string{"<<=", ">>=", "<=>"}
var punct2 = []string{"<<", ">>", "==", "!=", "<>", "<=", ">=", "&&", "||", "^^", "//", "%%", "$$"}

// lexLine tokenises one source line (without the trailing newline).
func lexLine(s string, line int) ([]token, error) {
	var toks []token
	i := 0
	n := len(s)
	for i < n {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == ';':
			i = n
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < n && s[j] != c {
				if c == '`' && s[j] == '\\' && j+1 < n {
					j++
				}
				j++
			}
			if j >= n {
				return nil, &lexError{line, "unterminated string"}
			}
			toks = append(toks, token{kind: tkString, text: s[i+1 : j], quote: c, line: line, col: i})
			i = j + 1
		case c == '$':
			if i+1 < n && s[i+1] == '$' {
				toks = append(toks, token{kind: tkDDollar, text: "$$", line: line, col: i})
				i += 2
			} else if i+1 < n && isHexDigit(s[i+1]) && looksLikeDollarHex(s[i+1:]) {
				j := i + 1
				for j < n && (isAlnum(s[j]) || s[j] == '_') {
					j++
				}
				toks = append(toks, token{kind: tkNumber, text: s[i:j], line: line, col: i})
				i = j
			} else if i+1 < n && (isAlpha(s[i+1]) || s[i+1] == '_') {
				// $name: identifier that would otherwise be a keyword
				j := i + 1
				for j < n && (isAlnum(s[j]) || s[j] == '_' || s[j] == '.' || s[j] == '@' || s[j] == '?' || s[j] == '#' || s[j] == '~') {
					j++
				}
				toks = append(toks, token{kind: tkIdent, text: s[i+1 : j], line: line, col: i})
				i = j
			} else {
				toks = append(toks, token{kind: tkDollar, text: "$", line: line, col: i})
				i++
			}
		case c == '%':
			// Macro parameter references inside macro bodies: %1, %0, %+,
			// %%label, %{...}. At line start the preprocessor has already
			// consumed directives.
			if i+1 < n && (isDigit(s[i+1]) || s[i+1] == '+' || s[i+1] == '%' || s[i+1] == '{' || s[i+1] == '-' || s[i+1] == '$' || s[i+1] == '?' || s[i+1] == '[' || s[i+1] == '!') {
				j := i + 1
				switch s[j] {
				case '{':
					for j < n && s[j] != '}' {
						j++
					}
					j++
				case '[':
					depth := 0
					for j < n {
						if s[j] == '[' {
							depth++
						} else if s[j] == ']' {
							depth--
							if depth == 0 {
								j++
								break
							}
						}
						j++
					}
				case '+', '?':
					j++
				case '-':
					j++
					for j < n && isDigit(s[j]) {
						j++
					}
				case '%', '$', '!':
					j++
					for j < n && (isAlnum(s[j]) || s[j] == '_' || s[j] == '.' || s[j] == '@' || s[j] == '?' || s[j] == '#') {
						j++
					}
				default:
					for j < n && isDigit(s[j]) {
						j++
					}
				}
				toks = append(toks, token{kind: tkMacroParam, text: s[i:j], line: line, col: i})
				i = j
			} else {
				toks = append(toks, token{kind: tkPunct, text: "%", line: line, col: i})
				i++
			}
		case isDigit(c) || (c == '.' && i+1 < n && isDigit(s[i+1])):
			j := i
			isFloat := false
			for j < n && (isAlnum(s[j]) || s[j] == '_' || s[j] == '.') {
				if s[j] == '.' {
					isFloat = true
				}
				// exponent sign: 1.5e-3
				if (s[j] == 'e' || s[j] == 'E') && isFloat && j+1 < n && (s[j+1] == '-' || s[j+1] == '+') {
					j++
				}
				j++
			}
			txt := s[i:j]
			if isFloat && !strings.HasPrefix(strings.ToLower(txt), "0x") {
				toks = append(toks, token{kind: tkFloat, text: txt, line: line, col: i})
			} else {
				toks = append(toks, token{kind: tkNumber, text: txt, line: line, col: i})
			}
			i = j
		case isAlpha(c) || c == '_' || c == '.' || c == '?' || c == '@':
			j := i
			for j < n && (isAlnum(s[j]) || s[j] == '_' || s[j] == '.' || s[j] == '@' || s[j] == '?' || s[j] == '#' || s[j] == '~' || s[j] == '$') {
				j++
			}
			toks = append(toks, token{kind: tkIdent, text: s[i:j], line: line, col: i})
			i = j
		default:
			matched := false
			for _, p := range punct3 {
				if strings.HasPrefix(s[i:], p) {
					toks = append(toks, token{kind: tkPunct, text: p, line: line, col: i})
					i += 3
					matched = true
					break
				}
			}
			if matched {
				continue
			}
			for _, p := range punct2 {
				if strings.HasPrefix(s[i:], p) {
					toks = append(toks, token{kind: tkPunct, text: p, line: line, col: i})
					i += 2
					matched = true
					break
				}
			}
			if matched {
				continue
			}
			if strings.ContainsRune("+-*/%&|^~!()[],:<>=?", rune(c)) {
				toks = append(toks, token{kind: tkPunct, text: string(c), line: line, col: i})
				i++
			} else {
				return nil, &lexError{line, fmt.Sprintf("unexpected character %q", c)}
			}
		}
	}
	return toks, nil
}

func isDigit(c byte) bool    { return c >= '0' && c <= '9' }
func isHexDigit(c byte) bool { return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') }
func isAlpha(c byte) bool    { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isAlnum(c byte) bool    { return isAlpha(c) || isDigit(c) }

// looksLikeDollarHex: "$1F" is hex only if every char is a hex digit
// (NASM: "$" followed by a digit means hex).
func looksLikeDollarHex(s string) bool {
	return isDigit(s[0])
}

// parseNumber converts a NASM numeric literal to an integer.
func parseNumber(txt string) (int64, error) {
	t := strings.ReplaceAll(txt, "_", "")
	if t == "" {
		return 0, fmt.Errorf("bad number %q", txt)
	}
	lower := strings.ToLower(t)
	base := 10
	digits := lower
	switch {
	case strings.HasPrefix(lower, "0x"):
		base, digits = 16, lower[2:]
	case strings.HasPrefix(lower, "$"):
		base, digits = 16, lower[1:]
	case strings.HasPrefix(lower, "0h"):
		base, digits = 16, lower[2:]
	case strings.HasPrefix(lower, "0b"):
		base, digits = 2, lower[2:]
	case strings.HasPrefix(lower, "0y"):
		base, digits = 2, lower[2:]
	case strings.HasPrefix(lower, "0o") || strings.HasPrefix(lower, "0q"):
		base, digits = 8, lower[2:]
	case strings.HasPrefix(lower, "0d") || strings.HasPrefix(lower, "0t"):
		base, digits = 10, lower[2:]
	case strings.HasSuffix(lower, "h") || strings.HasSuffix(lower, "x"):
		base, digits = 16, lower[:len(lower)-1]
	case strings.HasSuffix(lower, "b") || strings.HasSuffix(lower, "y"):
		base, digits = 2, lower[:len(lower)-1]
	case strings.HasSuffix(lower, "o") || strings.HasSuffix(lower, "q"):
		base, digits = 8, lower[:len(lower)-1]
	case strings.HasSuffix(lower, "d") || strings.HasSuffix(lower, "t"):
		base, digits = 10, lower[:len(lower)-1]
	}
	if digits == "" {
		return 0, fmt.Errorf("bad number %q", txt)
	}
	var v uint64
	for i := 0; i < len(digits); i++ {
		c := digits[i]
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		default:
			return 0, fmt.Errorf("bad number %q", txt)
		}
		if d >= base {
			return 0, fmt.Errorf("bad digit in number %q", txt)
		}
		v = v*uint64(base) + uint64(d)
	}
	return int64(v), nil
}
