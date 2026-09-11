package core

import (
	"github.com/infinilabs/analysis-ik/dic"
	"github.com/infinilabs/analysis-ik/jdk"
)

// quanSegmenterName identifies this sub-segmenter in the buffer lock set.
const quanSegmenterName = "QUAN_SEGMENTER"

// chnNum lists every character that can spell a Chinese numeral, everyday and
// financial forms alike.
const chnNum = "一二两三四五六七八九十零壹贰叁肆伍陆柒捌玖拾百千万亿拾佰仟萬億兆卅廿"

var chnNumberChars = func() map[jdk.Char]bool {
	chars := map[jdk.Char]bool{}
	for _, c := range jdk.EncodeUTF16(chnNum) {
		chars[c] = true
	}
	return chars
}()

// cnQuantifierSegmenter recognises Chinese numerals, and the quantifiers that
// follow them. A quantifier is only looked for next to a numeral, so a common
// word that happens to be a measure word is not split out of ordinary prose.
type cnQuantifierSegmenter struct {
	// nStart and nEnd delimit the numeral run in progress; -1 when none is.
	nStart int
	nEnd   int

	countHits []*dic.Hit
}

func newCNQuantifierSegmenter() *cnQuantifierSegmenter {
	return &cnQuantifierSegmenter{nStart: -1, nEnd: -1}
}

func (s *cnQuantifierSegmenter) analyze(context *analyzeContext) {
	s.processCNumber(context)
	s.processCount(context)

	if s.nStart == -1 && s.nEnd == -1 && len(s.countHits) == 0 {
		context.unlockBuffer(quanSegmenterName)
	} else {
		context.lockBuffer(quanSegmenterName)
	}
}

func (s *cnQuantifierSegmenter) reset() {
	s.nStart = -1
	s.nEnd = -1
	s.countHits = s.countHits[:0]
}

func (s *cnQuantifierSegmenter) processCNumber(context *analyzeContext) {
	if s.nStart == -1 && s.nEnd == -1 {
		if context.getCurrentCharType() == charChinese && chnNumberChars[context.getCurrentChar()] {
			s.nStart = context.getCursor()
			s.nEnd = context.getCursor()
		}
	} else {
		if context.getCurrentCharType() == charChinese && chnNumberChars[context.getCurrentChar()] {
			s.nEnd = context.getCursor()
		} else {
			s.outputNumLexeme(context)
			s.nStart = -1
			s.nEnd = -1
		}
	}

	if context.isBufferConsumed() && s.nStart != -1 && s.nEnd != -1 {
		s.outputNumLexeme(context)
		s.nStart = -1
		s.nEnd = -1
	}
}

func (s *cnQuantifierSegmenter) processCount(context *analyzeContext) {
	if !s.needCountScan(context) {
		return
	}

	if context.getCurrentCharType() == charChinese {
		// Extend the quantifiers already in progress before starting new ones.
		if len(s.countHits) > 0 {
			for _, hit := range append([]*dic.Hit(nil), s.countHits...) {
				hit = dic.GetSingleton().MatchWithHit(context.getSegmentBuff(), context.getCursor(), hit)
				switch {
				case hit.IsMatch():
					newLexeme := NewLexeme(context.getBufferOffset(), hit.Begin(),
						context.getCursor()-hit.Begin()+1, TypeCount)
					context.addLexeme(newLexeme)

					if !hit.IsPrefix() {
						s.removeHit(hit)
					}
				case hit.IsUnmatch():
					s.removeHit(hit)
				}
			}
		}

		// A single character only counts as a quantifier when a numeral ends
		// exactly where it begins; otherwise an ordinary word would be split.
		shouldMatchSingleChar := false
		if !context.getOrgLexemes().isEmpty() {
			l := context.getOrgLexemes().peekLast()
			if (l.LexemeType() == TypeCNum || l.LexemeType() == TypeArabic) &&
				l.Begin()+l.Length() == context.getCursor() {
				shouldMatchSingleChar = true
			}
		}
		if shouldMatchSingleChar || len(s.countHits) > 0 {
			singleCharHit := dic.GetSingleton().MatchInQuantifierDict(
				context.getSegmentBuff(), context.getCursor(), 1)
			switch {
			case singleCharHit.IsMatch():
				newLexeme := NewLexeme(context.getBufferOffset(), context.getCursor(), 1, TypeCount)
				context.addLexeme(newLexeme)

				if singleCharHit.IsPrefix() {
					s.countHits = append(s.countHits, singleCharHit)
				}
			case singleCharHit.IsPrefix():
				s.countHits = append(s.countHits, singleCharHit)
			}
		}
	} else {
		s.countHits = s.countHits[:0]
	}

	if context.isBufferConsumed() {
		s.countHits = s.countHits[:0]
	}
}

// needCountScan reports whether a quantifier could start here: a numeral run is
// open, a quantifier is half-matched, or the previous lexeme is a numeral that
// ends exactly at the cursor.
func (s *cnQuantifierSegmenter) needCountScan(context *analyzeContext) bool {
	if (s.nStart != -1 && s.nEnd != -1) || len(s.countHits) > 0 {
		return true
	}
	if !context.getOrgLexemes().isEmpty() {
		l := context.getOrgLexemes().peekLast()
		if (l.LexemeType() == TypeCNum || l.LexemeType() == TypeArabic) &&
			l.Begin()+l.Length() == context.getCursor() {
			return true
		}
	}
	return false
}

func (s *cnQuantifierSegmenter) outputNumLexeme(context *analyzeContext) {
	if s.nStart > -1 && s.nEnd > -1 {
		newLexeme := NewLexeme(context.getBufferOffset(), s.nStart, s.nEnd-s.nStart+1, TypeCNum)
		context.addLexeme(newLexeme)
	}
}

func (s *cnQuantifierSegmenter) removeHit(hit *dic.Hit) {
	for i, candidate := range s.countHits {
		if candidate == hit {
			s.countHits = append(s.countHits[:i], s.countHits[i+1:]...)
			return
		}
	}
}
