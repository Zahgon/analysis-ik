package jdk

// Char is one UTF-16 code unit: the Go stand-in for Java's char.
type Char = uint16

// UnicodeBlock identifies the Character.UnicodeBlock constants the analyzer
// tests for. Every code unit outside them reports BlockOther, which is
// indistinguishable from Java's answer because the original only ever compares
// the block against the constants below.
type UnicodeBlock int

// The blocks named by CharacterUtil.identifyCharType and
// CharacterHelper.isCJKCharacter.
const (
	BlockOther UnicodeBlock = iota
	CJKUnifiedIdeographs
	CJKCompatibilityIdeographs
	CJKUnifiedIdeographsExtensionA
	HalfwidthAndFullwidthForms
	HangulSyllables
	HangulJamo
	HangulCompatibilityJamo
	Hiragana
	Katakana
	KatakanaPhoneticExtensions
	HighSurrogates
	HighPrivateUseSurrogates
	LowSurrogates
)

type blockRange struct {
	lo, hi Char
	block  UnicodeBlock
}

// Boundaries as the JDK defines them, ordered so a linear scan can stop early.
var blockRanges = [...]blockRange{
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

// UnicodeBlockOf reports which of the blocks above contains c, mirroring
// Character.UnicodeBlock.of(char).
func UnicodeBlockOf(c Char) UnicodeBlock {
	for _, r := range blockRanges {
		if c < r.lo {
			return BlockOther
		}
		if c <= r.hi {
			return r.block
		}
	}
	return BlockOther
}

// IsHighSurrogate mirrors Character.isHighSurrogate.
func IsHighSurrogate(c Char) bool { return c >= 0xD800 && c <= 0xDBFF }

// IsLowSurrogate mirrors Character.isLowSurrogate.
func IsLowSurrogate(c Char) bool { return c >= 0xDC00 && c <= 0xDFFF }
