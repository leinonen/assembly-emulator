package bios

// Scancode set 1 → ASCII translation (US layout), indexed by make code.
// Each entry: normal, shift, ctrl, alt. 0 = no ASCII (extended keys use
// scancode<<8 with ASCII 0 or E0).

type keyEntry struct {
	normal, shift, ctrl, alt uint8
}

var keyTable = [0x60]keyEntry{
	0x01: {0x1B, 0x1B, 0x1B, 0x00}, // Esc
	0x02: {'1', '!', 0, 0},         // 1
	0x03: {'2', '@', 0x00, 0},      // 2 (ctrl-2 = NUL, special-cased)
	0x04: {'3', '#', 0, 0},
	0x05: {'4', '$', 0, 0},
	0x06: {'5', '%', 0, 0},
	0x07: {'6', '^', 0x1E, 0},
	0x08: {'7', '&', 0, 0},
	0x09: {'8', '*', 0, 0},
	0x0A: {'9', '(', 0, 0},
	0x0B: {'0', ')', 0, 0},
	0x0C: {'-', '_', 0x1F, 0},
	0x0D: {'=', '+', 0, 0},
	0x0E: {0x08, 0x08, 0x7F, 0}, // Backspace
	0x0F: {0x09, 0x00, 0x00, 0}, // Tab (shift-tab = 0F00)
	0x10: {'q', 'Q', 0x11, 0},
	0x11: {'w', 'W', 0x17, 0},
	0x12: {'e', 'E', 0x05, 0},
	0x13: {'r', 'R', 0x12, 0},
	0x14: {'t', 'T', 0x14, 0},
	0x15: {'y', 'Y', 0x19, 0},
	0x16: {'u', 'U', 0x15, 0},
	0x17: {'i', 'I', 0x09, 0},
	0x18: {'o', 'O', 0x0F, 0},
	0x19: {'p', 'P', 0x10, 0},
	0x1A: {'[', '{', 0x1B, 0},
	0x1B: {']', '}', 0x1D, 0},
	0x1C: {0x0D, 0x0D, 0x0A, 0}, // Enter
	0x1E: {'a', 'A', 0x01, 0},
	0x1F: {'s', 'S', 0x13, 0},
	0x20: {'d', 'D', 0x04, 0},
	0x21: {'f', 'F', 0x06, 0},
	0x22: {'g', 'G', 0x07, 0},
	0x23: {'h', 'H', 0x08, 0},
	0x24: {'j', 'J', 0x0A, 0},
	0x25: {'k', 'K', 0x0B, 0},
	0x26: {'l', 'L', 0x0C, 0},
	0x27: {';', ':', 0, 0},
	0x28: {'\'', '"', 0, 0},
	0x29: {'`', '~', 0, 0},
	0x2B: {'\\', '|', 0x1C, 0},
	0x2C: {'z', 'Z', 0x1A, 0},
	0x2D: {'x', 'X', 0x18, 0},
	0x2E: {'c', 'C', 0x03, 0},
	0x2F: {'v', 'V', 0x16, 0},
	0x30: {'b', 'B', 0x02, 0},
	0x31: {'n', 'N', 0x0E, 0},
	0x32: {'m', 'M', 0x0D, 0},
	0x33: {',', '<', 0, 0},
	0x34: {'.', '>', 0, 0},
	0x35: {'/', '?', 0, 0},
	0x37: {'*', '*', 0x72, 0},  // keypad *
	0x39: {' ', ' ', ' ', ' '}, // Space
	0x47: {'7', '7', 0, 0},     // keypad (numlock on)
	0x48: {'8', '8', 0, 0},
	0x49: {'9', '9', 0, 0},
	0x4A: {'-', '-', 0, 0},
	0x4B: {'4', '4', 0, 0},
	0x4C: {'5', '5', 0, 0},
	0x4D: {'6', '6', 0, 0},
	0x4E: {'+', '+', 0, 0},
	0x4F: {'1', '1', 0, 0},
	0x50: {'2', '2', 0, 0},
	0x51: {'3', '3', 0, 0},
	0x52: {'0', '0', 0, 0},
	0x53: {'.', '.', 0, 0},
}

// Shift-state bits in the BDA at 0040:0017.
const (
	kbRShift   = 0x01
	kbLShift   = 0x02
	kbCtrl     = 0x04
	kbAlt      = 0x08
	kbScroll   = 0x10
	kbNum      = 0x20
	kbCaps     = 0x40
	kbInsert   = 0x80
	kbLCtrl    = 0x01 // 0040:0018
	kbLAlt     = 0x02
	kbE0Prefix = 0x02 // 0040:0096
)
