package assembler

import (
	"assembly-emulator/emulator"
	"fmt"
	"strings"
)

// Parser parses tokens into instructions
type Parser struct {
	tokens  []Token
	pos     int
	labels  map[string]uint16 // Label name -> address
	program []byte            // Generated machine code
	address uint16            // Current address
}

// NewParser creates a new parser
func NewParser(tokens []Token) *Parser {
	return &Parser{
		tokens:  tokens,
		pos:     0,
		labels:  make(map[string]uint16),
		program: make([]byte, 0),
		address: 0,
	}
}

// Parse parses the tokens and generates machine code
func (p *Parser) Parse() ([]byte, error) {
	// First pass: collect labels
	if err := p.firstPass(); err != nil {
		return nil, err
	}

	// Reset for second pass
	p.pos = 0
	p.address = 0
	p.program = make([]byte, 0)

	// Second pass: generate code
	if err := p.secondPass(); err != nil {
		return nil, err
	}

	return p.program, nil
}

func (p *Parser) firstPass() error {
	for !p.isAtEnd() {
		token := p.current()

		switch token.Type {
		case TokenLabel:
			// Check if next token is colon (label definition)
			if p.peekType() == TokenColon {
				p.labels[strings.ToUpper(token.Value)] = p.address
				p.advance() // label
				p.advance() // colon
			} else if p.peekType() == TokenInstruction {
				// Label on same line as instruction
				p.labels[strings.ToUpper(token.Value)] = p.address
				p.advance() // label
			} else {
				p.advance()
			}

		case TokenDirective:
			if err := p.parseDirectiveSize(); err != nil {
				return err
			}

		case TokenInstruction:
			if err := p.parseInstructionSize(); err != nil {
				return err
			}

		case TokenNewline, TokenComment:
			p.advance()

		default:
			p.advance()
		}
	}

	return nil
}

func (p *Parser) secondPass() error {
	for !p.isAtEnd() {
		token := p.current()

		switch token.Type {
		case TokenLabel:
			if p.peekType() == TokenColon {
				p.advance() // label
				p.advance() // colon
			} else {
				p.advance()
			}

		case TokenDirective:
			if err := p.parseDirective(); err != nil {
				return err
			}

		case TokenInstruction:
			if err := p.parseInstruction(); err != nil {
				return err
			}

		case TokenNewline, TokenComment:
			p.advance()

		default:
			return fmt.Errorf("unexpected token: %s at line %d", token.Value, token.Line)
		}
	}

	return nil
}

func (p *Parser) parseDirectiveSize() error {
	directive := strings.ToUpper(p.current().Value)
	p.advance()

	switch directive {
	case ".CODE", ".DATA", ".STACK":
		// Section directives don't generate code
		return nil

	default:
		return fmt.Errorf("unknown directive: %s", directive)
	}
}

func (p *Parser) parseDirective() error {
	directive := strings.ToUpper(p.current().Value)
	p.advance()

	switch directive {
	case ".CODE", ".DATA", ".STACK":
		// Section directives
		return nil

	default:
		return fmt.Errorf("unknown directive: %s", directive)
	}
}

func (p *Parser) parseInstructionSize() error {
	_ = strings.ToUpper(p.current().Value)
	p.advance()

	// Calculate actual instruction size by parsing operands
	size := 1 // Opcode

	// Parse operands and calculate their sizes
	for !p.isAtEnd() && p.current().Type != TokenNewline && p.current().Type != TokenComment {
		if p.current().Type == TokenComma {
			p.advance()
			continue
		}

		operandSize := p.getOperandSize()
		size += operandSize
	}

	p.address += uint16(size)
	return nil
}

func (p *Parser) getOperandSize() int {
	token := p.current()

	switch token.Type {
	case TokenRegister:
		p.advance()
		return 2 // type (1) + register code (1)

	case TokenNumber:
		p.advance()
		val, err := ParseNumber(token.Value)
		if err != nil {
			return 2 // conservative estimate
		}
		if val <= 0xFF {
			return 2 // type (1) + byte (1)
		}
		return 3 // type (1) + word (2)

	case TokenLabel:
		p.advance()
		// Labels are treated as immediate addresses (16-bit)
		return 3 // type (1) + address (2)

	case TokenLeftBracket:
		// Memory operand [...]
		p.advance() // skip [

		if p.current().Type == TokenRegister {
			p.advance() // register

			// Check for offset [REG+offset]
			if p.current().Type == TokenPlus {
				p.advance() // +
				if p.current().Type == TokenNumber {
					p.advance() // offset value
				}
			}

			if p.current().Type == TokenRightBracket {
				p.advance() // ]
			}
			return 4 // type (1) + reg code (1) + offset (2)
		} else if p.current().Type == TokenNumber {
			p.advance() // address
			if p.current().Type == TokenRightBracket {
				p.advance() // ]
			}
			return 3 // type (1) + address (2)
		}

		// Skip to closing bracket
		for !p.isAtEnd() && p.current().Type != TokenRightBracket {
			p.advance()
		}
		if p.current().Type == TokenRightBracket {
			p.advance()
		}
		return 3 // conservative estimate

	default:
		p.advance()
		return 2 // conservative estimate
	}
}

