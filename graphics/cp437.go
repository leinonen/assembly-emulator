package graphics

import "fmt"

// CP437 character encoding lookup table
// Maps byte values (0-255) to Unicode runes for the IBM Code Page 437 character set
var cp437Runes = []rune{
	'\x00', '☺', '☻', '♥', '♦', '♣', '♠', '•', '◘', '○', '◙', '♂', '♀', '♪', '♫', '☼',
	'►', '◄', '↕', '‼', '¶', '§', '▬', '↨', '↑', '↓', '→', '←', '∟', '↔', '▲', '▼',
	' ', '!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '/',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ':', ';', '<', '=', '>', '?',
	'@', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O',
	'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', '[', '\\', ']', '^', '_',
	'`', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', '{', '|', '}', '~', '⌂',
	'Ç', 'ü', 'é', 'â', 'ä', 'à', 'å', 'ç', 'ê', 'ë', 'è', 'ï', 'î', 'ì', 'Ä', 'Å',
	'É', 'æ', 'Æ', 'ô', 'ö', 'ò', 'û', 'ù', 'ÿ', 'Ö', 'Ü', '¢', '£', '¥', '₧', 'ƒ',
	'á', 'í', 'ó', 'ú', 'ñ', 'Ñ', 'ª', 'º', '¿', '⌐', '¬', '½', '¼', '¡', '«', '»',
	'░', '▒', '▓', '│', '┤', '╡', '╢', '╖', '╕', '╣', '║', '╗', '╝', '╜', '╛', '┐',
	'└', '┴', '┬', '├', '─', '┼', '╞', '╟', '╚', '╔', '╩', '╦', '╠', '═', '╬', '╧',
	'╨', '╤', '╥', '╙', '╘', '╒', '╓', '╫', '╪', '┘', '┌', '█', '▄', '▌', '▐', '▀',
	'α', 'ß', 'Γ', 'π', 'Σ', 'σ', 'µ', 'τ', 'Φ', 'Θ', 'Ω', 'δ', '∞', 'φ', 'ε', '∩',
	'≡', '±', '≥', '≤', '⌠', '⌡', '÷', '≈', '°', '∙', '·', '√', 'ⁿ', '²', '■', '\u00A0',
}

// CP437ToRune converts a CP437 byte to its Unicode rune equivalent
func CP437ToRune(b byte) rune {
	return cp437Runes[b]
}

// CP437ToString converts a CP437 byte slice to a UTF-8 string
func CP437ToString(data []byte) string {
	runes := make([]rune, len(data))
	for i, b := range data {
		runes[i] = cp437Runes[b]
	}
	return string(runes)
}

// runeToByte is a reverse lookup map from rune to CP437 byte value
var runeToByte map[rune]byte

// init builds the reverse lookup map
func init() {
	runeToByte = make(map[rune]byte, 256)
	for i, r := range cp437Runes {
		runeToByte[r] = byte(i)
	}
}

// RuneToCP437 converts a Unicode rune to its CP437 byte equivalent
// Returns the byte value and true if the rune exists in CP437, or 0 and false otherwise
func RuneToCP437(r rune) (byte, bool) {
	b, ok := runeToByte[r]
	return b, ok
}

// StringToCP437 converts a UTF-8 string to CP437 bytes
// Returns an error if any character cannot be represented in CP437
func StringToCP437(s string) ([]byte, error) {
	result := make([]byte, 0, len(s))
	for _, r := range s {
		b, ok := RuneToCP437(r)
		if !ok {
			return nil, fmt.Errorf("character '%c' (U+%04X) cannot be represented in CP437", r, r)
		}
		result = append(result, b)
	}
	return result, nil
}
