package core

import (
	"strconv"
	"strings"

	"github.com/infinilabs/analysis-ik/jdk"
)

// Lexeme types. The values are a mix of bit flags and small integers, and both
// the numbers and the strings LexemeTypeString returns are user-visible through
// the token stream's type attribute.
const (
	// TypeUnknown is an unclassified lexeme.
	TypeUnknown = 0
	// TypeEnglish is a run of ASCII letters.
	TypeEnglish = 1
	// TypeArabic is a run of ASCII digits.
	TypeArabic = 2
	// TypeLetter is a mixed letter/digit run.
	TypeLetter = 3
	// TypeCNWord is a Chinese word from the dictionary.
	TypeCNWord = 4
	// TypeCNChar is a single Chinese character.
	TypeCNChar = 64
	// TypeOtherCJK is a single Japanese or Korean character.
	TypeOtherCJK = 8
	// TypeCNum is a Chinese numeral.
	TypeCNum = 16
	// TypeCount is a Chinese quantifier.
	TypeCount = 32
	// TypeCQuan is a Chinese numeral joined to its quantifier.
	TypeCQuan = 48
)

// Lexeme is one token: where it sits in the input, how long it is, its text and
// its type. Positions are counted in UTF-16 code units, like the Java chars the
// engine reads.
type Lexeme struct {
	offset     int
	begin      int
	length     int
	lexemeText string
	lexemeType int
}

// NewLexeme builds a lexeme. A negative length is a programming error and
// panics, as it did in the original.
func NewLexeme(offset, begin, length, lexemeType int) *Lexeme {
	if length < 0 {
		panic("length < 0")
	}
	return &Lexeme{offset: offset, begin: begin, length: length, lexemeType: lexemeType}
}

// Equals reports whether two lexemes cover the same span. Text and type are
// deliberately not compared.
func (l *Lexeme) Equals(other *Lexeme) bool {
	if other == nil {
		return false
	}
	if l == other {
		return true
	}
	return l.offset == other.Offset() &&
		l.begin == other.Begin() &&
		l.length == other.Length()
}

// HashCode reproduces the original's 32-bit hash, wrap-around included. It
// divides by the lexeme length, so a zero-length lexeme panics.
func (l *Lexeme) HashCode() int32 {
	absBegin := int32(l.BeginPosition())
	absEnd := int32(l.EndPosition())
	return (absBegin * 37) + (absEnd * 31) + ((absBegin*absEnd)%int32(l.Length()))*11
}

// CompareTo orders lexemes by start position ascending, then by length
// descending — the longest match at a given position sorts first.
func (l *Lexeme) CompareTo(other *Lexeme) int {
	switch {
	case l.begin < other.Begin():
		return -1
	case l.begin == other.Begin():
		switch {
		case l.length > other.Length():
			return -1
		case l.length == other.Length():
			return 0
		default:
			return 1
		}
	default:
		return 1
	}
}

// Offset is the position of the analysis buffer within the whole input.
func (l *Lexeme) Offset() int { return l.offset }

// SetOffset moves the lexeme's buffer origin.
func (l *Lexeme) SetOffset(offset int) { l.offset = offset }

// Begin is the lexeme's start relative to the analysis buffer.
func (l *Lexeme) Begin() int { return l.begin }

// BeginPosition is the lexeme's start within the whole input.
func (l *Lexeme) BeginPosition() int { return l.offset + l.begin }

// SetBegin moves the lexeme's start within the buffer.
func (l *Lexeme) SetBegin(begin int) { l.begin = begin }

// EndPosition is the lexeme's end within the whole input, exclusive.
func (l *Lexeme) EndPosition() int { return l.offset + l.begin + l.length }

// Length is the lexeme's length in UTF-16 code units.
func (l *Lexeme) Length() int { return l.length }

// SetLength resizes the lexeme. The guard reads the current length rather than
// the argument; that is what the original does, and callers depend on nothing
// else.
func (l *Lexeme) SetLength(length int) {
	if l.length < 0 {
		panic("length < 0")
	}
	l.length = length
}

// LexemeText is the lexeme's text, empty until the engine fills it in.
func (l *Lexeme) LexemeText() string { return l.lexemeText }

// SetLexemeText sets the text and resets the length to match it.
func (l *Lexeme) SetLexemeText(lexemeText string) {
	l.lexemeText = lexemeText
	l.length = len(jdk.EncodeUTF16(lexemeText))
}

// LexemeType is the lexeme's type constant.
func (l *Lexeme) LexemeType() int { return l.lexemeType }

// SetLexemeType changes the lexeme's type.
func (l *Lexeme) SetLexemeType(lexemeType int) { l.lexemeType = lexemeType }

// unknownLexemeTypeName is reported for a type with no name of its own.
const unknownLexemeTypeName = "UNKNOWN"

// lexemeTypeNames are the type names the token stream reports. The CNUM and
// CQUAN names keep their TYPE_ prefix while the others drop it; that
// inconsistency is part of the output contract.
var lexemeTypeNames = map[int]string{
	TypeEnglish:  "ENGLISH",
	TypeArabic:   "ARABIC",
	TypeLetter:   "LETTER",
	TypeCNWord:   "CN_WORD",
	TypeCNChar:   "CN_CHAR",
	TypeOtherCJK: "OTHER_CJK",
	TypeCount:    "COUNT",
	TypeCNum:     "TYPE_CNUM",
	TypeCQuan:    "TYPE_CQUAN",
}

// LexemeTypeString is the type name the token stream reports.
func (l *Lexeme) LexemeTypeString() string {
	if name, ok := lexemeTypeNames[l.lexemeType]; ok {
		return name
	}
	return unknownLexemeTypeName
}

// Append merges an immediately adjacent lexeme into this one and retypes the
// result. It reports whether the merge happened.
func (l *Lexeme) Append(other *Lexeme, lexemeType int) bool {
	if other != nil && l.EndPosition() == other.BeginPosition() {
		l.length += other.Length()
		l.lexemeType = lexemeType
		return true
	}
	return false
}

// String renders the lexeme as "<begin>-<end> : <text> : \t<type>".
func (l *Lexeme) String() string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(l.BeginPosition()))
	b.WriteString("-")
	b.WriteString(strconv.Itoa(l.EndPosition()))
	b.WriteString(" : ")
	b.WriteString(l.lexemeText)
	b.WriteString(" : \t")
	b.WriteString(l.LexemeTypeString())
	return b.String()
}
