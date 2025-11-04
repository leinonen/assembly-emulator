package assembler

import (
	"assembly-emulator/emulator"
	"fmt"
	"strings"
)

// SegmentType represents a memory segment type
type SegmentType int

const (
	SegmentCode SegmentType = iota
	SegmentData
	SegmentStack
)

// LabelInfo stores information about a label
type LabelInfo struct {
	Segment SegmentType
	Offset  uint16
}

// Program represents an assembled program with separate segments
type Program struct {
	CodeBytes []byte
	DataBytes []byte
	StackSize uint16
}

// Parser parses tokens into instructions
type Parser struct {
	tokens  []Token
	pos     int
	labels  map[string]LabelInfo // Label name -> segment and offset

	// Separate segments
	codeBytes []byte
	dataBytes []byte
	stackSize uint16

	// Current segment and address tracking
	currentSegment SegmentType
	codeAddress    uint16
	dataAddress    uint16
}

// NewParser creates a new parser
func NewParser(tokens []Token) *Parser {
	return &Parser{
		tokens:         tokens,
		pos:            0,
		labels:         make(map[string]LabelInfo),
		codeBytes:      make([]byte, 0),
		dataBytes:      make([]byte, 0),
		stackSize:      0x1000, // Default stack size of 4KB
		currentSegment: SegmentCode, // Start in code segment
		codeAddress:    0,
		dataAddress:    0,
	}
}

// Parse parses the tokens and generates machine code
func (p *Parser) Parse() (*Program, error) {
	// First pass: collect labels
	if err := p.firstPass(); err != nil {
		return nil, err
	}

	// Reset for second pass
	p.pos = 0
	p.codeAddress = 0
	p.dataAddress = 0
	p.codeBytes = make([]byte, 0)
	p.dataBytes = make([]byte, 0)
	p.currentSegment = SegmentCode // Reset to code segment

	// Second pass: generate code
	if err := p.secondPass(); err != nil {
		return nil, err
	}

	return &Program{
		CodeBytes: p.codeBytes,
		DataBytes: p.dataBytes,
		StackSize: p.stackSize,
	}, nil
}

