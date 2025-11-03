package emulator

import (
	"testing"
)

// TestBasicMOV tests basic MOV instructions
func TestBasicMOV(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 42
	cpu.Memory.RAM[0] = 0x01 // OpMOV
	cpu.Memory.RAM[1] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = 0x04 // OpTypeImm8
	cpu.Memory.RAM[4] = 42   // value

	// HLT
	cpu.Memory.RAM[5] = 0x52 // OpHLT

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	if cpu.AX != 42 {
		t.Errorf("Expected AX=42, got AX=%d", cpu.AX)
	}
}

// TestADD tests ADD instruction
func TestADD(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 10
	cpu.Memory.RAM[0] = 0x01 // OpMOV
	cpu.Memory.RAM[1] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = 0x04 // OpTypeImm8
	cpu.Memory.RAM[4] = 10

	// MOV BX, 32
	cpu.Memory.RAM[5] = 0x01  // OpMOV
	cpu.Memory.RAM[6] = 0x01  // OpTypeReg16
	cpu.Memory.RAM[7] = 0x01  // BX
	cpu.Memory.RAM[8] = 0x04  // OpTypeImm8
	cpu.Memory.RAM[9] = 32

	// ADD AX, BX
	cpu.Memory.RAM[10] = 0x10 // OpADD
	cpu.Memory.RAM[11] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[12] = 0x00 // AX
	cpu.Memory.RAM[13] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[14] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[15] = 0x52 // OpHLT

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	if cpu.AX != 42 {
		t.Errorf("Expected AX=42, got AX=%d", cpu.AX)
	}
}

// TestMUL tests MUL instruction
func TestMUL(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 100
	cpu.Memory.RAM[0] = 0x01 // OpMOV
	cpu.Memory.RAM[1] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = 0x04 // OpTypeImm8
	cpu.Memory.RAM[4] = 100

	// MOV DX, 320 (0x140)
	cpu.Memory.RAM[5] = 0x01  // OpMOV
	cpu.Memory.RAM[6] = 0x01  // OpTypeReg16
	cpu.Memory.RAM[7] = 0x03  // DX
	cpu.Memory.RAM[8] = 0x03  // OpTypeImm16
	cpu.Memory.RAM[9] = 0x40  // low byte
	cpu.Memory.RAM[10] = 0x01 // high byte

	// MUL DX
	cpu.Memory.RAM[11] = 0x12 // OpMUL
	cpu.Memory.RAM[12] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[13] = 0x03 // DX

	// HLT
	cpu.Memory.RAM[14] = 0x52 // OpHLT

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// 100 * 320 = 32000 = 0x7D00
	expected := uint16(32000)
	if cpu.AX != expected {
		t.Errorf("Expected AX=%d (0x%04X), got AX=%d (0x%04X)", expected, expected, cpu.AX, cpu.AX)
	}
	if cpu.DX != 0 {
		t.Errorf("Expected DX=0, got DX=%d", cpu.DX)
	}
}

// TestJumpAndLoop tests jump and loop instructions
func TestJumpAndLoop(t *testing.T) {
	cpu := NewCPU()

	// MOV CX, 3
	cpu.Memory.RAM[0] = 0x01 // OpMOV
	cpu.Memory.RAM[1] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[2] = 0x02 // CX
	cpu.Memory.RAM[3] = 0x04 // OpTypeImm8
	cpu.Memory.RAM[4] = 3

	// MOV AX, 0
	cpu.Memory.RAM[5] = 0x01 // OpMOV
	cpu.Memory.RAM[6] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[7] = 0x00 // AX
	cpu.Memory.RAM[8] = 0x04 // OpTypeImm8
	cpu.Memory.RAM[9] = 0

	// loop_start: (address 0x000A)
	// INC AX
	cpu.Memory.RAM[10] = 0x16 // OpINC
	cpu.Memory.RAM[11] = 0x01 // OpTypeReg16
	cpu.Memory.RAM[12] = 0x00 // AX

	// LOOP loop_start (back to 0x000A)
	cpu.Memory.RAM[13] = 0x4D // OpLOOP
	cpu.Memory.RAM[14] = 0x03 // OpTypeImm16
	cpu.Memory.RAM[15] = 0x0A // low byte of address
	cpu.Memory.RAM[16] = 0x00 // high byte of address

	// HLT
	cpu.Memory.RAM[17] = 0x52 // OpHLT

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// AX should be incremented 3 times
	if cpu.AX != 3 {
		t.Errorf("Expected AX=3, got AX=%d", cpu.AX)
	}
	// CX should be 0
	if cpu.CX != 0 {
		t.Errorf("Expected CX=0, got CX=%d", cpu.CX)
	}
}

