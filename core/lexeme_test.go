package core

import "testing"

func TestNewLexemeRejectsANegativeLength(t *testing.T) {
	defer func() {
		if r := recover(); r != "length < 0" {
			t.Errorf("recover() = %v, want \"length < 0\"", r)
		}
	}()
	NewLexeme(0, 0, -1, TypeCNWord)
}

// Equality is positional: two lexemes covering the same span are the same
// lexeme, whatever their type or text. QuickSortSet depends on it to drop the
// duplicates the three letter state machines produce.
func TestEqualsComparesTheSpanOnly(t *testing.T) {
	base := NewLexeme(10, 2, 3, TypeCNWord)
	base.SetLexemeText("abc")

	same := NewLexeme(10, 2, 3, TypeLetter)
	if !base.Equals(same) {
		t.Error("lexemes covering the same span should be equal")
	}
	if !base.Equals(base) {
		t.Error("a lexeme should equal itself")
	}
	if base.Equals(nil) {
		t.Error("a lexeme should not equal nil")
	}
	for _, other := range []*Lexeme{
		NewLexeme(11, 2, 3, TypeCNWord),
		NewLexeme(10, 3, 3, TypeCNWord),
		NewLexeme(10, 2, 4, TypeCNWord),
	} {
		if base.Equals(other) {
			t.Errorf("%v should not equal %v", base, other)
		}
	}
}

func TestHashCodeUsesWrappingIntArithmetic(t *testing.T) {
	l := NewLexeme(0, 0, 2, TypeCNWord)
	// absBegin 0, absEnd 2: (0*37) + (2*31) + ((0*2)%2)*11 = 62.
	if got := l.HashCode(); got != 62 {
		t.Errorf("HashCode = %d, want 62", got)
	}
	// A huge span overflows a 32-bit int rather than widening.
	big := NewLexeme(0, 1<<30, 1, TypeCNWord)
	_ = big.HashCode()
}

func TestCompareToOrdersByStartThenByDescendingLength(t *testing.T) {
	cases := []struct {
		left, right *Lexeme
		want        int
	}{
		{NewLexeme(0, 0, 2, TypeCNWord), NewLexeme(0, 1, 2, TypeCNWord), -1},
		{NewLexeme(0, 1, 2, TypeCNWord), NewLexeme(0, 0, 2, TypeCNWord), 1},
		{NewLexeme(0, 0, 3, TypeCNWord), NewLexeme(0, 0, 2, TypeCNWord), -1},
		{NewLexeme(0, 0, 2, TypeCNWord), NewLexeme(0, 0, 3, TypeCNWord), 1},
		{NewLexeme(0, 0, 2, TypeCNWord), NewLexeme(0, 0, 2, TypeLetter), 0},
	}
	for _, c := range cases {
		if got := c.left.CompareTo(c.right); got != c.want {
			t.Errorf("%v.CompareTo(%v) = %d, want %d", c.left, c.right, got, c.want)
		}
	}
}

func TestPositionsAreRelativeToTheBufferOrigin(t *testing.T) {
	l := NewLexeme(100, 5, 3, TypeCNWord)
	if got := l.BeginPosition(); got != 105 {
		t.Errorf("BeginPosition = %d, want 105", got)
	}
	if got := l.EndPosition(); got != 108 {
		t.Errorf("EndPosition = %d, want 108", got)
	}
	l.SetOffset(0)
	l.SetBegin(1)
	if got, want := l.BeginPosition(), 1; got != want {
		t.Errorf("BeginPosition = %d, want %d", got, want)
	}
}

// The text carries the length with it, and the length is in UTF-16 code units,
// so a non-BMP character counts as two.
func TestSetLexemeTextResetsTheLength(t *testing.T) {
	l := NewLexeme(0, 0, 7, TypeCNWord)
	l.SetLexemeText("中华")
	if got := l.Length(); got != 2 {
		t.Errorf("Length = %d, want 2", got)
	}
	l.SetLexemeText("\U000F112E")
	if got := l.Length(); got != 2 {
		t.Errorf("Length = %d, want 2 for a surrogate pair", got)
	}
	l.SetLexemeText("")
	if got, want := l.Length(), 0; got != want {
		t.Errorf("Length = %d, want %d", got, want)
	}
	if got := l.LexemeText(); got != "" {
		t.Errorf("LexemeText = %q, want empty", got)
	}
}

// SetLength's guard reads the current length rather than the argument. That is
// what the original does, and nothing calls it with a negative length.
func TestSetLengthChecksTheCurrentLength(t *testing.T) {
	l := NewLexeme(0, 0, 3, TypeCNWord)
	l.SetLength(9)
	if got := l.Length(); got != 9 {
		t.Errorf("Length = %d, want 9", got)
	}
}

func TestLexemeTypeStringKeepsTheOriginalInconsistency(t *testing.T) {
	cases := map[int]string{
		TypeUnknown:  "UNKNOWN",
		TypeEnglish:  "ENGLISH",
		TypeArabic:   "ARABIC",
		TypeLetter:   "LETTER",
		TypeCNWord:   "CN_WORD",
		TypeCNChar:   "CN_CHAR",
		TypeOtherCJK: "OTHER_CJK",
		TypeCount:    "COUNT",
		// These two keep the TYPE_ prefix the others drop.
		TypeCNum:  "TYPE_CNUM",
		TypeCQuan: "TYPE_CQUAN",
		999:       "UNKNOWN",
	}
	for lexemeType, want := range cases {
		l := NewLexeme(0, 0, 1, lexemeType)
		if got := l.LexemeTypeString(); got != want {
			t.Errorf("type %d = %q, want %q", lexemeType, got, want)
		}
	}
	l := NewLexeme(0, 0, 1, TypeCNWord)
	l.SetLexemeType(TypeCount)
	if got := l.LexemeTypeString(); got != "COUNT" {
		t.Errorf("after SetLexemeType = %q, want COUNT", got)
	}
}

func TestAppendMergesOnlyAdjacentLexemes(t *testing.T) {
	l := NewLexeme(0, 0, 4, TypeArabic)
	if !l.Append(NewLexeme(0, 4, 1, TypeCount), TypeCQuan) {
		t.Fatal("adjacent lexemes should merge")
	}
	if got, want := l.Length(), 5; got != want {
		t.Errorf("Length = %d, want %d", got, want)
	}
	if got := l.LexemeType(); got != TypeCQuan {
		t.Errorf("LexemeType = %d, want %d", got, TypeCQuan)
	}
	if l.Append(NewLexeme(0, 6, 1, TypeCount), TypeCQuan) {
		t.Error("a gap should prevent the merge")
	}
	if l.Append(nil, TypeCQuan) {
		t.Error("nil should not merge")
	}
}

func TestStringRendersSpanTextAndType(t *testing.T) {
	l := NewLexeme(10, 2, 3, TypeCNWord)
	l.SetLexemeText("数据库")
	if got, want := l.String(), "12-15 : 数据库 : \tCN_WORD"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}
