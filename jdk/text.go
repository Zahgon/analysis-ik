package jdk

import (
	"strings"
	"unicode/utf8"
)

// EncodeUTF16 converts a Go string into the UTF-16 code units a Java String
// would hold. Three-byte sequences that encode an unpaired surrogate (WTF-8,
// which is what DecodeUTF16 emits for a lone surrogate) survive the round trip
// instead of collapsing to U+FFFD.
func EncodeUTF16(s string) []Char {
	out := make([]Char, 0, len(s))
	for i := 0; i < len(s); {
		r, size := decodeWTF8(s[i:])
		i += size
		switch {
		case r < 0x10000:
			out = append(out, Char(r))
		default:
			r -= 0x10000
			out = append(out, Char(0xD800+(r>>10)), Char(0xDC00+(r&0x3FF)))
		}
	}
	return out
}

// DecodeUTF16 converts UTF-16 code units back into a Go string. A surrogate
// with no partner is written as the three-byte encoding of its own code point
// so that it stays distinguishable, which matters because
// SurrogatePairSegmenter emits exactly such a lexeme.
func DecodeUTF16(units []Char) string {
	var b strings.Builder
	b.Grow(len(units) * 3)
	for i := 0; i < len(units); i++ {
		c := units[i]
		if IsHighSurrogate(c) && i+1 < len(units) && IsLowSurrogate(units[i+1]) {
			b.WriteRune(rune(0x10000 + (rune(c)-0xD800)<<10 + (rune(units[i+1]) - 0xDC00)))
			i++
			continue
		}
		if IsHighSurrogate(c) || IsLowSurrogate(c) {
			b.WriteByte(byte(0xE0 | c>>12))
			b.WriteByte(byte(0x80 | (c>>6)&0x3F))
			b.WriteByte(byte(0x80 | c&0x3F))
			continue
		}
		b.WriteRune(rune(c))
	}
	return b.String()
}

// String renders count code units starting at off, the way
// String.valueOf(char[], int, int) does.
func String(units []Char, off, count int) string {
	return DecodeUTF16(units[off : off+count])
}

func decodeWTF8(s string) (rune, int) {
	r, size := utf8.DecodeRuneInString(s)
	if r != utf8.RuneError || size != 1 {
		return r, size
	}
	if len(s) >= 3 && s[0]&0xF0 == 0xE0 && s[1]&0xC0 == 0x80 && s[2]&0xC0 == 0x80 {
		v := rune(s[0]&0x0F)<<12 | rune(s[1]&0x3F)<<6 | rune(s[2]&0x3F)
		if v >= 0xD800 && v <= 0xDFFF {
			return v, 3
		}
	}
	return r, size
}

// Trim mirrors java.lang.String.trim: it removes every leading and trailing
// code unit less than or equal to U+0020, which is not the same set as the
// Unicode whitespace strings.TrimSpace uses.
func Trim(s string) string {
	units := EncodeUTF16(s)
	start, end := 0, len(units)
	for start < end && units[start] <= ' ' {
		start++
	}
	for start < end && units[end-1] <= ' ' {
		end--
	}
	return DecodeUTF16(units[start:end])
}

// ToLower mirrors java.lang.String.toLowerCase for the locale-independent case
// mapping the dictionary loader relies on.
func ToLower(s string) string { return strings.ToLower(s) }