// TestVGAMemory tests VGA memory access
func TestVGAMemory(t *testing.T) {
	cpu := NewCPU()

	// Write to VGA memory at linear address 0xA0000 (segment 0xA000, offset 0)
	cpu.Memory.WriteByteLinear(0xA0000, 15) // White pixel

	// Read it back
	val := cpu.Memory.ReadByteLinear(0xA0000)
	if val != 15 {
		t.Errorf("Expected VGA[0]=15, got %d", val)
	}

	// Verify it's in VGA memory
	if cpu.Memory.VGA[0] != 15 {
		t.Error("Value was not written to VGA memory")
	}

	// Verify it's also mirrored in RAM at 0xA0000
	if cpu.Memory.RAM[0xA0000] != 15 {
		t.Error("Value was not mirrored to RAM[0xA0000]")
	}
}

// TestSegmentedAddressing tests segment:offset to linear address conversion
func TestSegmentedAddressing(t *testing.T) {
	cpu := NewCPU()

	// Test that segment:offset addressing works correctly
	// Write to segment 0x1000, offset 0x0050 (linear = 0x10050)
	linearAddr := CalculateLinearAddress(0x1000, 0x0050)
	if linearAddr != 0x10050 {
		t.Errorf("Expected linear address 0x10050, got 0x%05X", linearAddr)
	}

	// Write and read back
	cpu.Memory.WriteByteLinear(linearAddr, 0x42)
	val := cpu.Memory.ReadByteLinear(linearAddr)
	if val != 0x42 {
		t.Errorf("Expected 0x42, got 0x%02X", val)
	}
}

// TestSUB tests SUB instruction
func TestSUB(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 50
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 50

	// MOV BX, 8
	cpu.Memory.RAM[5] = byte(OpMOV)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x01 // BX
	cpu.Memory.RAM[8] = byte(OpTypeImm8)
	cpu.Memory.RAM[9] = 8

	// SUB AX, BX
	cpu.Memory.RAM[10] = byte(OpSUB)
	cpu.Memory.RAM[11] = byte(OpTypeReg16)
	cpu.Memory.RAM[12] = 0x00 // AX
	cpu.Memory.RAM[13] = byte(OpTypeReg16)
	cpu.Memory.RAM[14] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[15] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	if cpu.AX != 42 {
		t.Errorf("Expected AX=42, got AX=%d", cpu.AX)
	}

	// Flags should be updated
	if cpu.Flags.ZF {
		t.Error("Expected ZF=false")
	}
	if cpu.Flags.CF {
		t.Error("Expected CF=false (no borrow)")
	}
}

// TestINC tests INC instruction
func TestINC(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 5
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 5

	// INC AX
	cpu.Memory.RAM[5] = byte(OpINC)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x00 // AX

	// INC AX
	cpu.Memory.RAM[8] = byte(OpINC)
	cpu.Memory.RAM[9] = byte(OpTypeReg16)
	cpu.Memory.RAM[10] = 0x00 // AX

	// HLT
	cpu.Memory.RAM[11] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	if cpu.AX != 7 {
		t.Errorf("Expected AX=7, got AX=%d", cpu.AX)
	}
}

// TestDEC tests DEC instruction
func TestDEC(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 10
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 10

	// DEC AX
	cpu.Memory.RAM[5] = byte(OpDEC)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x00 // AX

	// DEC AX
	cpu.Memory.RAM[8] = byte(OpDEC)
	cpu.Memory.RAM[9] = byte(OpTypeReg16)
	cpu.Memory.RAM[10] = 0x00 // AX

	// HLT
	cpu.Memory.RAM[11] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	if cpu.AX != 8 {
		t.Errorf("Expected AX=8, got AX=%d", cpu.AX)
	}
}

