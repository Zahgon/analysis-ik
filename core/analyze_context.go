package core

import (
	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/dic"
	"github.com/infinilabs/analysis-ik/jdk"
)

const (
	// buffSize is how many code units the engine analyses at a time.
	buffSize = 4096
	// buffExhaustCritical is how close to the end of the buffer the cursor has
	// to get before more input is pulled in.
	buffExhaustCritical = 100
)

// analyzeContext is the state one segmentation run shares: the character
// buffer, the cursor walking it, the locks the sub-segmenters take on it, and
// the results they produce.
type analyzeContext struct {
	segmentBuff []jdk.Char
	charTypes   []int

	// buffOffset is where the current buffer starts within the whole input.
	buffOffset int
	cursor     int
	// available is how many code units the last read made usable.
	available int
	// lastUselessCharNum counts the non-CJK characters at the end of the input.
	lastUselessCharNum int

	// buffLocker holds the names of the sub-segmenters that are mid-token and
	// therefore need the buffer to stay put.
	buffLocker map[string]bool

	// orgLexemes are the raw candidates, before ambiguity arbitration.
	orgLexemes *quickSortSet
	// pathMap indexes the arbitrated paths by their start position.
	pathMap map[int]*lexemePath
	results []*Lexeme

	cfg cfg.Configuration

	// maxConsumedEndPosition is the furthest end position of any lexeme that has
	// been consumed, including the ones dropped as stop words. It is what lets
	// end() report the right final offset for a value that is entirely stop
	// words (issue #921).
	maxConsumedEndPosition int
	// lastSkippedCount is how many stop words the last NextLexeme skipped.
	lastSkippedCount int
}

func newAnalyzeContext(configuration cfg.Configuration) *analyzeContext {
	return &analyzeContext{
		segmentBuff: make([]jdk.Char, buffSize),
		charTypes:   make([]int, buffSize),
		buffLocker:  map[string]bool{},
		orgLexemes:  &quickSortSet{},
		pathMap:     map[int]*lexemePath{},
		results:     make([]*Lexeme, 0),
		cfg:         configuration,
	}
}

func (c *analyzeContext) getCursor() int             { return c.cursor }
func (c *analyzeContext) getSegmentBuff() []jdk.Char { return c.segmentBuff }
func (c *analyzeContext) getCurrentChar() jdk.Char   { return c.segmentBuff[c.cursor] }
func (c *analyzeContext) getCurrentCharType() int    { return c.charTypes[c.cursor] }
func (c *analyzeContext) getBufferOffset() int       { return c.buffOffset }

// fillBuffer tops the buffer up, moving whatever the cursor has not reached yet
// to the front first. It returns how many code units are now analysable, or a
// non-positive count once the input is exhausted.
func (c *analyzeContext) fillBuffer(reader jdk.Reader) (int, error) {
	readCount := 0
	if c.buffOffset == 0 {
		n, err := jdk.ReadInto(reader, c.segmentBuff)
		if err != nil {
			return 0, err
		}
		readCount = n
		c.lastUselessCharNum = 0
	} else {
		// The cursor sits on a character that has been processed, so the
		// unprocessed tail starts one past it.
		offset := c.available - (c.cursor + 1)
		if offset > 0 {
			copy(c.segmentBuff[:offset], c.segmentBuff[c.cursor+1:c.cursor+1+offset])
			readCount = offset
		}
		numRead, err := reader.Read(c.segmentBuff, offset, buffSize-offset)
		if err != nil {
			return 0, err
		}
		if numRead != jdk.EOF {
			readCount += numRead
		}
	}
	c.available = readCount
	c.cursor = 0
	return readCount, nil
}

// initCursor places the cursor on the first character and classifies it.
func (c *analyzeContext) initCursor() {
	c.cursor = 0
	c.segmentBuff[c.cursor] = regularize(c.segmentBuff[c.cursor], c.cfg.EnableLowercase())
	c.charTypes[c.cursor] = identifyCharType(c.segmentBuff[c.cursor])
}

// moveCursor advances one character, classifying it, and reports false once the
// buffer is used up.
func (c *analyzeContext) moveCursor() bool {
	if c.cursor < c.available-1 {
		c.cursor++
		c.segmentBuff[c.cursor] = regularize(c.segmentBuff[c.cursor], c.cfg.EnableLowercase())
		c.charTypes[c.cursor] = identifyCharType(c.segmentBuff[c.cursor])
		return true
	}
	return false
}

// lockBuffer records that a sub-segmenter is mid-token.
func (c *analyzeContext) lockBuffer(segmenterName string) { c.buffLocker[segmenterName] = true }

// unlockBuffer records that a sub-segmenter has finished its token.
func (c *analyzeContext) unlockBuffer(segmenterName string) { delete(c.buffLocker, segmenterName) }

// isBufferLocked reports whether any sub-segmenter still needs the buffer.
func (c *analyzeContext) isBufferLocked() bool { return len(c.buffLocker) > 0 }

// isBufferConsumed reports whether the cursor has reached the last usable
// character.
func (c *analyzeContext) isBufferConsumed() bool { return c.cursor == c.available-1 }

// needRefillBuffer reports whether the buffer is full, the cursor has entered
// the critical zone near its end, and no sub-segmenter is holding it.
func (c *analyzeContext) needRefillBuffer() bool {
	return c.available == buffSize &&
		c.cursor < c.available-1 &&
		c.cursor > c.available-buffExhaustCritical &&
		!c.isBufferLocked()
}