func (p *Parser) parseInstruction() error {
	instrToken := p.current()
	instr := strings.ToUpper(instrToken.Value)
	p.advance()

	// Parse operands
	var operands []Operand

	for !p.isAtEnd() && p.current().Type != TokenNewline && p.current().Type != TokenComment {
		if p.current().Type == TokenComma {
			p.advance()
			continue
		}

		operand, err := p.parseOperand()
		if err != nil {
			return fmt.Errorf("error parsing operand at line %d: %v", instrToken.Line, err)
		}

		operands = append(operands, operand)
	}

	// Generate code for instruction
	if err := p.generateInstruction(instr, operands); err != nil {
		return fmt.Errorf("error at line %d: %v", instrToken.Line, err)
	}

	return nil
}

func (p *Parser) parseOperand() (Operand, error) {
	token := p.current()

	switch token.Type {
	case TokenRegister:
		p.advance()
		return Operand{
			Type: OperandTypeRegister,
			Reg:  strings.ToUpper(token.Value),
		}, nil

	case TokenNumber:
		p.advance()
		val, err := ParseNumber(token.Value)
		if err != nil {
			return Operand{}, err
		}
		return Operand{
			Type:      OperandTypeImmediate,
			Immediate: val,
		}, nil

	case TokenLabel:
		// Label reference (for jumps)
		labelName := strings.ToUpper(token.Value)
		p.advance()

		addr, ok := p.labels[labelName]
		if !ok {
			return Operand{}, fmt.Errorf("undefined label: %s", labelName)
		}

		return Operand{
			Type:      OperandTypeImmediate,
			Immediate: addr,
			IsLabel:   true, // Mark as label so it always uses 16-bit encoding
		}, nil

	case TokenLeftBracket:
		// Memory operand [...]
		p.advance() // skip [

		if p.current().Type == TokenRegister {
			reg := strings.ToUpper(p.current().Value)
			p.advance()

			offset := uint16(0)

			// Check for offset [REG+offset]
			if p.current().Type == TokenPlus {
				p.advance()
				if p.current().Type == TokenNumber {
					val, err := ParseNumber(p.current().Value)
					if err != nil {
						return Operand{}, err
					}
					offset = val
					p.advance()
				}
			}

			if p.current().Type != TokenRightBracket {
				return Operand{}, fmt.Errorf("expected ] but got %s", p.current().Value)
			}
			p.advance() // skip ]

			return Operand{
				Type:   OperandTypeMemoryReg,
				Reg:    reg,
				Offset: offset,
			}, nil
		} else if p.current().Type == TokenNumber {
			// Direct memory address [1234h]
			val, err := ParseNumber(p.current().Value)
			if err != nil {
				return Operand{}, err
			}
			p.advance()

			if p.current().Type != TokenRightBracket {
				return Operand{}, fmt.Errorf("expected ]")
			}
			p.advance()

			return Operand{
				Type:    OperandTypeMemory,
				Address: val,
			}, nil
		}

		return Operand{}, fmt.Errorf("invalid memory operand")

	default:
		return Operand{}, fmt.Errorf("unexpected token in operand: %s", token.Value)
	}
}

func (p *Parser) skipOperand() {
	if p.current().Type == TokenLeftBracket {
		// Skip memory operand
		for !p.isAtEnd() && p.current().Type != TokenRightBracket {
			p.advance()
		}
		if p.current().Type == TokenRightBracket {
			p.advance()
		}
	} else {
		p.advance()
	}
}

func (p *Parser) skipToNewline() {
	for !p.isAtEnd() && p.current().Type != TokenNewline {
		p.advance()
	}
}

func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekType() TokenType {
	if p.pos+1 >= len(p.tokens) {
		return TokenEOF
	}
	return p.tokens[p.pos+1].Type
}

func (p *Parser) advance() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

func (p *Parser) isAtEnd() bool {
	return p.pos >= len(p.tokens) || p.current().Type == TokenEOF
}

// Operand represents a parsed operand
type Operand struct {
	Type      OperandType
	Reg       string
	Immediate uint16
	Address   uint16
	Offset    uint16
	IsLabel   bool // True if this immediate operand came from a label
}

type OperandType int

const (
	OperandTypeNone OperandType = iota
	OperandTypeRegister
	OperandTypeImmediate
	OperandTypeMemory
	OperandTypeMemoryReg
)