// TestNEG tests NEG instruction
func TestNEG(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 42
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 42

	// NEG AX
	cpu.Memory.RAM[5] = byte(OpNEG)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x00 // AX

	// HLT
	cpu.Memory.RAM[8] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// -42 in two's complement (16-bit) is 65494
	expected := uint16(65494)
	if cpu.AX != expected {
		t.Errorf("Expected AX=%d (0x%04X), got AX=%d (0x%04X)", expected, expected, cpu.AX, cpu.AX)
	}

	// CF should be set (NEG sets CF when operand is non-zero)
	if !cpu.Flags.CF {
		t.Error("Expected CF=true")
	}
}

// TestDIV tests DIV instruction
func TestDIV(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 100
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 100

	// MOV DX, 0 (high word of dividend)
	cpu.Memory.RAM[5] = byte(OpMOV)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x03 // DX
	cpu.Memory.RAM[8] = byte(OpTypeImm8)
	cpu.Memory.RAM[9] = 0

	// MOV BX, 7 (divisor)
	cpu.Memory.RAM[10] = byte(OpMOV)
	cpu.Memory.RAM[11] = byte(OpTypeReg16)
	cpu.Memory.RAM[12] = 0x01 // BX
	cpu.Memory.RAM[13] = byte(OpTypeImm8)
	cpu.Memory.RAM[14] = 7

	// DIV BX
	cpu.Memory.RAM[15] = byte(OpDIV)
	cpu.Memory.RAM[16] = byte(OpTypeReg16)
	cpu.Memory.RAM[17] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[18] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// 100 / 7 = 14 remainder 2
	if cpu.AX != 14 {
		t.Errorf("Expected AX=14 (quotient), got AX=%d", cpu.AX)
	}
	if cpu.DX != 2 {
		t.Errorf("Expected DX=2 (remainder), got DX=%d", cpu.DX)
	}
}

// TestAND tests AND instruction
func TestAND(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 0xF0F0
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm16)
	cpu.Memory.RAM[4] = 0xF0 // low byte
	cpu.Memory.RAM[5] = 0xF0 // high byte

	// MOV BX, 0xFF00
	cpu.Memory.RAM[6] = byte(OpMOV)
	cpu.Memory.RAM[7] = byte(OpTypeReg16)
	cpu.Memory.RAM[8] = 0x01  // BX
	cpu.Memory.RAM[9] = byte(OpTypeImm16)
	cpu.Memory.RAM[10] = 0x00 // low byte
	cpu.Memory.RAM[11] = 0xFF // high byte

	// AND AX, BX
	cpu.Memory.RAM[12] = byte(OpAND)
	cpu.Memory.RAM[13] = byte(OpTypeReg16)
	cpu.Memory.RAM[14] = 0x00 // AX
	cpu.Memory.RAM[15] = byte(OpTypeReg16)
	cpu.Memory.RAM[16] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[17] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// 0xF0F0 AND 0xFF00 = 0xF000
	expected := uint16(0xF000)
	if cpu.AX != expected {
		t.Errorf("Expected AX=0x%04X, got AX=0x%04X", expected, cpu.AX)
	}

	// CF and OF should be cleared
	if cpu.Flags.CF {
		t.Error("Expected CF=false")
	}
	if cpu.Flags.OF {
		t.Error("Expected OF=false")
	}
}

// TestOR tests OR instruction
func TestOR(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 0x00F0
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm16)
	cpu.Memory.RAM[4] = 0xF0 // low byte
	cpu.Memory.RAM[5] = 0x00 // high byte

	// MOV BX, 0x0F00
	cpu.Memory.RAM[6] = byte(OpMOV)
	cpu.Memory.RAM[7] = byte(OpTypeReg16)
	cpu.Memory.RAM[8] = 0x01  // BX
	cpu.Memory.RAM[9] = byte(OpTypeImm16)
	cpu.Memory.RAM[10] = 0x00 // low byte
	cpu.Memory.RAM[11] = 0x0F // high byte

	// OR AX, BX
	cpu.Memory.RAM[12] = byte(OpOR)
	cpu.Memory.RAM[13] = byte(OpTypeReg16)
	cpu.Memory.RAM[14] = 0x00 // AX
	cpu.Memory.RAM[15] = byte(OpTypeReg16)
	cpu.Memory.RAM[16] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[17] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// 0x00F0 OR 0x0F00 = 0x0FF0
	expected := uint16(0x0FF0)
	if cpu.AX != expected {
		t.Errorf("Expected AX=0x%04X, got AX=0x%04X", expected, cpu.AX)
	}

	// CF and OF should be cleared
	if cpu.Flags.CF {
		t.Error("Expected CF=false")
	}
	if cpu.Flags.OF {
		t.Error("Expected OF=false")
	}
}

