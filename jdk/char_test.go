package jdk

import "testing"

// jdkBlocks is the answer Character.UnicodeBlock.of gives for every code unit,
// captured from the JDK the original was built against. The port owes the
// analyzer these boundaries exactly: they decide which characters are Chinese,
// which are other CJK, and which are surrogates.
var jdkBlocks = []struct {
	lo, hi Char
	block  UnicodeBlock
}{
	{0x1100, 0x11FF, HangulJamo},
	{0x3040, 0x309F, Hiragana},
	{0x30A0, 0x30FF, Katakana},
	{0x3130, 0x318F, HangulCompatibilityJamo},
	{0x31F0, 0x31FF, KatakanaPhoneticExtensions},
	{0x3400, 0x4DBF, CJKUnifiedIdeographsExtensionA},
	{0x4E00, 0x9FFF, CJKUnifiedIdeographs},
	{0xAC00, 0xD7AF, HangulSyllables},
	{0xD800, 0xDB7F, HighSurrogates},
	{0xDB80, 0xDBFF, HighPrivateUseSurrogates},
	{0xDC00, 0xDFFF, LowSurrogates},
	{0xF900, 0xFAFF, CJKCompatibilityIdeographs},
	{0xFF00, 0xFFEF, HalfwidthAndFullwidthForms},
}

func TestUnicodeBlockOfMatchesTheJDKForEveryCodeUnit(t *testing.T) {
	expected := func(c Char) UnicodeBlock {
		for _, r := range jdkBlocks {
			if c >= r.lo && c <= r.hi {
				return r.block
			}
		}
		return BlockOther
	}
	for i := 0; i <= 0xFFFF; i++ {
		c := Char(i)
		if got, want := UnicodeBlockOf(c), expected(c); got != want {
			t.Fatalf("UnicodeBlockOf(%04X) = %d, want %d", i, got, want)
		}
	}
}

func TestUnicodeBlockBoundariesAreExclusive(t *testing.T) {
	for _, r := range jdkBlocks {
		if got := UnicodeBlockOf(r.lo); got != r.block {
			t.Errorf("UnicodeBlockOf(%04X) = %d, want %d", r.lo, got, r.block)
		}
		if got := UnicodeBlockOf(r.hi); got != r.block {
			t.Errorf("UnicodeBlockOf(%04X) = %d, want %d", r.hi, got, r.block)
		}
		if r.lo > 0 {
			if got := UnicodeBlockOf(r.lo - 1); got == r.block {
				t.Errorf("UnicodeBlockOf(%04X) leaked into block %d", r.lo-1, r.block)
			}
		}
		if r.hi < 0xFFFF {
			if got := UnicodeBlockOf(r.hi + 1); got == r.block {
				t.Errorf("UnicodeBlockOf(%04X) leaked into block %d", r.hi+1, r.block)
			}
		}
	}
}

func TestSurrogatePredicates(t *testing.T) {
	for i := 0; i <= 0xFFFF; i++ {
		c := Char(i)
		if got, want := IsHighSurrogate(c), i >= 0xD800 && i <= 0xDBFF; got != want {
			t.Fatalf("IsHighSurrogate(%04X) = %t, want %t", i, got, want)
		}
		if got, want := IsLowSurrogate(c), i >= 0xDC00 && i <= 0xDFFF; got != want {
			t.Fatalf("IsLowSurrogate(%04X) = %t, want %t", i, got, want)
		}
	}
}
