package assembler

import (
	"fmt"
	"strings"
)

// ConstantInfo stores information about a constant
type ConstantInfo struct {
	Value uint16
	Line  int // Line where defined (for error messages)
}

// Preprocessor handles constant definitions and substitution
type Preprocessor struct {
	constants map[string]*ConstantInfo
}

// NewPreprocessor creates a new preprocessor
func NewPreprocessor() *Preprocessor {
	return &Preprocessor{
		constants: make(map[string]*ConstantInfo),
	}
}

// Process preprocesses tokens to handle constant definitions and substitutions
func (pp *Preprocessor) Process(tokens []Token) ([]Token, error) {
	result := make([]Token, 0, len(tokens))

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		// Check for constant definition: LABEL EQU expr
		if token.Type == TokenLabel && i+1 < len(tokens) {
			next := tokens[i+1]

			// Check for EQU token
			if next.Type == TokenInstruction && strings.ToUpper(next.Value) == "EQU" {
				if i+2 >= len(tokens) {
					return nil, fmt.Errorf("expected expression after EQU at line %d", next.Line)
				}

				// Extract the expression (everything until newline/comment)
				exprTokens := []string{}
				exprStart := i + 2

				for j := exprStart; j < len(tokens) && tokens[j].Type != TokenNewline && tokens[j].Type != TokenComment; j++ {
					// Build expression string from tokens
					tok := tokens[j]
					if tok.Type == TokenComma {
						// Commas shouldn't be in constant expressions
						break
					}
					exprTokens = append(exprTokens, tok.Value)
				}

				if len(exprTokens) == 0 {
					return nil, fmt.Errorf("expected expression after EQU at line %d", next.Line)
				}

				// Join tokens into expression string
				exprStr := strings.Join(exprTokens, " ")

				// Evaluate the expression
				evaluator := NewExpressionEvaluator(pp.getConstantValues())
				value, err := evaluator.Evaluate(exprStr)
				if err != nil {
					return nil, fmt.Errorf("error evaluating constant '%s' at line %d: %v",
						token.Value, token.Line, err)
				}

				// Check if constant already exists (EQU constants cannot be redefined)
				constantName := strings.ToUpper(token.Value)
				if existing, exists := pp.constants[constantName]; exists {
					return nil, fmt.Errorf("cannot redefine constant '%s' at line %d (previously defined at line %d)",
						token.Value, token.Line, existing.Line)
				}

				// Store the constant
				pp.constants[constantName] = &ConstantInfo{
					Value: value,
					Line:  token.Line,
				}

				// Skip the constant definition (don't add to result)
				// Skip to end of line or comment
				i += 1 + len(exprTokens) // Skip label, EQU, and expression tokens
				for i+1 < len(tokens) && tokens[i+1].Type != TokenNewline {
					i++
				}

				continue
			}
		}

		// Substitute constant references with their values
		if token.Type == TokenLabel {
			constantName := strings.ToUpper(token.Value)
			if info, ok := pp.constants[constantName]; ok {
				// Replace label token with number token
				result = append(result, Token{
					Type:   TokenNumber,
					Value:  fmt.Sprintf("%d", info.Value),
					Line:   token.Line,
					Column: token.Column,
				})
				continue
			}
		}

		// Keep the token as-is
		result = append(result, token)
	}

	return result, nil
}

// getConstantValues returns a map of constant names to values for expression evaluation
func (pp *Preprocessor) getConstantValues() map[string]uint16 {
	values := make(map[string]uint16)
	for name, info := range pp.constants {
		values[name] = info.Value
	}
	return values
}

// GetConstants returns all defined constant values (useful for debugging)
func (pp *Preprocessor) GetConstants() map[string]uint16 {
	return pp.getConstantValues()
}
