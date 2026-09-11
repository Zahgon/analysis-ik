package core

// Thresholds that decide when a crossing path is too tangled to arbitrate
// properly. Backtracking over a dense overlap is exponential, so the guard
// trades the best split for a bounded one.
const (
	longCNWordLengthThreshold   = 10
	longCNWordCountThreshold    = 5
	totalLexemeCountThreshold   = 50
	denseCrossPathSizeThreshold = 20
	denseOverlapRatioThreshold  = 4
)

// ikArbitrator resolves ambiguous splits: where several candidate lexemes
// overlap, it picks the combination that scores best under lexemePath.compareTo.
type ikArbitrator struct{}

func newIKArbitrator() *ikArbitrator { return &ikArbitrator{} }

// process groups the raw candidates into crossing paths and hands each one that
// is actually ambiguous to judge. With useSmart off nothing is arbitrated and
// every candidate survives.
func (a *ikArbitrator) process(context *analyzeContext, useSmart bool) {
	orgLexemes := context.getOrgLexemes()
	orgLexeme := orgLexemes.pollFirst()

	crossPath := newLexemePath()
	for orgLexeme != nil {
		if !crossPath.addCrossLexeme(orgLexeme) {
			// This lexeme starts a new, disjoint crossing path.
			if crossPath.getSize() == 1 || !useSmart {
				context.addLexemePath(crossPath)
			} else {
				context.addLexemePath(a.judge(crossPath))
			}

			crossPath = newLexemePath()
			crossPath.addCrossLexeme(orgLexeme)
		}
		orgLexeme = orgLexemes.pollFirst()
	}

	if crossPath.getSize() == 1 || !useSmart {
		context.addLexemePath(crossPath)
	} else {
		context.addLexemePath(a.judge(crossPath))
	}
}

// tryBuildFallbackPathForComplexCrossPath returns a cheap path for a crossing
// path too dense to arbitrate, and nil when the normal backtracking can run.
func (a *ikArbitrator) tryBuildFallbackPathForComplexCrossPath(crossPath *lexemePath) *lexemePath {
	if crossPath == nil || crossPath.isEmpty() {
		return nil
	}
	if !a.shouldFallbackForComplexCrossPath(crossPath) {
		return nil
	}
	return a.buildFallbackPath(crossPath)
}

// shouldFallbackForComplexCrossPath reports whether the path is too tangled:
// either it holds an outright excessive number of lexemes, or it is a dense mat
// of long overlapping words.
func (a *ikArbitrator) shouldFallbackForComplexCrossPath(crossPath *lexemePath) bool {
	if crossPath.getSize() > totalLexemeCountThreshold {
		return true
	}

	longCNWordCount := 0
	totalLexemeLength := int64(0)
	for current := crossPath.getHead(); current != nil && current.getLexeme() != nil; current = current.getNext() {
		lexeme := current.getLexeme()
		totalLexemeLength += int64(lexeme.Length())
		if lexeme.LexemeType() == TypeCNWord && lexeme.Length() > longCNWordLengthThreshold {
			longCNWordCount++
		}
	}

	pathLength := crossPath.getPathEnd() - crossPath.getPathBegin()
	if pathLength <= 0 {
		return false
	}

	return longCNWordCount > longCNWordCountThreshold &&
		crossPath.getSize() >= denseCrossPathSizeThreshold &&
		totalLexemeLength >= int64(pathLength)*denseOverlapRatioThreshold
}

// buildFallbackPath keeps the first lexeme and collapses everything after it
// into a single synthetic word, so offsets still advance monotonically.
func (a *ikArbitrator) buildFallbackPath(crossPath *lexemePath) *lexemePath {
	firstLexeme := crossPath.peekFirst()
	if firstLexeme == nil {
		return nil
	}

	fallbackPath := newLexemePath()
	if !fallbackPath.addNotCrossLexeme(firstLexeme) {
		return nil
	}

	remainStart := firstLexeme.Begin() + firstLexeme.Length()
	remainLength := crossPath.getPathEnd() - remainStart

	if remainLength > 0 {
		remainLexeme := NewLexeme(firstLexeme.Offset(), remainStart, remainLength, TypeCNWord)
		if !fallbackPath.addNotCrossLexeme(remainLexeme) {
			return nil
		}
	}

	return fallbackPath
}

// judge enumerates the non-overlapping combinations of a crossing path and
// returns the best-ranked one.
func (a *ikArbitrator) judge(crossPath *lexemePath) *lexemePath {
	if fallbackPath := a.tryBuildFallbackPathForComplexCrossPath(crossPath); fallbackPath != nil {
		return fallbackPath
	}

	lexemeCell := crossPath.getHead()

	option := newLexemePath()
	// One forward pass gives a first candidate plus the lexemes it had to skip.
	lexemeStack := a.forwardPath(lexemeCell, option)

	// best keeps the earliest-inserted candidate among equals, which is what a
	// TreeSet of paths ordered by compareTo returned from first().
	best := option.copy()

	for len(lexemeStack) > 0 {
		c := lexemeStack[len(lexemeStack)-1]
		lexemeStack = lexemeStack[:len(lexemeStack)-1]
		// Roll the chain back far enough to accept the skipped lexeme, then
		// carry on from there to build an alternative.
		a.backPath(c.getLexeme(), option)
		a.forwardPath(c, option)
		if candidate := option.copy(); candidate.compareTo(best) < 0 {
			best = candidate
		}
	}

	return best
}

// forwardPath walks the chain adding every lexeme that fits, and returns the
// ones that collided, most recent first.
func (a *ikArbitrator) forwardPath(lexemeCell *cell, option *lexemePath) []*cell {
	var conflictStack []*cell
	for c := lexemeCell; c != nil && c.getLexeme() != nil; c = c.getNext() {
		if !option.addNotCrossLexeme(c.getLexeme()) {
			conflictStack = append(conflictStack, c)
		}
	}
	return conflictStack
}

// backPath drops lexemes from the end of the chain until l no longer overlaps.
func (a *ikArbitrator) backPath(l *Lexeme, option *lexemePath) {
	for option.checkCross(l) {
		option.removeTail()
	}
}