// markBufferOffset advances the buffer's origin past everything analysed.
func (c *analyzeContext) markBufferOffset() { c.buffOffset += c.cursor + 1 }

// addLexeme records one raw candidate.
func (c *analyzeContext) addLexeme(lexeme *Lexeme) { c.orgLexemes.addLexeme(lexeme) }

// addLexemePath records one arbitrated path, indexed by where it starts.
func (c *analyzeContext) addLexemePath(path *lexemePath) {
	if path != nil {
		c.pathMap[path.getPathBegin()] = path
	}
}

// getOrgLexemes returns the raw candidates.
func (c *analyzeContext) getOrgLexemes() *quickSortSet { return c.orgLexemes }

// outputToResult walks the analysed span, emitting the arbitrated paths and
// filling every gap between them with single-character lexemes.
func (c *analyzeContext) outputToResult() {
	index := 0
	for index <= c.cursor {
		if c.charTypes[index] == charUseless {
			index++
			c.lastUselessCharNum++
			continue
		}
		c.lastUselessCharNum = 0

		path := c.pathMap[index]
		if path == nil {
			c.outputSingleCJK(index)
			index++
			continue
		}
		l := path.pollFirst()
		for l != nil {
			c.results = append(c.results, l)
			index = l.Begin() + l.Length()
			l = path.pollFirst()
			if l != nil {
				// Emit the single characters the path skipped between lexemes.
				for ; index < l.Begin(); index++ {
					c.outputSingleCJK(index)
				}
			}
		}
	}
	clear(c.pathMap)
}

// outputSingleCJK emits one character as its own lexeme, but only for Chinese
// and other CJK characters — everything else is dropped.
func (c *analyzeContext) outputSingleCJK(index int) {
	switch c.charTypes[index] {
	case charChinese:
		c.results = append(c.results, NewLexeme(c.buffOffset, index, 1, TypeCNChar))
	case charOtherCJK:
		c.results = append(c.results, NewLexeme(c.buffOffset, index, 1, TypeOtherCJK))
	}
}

func (c *analyzeContext) pollResult() *Lexeme {
	if len(c.results) == 0 {
		return nil
	}
	first := c.results[0]
	c.results = c.results[1:]
	return first
}

func (c *analyzeContext) peekResult() *Lexeme {
	if len(c.results) == 0 {
		return nil
	}
	return c.results[0]
}

// getNextLexeme returns the next token: quantity words are compounded, stop
// words are dropped, and the survivor's text is read out of the buffer.
func (c *analyzeContext) getNextLexeme() *Lexeme {
	c.lastSkippedCount = 0
	result := c.pollResult()
	for result != nil {
		c.compound(result)
		// Track every consumed lexeme, including the ones about to be dropped,
		// so end() can still report the right final offset.
		if endPos := result.BeginPosition() + result.Length(); endPos > c.maxConsumedEndPosition {
			c.maxConsumedEndPosition = endPos
		}
		if dic.GetSingleton().IsStopWord(c.segmentBuff, result.Begin(), result.Length()) {
			c.lastSkippedCount++
			result = c.pollResult()
			continue
		}
		result.SetLexemeText(jdk.String(c.segmentBuff, result.Begin(), result.Length()))
		break
	}
	return result
}

// getMaxConsumedEndPosition is the furthest end position consumed so far,
// stop-word-filtered lexemes included.
func (c *analyzeContext) getMaxConsumedEndPosition() int { return c.maxConsumedEndPosition }

// getLastSkippedCount is how many stop words the last getNextLexeme skipped.
func (c *analyzeContext) getLastSkippedCount() int { return c.lastSkippedCount }

// getLastUselessCharNum is how many non-CJK characters trail the input.
func (c *analyzeContext) getLastUselessCharNum() int { return c.lastUselessCharNum }

// reset returns the context to its initial state.
func (c *analyzeContext) reset() {
	clear(c.buffLocker)
	c.orgLexemes = &quickSortSet{}
	c.available = 0
	c.buffOffset = 0
	c.charTypes = make([]int, buffSize)
	c.cursor = 0
	c.results = c.results[:0]
	c.segmentBuff = make([]jdk.Char, buffSize)
	clear(c.pathMap)
	c.maxConsumedEndPosition = 0
	c.lastSkippedCount = 0
}

// compound merges a numeral with the quantifier that follows it. It only runs
// in smart mode.
func (c *analyzeContext) compound(result *Lexeme) {
	if !c.cfg.UseSmart() {
		return
	}
	if len(c.results) == 0 {
		return
	}

	if result.LexemeType() == TypeArabic {
		nextLexeme := c.peekResult()
		appendOk := false
		switch nextLexeme.LexemeType() {
		case TypeCNum:
			appendOk = result.Append(nextLexeme, TypeCNum)
		case TypeCount:
			appendOk = result.Append(nextLexeme, TypeCQuan)
		}
		if appendOk {
			c.pollResult()
		}
	}

	// A numeral formed by the merge above can still absorb a quantifier.
	if result.LexemeType() == TypeCNum && len(c.results) > 0 {
		nextLexeme := c.peekResult()
		appendOk := false
		if nextLexeme.LexemeType() == TypeCount {
			appendOk = result.Append(nextLexeme, TypeCQuan)
		}
		if appendOk {
			c.pollResult()
		}
	}
}
