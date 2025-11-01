package assembler

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// TokenType represents the type of token
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenNewline
	TokenLabel
	TokenInstruction
	TokenRegister
	TokenNumber
	TokenString
	TokenComma
	TokenColon
	TokenLeftBracket
	TokenRightBracket
	TokenPlus
	TokenMinus
	TokenDirective
	TokenComment
)

// Token represents a lexical token
type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

// Lexer tokenizes assembly source code
type Lexer struct {
	input  string
	pos    int
	line   int
	column int
	tokens []Token
}

// NewLexer creates a new lexer
func NewLexer(input string) *Lexer {
	return &Lexer{
		input:  input,
		pos:    0,
		line:   1,
		column: 1,
		tokens: make([]Token, 0),
	}
}

// Tokenize converts the input into tokens
func (l *Lexer) Tokenize() ([]Token, error) {
	for l.pos < len(l.input) {
		ch := l.current()

		// Skip whitespace (except newlines)
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.advance()
			continue
		}

		// Newline
		if ch == '\n' {
			l.addToken(TokenNewline, "\n")
			l.line++
			l.column = 1
			l.advance()
			continue
		}

		// Comment
		if ch == ';' {
			l.skipComment()
			continue
		}

		// Comma
		if ch == ',' {
			l.addToken(TokenComma, ",")
			l.advance()
			continue
		}

		// Colon
		if ch == ':' {
			l.addToken(TokenColon, ":")
			l.advance()
			continue
		}

		// Left bracket
		if ch == '[' {
			l.addToken(TokenLeftBracket, "[")
			l.advance()
			continue
		}

		// Right bracket
		if ch == ']' {
			l.addToken(TokenRightBracket, "]")
			l.advance()
			continue
		}

		// Plus
		if ch == '+' {
			l.addToken(TokenPlus, "+")
			l.advance()
			continue
		}

		// Minus (could be part of number)
		if ch == '-' {
			if l.peek() != 0 && unicode.IsDigit(rune(l.peek())) {
				l.readNumber()
			} else {
				l.addToken(TokenMinus, "-")
				l.advance()
			}
			continue
		}

		// Number
		if unicode.IsDigit(rune(ch)) || (ch == '0' && l.peek() == 'x') {
			l.readNumber()
			continue
		}

		// String
		if ch == '"' || ch == '\'' {
			if err := l.readString(); err != nil {
				return nil, err
			}
			continue
		}

		// Identifier (instruction, register, label, directive)
		if unicode.IsLetter(rune(ch)) || ch == '.' || ch == '_' {
			l.readIdentifier()
			continue
		}

		return nil, fmt.Errorf("unexpected character '%c' at line %d, column %d", ch, l.line, l.column)
	}

	l.addToken(TokenEOF, "")
	return l.tokens, nil
}

func (l *Lexer) current() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) peek() byte {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *Lexer) advance() {
	if l.pos < len(l.input) {
		l.pos++
		l.column++
	}
}

func (l *Lexer) addToken(tokenType TokenType, value string) {
	l.tokens = append(l.tokens, Token{
		Type:   tokenType,
		Value:  value,
		Line:   l.line,
		Column: l.column,
	})
}

func (l *Lexer) skipComment() {
	for l.current() != '\n' && l.current() != 0 {
		l.advance()
	}
}

func (l *Lexer) readNumber() {
	start := l.pos
	startCol := l.column

	// Handle negative sign
	if l.current() == '-' {
		l.advance()
	}

	// Hexadecimal (0x prefix)
	if l.current() == '0' && (l.peek() == 'x' || l.peek() == 'X') {
		l.advance() // 0
		l.advance() // x
		for unicode.IsDigit(rune(l.current())) || (l.current() >= 'a' && l.current() <= 'f') || (l.current() >= 'A' && l.current() <= 'F') {
			l.advance()
		}
	} else if l.current() != 0 && unicode.IsDigit(rune(l.current())) {
		// Could be decimal or hex with 'h' suffix
		// First, read all hex digits
		savedPos := l.pos
		for unicode.IsDigit(rune(l.current())) || (l.current() >= 'a' && l.current() <= 'f') || (l.current() >= 'A' && l.current() <= 'F') {
			l.advance()
		}
		// Check for 'h' suffix (indicates hexadecimal)
		if l.current() == 'h' || l.current() == 'H' {
			l.advance()
		} else {
			// No 'h' suffix, so it should be decimal only - rewind and read only digits
			l.pos = savedPos
			l.column = startCol + (l.pos - start)
			for unicode.IsDigit(rune(l.current())) {
				l.advance()
			}
		}
	}

	value := l.input[start:l.pos]
	l.tokens = append(l.tokens, Token{
		Type:   TokenNumber,
		Value:  value,
		Line:   l.line,
		Column: startCol,
	})
}

