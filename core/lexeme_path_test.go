package core

import (
	"math"
	"strings"
	"testing"
)

func TestAddCrossLexemeGrowsTheSpan(t *testing.T) {
	p := newLexemePath()
	if !p.addCrossLexeme(NewLexeme(0, 0, 4, TypeCNWord)) {
		t.Fatal("the first lexeme should always be accepted")
	}
	if got, want := p.getPathBegin(), 0; got != want {
		t.Errorf("pathBegin = %d, want %d", got, want)
	}
	if got, want := p.getPathEnd(), 4; got != want {
		t.Errorf("pathEnd = %d, want %d", got, want)
	}
	if got, want := p.getPayloadLength(), 4; got != want {
		t.Errorf("payloadLength = %d, want %d", got, want)
	}

	if !p.addCrossLexeme(NewLexeme(0, 2, 4, TypeCNWord)) {
		t.Fatal("an overlapping lexeme should be accepted")
	}
	if got, want := p.getPathEnd(), 6; got != want {
		t.Errorf("pathEnd = %d, want %d", got, want)
	}
	// Once the path overlaps, the payload is the span, not the sum of lengths.
	if got, want := p.getPayloadLength(), 6; got != want {
		t.Errorf("payloadLength = %d, want %d", got, want)
	}
	if p.addCrossLexeme(NewLexeme(0, 10, 2, TypeCNWord)) {
		t.Error("a disjoint lexeme should be rejected")
	}
}

func TestAddNotCrossLexemeRejectsOverlaps(t *testing.T) {
	p := newLexemePath()
	p.addNotCrossLexeme(NewLexeme(0, 0, 4, TypeCNWord))
	if p.addNotCrossLexeme(NewLexeme(0, 2, 4, TypeCNWord)) {
		t.Error("an overlapping lexeme should be rejected")
	}
	if !p.addNotCrossLexeme(NewLexeme(0, 4, 2, TypeCNWord)) {
		t.Fatal("an adjacent lexeme should be accepted")
	}
	if got, want := p.getPayloadLength(), 6; got != want {
		t.Errorf("payloadLength = %d, want %d", got, want)
	}
	if got, want := p.getPathLength(), 6; got != want {
		t.Errorf("pathLength = %d, want %d", got, want)
	}
}

func TestRemoveTailShrinksTheSpanAndEmptiesCleanly(t *testing.T) {
	p := newLexemePath()
	p.addNotCrossLexeme(NewLexeme(0, 0, 2, TypeCNWord))
	p.addNotCrossLexeme(NewLexeme(0, 2, 3, TypeCNWord))

	if got := p.removeTail().Begin(); got != 2 {
		t.Errorf("removeTail returned begin %d, want 2", got)
	}
	if got, want := p.getPathEnd(), 2; got != want {
		t.Errorf("pathEnd = %d, want %d", got, want)
	}
	p.removeTail()
	if got, want := p.getPathBegin(), -1; got != want {
		t.Errorf("pathBegin = %d, want %d", got, want)
	}
	if got, want := p.getPayloadLength(), 0; got != want {
		t.Errorf("payloadLength = %d, want %d", got, want)
	}
}

func TestCheckCrossCoversBothOverlapDirections(t *testing.T) {
	p := newLexemePath()
	p.addNotCrossLexeme(NewLexeme(0, 4, 4, TypeCNWord)) // covers [4,8)

	if !p.checkCross(NewLexeme(0, 6, 4, TypeCNWord)) {
		t.Error("a lexeme starting inside the path crosses it")
	}
	if !p.checkCross(NewLexeme(0, 2, 4, TypeCNWord)) {
		t.Error("a lexeme the path starts inside crosses it")
	}
	if p.checkCross(NewLexeme(0, 8, 2, TypeCNWord)) {
		t.Error("an adjacent lexeme does not cross")
	}
	if p.checkCross(NewLexeme(0, 0, 4, TypeCNWord)) {
		t.Error("a preceding lexeme does not cross")
	}
}

