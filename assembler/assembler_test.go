package assembler

import (
	"testing"
)

// TestLexerBasic tests basic tokenization
func TestLexerBasic(t *testing.T) {
	source := `MOV AX, 42
HLT`

	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Lexer failed: %v", err)
	}

	// Expected tokens: MOV, AX, comma, 42, newline, HLT, EOF
	// Note: Final newline may or may not be present depending on input
	expectedMinTokens := 7

	if len(tokens) < expectedMinTokens {
		t.Fatalf("Expected at least %d tokens, got %d", expectedMinTokens, len(tokens))
	}

	// Check the actual token types (skip final newline check as it may vary)
	expectedTypes := []TokenType{
		TokenInstruction, // MOV
		TokenRegister,    // AX
		TokenComma,       // ,
		TokenNumber,      // 42
		TokenNewline,     // \n
		TokenInstruction, // HLT
	}

	for i, expected := range expectedTypes {
		if tokens[i].Type != expected {
			t.Errorf("Token %d: expected type %d, got %d (value: %q)",
				i, expected, tokens[i].Type, tokens[i].Value)
		}
	}
}

// TestLexerLabelWithColon tests that labels followed by colons are tokenized correctly
func TestLexerLabelWithColon(t *testing.T) {
	source := `loop:
    MOV AX, BX
    JMP loop`

	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Lexer failed: %v", err)
	}

	// Find the "loop" token at the beginning
	if tokens[0].Type != TokenLabel {
		t.Errorf("Expected 'loop' to be TokenLabel, got %d", tokens[0].Type)
	}

	// The next token should be a colon
	if tokens[1].Type != TokenColon {
		t.Errorf("Expected colon after label, got %d", tokens[1].Type)
	}
}

// TestLexerInstructionNamedLoop tests that LOOP instruction is not confused with a label
func TestLexerInstructionNamedLoop(t *testing.T) {
	// When "loop" appears without a colon, it should be an instruction
	source := `LOOP target`

	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Lexer failed: %v", err)
	}

	if tokens[0].Type != TokenInstruction {
		t.Errorf("Expected 'LOOP' to be TokenInstruction, got %d", tokens[0].Type)
	}
}

// TestParserSimple tests basic instruction parsing
func TestParserSimple(t *testing.T) {
	source := `.code
MOV AX, 42
HLT`

	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Lexer failed: %v", err)
	}

	parser := NewParser(tokens)
	bytecode, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parser failed: %v", err)
	}

	// Should generate bytecode
	if len(bytecode) == 0 {
		t.Fatal("No bytecode generated")
	}

	// First byte should be OpMOV (0x01)
	if bytecode[0] != 0x01 {
		t.Errorf("Expected first byte to be 0x01 (MOV), got 0x%02X", bytecode[0])
	}

	// Last byte should be OpHLT (0x52)
	if bytecode[len(bytecode)-1] != 0x52 {
		t.Errorf("Expected last byte to be 0x52 (HLT), got 0x%02X", bytecode[len(bytecode)-1])
	}
}

// TestParserLabels tests label resolution
func TestParserLabels(t *testing.T) {
	source := `.code
start:
    MOV AX, 10
    INC AX
    JMP start`

	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Lexer failed: %v", err)
	}

	parser := NewParser(tokens)
	bytecode, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parser failed: %v", err)
	}

	// Debug: print bytecode
	t.Logf("Bytecode (%d bytes): %X", len(bytecode), bytecode)

	// The JMP instruction should jump to address 0 (start of start label)
	// Find the JMP instruction (opcode 0x40)
	jmpIdx := -1
	for i, b := range bytecode {
		if b == 0x40 { // OpJMP
			jmpIdx = i
			break
		}
	}

	if jmpIdx == -1 {
		t.Fatal("JMP instruction not found in bytecode")
	}

	// The jump target should be at jmpIdx+2 (after opcode and operand type)
	// We need at least 2 more bytes (operand type and value/address)
	if jmpIdx+2 >= len(bytecode) {
		t.Fatalf("JMP instruction incomplete, bytecode length: %d, jmpIdx: %d", len(bytecode), jmpIdx)
	}

	// JMP with immediate is encoded as: opcode (1 byte) + operand type (1 byte) + address (2 bytes)
	// But the actual address may be encoded as 8-bit or 16-bit depending on the value
	// Since the target is 0, it's encoded as OpTypeImm8
	if bytecode[jmpIdx+1] == 0x04 { // OpTypeImm8
		targetAddr := uint16(bytecode[jmpIdx+2])
		if targetAddr != 0 {
			t.Errorf("JMP target should be 0, got %d", targetAddr)
		}
	} else if bytecode[jmpIdx+1] == 0x03 { // OpTypeImm16
		targetAddr := uint16(bytecode[jmpIdx+2]) | (uint16(bytecode[jmpIdx+3]) << 8)
		if targetAddr != 0 {
			t.Errorf("JMP target should be 0, got %d (0x%04X)", targetAddr, targetAddr)
		}
	} else {
		t.Errorf("Unexpected operand type for JMP: 0x%02X", bytecode[jmpIdx+1])
	}
}

