package assembler

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ExpressionEvaluator evaluates arithmetic expressions in constant definitions
type ExpressionEvaluator struct {
	constants map[string]uint16
	input     string
	pos       int
}

// NewExpressionEvaluator creates a new expression evaluator
func NewExpressionEvaluator(constants map[string]uint16) *ExpressionEvaluator {
	return &ExpressionEvaluator{
		constants: constants,
	}
}

// Evaluate parses and evaluates an expression string
func (e *ExpressionEvaluator) Evaluate(expr string) (uint16, error) {
	e.input = strings.TrimSpace(expr)
	e.pos = 0

	if e.input == "" {
		return 0, fmt.Errorf("empty expression")
	}

	result, err := e.parseExpression()
	if err != nil {
		return 0, err
	}

	// Make sure we consumed the entire input
	e.skipWhitespace()
	if e.pos < len(e.input) {
		return 0, fmt.Errorf("unexpected character at position %d: '%c'", e.pos, e.input[e.pos])
	}

	return uint16(result), nil
}

// parseExpression parses addition and subtraction (lowest precedence)
func (e *ExpressionEvaluator) parseExpression() (int, error) {
	left, err := e.parseTerm()
	if err != nil {
		return 0, err
	}

	for {
		e.skipWhitespace()
		if e.pos >= len(e.input) {
			break
		}

		op := e.input[e.pos]
		if op != '+' && op != '-' {
			break
		}

		e.pos++
		right, err := e.parseTerm()
		if err != nil {
			return 0, err
		}

		if op == '+' {
			left = left + right
		} else {
			left = left - right
		}
	}

	return left, nil
}

// parseTerm parses multiplication, division, and modulo (higher precedence)
func (e *ExpressionEvaluator) parseTerm() (int, error) {
	left, err := e.parseBitwise()
	if err != nil {
		return 0, err
	}

	for {
		e.skipWhitespace()
		if e.pos >= len(e.input) {
			break
		}

		// Check for operators
		var op string
		if e.matchKeyword("MOD") {
			op = "MOD"
		} else if e.input[e.pos] == '*' {
			op = "*"
			e.pos++
		} else if e.input[e.pos] == '/' {
			op = "/"
			e.pos++
		} else {
			break
		}

		right, err := e.parseBitwise()
		if err != nil {
			return 0, err
		}

		switch op {
		case "*":
			left = left * right
		case "/":
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left = left / right
		case "MOD":
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			left = left % right
		}
	}

	return left, nil
}

// parseBitwise parses bitwise operations (AND, OR, XOR, SHL, SHR)
func (e *ExpressionEvaluator) parseBitwise() (int, error) {
	left, err := e.parseUnary()
	if err != nil {
		return 0, err
	}

	for {
		e.skipWhitespace()
		if e.pos >= len(e.input) {
			break
		}

		var op string
		if e.matchKeyword("SHL") {
			op = "SHL"
		} else if e.matchKeyword("SHR") {
			op = "SHR"
		} else if e.matchKeyword("AND") {
			op = "AND"
		} else if e.matchKeyword("OR") {
			op = "OR"
		} else if e.matchKeyword("XOR") {
			op = "XOR"
		} else {
			break
		}

		right, err := e.parseUnary()
		if err != nil {
			return 0, err
		}

		switch op {
		case "SHL":
			left = left << uint(right)
		case "SHR":
			left = left >> uint(right)
		case "AND":
			left = left & right
		case "OR":
			left = left | right
		case "XOR":
			left = left ^ right
		}
	}

	return left, nil
}