func (l *Lexer) readString() error {
	quote := l.current()
	l.advance() // Skip opening quote

	start := l.pos
	for l.current() != quote && l.current() != 0 && l.current() != '\n' {
		l.advance()
	}

	if l.current() != quote {
		return fmt.Errorf("unterminated string at line %d", l.line)
	}

	value := l.input[start:l.pos]
	l.advance() // Skip closing quote

	l.addToken(TokenString, value)
	return nil
}

func (l *Lexer) readIdentifier() {
	start := l.pos
	startCol := l.column

	// Read identifier
	for {
		ch := l.current()
		if ch == 0 {
			break
		}
		if !unicode.IsLetter(rune(ch)) && !unicode.IsDigit(rune(ch)) && ch != '_' && ch != '.' {
			break
		}
		l.advance()
	}

	value := l.input[start:l.pos]
	valueUpper := strings.ToUpper(value)

	// Determine token type
	var tokenType TokenType

	// Directive (starts with .)
	if value[0] == '.' {
		tokenType = TokenDirective
	} else if isRegister(valueUpper) {
		tokenType = TokenRegister
	} else if isInstruction(valueUpper) {
		// Check if this is followed by a colon - if so, it's a label, not an instruction
		if l.current() == ':' {
			tokenType = TokenLabel
		} else {
			tokenType = TokenInstruction
		}
	} else {
		tokenType = TokenLabel
	}

	l.tokens = append(l.tokens, Token{
		Type:   tokenType,
		Value:  value,
		Line:   l.line,
		Column: startCol,
	})
}

// ParseNumber parses a number token value
func ParseNumber(value string) (uint16, error) {
	value = strings.TrimSpace(value)

	// Negative numbers
	negative := false
	if strings.HasPrefix(value, "-") {
		negative = true
		value = value[1:]
	}

	var num int64
	var err error

	// Hexadecimal with 0x prefix
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		num, err = strconv.ParseInt(value[2:], 16, 32)
	} else if strings.HasSuffix(value, "h") || strings.HasSuffix(value, "H") {
		// Hexadecimal with h suffix
		num, err = strconv.ParseInt(value[:len(value)-1], 16, 32)
	} else if strings.HasPrefix(value, "0b") || strings.HasPrefix(value, "0B") {
		// Binary
		num, err = strconv.ParseInt(value[2:], 2, 32)
	} else {
		// Decimal
		num, err = strconv.ParseInt(value, 10, 32)
	}

	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", value)
	}

	if negative {
		num = -num
	}

	return uint16(num), nil
}

func isRegister(name string) bool {
	registers := []string{
		"AX", "BX", "CX", "DX",
		"AL", "AH", "BL", "BH", "CL", "CH", "DL", "DH",
		"SI", "DI", "BP", "SP",
		"IP", "FLAGS",
		"ES", "CS", "SS", "DS", // Segment registers (for compatibility)
	}

	for _, reg := range registers {
		if name == reg {
			return true
		}
	}
	return false
}

func isInstruction(name string) bool {
	instructions := []string{
		"MOV", "PUSH", "POP", "XCHG",
		"ADD", "SUB", "MUL", "DIV", "IMUL", "IDIV", "INC", "DEC", "NEG",
		"AND", "OR", "XOR", "NOT",
		"SHL", "SHR", "SAL", "SAR", "ROL", "ROR",
		"CMP", "TEST",
		"JMP", "JE", "JZ", "JNE", "JNZ",
		"JG", "JNLE", "JGE", "JNL", "JL", "JNGE", "JLE", "JNG",
		"JA", "JNBE", "JAE", "JNB", "JB", "JNAE", "JBE", "JNA",
		"CALL", "RET",
		"LOOP", "LOOPE", "LOOPZ", "LOOPNE", "LOOPNZ",
		"INT", "NOP", "HLT",
		"IN", "OUT", // I/O instructions
		"MOVSB", "MOVSW", "STOSB", "STOSW", // String instructions
		"REP", // REP prefix
		"DB", "DW", "DD", // Data directives
		"BYTE", "WORD", "DWORD",
	}

	for _, instr := range instructions {
		if name == instr {
			return true
		}
	}
	return false
}