func (p *Parser) firstPass() error {
	for !p.isAtEnd() {
		token := p.current()

		switch token.Type {
		case TokenLabel:
			// Check if next token is colon (label definition)
			if p.peekType() == TokenColon {
				p.labels[strings.ToUpper(token.Value)] = LabelInfo{
					Segment: p.currentSegment,
					Offset:  p.getCurrentAddress(),
				}
				p.advance() // label
				p.advance() // colon
			} else if p.peekType() == TokenInstruction {
				// Label on same line as instruction
				p.labels[strings.ToUpper(token.Value)] = LabelInfo{
					Segment: p.currentSegment,
					Offset:  p.getCurrentAddress(),
				}
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

// getCurrentAddress returns the current address based on the active segment
func (p *Parser) getCurrentAddress() uint16 {
	switch p.currentSegment {
	case SegmentCode:
		return p.codeAddress
	case SegmentData:
		return p.dataAddress
	default:
		return 0
	}
}

// incrementAddress increments the current segment's address
func (p *Parser) incrementAddress(amount uint16) {
	switch p.currentSegment {
	case SegmentCode:
		p.codeAddress += amount
	case SegmentData:
		p.dataAddress += amount
	}
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
	case ".CODE":
		p.currentSegment = SegmentCode
		return nil
	case ".DATA":
		p.currentSegment = SegmentData
		return nil
	case ".STACK":
		p.currentSegment = SegmentStack
		// Optionally parse stack size if provided
		// For now, use default stack size
		return nil

	default:
		return fmt.Errorf("unknown directive: %s", directive)
	}
}

func (p *Parser) parseDirective() error {
	directive := strings.ToUpper(p.current().Value)
	p.advance()

	switch directive {
	case ".CODE":
		p.currentSegment = SegmentCode
		return nil
	case ".DATA":
		p.currentSegment = SegmentData
		return nil
	case ".STACK":
		p.currentSegment = SegmentStack
		return nil

	default:
		return fmt.Errorf("unknown directive: %s", directive)
	}
}

func (p *Parser) parseInstructionSize() error {
	instr := strings.ToUpper(p.current().Value)
	p.advance()

	// Handle DB/DW/DD directives - each value is 1/2/4 bytes
	if instr == "DB" || instr == "DW" || instr == "DD" {
		var bytesPerValue uint16
		switch instr {
		case "DB":
			bytesPerValue = 1
		case "DW":
			bytesPerValue = 2
		case "DD":
			bytesPerValue = 4
		}

		// Count the total number of bytes
		totalBytes := uint16(0)
		for !p.isAtEnd() && p.current().Type != TokenNewline && p.current().Type != TokenComment {
			if p.current().Type == TokenComma {
				p.advance()
				continue
			}

			// Handle strings - each character becomes one byte
			if p.current().Type == TokenString {
				// For DB, each character is 1 byte
				// For DW/DD, this would be an error (handled in parseDataDirective)
				// Count RUNES (characters), not UTF-8 bytes!
				totalBytes += uint16(len([]rune(p.current().Value)))
				p.advance()
			} else {
				// Regular numeric value
				totalBytes += bytesPerValue
				p.advance()
			}
		}

		p.incrementAddress(totalBytes)
		return nil
	}

	// Calculate actual instruction size by parsing operands
	size := 1 // Opcode

	// Check for REP prefix
	if instr == "REP" {
		size++ // REP prefix byte (0xF3)
		// Get the actual instruction after REP
		if !p.isAtEnd() && p.current().Type == TokenInstruction {
			p.advance()
		}
	}

	// Parse operands and calculate their sizes
	for !p.isAtEnd() && p.current().Type != TokenNewline && p.current().Type != TokenComment {
		if p.current().Type == TokenComma {
			p.advance()
			continue
		}

		operandSize := p.getOperandSize()
		size += operandSize
	}

	p.incrementAddress(uint16(size))
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

	// Handle DB (define byte) directive
	if instr == "DB" || instr == "DW" || instr == "DD" {
		return p.parseDataDirective(instr, instrToken.Line)
	}

	// Check for REP prefix
	hasREP := false
	if instr == "REP" {
		hasREP = true
		// Get the actual instruction after REP
		if p.isAtEnd() || p.current().Type != TokenInstruction {
			return fmt.Errorf("expected instruction after REP prefix at line %d", instrToken.Line)
		}
		instrToken = p.current()
		instr = strings.ToUpper(instrToken.Value)
		p.advance()
	}

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
	if err := p.generateInstruction(instr, operands, hasREP); err != nil {
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
		// Label reference (for jumps or data access)
		labelName := strings.ToUpper(token.Value)
		p.advance()

		labelInfo, ok := p.labels[labelName]
		if !ok {
			return Operand{}, fmt.Errorf("undefined label: %s", labelName)
		}

		// For now, we'll use the offset within the segment
		// TODO: Handle cross-segment references (code accessing data labels)
		// For data labels referenced from code, we'll need to calculate the linear address
		addr := labelInfo.Offset

		// If this is a data label referenced from code segment, calculate actual address
		if p.currentSegment == SegmentCode && labelInfo.Segment == SegmentData {
			// Data will be loaded after code, so calculate the actual linear address
			// This will be: codeSize + dataOffset
			// We'll need to fix this up later in a more sophisticated way
			// For now, just use the offset - we'll handle this properly in the loader
			addr = labelInfo.Offset
		}

		return Operand{
			Type:         OperandTypeImmediate,
			Immediate:    addr,
			IsLabel:      true,          // Mark as label so it always uses 16-bit encoding
			LabelSegment: labelInfo.Segment, // Store segment info for later use
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
	Type         OperandType
	Reg          string
	Immediate    uint16
	Address      uint16
	Offset       uint16
	IsLabel      bool        // True if this immediate operand came from a label
	LabelSegment SegmentType // Segment the label belongs to (for cross-segment refs)
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
func (p *Parser) generateInstruction(instr string, operands []Operand, hasREP bool) error {
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

		// String instructions
		"MOVSB": emulator.OpMOVSB,
		"MOVSW": emulator.OpMOVSW,
		"STOSB": emulator.OpSTOSB,
		"STOSW": emulator.OpSTOSW,
		"LODSB": emulator.OpLODSB,
		"LODSW": emulator.OpLODSW,
	}

	opcode, ok = opcodeMap[instr]
	if !ok {
		return fmt.Errorf("unknown instruction: %s", instr)
	}

	// Emit REP prefix if present
	if hasREP {
		p.emit(0xF3)
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
	switch p.currentSegment {
	case SegmentCode:
		p.codeBytes = append(p.codeBytes, b)
		p.codeAddress++
	case SegmentData:
		p.dataBytes = append(p.dataBytes, b)
		p.dataAddress++
	}
}

func (p *Parser) emitWord(w uint16) {
	switch p.currentSegment {
	case SegmentCode:
		p.codeBytes = append(p.codeBytes, byte(w&0xFF))
		p.codeBytes = append(p.codeBytes, byte((w>>8)&0xFF))
		p.codeAddress += 2
	case SegmentData:
		p.dataBytes = append(p.dataBytes, byte(w&0xFF))
		p.dataBytes = append(p.dataBytes, byte((w>>8)&0xFF))
		p.dataAddress += 2
	}
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

func (p *Parser) parseDataDirective(directive string, line int) error {
	// Parse comma-separated list of values and emit them as raw bytes
	for !p.isAtEnd() && p.current().Type != TokenNewline && p.current().Type != TokenComment {
		if p.current().Type == TokenComma {
			p.advance()
			continue
		}

		// Handle string literals (only valid for DB directive)
		if p.current().Type == TokenString {
			if directive != "DB" {
				return fmt.Errorf("string literals are only supported in DB directive at line %d", line)
			}

			// Process escape sequences and convert to CP437
			stringValue := p.processEscapeSequences(p.current().Value)
			bytes, err := p.stringToCP437Bytes(stringValue)
			if err != nil {
				return fmt.Errorf("error converting string to CP437 at line %d: %v", line, err)
			}

			// Emit each byte
			for _, b := range bytes {
				p.emit(b)
			}

			p.advance()
			continue
		}

		// Handle numeric values
		if p.current().Type != TokenNumber {
			return fmt.Errorf("expected number or string in %s directive at line %d", directive, line)
		}

		val, err := ParseNumber(p.current().Value)
		if err != nil {
			return fmt.Errorf("invalid number in %s directive at line %d: %v", directive, line, err)
		}

		switch directive {
		case "DB":
			// Emit byte
			if val > 0xFF {
				return fmt.Errorf("value %d too large for DB directive at line %d", val, line)
			}
			p.emit(byte(val))

		case "DW":
			// Emit word (16-bit, little-endian)
			p.emitWord(val)

		case "DD":
			// Emit dword (32-bit, little-endian)
			p.emit(byte(val & 0xFF))
			p.emit(byte((val >> 8) & 0xFF))
			p.emit(byte((val >> 16) & 0xFF))
			p.emit(byte((val >> 24) & 0xFF))
		}

		p.advance()
	}

	return nil
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

// processEscapeSequences processes escape sequences in a string
// Supports: \n (newline), \r (carriage return), \t (tab), \\ (backslash), \" (quote), \' (single quote)
func (p *Parser) processEscapeSequences(s string) string {
	result := make([]rune, 0, len(s))
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] == '\\' && i+1 < len(runes) {
			// Escape sequence
			switch runes[i+1] {
			case 'n':
				result = append(result, '\n')
				i += 2
			case 'r':
				result = append(result, '\r')
				i += 2
			case 't':
				result = append(result, '\t')
				i += 2
			case '\\':
				result = append(result, '\\')
				i += 2
			case '"':
				result = append(result, '"')
				i += 2
			case '\'':
				result = append(result, '\'')
				i += 2
			default:
				// Unknown escape sequence - keep the backslash
				result = append(result, runes[i])
				i++
			}
		} else {
			result = append(result, runes[i])
			i++
		}
	}
	return string(result)
}

// stringToCP437Bytes converts a UTF-8 string to CP437 bytes
// Uses the CP437 encoding from the graphics package
func (p *Parser) stringToCP437Bytes(s string) ([]byte, error) {
	// Import the graphics package function
	// We'll use a local implementation to avoid circular dependencies
	result := make([]byte, 0, len(s))
	for _, r := range s {
		// Use the CP437 encoding table
		b, err := runeToCP437Byte(r)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}

// runeToCP437Byte converts a single rune to its CP437 byte equivalent
// This is a simplified implementation that covers the most common cases
func runeToCP437Byte(r rune) (byte, error) {
	// Build a reverse mapping from the CP437 table
	// For simplicity, we'll hardcode the common box-drawing characters and ASCII

	// ASCII range (0-127) maps directly
	if r >= 0 && r <= 127 {
		return byte(r), nil
	}

	// Extended ASCII / special characters mapping
	cp437Map := map[rune]byte{
		'☺': 0x01, '☻': 0x02, '♥': 0x03, '♦': 0x04, '♣': 0x05, '♠': 0x06,
		'•': 0x07, '◘': 0x08, '○': 0x09, '◙': 0x0A, '♂': 0x0B, '♀': 0x0C,
		'♪': 0x0D, '♫': 0x0E, '☼': 0x0F, '►': 0x10, '◄': 0x11, '↕': 0x12,
		'‼': 0x13, '¶': 0x14, '§': 0x15, '▬': 0x16, '↨': 0x17, '↑': 0x18,
		'↓': 0x19, '→': 0x1A, '←': 0x1B, '∟': 0x1C, '↔': 0x1D, '▲': 0x1E,
		'▼': 0x1F, '⌂': 0x7F,
		// Extended Latin characters
		'Ç': 0x80, 'ü': 0x81, 'é': 0x82, 'â': 0x83, 'ä': 0x84, 'à': 0x85,
		'å': 0x86, 'ç': 0x87, 'ê': 0x88, 'ë': 0x89, 'è': 0x8A, 'ï': 0x8B,
		'î': 0x8C, 'ì': 0x8D, 'Ä': 0x8E, 'Å': 0x8F, 'É': 0x90, 'æ': 0x91,
		'Æ': 0x92, 'ô': 0x93, 'ö': 0x94, 'ò': 0x95, 'û': 0x96, 'ù': 0x97,
		'ÿ': 0x98, 'Ö': 0x99, 'Ü': 0x9A, '¢': 0x9B, '£': 0x9C, '¥': 0x9D,
		'₧': 0x9E, 'ƒ': 0x9F, 'á': 0xA0, 'í': 0xA1, 'ó': 0xA2, 'ú': 0xA3,
		'ñ': 0xA4, 'Ñ': 0xA5, 'ª': 0xA6, 'º': 0xA7, '¿': 0xA8, '⌐': 0xA9,
		'¬': 0xAA, '½': 0xAB, '¼': 0xAC, '¡': 0xAD, '«': 0xAE, '»': 0xAF,
		// Box-drawing characters (the most important ones for this use case)
		'░': 0xB0, '▒': 0xB1, '▓': 0xB2, '│': 0xB3, '┤': 0xB4, '╡': 0xB5,
		'╢': 0xB6, '╖': 0xB7, '╕': 0xB8, '╣': 0xB9, '║': 0xBA, '╗': 0xBB,
		'╝': 0xBC, '╜': 0xBD, '╛': 0xBE, '┐': 0xBF, '└': 0xC0, '┴': 0xC1,
		'┬': 0xC2, '├': 0xC3, '─': 0xC4, '┼': 0xC5, '╞': 0xC6, '╟': 0xC7,
		'╚': 0xC8, '╔': 0xC9, '╩': 0xCA, '╦': 0xCB, '╠': 0xCC, '═': 0xCD,
		'╬': 0xCE, '╧': 0xCF, '╨': 0xD0, '╤': 0xD1, '╥': 0xD2, '╙': 0xD3,
		'╘': 0xD4, '╒': 0xD5, '╓': 0xD6, '╫': 0xD7, '╪': 0xD8, '┘': 0xD9,
		'┌': 0xDA, '█': 0xDB, '▄': 0xDC, '▌': 0xDD, '▐': 0xDE, '▀': 0xDF,
		// Greek and math symbols
		'α': 0xE0, 'ß': 0xE1, 'Γ': 0xE2, 'π': 0xE3, 'Σ': 0xE4, 'σ': 0xE5,
		'µ': 0xE6, 'τ': 0xE7, 'Φ': 0xE8, 'Θ': 0xE9, 'Ω': 0xEA, 'δ': 0xEB,
		'∞': 0xEC, 'φ': 0xED, 'ε': 0xEE, '∩': 0xEF, '≡': 0xF0, '±': 0xF1,
		'≥': 0xF2, '≤': 0xF3, '⌠': 0xF4, '⌡': 0xF5, '÷': 0xF6, '≈': 0xF7,
		'°': 0xF8, '∙': 0xF9, '·': 0xFA, '√': 0xFB, 'ⁿ': 0xFC, '²': 0xFD,
		'■': 0xFE, '\u00A0': 0xFF,
	}

	if b, ok := cp437Map[r]; ok {
		return b, nil
	}

	return 0, fmt.Errorf("character '%c' (U+%04X) cannot be represented in CP437", r, r)
}
