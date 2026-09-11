package core

import (
	"sync"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/jdk"
)

// IKSegmenter is the segmentation engine: it pulls characters from a reader,
// runs the four sub-segmenters over them, arbitrates the ambiguities and hands
// out one lexeme at a time.
type IKSegmenter struct {
	mu sync.Mutex

	input      jdk.Reader
	context    *analyzeContext
	segmenters []segmenter
	arbitrator *ikArbitrator

	configuration cfg.Configuration

	// savedMaxConsumedEndPosition preserves the context's furthest consumed
	// position across the reset that end-of-input triggers.
	savedMaxConsumedEndPosition int
	// savedSkippedCount accumulates the stop words skipped in one Next call.
	savedSkippedCount int
}

// NewIKSegmenter builds a segmenter reading from input.
func NewIKSegmenter(input jdk.Reader, configuration cfg.Configuration) *IKSegmenter {
	s := &IKSegmenter{input: input, configuration: configuration}
	s.context = newAnalyzeContext(configuration)
	s.segmenters = loadSegmenters()
	s.arbitrator = newIKArbitrator()
	return s
}

// loadSegmenters returns the sub-segmenters in the order they run. The order is
// observable: the result set drops a later duplicate of a span an earlier
// sub-segmenter already produced.
func loadSegmenters() []segmenter {
	return []segmenter{
		newLetterSegmenter(),
		newSurrogatePairSegmenter(),
		newCNQuantifierSegmenter(),
		newCJKSegmenter(),
	}
}

// Next returns the next lexeme, or nil once the input is exhausted.
func (s *IKSegmenter) Next() (*Lexeme, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.savedSkippedCount = 0
	for {
		if l := s.context.getNextLexeme(); l != nil {
			s.savedSkippedCount += s.context.getLastSkippedCount()
			return l, nil
		}

		available, err := s.context.fillBuffer(s.input)
		if err != nil {
			return nil, err
		}
		if available <= 0 {
			// Snapshot before reset, so end() can still report a final offset.
			s.savedMaxConsumedEndPosition = s.context.getMaxConsumedEndPosition()
			s.context.reset()
			return nil, nil
		}

		s.context.initCursor()
		for {
			for _, sub := range s.segmenters {
				sub.analyze(s.context)
			}
			if s.context.needRefillBuffer() {
				break
			}
			if !s.context.moveCursor() {
				break
			}
		}
		for _, sub := range s.segmenters {
			sub.reset()
		}

		s.arbitrator.process(s.context, s.configuration.UseSmart())
		s.context.outputToResult()
		s.context.markBufferOffset()
	}
}

// Reset points the segmenter at a new input and clears every bit of state.
func (s *IKSegmenter) Reset(input jdk.Reader) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.input = input
	s.context.reset()
	for _, sub := range s.segmenters {
		sub.reset()
	}
	s.savedMaxConsumedEndPosition = 0
	s.savedSkippedCount = 0
}

// LastUselessCharNum is how many non-CJK characters trail the input.
func (s *IKSegmenter) LastUselessCharNum() int { return s.context.getLastUselessCharNum() }

// SavedMaxConsumedEndPosition is the furthest position any consumed lexeme
// reached, stop-word-filtered ones included. It is what lets the tokenizer
// report a correct final offset for a value that is entirely stop words.
func (s *IKSegmenter) SavedMaxConsumedEndPosition() int {
	return max(s.savedMaxConsumedEndPosition, s.context.getMaxConsumedEndPosition())
}

// SavedSkippedCount is how many stop words the last Next skipped, which becomes
// the token's position increment.
func (s *IKSegmenter) SavedSkippedCount() int { return s.savedSkippedCount }
