package core

import "github.com/infinilabs/analysis-ik/dic"

// cjkSegmenterName identifies this sub-segmenter in the buffer lock set.
const cjkSegmenterName = "CJK_SEGMENTER"

// cjkSegmenter matches Chinese, Japanese and Korean words against the main
// dictionary. It keeps one pending hit per word still being spelled out.
type cjkSegmenter struct {
	tmpHits []*dic.Hit
}

func newCJKSegmenter() *cjkSegmenter { return &cjkSegmenter{} }

func (s *cjkSegmenter) analyze(context *analyzeContext) {
	if context.getCurrentCharType() != charUseless {
		// Extend the words already in progress before starting new ones.
		if len(s.tmpHits) > 0 {
			for _, hit := range append([]*dic.Hit(nil), s.tmpHits...) {
				hit = dic.GetSingleton().MatchWithHit(context.getSegmentBuff(), context.getCursor(), hit)
				switch {
				case hit.IsMatch():
					newLexeme := NewLexeme(context.getBufferOffset(), hit.Begin(),
						context.getCursor()-hit.Begin()+1, TypeCNWord)
					context.addLexeme(newLexeme)

					if !hit.IsPrefix() {
						// No longer word can extend this one; stop tracking it.
						s.removeHit(hit)
					}
				case hit.IsUnmatch():
					s.removeHit(hit)
				}
			}
		}

		// Then try the character under the cursor on its own.
		singleCharHit := dic.GetSingleton().MatchInMainDict(context.getSegmentBuff(), context.getCursor(), 1)
		switch {
		case singleCharHit.IsMatch():
			newLexeme := NewLexeme(context.getBufferOffset(), context.getCursor(), 1, TypeCNWord)
			context.addLexeme(newLexeme)

			if singleCharHit.IsPrefix() {
				s.tmpHits = append(s.tmpHits, singleCharHit)
			}
		case singleCharHit.IsPrefix():
			s.tmpHits = append(s.tmpHits, singleCharHit)
		}
	} else {
		s.tmpHits = s.tmpHits[:0]
	}

	if context.isBufferConsumed() {
		s.tmpHits = s.tmpHits[:0]
	}

	if len(s.tmpHits) == 0 {
		context.unlockBuffer(cjkSegmenterName)
	} else {
		context.lockBuffer(cjkSegmenterName)
	}
}

func (s *cjkSegmenter) removeHit(hit *dic.Hit) {
	for i, candidate := range s.tmpHits {
		if candidate == hit {
			s.tmpHits = append(s.tmpHits[:i], s.tmpHits[i+1:]...)
			return
		}
	}
}

func (s *cjkSegmenter) reset() { s.tmpHits = s.tmpHits[:0] }