// Generate instruction bytecode (simplified encoding)
func (p *Parser) generateInstruction(instr string, operands []Operand) error {
	// Map instruction to opcode
	var opcode emulator.Opcode
	var ok bool

	opcodeMap := map[string]emulator.Opcode{
		"MOV":  emulator.OpMOV,
		"PUSH": emulator.OpPUSH,
		"POP":  emulator.OpPOP,
		"XCHG": emulator.OpXCHG,

		"ADD":  emulator.OpADD,
		"SUB":  emulator.OpSUB,
		"MUL":  emulator.OpMUL,
		"DIV":  emulator.OpDIV,
		"IMUL": emulator.OpIMUL,
		"IDIV": emulator.OpIDIV,
		"INC":  emulator.OpINC,
		"DEC":  emulator.OpDEC,
		"NEG":  emulator.OpNEG,

		"AND": emulator.OpAND,
		"OR":  emulator.OpOR,
		"XOR": emulator.OpXOR,
		"NOT": emulator.OpNOT,
		"SHL": emulator.OpSHL,
		"SHR": emulator.OpSHR,
		"SAL": emulator.OpSAL,
		"SAR": emulator.OpSAR,
		"ROL": emulator.OpROL,
		"ROR": emulator.OpROR,

		"CMP":  emulator.OpCMP,
		"TEST": emulator.OpTEST,

		"JMP":    emulator.OpJMP,
		"JE":     emulator.OpJE,
		"JZ":     emulator.OpJE,
		"JNE":    emulator.OpJNE,
		"JNZ":    emulator.OpJNE,
		"JG":     emulator.OpJG,
		"JNLE":   emulator.OpJG,
		"JGE":    emulator.OpJGE,
		"JNL":    emulator.OpJGE,
		"JL":     emulator.OpJL,
		"JNGE":   emulator.OpJL,
		"JLE":    emulator.OpJLE,
		"JNG":    emulator.OpJLE,
		"JA":     emulator.OpJA,
		"JAE":    emulator.OpJAE,
		"JB":     emulator.OpJB,
		"JBE":    emulator.OpJBE,
		"CALL":   emulator.OpCALL,
		"RET":    emulator.OpRET,
		"LOOP":   emulator.OpLOOP,
		"LOOPZ":  emulator.OpLOOPZ,
		"LOOPNZ": emulator.OpLOOPNZ,

		"INT": emulator.OpINT,
		"NOP": emulator.OpNOP,
		"HLT": emulator.OpHLT,

		"IN":  emulator.OpIN,
		"OUT": emulator.OpOUT,
	}

	opcode, ok = opcodeMap[instr]
	if !ok {
		return fmt.Errorf("unknown instruction: %s", instr)
	}

	// Emit opcode
	p.emit(byte(opcode))

	// Emit operands (simplified encoding)
	for _, op := range operands {
		p.emitOperand(op)
	}

	return nil
}

func (p *Parser) emit(b byte) {
	p.program = append(p.program, b)
	p.address++
}

func (p *Parser) emitWord(w uint16) {
	p.program = append(p.program, byte(w&0xFF))
	p.program = append(p.program, byte((w>>8)&0xFF))
	p.address += 2
}

func (p *Parser) emitOperand(op Operand) {
	switch op.Type {
	case OperandTypeRegister:
		// Check if register is 8-bit or 16-bit
		if is8BitRegister(op.Reg) {
			p.emit(byte(emulator.OpTypeReg8))
		} else {
			p.emit(byte(emulator.OpTypeReg16))
		}
		p.emit(encodeRegister(op.Reg))

	case OperandTypeImmediate:
		// Labels always use 16-bit encoding to match size calculation
		if op.IsLabel || op.Immediate > 0xFF {
			p.emit(byte(emulator.OpTypeImm16))
			p.emitWord(op.Immediate)
		} else {
			p.emit(byte(emulator.OpTypeImm8))
			p.emit(byte(op.Immediate))
		}

	case OperandTypeMemory:
		p.emit(byte(emulator.OpTypeMem))
		p.emitWord(op.Address)

	case OperandTypeMemoryReg:
		p.emit(byte(emulator.OpTypeMemReg))
		p.emit(encodeRegister(op.Reg))
		p.emitWord(op.Offset)
	}
}

func is8BitRegister(reg string) bool {
	switch reg {
	case "AL", "AH", "BL", "BH", "CL", "CH", "DL", "DH":
		return true
	default:
		return false
	}
}

func encodeRegister(reg string) byte {
	regMap := map[string]byte{
		"AX": 0, "BX": 1, "CX": 2, "DX": 3,
		"AL": 4, "AH": 5, "BL": 6, "BH": 7,
		"CL": 8, "CH": 9, "DL": 10, "DH": 11,
		"SI": 12, "DI": 13, "BP": 14, "SP": 15,
		// Segment registers
		"CS": 16, "DS": 17, "ES": 18, "SS": 19,
	}

	if code, ok := regMap[reg]; ok {
		return code
	}
	return 0
}