// parseUnary parses unary operators (NOT, -)
func (e *ExpressionEvaluator) parseUnary() (int, error) {
	e.skipWhitespace()

	if e.pos >= len(e.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	// Unary NOT
	if e.matchKeyword("NOT") {
		value, err := e.parseUnary()
		if err != nil {
			return 0, err
		}
		return ^value, nil
	}

	// Unary minus
	if e.input[e.pos] == '-' {
		e.pos++
		value, err := e.parseUnary()
		if err != nil {
			return 0, err
		}
		return -value, nil
	}

	// Unary plus (just ignore it)
	if e.input[e.pos] == '+' {
		e.pos++
		return e.parseUnary()
	}

	return e.parseFactor()
}

// parseFactor parses numbers, constants, and parenthesized expressions
func (e *ExpressionEvaluator) parseFactor() (int, error) {
	e.skipWhitespace()

	if e.pos >= len(e.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	// Parenthesized expression
	if e.input[e.pos] == '(' {
		e.pos++
		value, err := e.parseExpression()
		if err != nil {
			return 0, err
		}

		e.skipWhitespace()
		if e.pos >= len(e.input) || e.input[e.pos] != ')' {
			return 0, fmt.Errorf("expected closing parenthesis")
		}
		e.pos++
		return value, nil
	}

	// Number literal
	if unicode.IsDigit(rune(e.input[e.pos])) ||
	   (e.input[e.pos] == '0' && e.pos+1 < len(e.input) && (e.input[e.pos+1] == 'x' || e.input[e.pos+1] == 'X')) {
		return e.parseNumber()
	}

	// Hex number with h suffix (like A000h) or constant reference
	if unicode.IsLetter(rune(e.input[e.pos])) || e.input[e.pos] == '_' {
		// Try to parse as hex number with h suffix first
		if e.isHexNumberWithSuffix() {
			return e.parseNumber()
		}
		// Otherwise parse as constant
		return e.parseConstant()
	}

	return 0, fmt.Errorf("unexpected character: '%c'", e.input[e.pos])
}

// parseNumber parses a numeric literal (hex, decimal, binary)
func (e *ExpressionEvaluator) parseNumber() (int, error) {
	start := e.pos

	// Hexadecimal with 0x prefix
	if e.input[e.pos] == '0' && e.pos+1 < len(e.input) &&
	   (e.input[e.pos+1] == 'x' || e.input[e.pos+1] == 'X') {
		e.pos += 2
		for e.pos < len(e.input) &&
		    (unicode.IsDigit(rune(e.input[e.pos])) ||
		     (e.input[e.pos] >= 'a' && e.input[e.pos] <= 'f') ||
		     (e.input[e.pos] >= 'A' && e.input[e.pos] <= 'F')) {
			e.pos++
		}
		numStr := e.input[start+2 : e.pos]
		val, err := strconv.ParseInt(numStr, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid hexadecimal number: %s", numStr)
		}
		return int(val), nil
	}

	// Binary with 0b prefix
	if e.input[e.pos] == '0' && e.pos+1 < len(e.input) &&
	   (e.input[e.pos+1] == 'b' || e.input[e.pos+1] == 'B') {
		e.pos += 2
		for e.pos < len(e.input) && (e.input[e.pos] == '0' || e.input[e.pos] == '1') {
			e.pos++
		}
		numStr := e.input[start+2 : e.pos]
		val, err := strconv.ParseInt(numStr, 2, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid binary number: %s", numStr)
		}
		return int(val), nil
	}

	// Decimal or hex with 'h' suffix
	savedPos := e.pos
	for e.pos < len(e.input) &&
	    (unicode.IsDigit(rune(e.input[e.pos])) ||
	     (e.input[e.pos] >= 'a' && e.input[e.pos] <= 'f') ||
	     (e.input[e.pos] >= 'A' && e.input[e.pos] <= 'F')) {
		e.pos++
	}

	// Check for 'h' suffix (hex)
	if e.pos < len(e.input) && (e.input[e.pos] == 'h' || e.input[e.pos] == 'H') {
		numStr := e.input[start:e.pos]
		e.pos++ // consume 'h'
		val, err := strconv.ParseInt(numStr, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid hexadecimal number: %s", numStr)
		}
		return int(val), nil
	}

	// Decimal number (rewind and read only digits)
	e.pos = savedPos
	for e.pos < len(e.input) && unicode.IsDigit(rune(e.input[e.pos])) {
		e.pos++
	}

	numStr := e.input[start:e.pos]
	val, err := strconv.ParseInt(numStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid decimal number: %s", numStr)
	}
	return int(val), nil
}

// parseConstant parses a constant reference
func (e *ExpressionEvaluator) parseConstant() (int, error) {
	start := e.pos

	for e.pos < len(e.input) &&
	    (unicode.IsLetter(rune(e.input[e.pos])) ||
	     unicode.IsDigit(rune(e.input[e.pos])) ||
	     e.input[e.pos] == '_') {
		e.pos++
	}

	name := strings.ToUpper(e.input[start:e.pos])

	// Check if it's actually an operator keyword
	if name == "MOD" || name == "SHL" || name == "SHR" ||
	   name == "AND" || name == "OR" || name == "XOR" || name == "NOT" {
		// Reset position and return error - this should be handled by operator parsing
		e.pos = start
		return 0, fmt.Errorf("unexpected operator: %s", name)
	}

	value, ok := e.constants[name]
	if !ok {
		return 0, fmt.Errorf("undefined constant: %s", name)
	}

	return int(value), nil
}

// matchKeyword checks if the current position matches a keyword and advances if so
func (e *ExpressionEvaluator) matchKeyword(keyword string) bool {
	e.skipWhitespace()

	if e.pos+len(keyword) > len(e.input) {
		return false
	}

	// Extract potential keyword
	potential := strings.ToUpper(e.input[e.pos : e.pos+len(keyword)])

	// Must match and be followed by non-letter (word boundary)
	if potential == keyword {
		// Check word boundary
		if e.pos+len(keyword) < len(e.input) {
			next := e.input[e.pos+len(keyword)]
			if unicode.IsLetter(rune(next)) || unicode.IsDigit(rune(next)) || next == '_' {
				return false
			}
		}
		e.pos += len(keyword)
		return true
	}

	return false
}

// skipWhitespace skips whitespace characters
func (e *ExpressionEvaluator) skipWhitespace() {
	for e.pos < len(e.input) &&
	    (e.input[e.pos] == ' ' || e.input[e.pos] == '\t') {
		e.pos++
	}
}

// isHexNumberWithSuffix checks if current position is a hex number with 'h' suffix
func (e *ExpressionEvaluator) isHexNumberWithSuffix() bool {
	// Look ahead to see if this looks like a hex number with h suffix
	saved := e.pos
	hasHexDigits := false

	// Must be all hex digits followed by 'h' or 'H'
	for e.pos < len(e.input) {
		ch := e.input[e.pos]
		if unicode.IsDigit(rune(ch)) ||
		   (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			hasHexDigits = true
			e.pos++
		} else if (ch == 'h' || ch == 'H') && hasHexDigits {
			// Found hex suffix
			e.pos = saved
			return true
		} else {
			break
		}
	}

	e.pos = saved
	return false
}