// TestXOR tests XOR instruction
func TestXOR(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 0xAAAA
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm16)
	cpu.Memory.RAM[4] = 0xAA // low byte
	cpu.Memory.RAM[5] = 0xAA // high byte

	// MOV BX, 0x5555
	cpu.Memory.RAM[6] = byte(OpMOV)
	cpu.Memory.RAM[7] = byte(OpTypeReg16)
	cpu.Memory.RAM[8] = 0x01  // BX
	cpu.Memory.RAM[9] = byte(OpTypeImm16)
	cpu.Memory.RAM[10] = 0x55 // low byte
	cpu.Memory.RAM[11] = 0x55 // high byte

	// XOR AX, BX
	cpu.Memory.RAM[12] = byte(OpXOR)
	cpu.Memory.RAM[13] = byte(OpTypeReg16)
	cpu.Memory.RAM[14] = 0x00 // AX
	cpu.Memory.RAM[15] = byte(OpTypeReg16)
	cpu.Memory.RAM[16] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[17] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// 0xAAAA XOR 0x5555 = 0xFFFF
	expected := uint16(0xFFFF)
	if cpu.AX != expected {
		t.Errorf("Expected AX=0x%04X, got AX=0x%04X", expected, cpu.AX)
	}

	// CF and OF should be cleared
	if cpu.Flags.CF {
		t.Error("Expected CF=false")
	}
	if cpu.Flags.OF {
		t.Error("Expected OF=false")
	}
}

// TestNOT tests NOT instruction
func TestNOT(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 0xAAAA
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm16)
	cpu.Memory.RAM[4] = 0xAA // low byte
	cpu.Memory.RAM[5] = 0xAA // high byte

	// NOT AX
	cpu.Memory.RAM[6] = byte(OpNOT)
	cpu.Memory.RAM[7] = byte(OpTypeReg16)
	cpu.Memory.RAM[8] = 0x00 // AX

	// HLT
	cpu.Memory.RAM[9] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// NOT 0xAAAA = 0x5555
	expected := uint16(0x5555)
	if cpu.AX != expected {
		t.Errorf("Expected AX=0x%04X, got AX=0x%04X", expected, cpu.AX)
	}
}

// TestSHL tests SHL (shift left) instruction
func TestSHL(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 0x0003
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm16)
	cpu.Memory.RAM[4] = 0x03 // low byte
	cpu.Memory.RAM[5] = 0x00 // high byte

	// MOV CX, 4 (shift count)
	cpu.Memory.RAM[6] = byte(OpMOV)
	cpu.Memory.RAM[7] = byte(OpTypeReg16)
	cpu.Memory.RAM[8] = 0x02 // CX
	cpu.Memory.RAM[9] = byte(OpTypeImm8)
	cpu.Memory.RAM[10] = 4

	// SHL AX, CX
	cpu.Memory.RAM[11] = byte(OpSHL)
	cpu.Memory.RAM[12] = byte(OpTypeReg16)
	cpu.Memory.RAM[13] = 0x00 // AX
	cpu.Memory.RAM[14] = byte(OpTypeReg16)
	cpu.Memory.RAM[15] = 0x02 // CX

	// HLT
	cpu.Memory.RAM[16] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// 0x0003 << 4 = 0x0030
	expected := uint16(0x0030)
	if cpu.AX != expected {
		t.Errorf("Expected AX=0x%04X, got AX=0x%04X", expected, cpu.AX)
	}
}

// TestSHR tests SHR (shift right logical) instruction
func TestSHR(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 0x00F0
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm16)
	cpu.Memory.RAM[4] = 0xF0 // low byte
	cpu.Memory.RAM[5] = 0x00 // high byte

	// MOV CX, 4 (shift count)
	cpu.Memory.RAM[6] = byte(OpMOV)
	cpu.Memory.RAM[7] = byte(OpTypeReg16)
	cpu.Memory.RAM[8] = 0x02 // CX
	cpu.Memory.RAM[9] = byte(OpTypeImm8)
	cpu.Memory.RAM[10] = 4

	// SHR AX, CX
	cpu.Memory.RAM[11] = byte(OpSHR)
	cpu.Memory.RAM[12] = byte(OpTypeReg16)
	cpu.Memory.RAM[13] = 0x00 // AX
	cpu.Memory.RAM[14] = byte(OpTypeReg16)
	cpu.Memory.RAM[15] = 0x02 // CX

	// HLT
	cpu.Memory.RAM[16] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// 0x00F0 >> 4 = 0x000F
	expected := uint16(0x000F)
	if cpu.AX != expected {
		t.Errorf("Expected AX=0x%04X, got AX=0x%04X", expected, cpu.AX)
	}
}

