// Package core is the segmentation engine: the lexeme model, the four
// sub-segmenters, the ambiguity arbitrator and the driver that runs them.
package core

import "github.com/infinilabs/analysis-ik/jdk"

// Character classes. Note that charSurrogate is 0x16 rather than the next free
// bit — the value is compared with equality everywhere, never masked, and it is
// reproduced here exactly as the original defined it.
const (
	charUseless   = 0
	charArabic    = 0x00000001
	charEnglish   = 0x00000002
	charChinese   = 0x00000004
	charOtherCJK  = 0x00000008
	charSurrogate = 0x00000016
)

// identifyCharType classifies one UTF-16 code unit.
func identifyCharType(input jdk.Char) int {
	switch {
	case input >= '0' && input <= '9':
		return charArabic
	case (input >= 'a' && input <= 'z') || (input >= 'A' && input <= 'Z'):
		return charEnglish
	}

	switch jdk.UnicodeBlockOf(input) {
	case jdk.CJKUnifiedIdeographs,
		jdk.CJKCompatibilityIdeographs,
		jdk.CJKUnifiedIdeographsExtensionA:
		return charChinese
	case jdk.HalfwidthAndFullwidthForms,
		jdk.HangulSyllables,
		jdk.HangulJamo,
		jdk.HangulCompatibilityJamo,
		jdk.Hiragana,
		jdk.Katakana,
		jdk.KatakanaPhoneticExtensions:
		return charOtherCJK
	case jdk.HighSurrogates,
		jdk.LowSurrogates,
		jdk.HighPrivateUseSurrogates:
		return charSurrogate
	}
	return charUseless
}

// regularize folds the ideographic space and the fullwidth forms down to ASCII,
// and lowercases A-Z when the configuration asks for it.
func regularize(input jdk.Char, lowercase bool) jdk.Char {
	switch {
	case input == 12288:
		input = 32
	case input > 65280 && input < 65375:
		input -= 65248
	case input >= 'A' && input <= 'Z' && lowercase:
		input += 32
	}
	return input
}
