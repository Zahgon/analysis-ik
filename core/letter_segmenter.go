package core

import "github.com/infinilabs/analysis-ik/jdk"

// letterSegmenterName identifies this sub-segmenter in the buffer lock set.
const letterSegmenterName = "LETTER_SEGMENTER"

// letterConnectors may appear inside a mixed letter/digit token — an e-mail
// address or a product code — without ending it.
var letterConnectors = []jdk.Char{'#', '&', '+', '-', '.', '@', '_'}

// numConnectors may appear inside a run of digits without ending it, so a
// thousands separator or a decimal point does not split the number.
var numConnectors = []jdk.Char{',', '.'}

// letterSegmenter handles ASCII letters and digits. It runs three independent
// state machines over the same input — pure letters, pure digits, and the two
// mixed together — and lets the result set drop whichever spans coincide.
type letterSegmenter struct {
	// start and end delimit the mixed letter/digit token in progress; start is
	// -1 when none is. end is the last letter or digit seen, never a connector.
	start int
	end   int

	englishStart int
	englishEnd   int

	arabicStart int
	arabicEnd   int
}

func newLetterSegmenter() *letterSegmenter {
	return &letterSegmenter{start: -1, end: -1, englishStart: -1, englishEnd: -1, arabicStart: -1, arabicEnd: -1}
}

func (s *letterSegmenter) analyze(context *analyzeContext) {
	bufferLockFlag := false
	bufferLockFlag = s.processEnglishLetter(context) || bufferLockFlag
	bufferLockFlag = s.processArabicLetter(context) || bufferLockFlag
	// The mixed pass runs last so QuickSortSet can drop the spans the two pure
	// passes already produced.
	bufferLockFlag = s.processMixLetter(context) || bufferLockFlag

	if bufferLockFlag {
		context.lockBuffer(letterSegmenterName)
	} else {
		context.unlockBuffer(letterSegmenterName)
	}
}

func (s *letterSegmenter) reset() {
	s.start = -1
	s.end = -1
	s.englishStart = -1
	s.englishEnd = -1
	s.arabicStart = -1
	s.arabicEnd = -1
}

// processMixLetter emits tokens such as "windows2000" or "linliangyi2005@gmail.com".
func (s *letterSegmenter) processMixLetter(context *analyzeContext) bool {
	if s.start == -1 {
		if context.getCurrentCharType() == charArabic || context.getCurrentCharType() == charEnglish {
			s.start = context.getCursor()
			s.end = s.start
		}
	} else {
		switch {
		case context.getCurrentCharType() == charArabic || context.getCurrentCharType() == charEnglish:
			s.end = context.getCursor()
		case context.getCurrentCharType() == charUseless && isLetterConnector(context.getCurrentChar()):
			s.end = context.getCursor()
		default:
			newLexeme := NewLexeme(context.getBufferOffset(), s.start, s.end-s.start+1, TypeLetter)
			context.addLexeme(newLexeme)
			s.start = -1
			s.end = -1
		}
	}

	if context.isBufferConsumed() && s.start != -1 && s.end != -1 {
		newLexeme := NewLexeme(context.getBufferOffset(), s.start, s.end-s.start+1, TypeLetter)
		context.addLexeme(newLexeme)
		s.start = -1
		s.end = -1
	}

	return !(s.start == -1 && s.end == -1)
}

// processEnglishLetter emits runs of ASCII letters.
func (s *letterSegmenter) processEnglishLetter(context *analyzeContext) bool {
	if s.englishStart == -1 {
		if context.getCurrentCharType() == charEnglish {
			s.englishStart = context.getCursor()
			s.englishEnd = s.englishStart
		}
	} else {
		if context.getCurrentCharType() == charEnglish {
			s.englishEnd = context.getCursor()
		} else {
			newLexeme := NewLexeme(context.getBufferOffset(), s.englishStart,
				s.englishEnd-s.englishStart+1, TypeEnglish)
			context.addLexeme(newLexeme)
			s.englishStart = -1
			s.englishEnd = -1
		}
	}

	if context.isBufferConsumed() && s.englishStart != -1 && s.englishEnd != -1 {
		newLexeme := NewLexeme(context.getBufferOffset(), s.englishStart,
			s.englishEnd-s.englishStart+1, TypeEnglish)
		context.addLexeme(newLexeme)
		s.englishStart = -1
		s.englishEnd = -1
	}

	return !(s.englishStart == -1 && s.englishEnd == -1)
}

// processArabicLetter emits runs of ASCII digits.
func (s *letterSegmenter) processArabicLetter(context *analyzeContext) bool {
	if s.arabicStart == -1 {
		if context.getCurrentCharType() == charArabic {
			s.arabicStart = context.getCursor()
			s.arabicEnd = s.arabicStart
		}
	} else {
		switch {
		case context.getCurrentCharType() == charArabic:
			s.arabicEnd = context.getCursor()
		case context.getCurrentCharType() == charUseless && isNumConnector(context.getCurrentChar()):
			// A separator does not extend the number, but does not end it either.
		default:
			newLexeme := NewLexeme(context.getBufferOffset(), s.arabicStart,
				s.arabicEnd-s.arabicStart+1, TypeArabic)
			context.addLexeme(newLexeme)
			s.arabicStart = -1
			s.arabicEnd = -1
		}
	}

	if context.isBufferConsumed() && s.arabicStart != -1 && s.arabicEnd != -1 {
		newLexeme := NewLexeme(context.getBufferOffset(), s.arabicStart,
			s.arabicEnd-s.arabicStart+1, TypeArabic)
		context.addLexeme(newLexeme)
		s.arabicStart = -1
		s.arabicEnd = -1
	}

	return !(s.arabicStart == -1 && s.arabicEnd == -1)
}

func isLetterConnector(input jdk.Char) bool { return containsChar(letterConnectors, input) }

func isNumConnector(input jdk.Char) bool { return containsChar(numConnectors, input) }

func containsChar(table []jdk.Char, input jdk.Char) bool {
	for _, c := range table {
		if c == input {
			return true
		}
	}
	return false
}