// TestRegisterSizes tests that 8-bit and 16-bit registers are encoded correctly
func TestRegisterSizes(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected byte // expected operand type
	}{
		{"16-bit register AX", "MOV AX, 42", 0x01}, // OpTypeReg16
		{"8-bit register AL", "MOV AL, 42", 0x02},  // OpTypeReg8
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.source)
			tokens, _ := lexer.Tokenize()
			parser := NewParser(tokens)
			bytecode, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parser failed: %v", err)
			}

			// bytecode[0] = opcode, bytecode[1] = operand type
			if bytecode[1] != tt.expected {
				t.Errorf("Expected operand type 0x%02X, got 0x%02X", tt.expected, bytecode[1])
			}
		})
	}
}

// TestNumberParsing tests different number formats
func TestNumberParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected uint16
	}{
		{"42", 42},
		{"0x2A", 42},
		{"2Ah", 42},
		{"0b101010", 42},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseNumber(tt.input)
			if err != nil {
				t.Fatalf("ParseNumber(%q) failed: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("ParseNumber(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestStringInstructions tests that string instructions are recognized and encoded correctly
func TestStringInstructions(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected byte // expected opcode
	}{
		{"MOVSB", "MOVSB", 0x70},
		{"MOVSW", "MOVSW", 0x71},
		{"STOSB", "STOSB", 0x72},
		{"STOSW", "STOSW", 0x73},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.source)
			tokens, err := lexer.Tokenize()
			if err != nil {
				t.Fatalf("Lexer failed: %v", err)
			}

			parser := NewParser(tokens)
			bytecode, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parser failed: %v", err)
			}

			// First byte should be the instruction opcode
			if len(bytecode) == 0 {
				t.Fatal("No bytecode generated")
			}

			if bytecode[0] != tt.expected {
				t.Errorf("Expected opcode 0x%02X, got 0x%02X", tt.expected, bytecode[0])
			}
		})
	}
}

// TestREPPrefix tests that REP prefix is handled correctly
func TestREPPrefix(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		expectedPrefix byte // REP prefix
		expectedOpcode byte // instruction opcode
	}{
		{"REP MOVSB", "REP MOVSB", 0xF3, 0x70},
		{"REP MOVSW", "REP MOVSW", 0xF3, 0x71},
		{"REP STOSB", "REP STOSB", 0xF3, 0x72},
		{"REP STOSW", "REP STOSW", 0xF3, 0x73},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.source)
			tokens, err := lexer.Tokenize()
			if err != nil {
				t.Fatalf("Lexer failed: %v", err)
			}

			parser := NewParser(tokens)
			bytecode, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parser failed: %v", err)
			}

			// Should have at least 2 bytes (REP prefix + opcode)
			if len(bytecode) < 2 {
				t.Fatalf("Expected at least 2 bytes, got %d", len(bytecode))
			}

			// First byte should be REP prefix (0xF3)
			if bytecode[0] != tt.expectedPrefix {
				t.Errorf("Expected REP prefix 0x%02X, got 0x%02X", tt.expectedPrefix, bytecode[0])
			}

			// Second byte should be the instruction opcode
			if bytecode[1] != tt.expectedOpcode {
				t.Errorf("Expected opcode 0x%02X, got 0x%02X", tt.expectedOpcode, bytecode[1])
			}
		})
	}
}
