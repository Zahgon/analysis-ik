package help

import "github.com/infinilabs/analysis-ik/jdk"

// IsSpaceLetter reports whether input is one of the six code units the analyzer
// treats as whitespace.
func IsSpaceLetter(input jdk.Char) bool {
	return input == 8 || input == 9 ||
		input == 10 || input == 13 ||
		input == 32 || input == 160
}

// IsEnglishLetter reports whether input is an ASCII letter.
func IsEnglishLetter(input jdk.Char) bool {
	return (input >= 'a' && input <= 'z') ||
		(input >= 'A' && input <= 'Z')
}

// IsArabicNumber reports whether input is an ASCII digit.
func IsArabicNumber(input jdk.Char) bool { return input >= '0' && input <= '9' }

// IsCJKCharacter reports whether input belongs to one of the CJK, Hangul or
// kana blocks.
func IsCJKCharacter(input jdk.Char) bool {
	switch jdk.UnicodeBlockOf(input) {
	case jdk.CJKUnifiedIdeographs,
		jdk.CJKCompatibilityIdeographs,
		jdk.CJKUnifiedIdeographsExtensionA,
		jdk.HalfwidthAndFullwidthForms,
		jdk.HangulSyllables,
		jdk.HangulJamo,
		jdk.HangulCompatibilityJamo,
		jdk.Hiragana,
		jdk.Katakana,
		jdk.KatakanaPhoneticExtensions:
		return true
	default:
		return false
	}
}

// Regularize folds the ideographic space and the fullwidth forms down to ASCII
// and lowercases A-Z. Unlike CharacterUtil's version the lowercasing is not
// optional here.
func Regularize(input jdk.Char) jdk.Char {
	switch {
	case input == 12288:
		input = 32
	case input > 65280 && input < 65375:
		input -= 65248
	case input >= 'A' && input <= 'Z':
		input += 32
	}
	return input
}