// The X weight clamps each length at 10 and saturates rather than overflowing,
// so a path of many long words cannot wrap round to a small number.
func TestXWeightClampsAndSaturates(t *testing.T) {
	p := newLexemePath()
	p.addNotCrossLexeme(NewLexeme(0, 0, 3, TypeCNWord))
	p.addNotCrossLexeme(NewLexeme(0, 3, 4, TypeCNWord))
	if got, want := p.getXWeight(), 12; got != want {
		t.Errorf("XWeight = %d, want %d", got, want)
	}

	saturating := newLexemePath()
	for i := 0; i < 12; i++ {
		saturating.addNotCrossLexeme(NewLexeme(0, i*20, 20, TypeCNWord))
	}
	if got := saturating.getXWeight(); got != math.MaxInt32 {
		t.Errorf("XWeight = %d, want %d", got, math.MaxInt32)
	}
}

func TestPWeightWeightsLaterLexemesMore(t *testing.T) {
	p := newLexemePath()
	p.addNotCrossLexeme(NewLexeme(0, 0, 2, TypeCNWord))
	p.addNotCrossLexeme(NewLexeme(0, 2, 3, TypeCNWord))
	// 1*2 + 2*3
	if got, want := p.getPWeight(), 8; got != want {
		t.Errorf("PWeight = %d, want %d", got, want)
	}
}

func TestCopyIsIndependentOfItsSource(t *testing.T) {
	p := newLexemePath()
	p.addNotCrossLexeme(NewLexeme(0, 0, 2, TypeCNWord))
	p.addNotCrossLexeme(NewLexeme(0, 2, 2, TypeCNWord))

	c := p.copy()
	p.removeTail()

	if got, want := c.getSize(), 2; got != want {
		t.Errorf("copy size = %d, want %d", got, want)
	}
	if got, want := c.getPathEnd(), 4; got != want {
		t.Errorf("copy pathEnd = %d, want %d", got, want)
	}
}

// The ranking is what picks a segmentation out of an ambiguous crossing path,
// so each tier is checked with the tiers above it held equal.
func TestCompareToRanksCandidates(t *testing.T) {
	path := func(spans ...[2]int) *lexemePath {
		p := newLexemePath()
		for _, s := range spans {
			p.addNotCrossLexeme(NewLexeme(0, s[0], s[1], TypeCNWord))
		}
		return p
	}

	// 1. More text covered wins.
	more, less := path([2]int{0, 4}), path([2]int{0, 2})
	if got := more.compareTo(less); got != -1 {
		t.Errorf("payload: got %d, want -1", got)
	}
	if got := less.compareTo(more); got != 1 {
		t.Errorf("payload: got %d, want 1", got)
	}

	// 2. Equal payload, fewer lexemes wins.
	one, two := path([2]int{0, 4}), path([2]int{0, 2}, [2]int{2, 2})
	if got := one.compareTo(two); got != -1 {
		t.Errorf("size: got %d, want -1", got)
	}

	// 3. Equal payload and size, the wider span wins.
	wide, narrow := path([2]int{0, 2}, [2]int{4, 2}), path([2]int{0, 2}, [2]int{2, 2})
	if got := wide.compareTo(narrow); got != -1 {
		t.Errorf("pathLength: got %d, want -1", got)
	}

	// 4. Equal so far, the later end wins — reverse segmentation is usually right.
	later, earlier := path([2]int{2, 2}, [2]int{6, 2}), path([2]int{0, 2}, [2]int{4, 2})
	if got := later.compareTo(earlier); got != -1 {
		t.Errorf("pathEnd: got %d, want -1", got)
	}

	// 5. Identical paths compare equal.
	if got := path([2]int{0, 2}).compareTo(path([2]int{0, 2})); got != 0 {
		t.Errorf("identical: got %d, want 0", got)
	}
}

func TestStringUsesCarriageReturnLineEndings(t *testing.T) {
	p := newLexemePath()
	p.addNotCrossLexeme(NewLexeme(0, 0, 2, TypeCNWord))
	s := p.String()
	if !strings.Contains(s, "pathBegin  : 0\r\n") {
		t.Errorf("String = %q", s)
	}
	if !strings.Contains(s, "lexeme : ") {
		t.Errorf("String = %q", s)
	}
}