// TestSAR tests SAR (shift arithmetic right - preserves sign) instruction
func TestSAR(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 0xFF00 (negative number in signed representation)
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm16)
	cpu.Memory.RAM[4] = 0x00 // low byte
	cpu.Memory.RAM[5] = 0xFF // high byte

	// MOV CX, 4 (shift count)
	cpu.Memory.RAM[6] = byte(OpMOV)
	cpu.Memory.RAM[7] = byte(OpTypeReg16)
	cpu.Memory.RAM[8] = 0x02 // CX
	cpu.Memory.RAM[9] = byte(OpTypeImm8)
	cpu.Memory.RAM[10] = 4

	// SAR AX, CX
	cpu.Memory.RAM[11] = byte(OpSAR)
	cpu.Memory.RAM[12] = byte(OpTypeReg16)
	cpu.Memory.RAM[13] = 0x00 // AX
	cpu.Memory.RAM[14] = byte(OpTypeReg16)
	cpu.Memory.RAM[15] = 0x02 // CX

	// HLT
	cpu.Memory.RAM[16] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// 0xFF00 SAR 4 = 0xFFF0 (sign bit preserved)
	expected := uint16(0xFFF0)
	if cpu.AX != expected {
		t.Errorf("Expected AX=0x%04X, got AX=0x%04X", expected, cpu.AX)
	}
}

// TestCMP tests CMP instruction (comparison without storing result)
func TestCMP(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 10
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 10

	// MOV BX, 10
	cpu.Memory.RAM[5] = byte(OpMOV)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x01 // BX
	cpu.Memory.RAM[8] = byte(OpTypeImm8)
	cpu.Memory.RAM[9] = 10

	// CMP AX, BX
	cpu.Memory.RAM[10] = byte(OpCMP)
	cpu.Memory.RAM[11] = byte(OpTypeReg16)
	cpu.Memory.RAM[12] = 0x00 // AX
	cpu.Memory.RAM[13] = byte(OpTypeReg16)
	cpu.Memory.RAM[14] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[15] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// AX should remain unchanged
	if cpu.AX != 10 {
		t.Errorf("Expected AX=10, got AX=%d", cpu.AX)
	}

	// ZF should be set (values are equal)
	if !cpu.Flags.ZF {
		t.Error("Expected ZF=true (equal values)")
	}

	// CF should be clear (no borrow)
	if cpu.Flags.CF {
		t.Error("Expected CF=false")
	}
}

// TestJE tests JE (jump if equal) instruction
func TestJE(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 5
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 5

	// CMP AX, 5
	cpu.Memory.RAM[5] = byte(OpCMP)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x00 // AX
	cpu.Memory.RAM[8] = byte(OpTypeImm8)
	cpu.Memory.RAM[9] = 5

	// JE to address 0x0011 (skip INC)
	cpu.Memory.RAM[10] = byte(OpJE)
	cpu.Memory.RAM[11] = byte(OpTypeImm16)
	cpu.Memory.RAM[12] = 0x11 // low byte
	cpu.Memory.RAM[13] = 0x00 // high byte

	// INC AX (should be skipped)
	cpu.Memory.RAM[14] = byte(OpINC)
	cpu.Memory.RAM[15] = byte(OpTypeReg16)
	cpu.Memory.RAM[16] = 0x00 // AX

	// HLT (address 0x0011)
	cpu.Memory.RAM[17] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// AX should still be 5 (INC was skipped)
	if cpu.AX != 5 {
		t.Errorf("Expected AX=5, got AX=%d", cpu.AX)
	}
}

