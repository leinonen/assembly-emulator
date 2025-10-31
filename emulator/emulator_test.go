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

	// Write to VGA memory at 0xA000
	cpu.Memory.WriteByte(0xA000, 15) // White pixel

	// Read it back
	val := cpu.Memory.ReadByte(0xA000)
	if val != 15 {
		t.Errorf("Expected VGA[0]=15, got %d", val)
	}

	// Verify it's in VGA, not RAM
	if cpu.Memory.RAM[0xA000] == 15 {
		t.Error("Value was written to RAM instead of VGA")
	}
	if cpu.Memory.VGA[0] != 15 {
		t.Error("Value was not written to VGA memory")
	}
}

// TestProtectedMemory tests that low memory is protected from writes
func TestProtectedMemory(t *testing.T) {
	cpu := NewCPU()

	// Load some code
	cpu.Memory.RAM[0] = 0x01 // OpMOV

	// Try to write to protected low memory
	cpu.Memory.WriteByte(0, 0xFF)

	// Verify it was not written
	if cpu.Memory.RAM[0] != 0x01 {
		t.Error("Protected memory was overwritten")
	}
}
