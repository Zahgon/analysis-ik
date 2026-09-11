package core

import "github.com/infinilabs/analysis-ik/jdk"

// surrogatePairSegmenterName identifies this sub-segmenter in the buffer lock set.
const surrogatePairSegmenterName = "SURROGATE_PAIR_SEGMENTER"

// surrogatePairSegmenter keeps a non-BMP character together: the two code units
// that spell it become one lexeme rather than two unusable halves.
type surrogatePairSegmenter struct {
	start int
	end   int

	highSurrogate    jdk.Char
	hasHighSurrogate bool
}

func newSurrogatePairSegmenter() *surrogatePairSegmenter {
	return &surrogatePairSegmenter{start: -1, end: -1}
}

func (s *surrogatePairSegmenter) analyze(context *analyzeContext) {
	s.processSurrogatePairs(context)

	if s.start == -1 && s.end == -1 && !s.hasHighSurrogate {
		context.unlockBuffer(surrogatePairSegmenterName)
	} else {
		context.lockBuffer(surrogatePairSegmenterName)
	}
}

func (s *surrogatePairSegmenter) reset() {
	s.start = -1
	s.end = -1
	s.hasHighSurrogate = false
}

func (s *surrogatePairSegmenter) processSurrogatePairs(context *analyzeContext) {
	currentChar := context.getCurrentChar()

	switch {
	case jdk.IsHighSurrogate(currentChar):
		s.highSurrogate, s.hasHighSurrogate = currentChar, true
		s.start = context.getCursor()
	case jdk.IsLowSurrogate(currentChar) && s.hasHighSurrogate:
		s.end = context.getCursor()
		s.outputSurrogatePairLexeme(context)
		s.hasHighSurrogate = false
	default:
		if s.hasHighSurrogate {
			// The high surrogate never found its partner; emit it alone.
			s.outputSingleCharLexeme(context, s.start)
			s.hasHighSurrogate = false
		}
		s.start = -1
		s.end = -1
	}

	if context.isBufferConsumed() && s.hasHighSurrogate {
		s.outputSingleCharLexeme(context, s.start)
		s.hasHighSurrogate = false
		s.start = -1
		s.end = -1
	}
}

func (s *surrogatePairSegmenter) outputSurrogatePairLexeme(context *analyzeContext) {
	if s.start > -1 && s.end > -1 {
		lexemeText := jdk.DecodeUTF16([]jdk.Char{
			context.getSegmentBuff()[s.start],
			context.getSegmentBuff()[s.end],
		})
		lexeme := NewLexeme(context.getBufferOffset(), s.start, s.end-s.start+1, TypeCNChar)
		lexeme.SetLexemeText(lexemeText)
		context.addLexeme(lexeme)
	}
}

func (s *surrogatePairSegmenter) outputSingleCharLexeme(context *analyzeContext, position int) {
	if position > -1 {
		lexemeText := jdk.DecodeUTF16([]jdk.Char{context.getSegmentBuff()[position]})
		lexeme := NewLexeme(context.getBufferOffset(), position, 1, TypeCNWord)
		lexeme.SetLexemeText(lexemeText)
		context.addLexeme(lexeme)
	}
}