// TestJNE tests JNE (jump if not equal) instruction
func TestJNE(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 5
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 5

	// CMP AX, 10
	cpu.Memory.RAM[5] = byte(OpCMP)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x00 // AX
	cpu.Memory.RAM[8] = byte(OpTypeImm8)
	cpu.Memory.RAM[9] = 10

	// JNE to address 0x0011 (skip INC)
	cpu.Memory.RAM[10] = byte(OpJNE)
	cpu.Memory.RAM[11] = byte(OpTypeImm16)
	cpu.Memory.RAM[12] = 0x11 // low byte
	cpu.Memory.RAM[13] = 0x00 // high byte

	// INC AX (should be skipped)
	cpu.Memory.RAM[14] = byte(OpINC)
	cpu.Memory.RAM[15] = byte(OpTypeReg16)
	cpu.Memory.RAM[16] = 0x00 // AX

	// HLT (address 0x0011)
	cpu.Memory.RAM[17] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// AX should still be 5 (INC was skipped)
	if cpu.AX != 5 {
		t.Errorf("Expected AX=5, got AX=%d", cpu.AX)
	}
}

// TestJG tests JG (jump if greater - signed) instruction
func TestJG(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 10
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 10

	// CMP AX, 5
	cpu.Memory.RAM[5] = byte(OpCMP)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x00 // AX
	cpu.Memory.RAM[8] = byte(OpTypeImm8)
	cpu.Memory.RAM[9] = 5

	// JG to address 0x0011 (skip INC since 10 > 5)
	cpu.Memory.RAM[10] = byte(OpJG)
	cpu.Memory.RAM[11] = byte(OpTypeImm16)
	cpu.Memory.RAM[12] = 0x11 // low byte
	cpu.Memory.RAM[13] = 0x00 // high byte

	// INC AX (should be skipped)
	cpu.Memory.RAM[14] = byte(OpINC)
	cpu.Memory.RAM[15] = byte(OpTypeReg16)
	cpu.Memory.RAM[16] = 0x00 // AX

	// HLT (address 0x0011)
	cpu.Memory.RAM[17] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// AX should still be 10 (INC was skipped)
	if cpu.AX != 10 {
		t.Errorf("Expected AX=10, got AX=%d", cpu.AX)
	}
}

// TestJL tests JL (jump if less - signed) instruction
func TestJL(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 5
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 5

	// CMP AX, 10
	cpu.Memory.RAM[5] = byte(OpCMP)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x00 // AX
	cpu.Memory.RAM[8] = byte(OpTypeImm8)
	cpu.Memory.RAM[9] = 10

	// JL to address 0x0011 (skip INC since 5 < 10)
	cpu.Memory.RAM[10] = byte(OpJL)
	cpu.Memory.RAM[11] = byte(OpTypeImm16)
	cpu.Memory.RAM[12] = 0x11 // low byte
	cpu.Memory.RAM[13] = 0x00 // high byte

	// INC AX (should be skipped)
	cpu.Memory.RAM[14] = byte(OpINC)
	cpu.Memory.RAM[15] = byte(OpTypeReg16)
	cpu.Memory.RAM[16] = 0x00 // AX

	// HLT (address 0x0011)
	cpu.Memory.RAM[17] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// AX should still be 5 (INC was skipped)
	if cpu.AX != 5 {
		t.Errorf("Expected AX=5, got AX=%d", cpu.AX)
	}
}

// TestPUSHPOP tests PUSH and POP instructions
func TestPUSHPOP(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 42
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 42

	// PUSH AX
	cpu.Memory.RAM[5] = byte(OpPUSH)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x00 // AX

	// MOV AX, 0 (clear AX)
	cpu.Memory.RAM[8] = byte(OpMOV)
	cpu.Memory.RAM[9] = byte(OpTypeReg16)
	cpu.Memory.RAM[10] = 0x00 // AX
	cpu.Memory.RAM[11] = byte(OpTypeImm8)
	cpu.Memory.RAM[12] = 0

	// POP BX (restore value to BX)
	cpu.Memory.RAM[13] = byte(OpPOP)
	cpu.Memory.RAM[14] = byte(OpTypeReg16)
	cpu.Memory.RAM[15] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[16] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// AX should be 0 (we cleared it)
	if cpu.AX != 0 {
		t.Errorf("Expected AX=0, got AX=%d", cpu.AX)
	}

	// BX should be 42 (popped value)
	if cpu.BX != 42 {
		t.Errorf("Expected BX=42, got BX=%d", cpu.BX)
	}
}

