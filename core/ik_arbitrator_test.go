package core

import "testing"

func TestFallbackPathUsesFullCrossPathEndAndStableOffset(t *testing.T) {
	crossPath := newLexemePath()
	crossPath.addCrossLexeme(NewLexeme(0, 0, 21, TypeCNWord))
	crossPath.addCrossLexeme(NewLexeme(0, 2, 60, TypeCNWord))
	for i := 4; i <= 21; i++ {
		crossPath.addCrossLexeme(NewLexeme(0, i, 21, TypeCNWord))
	}

	if got := crossPath.getSize(); got != 20 {
		t.Fatalf("crossPath size: expected 20, got %d", got)
	}

	fallback := buildFallback(crossPath)

	if fallback == nil {
		t.Fatal("fallback must not be nil")
	}
	if got := fallback.getSize(); got != 2 {
		t.Fatalf("fallback size: expected 2, got %d", got)
	}

	first := fallback.pollFirst()
	remain := fallback.pollFirst()

	if got := first.BeginPosition(); got != 0 {
		t.Errorf("first beginPosition: expected 0, got %d", got)
	}
	if got := first.EndPosition(); got != 21 {
		t.Errorf("first endPosition: expected 21, got %d", got)
	}

	if got := remain.Offset(); got != 0 {
		t.Errorf("remain offset: expected 0, got %d", got)
	}
	if got := remain.BeginPosition(); got != 21 {
		t.Errorf("remain beginPosition: expected 21, got %d", got)
	}
	if got := remain.EndPosition(); got != 62 {
		t.Errorf("remain endPosition: expected 62, got %d", got)
	}
	if got := remain.LexemeType(); got != TypeCNWord {
		t.Errorf("remain lexemeType: expected %d, got %d", TypeCNWord, got)
	}
}

func TestFallbackPathDoesNotTriggerForSmallLongCnWordCrossPath(t *testing.T) {
	crossPath := newLexemePath()
	for i := 0; i < 6; i++ {
		crossPath.addCrossLexeme(NewLexeme(0, i*2, 11, TypeCNWord))
	}

	if got := crossPath.getSize(); got != 6 {
		t.Fatalf("crossPath size: expected 6, got %d", got)
	}
	if fallback := buildFallback(crossPath); fallback != nil {
		t.Errorf("fallback must be nil, got %v", fallback)
	}
}

func TestFallbackPathStillTriggersWhenTotalLexemeCountIsTooHigh(t *testing.T) {
	crossPath := newLexemePath()
	crossPath.addCrossLexeme(NewLexeme(0, 0, 100, TypeCNWord))
	for i := 1; i <= 50; i++ {
		crossPath.addCrossLexeme(NewLexeme(0, i, 1, TypeCNWord))
	}

	fallback := buildFallback(crossPath)

	if fallback == nil {
		t.Fatal("fallback must not be nil")
	}
	if got := fallback.getSize(); got != 1 {
		t.Fatalf("fallback size: expected 1, got %d", got)
	}
	if got := fallback.peekFirst().EndPosition(); got != 100 {
		t.Errorf("first endPosition: expected 100, got %d", got)
	}
}

func buildFallback(crossPath *lexemePath) *lexemePath {
	return newIKArbitrator().tryBuildFallbackPathForComplexCrossPath(crossPath)
}
