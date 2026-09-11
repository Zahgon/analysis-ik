package lucene

import (
	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/core"
	"github.com/infinilabs/analysis-ik/jdk"
)

// IKTokenizer turns the segmenter's lexemes into tokens.
type IKTokenizer struct {
	AttributeSource

	ikImplement *core.IKSegmenter

	// endPosition is where the last token ended.
	endPosition int
	// skippedPositions is how many stop words preceded the last token.
	skippedPositions int

	// input is only installed by Reset; inputPending holds whatever SetReader
	// was given until then. Reading before Reset is a contract violation, and
	// the placeholder reader is what reports it.
	input        jdk.Reader
	inputPending jdk.Reader
}

// NewIKTokenizer builds a tokenizer. It has no input until SetReader and Reset
// have both run.
func NewIKTokenizer(configuration cfg.Configuration) *IKTokenizer {
	t := &IKTokenizer{
		AttributeSource: newAttributeSource(),
		input:           jdk.IllegalStateReader(),
		inputPending:    jdk.IllegalStateReader(),
	}
	t.ikImplement = core.NewIKSegmenter(t.input, configuration)
	return t
}

// SetReader stages the next input. It takes effect at the following Reset.
func (t *IKTokenizer) SetReader(reader jdk.Reader) { t.inputPending = reader }

// IncrementToken advances to the next token.
func (t *IKTokenizer) IncrementToken() (bool, error) {
	t.ClearAttributes()

	nextLexeme, err := t.ikImplement.Next()
	if err != nil {
		return false, err
	}
	if nextLexeme == nil {
		return false, nil
	}

	// The stop words the segmenter dropped become this token's extra positions.
	t.skippedPositions = t.ikImplement.SavedSkippedCount()
	t.PositionIncrementAttribute().SetPositionIncrement(t.skippedPositions + 1)

	t.CharTermAttribute().Append(nextLexeme.LexemeText())
	t.CharTermAttribute().SetLength(nextLexeme.Length())
	t.OffsetAttribute().SetOffset(
		t.correctOffset(nextLexeme.BeginPosition()),
		t.correctOffset(nextLexeme.EndPosition()))

	t.endPosition = nextLexeme.EndPosition()
	t.TypeAttribute().SetType(nextLexeme.LexemeTypeString())
	return true, nil
}

// Reset installs the staged input and clears the stream state.
func (t *IKTokenizer) Reset() error {
	t.input = t.inputPending
	t.inputPending = jdk.IllegalStateReader()
	t.ikImplement.Reset(t.input)
	t.skippedPositions = 0
	t.endPosition = 0
	return nil
}

// End publishes the final offset. It has to look past the last token that was
// actually emitted: when every lexeme in a value was filtered out as a stop
// word there is no last token, and reporting zero would shift the offsets of
// every following value in a multi-valued field (issue #921).
func (t *IKTokenizer) End() error {
	t.EndAttributes()

	maxEnd := max(t.endPosition, t.ikImplement.SavedMaxConsumedEndPosition())
	finalOffset := t.correctOffset(maxEnd + t.ikImplement.LastUselessCharNum())
	t.OffsetAttribute().SetOffset(finalOffset, finalOffset)
	t.PositionIncrementAttribute().SetPositionIncrement(
		t.PositionIncrementAttribute().PositionIncrement() + t.skippedPositions)
	return nil
}

// Close releases the input.
func (t *IKTokenizer) Close() error {
	err := t.input.Close()
	t.inputPending = jdk.IllegalStateReader()
	t.input = jdk.IllegalStateReader()
	return err
}

// correctOffset maps a position in the tokenizer's input back to a position in
// the original text. Nothing rewrites the input here, so it is the identity —
// the same answer Lucene gives a tokenizer with no character filter in front.
func (t *IKTokenizer) correctOffset(currentOff int) int { return currentOff }