// TestCALLRET tests CALL and RET instructions
func TestCALLRET(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 0
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 0

	// CALL subroutine at 0x000D
	cpu.Memory.RAM[5] = byte(OpCALL)
	cpu.Memory.RAM[6] = byte(OpTypeImm16)
	cpu.Memory.RAM[7] = 0x0D // low byte
	cpu.Memory.RAM[8] = 0x00 // high byte

	// INC AX (after return)
	cpu.Memory.RAM[9] = byte(OpINC)
	cpu.Memory.RAM[10] = byte(OpTypeReg16)
	cpu.Memory.RAM[11] = 0x00 // AX

	// HLT
	cpu.Memory.RAM[12] = byte(OpHLT)

	// Subroutine at 0x000D:
	// MOV AX, 10
	cpu.Memory.RAM[13] = byte(OpMOV)
	cpu.Memory.RAM[14] = byte(OpTypeReg16)
	cpu.Memory.RAM[15] = 0x00 // AX
	cpu.Memory.RAM[16] = byte(OpTypeImm8)
	cpu.Memory.RAM[17] = 10

	// RET
	cpu.Memory.RAM[18] = byte(OpRET)
	cpu.Memory.RAM[19] = byte(OpTypeNone)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// AX should be 11 (10 from subroutine + 1 from INC)
	if cpu.AX != 11 {
		t.Errorf("Expected AX=11, got AX=%d", cpu.AX)
	}
}

// TestStackMultiplePush tests multiple PUSH operations
func TestStackMultiplePush(t *testing.T) {
	cpu := NewCPU()

	// MOV AX, 1
	cpu.Memory.RAM[0] = byte(OpMOV)
	cpu.Memory.RAM[1] = byte(OpTypeReg16)
	cpu.Memory.RAM[2] = 0x00 // AX
	cpu.Memory.RAM[3] = byte(OpTypeImm8)
	cpu.Memory.RAM[4] = 1

	// MOV BX, 2
	cpu.Memory.RAM[5] = byte(OpMOV)
	cpu.Memory.RAM[6] = byte(OpTypeReg16)
	cpu.Memory.RAM[7] = 0x01 // BX
	cpu.Memory.RAM[8] = byte(OpTypeImm8)
	cpu.Memory.RAM[9] = 2

	// MOV CX, 3
	cpu.Memory.RAM[10] = byte(OpMOV)
	cpu.Memory.RAM[11] = byte(OpTypeReg16)
	cpu.Memory.RAM[12] = 0x02 // CX
	cpu.Memory.RAM[13] = byte(OpTypeImm8)
	cpu.Memory.RAM[14] = 3

	// PUSH AX
	cpu.Memory.RAM[15] = byte(OpPUSH)
	cpu.Memory.RAM[16] = byte(OpTypeReg16)
	cpu.Memory.RAM[17] = 0x00 // AX

	// PUSH BX
	cpu.Memory.RAM[18] = byte(OpPUSH)
	cpu.Memory.RAM[19] = byte(OpTypeReg16)
	cpu.Memory.RAM[20] = 0x01 // BX

	// PUSH CX
	cpu.Memory.RAM[21] = byte(OpPUSH)
	cpu.Memory.RAM[22] = byte(OpTypeReg16)
	cpu.Memory.RAM[23] = 0x02 // CX

	// POP into DX (should get 3 - last pushed)
	cpu.Memory.RAM[24] = byte(OpPOP)
	cpu.Memory.RAM[25] = byte(OpTypeReg16)
	cpu.Memory.RAM[26] = 0x03 // DX

	// POP into CX (should get 2)
	cpu.Memory.RAM[27] = byte(OpPOP)
	cpu.Memory.RAM[28] = byte(OpTypeReg16)
	cpu.Memory.RAM[29] = 0x02 // CX

	// POP into BX (should get 1)
	cpu.Memory.RAM[30] = byte(OpPOP)
	cpu.Memory.RAM[31] = byte(OpTypeReg16)
	cpu.Memory.RAM[32] = 0x01 // BX

	// HLT
	cpu.Memory.RAM[33] = byte(OpHLT)

	err := cpu.Run()
	if err != nil {
		t.Fatalf("CPU.Run() failed: %v", err)
	}

	// Check LIFO order
	if cpu.DX != 3 {
		t.Errorf("Expected DX=3, got DX=%d", cpu.DX)
	}
	if cpu.CX != 2 {
		t.Errorf("Expected CX=2, got CX=%d", cpu.CX)
	}
	if cpu.BX != 1 {
		t.Errorf("Expected BX=1, got BX=%d", cpu.BX)
	}
}
