package assembler

import (
	"testing"
)

// TestExpressionBasicArithmetic tests basic arithmetic operations
func TestExpressionBasicArithmetic(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected uint16
	}{
		{"simple addition", "2 + 3", 5},
		{"simple subtraction", "10 - 3", 7},
		{"simple multiplication", "4 * 5", 20},
		{"simple division", "20 / 4", 5},
		{"modulo", "17 MOD 5", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewExpressionEvaluator(make(map[string]uint16))
			result, err := eval.Evaluate(tt.expr)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestExpressionPrecedence tests operator precedence
func TestExpressionPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected uint16
	}{
		{"mult before add", "2 + 3 * 4", 14},
		{"div before sub", "20 - 12 / 3", 16},
		{"left to right", "10 - 5 - 2", 3},
		{"parentheses override", "(2 + 3) * 4", 20},
		{"nested parentheses", "((10 + 5) * 2) / 3", 10},
		{"complex", "100 / (2 + 3) * 4", 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewExpressionEvaluator(make(map[string]uint16))
			result, err := eval.Evaluate(tt.expr)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %d, got %d for '%s'", tt.expected, result, tt.expr)
			}
		})
	}
}

// TestExpressionBitwise tests bitwise operations
func TestExpressionBitwise(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected uint16
	}{
		{"shift left", "1 SHL 3", 8},
		{"shift right", "16 SHR 2", 4},
		{"and", "15 AND 7", 7},
		{"or", "8 OR 4", 12},
		{"xor", "15 XOR 7", 8},
		{"not", "NOT 0", 65535}, // NOT 0 = 0xFFFF
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewExpressionEvaluator(make(map[string]uint16))
			result, err := eval.Evaluate(tt.expr)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %d, got %d for '%s'", tt.expected, result, tt.expr)
			}
		})
	}
}

// TestExpressionConstants tests constant references
func TestExpressionConstants(t *testing.T) {
	constants := map[string]uint16{
		"WIDTH":  320,
		"HEIGHT": 200,
		"OFFSET": 100,
	}

	tests := []struct {
		name     string
		expr     string
		expected uint16
	}{
		{"simple reference", "WIDTH", 320},
		{"multiplication", "WIDTH * HEIGHT", 64000},
		{"division", "(WIDTH * HEIGHT) / 2", 32000},
		{"complex", "WIDTH * 2 + HEIGHT", 840},
		{"with offset", "WIDTH + OFFSET", 420},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewExpressionEvaluator(constants)
			result, err := eval.Evaluate(tt.expr)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %d, got %d for '%s'", tt.expected, result, tt.expr)
			}
		})
	}
}

// TestExpressionHexNumbers tests hexadecimal number parsing
func TestExpressionHexNumbers(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected uint16
	}{
		{"0x prefix", "0xA000", 0xA000},
		{"h suffix", "A000h", 0xA000},
		{"mixed hex", "0x10 + 20h", 0x30},
		{"hex mult", "0x10 * 2", 0x20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewExpressionEvaluator(make(map[string]uint16))
			result, err := eval.Evaluate(tt.expr)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected 0x%X, got 0x%X for '%s'", tt.expected, result, tt.expr)
			}
		})
	}
}

// TestExpressionUnary tests unary operators
func TestExpressionUnary(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected uint16
	}{
		{"unary minus", "-5 + 10", 5},
		{"unary plus", "+10", 10},
		{"double negative", "--5", 5},
		{"unary in expr", "10 + -5", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewExpressionEvaluator(make(map[string]uint16))
			result, err := eval.Evaluate(tt.expr)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %d, got %d for '%s'", tt.expected, result, tt.expr)
			}
		})
	}
}

// TestExpressionErrors tests error handling
func TestExpressionErrors(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		contains string // substring that should be in error message
	}{
		{"undefined constant", "UNDEFINED", "undefined constant"},
		{"division by zero", "10 / 0", "division by zero"},
		{"modulo by zero", "10 MOD 0", "modulo by zero"},
		{"missing closing paren", "(10 + 5", "closing parenthesis"},
		{"empty expression", "", "empty expression"},
		{"unexpected char", "10 @ 5", "unexpected character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewExpressionEvaluator(make(map[string]uint16))
			_, err := eval.Evaluate(tt.expr)
			if err == nil {
				t.Fatalf("Expected error for '%s', got none", tt.expr)
			}
			if !contains(err.Error(), tt.contains) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.contains, err)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		 findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestPreprocessorExpressions tests constant definitions with expressions
func TestPreprocessorExpressions(t *testing.T) {
	source := `WIDTH EQU 320
HEIGHT EQU 200
TOTAL EQU WIDTH * HEIGHT
HALF EQU TOTAL / 2
MOV AX, HALF
HLT`

	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Lexer failed: %v", err)
	}

	preprocessor := NewPreprocessor()
	tokens, err = preprocessor.Process(tokens)
	if err != nil {
		t.Fatalf("Preprocessor failed: %v", err)
	}

	parser := NewParser(tokens)
	program, err := parser.Parse(); bytecode := program.CodeBytes
	if err != nil {
		t.Fatalf("Parser failed: %v", err)
	}

	if len(bytecode) == 0 {
		t.Fatal("Expected bytecode to be generated")
	}

	// Verify constants
	constants := preprocessor.GetConstants()
	if val, ok := constants["WIDTH"]; !ok || val != 320 {
		t.Errorf("Expected WIDTH to be 320, got %d (exists: %v)", val, ok)
	}
	if val, ok := constants["HEIGHT"]; !ok || val != 200 {
		t.Errorf("Expected HEIGHT to be 200, got %d (exists: %v)", val, ok)
	}
	if val, ok := constants["TOTAL"]; !ok || val != 64000 {
		t.Errorf("Expected TOTAL to be 64000, got %d (exists: %v)", val, ok)
	}
	if val, ok := constants["HALF"]; !ok || val != 32000 {
		t.Errorf("Expected HALF to be 32000, got %d (exists: %v)", val, ok)
	}
}

// TestPreprocessorEQURedefinition tests that EQU constants cannot be redefined
func TestPreprocessorEQURedefinition(t *testing.T) {
	// EQU should not allow redefinition
	source := `VALUE EQU 10
VALUE EQU 20
HLT`

	lexer := NewLexer(source)
	tokens, _ := lexer.Tokenize()
	preprocessor := NewPreprocessor()
	_, err := preprocessor.Process(tokens)
	if err == nil {
		t.Fatal("Expected error when redefining EQU constant, got none")
	}
	if !contains(err.Error(), "cannot redefine") {
		t.Errorf("Expected 'cannot redefine' error, got: %v", err)
	}
}
