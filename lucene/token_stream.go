// Package lucene adapts the segmentation engine to a token-stream API: the
// analyzer, the tokenizer, and the four token attributes a consumer reads.
//
// The original implemented Lucene's Analyzer and Tokenizer directly. Lucene is
// a Java library with no Go counterpart, so the part of its contract the
// analyzer actually exposes is reproduced here — the reset/incrementToken/end
// lifecycle, the attribute defaults, and the way end() resets the attributes
// before publishing the final offset. Those are the behaviours the tests assert
// and the behaviours a consumer sees; nothing else of Lucene is imitated.
package lucene

import "github.com/infinilabs/analysis-ik/jdk"

// minBufferSize is the smallest term buffer allocated, matching the smallest
// one Lucene's CharTermAttribute hands out.
const minBufferSize = 10

// DefaultType is the token type before a tokenizer sets one.
const DefaultType = "word"

// CharTermAttribute holds a token's text as UTF-16 code units.
type CharTermAttribute struct {
	buf    []jdk.Char
	length int
}

func newCharTermAttribute() *CharTermAttribute {
	return &CharTermAttribute{buf: make([]jdk.Char, minBufferSize)}
}

// Buffer is the backing array. Only the first Length code units are the term;
// the rest is spare capacity, exactly as Lucene exposes it.
func (a *CharTermAttribute) Buffer() []jdk.Char { return a.buf }

// Length is the number of code units in the term.
func (a *CharTermAttribute) Length() int { return a.length }

// SetLength truncates or extends the term within the buffer.
func (a *CharTermAttribute) SetLength(length int) {
	a.grow(length)
	a.length = length
}

// Append adds text to the end of the term.
func (a *CharTermAttribute) Append(text string) {
	units := jdk.EncodeUTF16(text)
	a.grow(a.length + len(units))
	copy(a.buf[a.length:], units)
	a.length += len(units)
}

// String renders the term.
func (a *CharTermAttribute) String() string { return jdk.DecodeUTF16(a.buf[:a.length]) }

func (a *CharTermAttribute) grow(size int) {
	if size <= len(a.buf) {
		return
	}
	grown := make([]jdk.Char, max(size, 2*len(a.buf)))
	copy(grown, a.buf[:a.length])
	a.buf = grown
}

func (a *CharTermAttribute) clear() { a.length = 0 }

// OffsetAttribute holds where a token starts and ends in the input, counted in
// UTF-16 code units.
type OffsetAttribute struct {
	startOffset int
	endOffset   int
}

// StartOffset is the token's first position.
func (a *OffsetAttribute) StartOffset() int { return a.startOffset }

// EndOffset is the position just past the token.
func (a *OffsetAttribute) EndOffset() int { return a.endOffset }

// SetOffset records the token's span.
func (a *OffsetAttribute) SetOffset(startOffset, endOffset int) {
	a.startOffset, a.endOffset = startOffset, endOffset
}

func (a *OffsetAttribute) clear() { a.startOffset, a.endOffset = 0, 0 }

// TypeAttribute holds a token's type name.
type TypeAttribute struct {
	tokenType string
}

// Type is the token's type name.
func (a *TypeAttribute) Type() string { return a.tokenType }

// SetType records the token's type name.
func (a *TypeAttribute) SetType(tokenType string) { a.tokenType = tokenType }

func (a *TypeAttribute) clear() { a.tokenType = DefaultType }

// PositionIncrementAttribute holds how many positions a token advances: one for
// an ordinary token, more when stop words were dropped before it.
type PositionIncrementAttribute struct {
	positionIncrement int
}

// PositionIncrement is how far this token advances the position counter.
func (a *PositionIncrementAttribute) PositionIncrement() int { return a.positionIncrement }

// SetPositionIncrement records how far this token advances the position counter.
func (a *PositionIncrementAttribute) SetPositionIncrement(positionIncrement int) {
	a.positionIncrement = positionIncrement
}

func (a *PositionIncrementAttribute) clear() { a.positionIncrement = 1 }

// end is the reset applied when the stream finishes. Unlike clear it leaves the
// increment at zero, which is what Lucene does and what makes the tokenizer's
// End produce the increments the tests assert.
func (a *PositionIncrementAttribute) end() { a.positionIncrement = 0 }

// AttributeSource is the set of attributes a token stream publishes. Embed it
// in a stream implementation.
type AttributeSource struct {
	charTerm *CharTermAttribute
	offset   *OffsetAttribute
	typ      *TypeAttribute
	posIncr  *PositionIncrementAttribute
}

func newAttributeSource() AttributeSource {
	source := AttributeSource{
		charTerm: newCharTermAttribute(),
		offset:   &OffsetAttribute{},
		typ:      &TypeAttribute{},
		posIncr:  &PositionIncrementAttribute{},
	}
	source.ClearAttributes()
	return source
}

// CharTermAttribute returns the term attribute.
func (s *AttributeSource) CharTermAttribute() *CharTermAttribute { return s.charTerm }

// OffsetAttribute returns the offset attribute.
func (s *AttributeSource) OffsetAttribute() *OffsetAttribute { return s.offset }

// TypeAttribute returns the type attribute.
func (s *AttributeSource) TypeAttribute() *TypeAttribute { return s.typ }

// PositionIncrementAttribute returns the position increment attribute.
func (s *AttributeSource) PositionIncrementAttribute() *PositionIncrementAttribute {
	return s.posIncr
}

// ClearAttributes resets every attribute to its default, ready for the next
// token.
func (s *AttributeSource) ClearAttributes() {
	s.charTerm.clear()
	s.offset.clear()
	s.typ.clear()
	s.posIncr.clear()
}

// EndAttributes resets every attribute for the end of the stream. It differs
// from ClearAttributes in one place: the position increment goes to zero.
func (s *AttributeSource) EndAttributes() {
	s.charTerm.clear()
	s.offset.clear()
	s.typ.clear()
	s.posIncr.end()
}

// TokenStream is a sequence of tokens, read by calling Reset once, then
// IncrementToken until it reports false, then End.
type TokenStream interface {
	// CharTermAttribute returns the term attribute.
	CharTermAttribute() *CharTermAttribute
	// OffsetAttribute returns the offset attribute.
	OffsetAttribute() *OffsetAttribute
	// TypeAttribute returns the type attribute.
	TypeAttribute() *TypeAttribute
	// PositionIncrementAttribute returns the position increment attribute.
	PositionIncrementAttribute() *PositionIncrementAttribute
	// Reset prepares the stream for a new pass over its input.
	Reset() error
	// IncrementToken advances to the next token, reporting false at the end.
	IncrementToken() (bool, error)
	// End publishes the final offset once the tokens are exhausted.
	End() error
	// Close releases the input.
	Close() error
}

// TokenStreamComponents pairs a stream with the hook that points it at an
// input, which is how an analyzer swaps one text for the next.
type TokenStreamComponents struct {
	source func(jdk.Reader)
	stream TokenStream
}

// NewTokenStreamComponents wraps a tokenizer as the source of a stream.
func NewTokenStreamComponents(tokenizer *IKTokenizer) *TokenStreamComponents {
	return &TokenStreamComponents{source: tokenizer.SetReader, stream: tokenizer}
}

// SetReader points the stream at a new input.
func (c *TokenStreamComponents) SetReader(reader jdk.Reader) { c.source(reader) }

// TokenStream returns the stream itself.
func (c *TokenStreamComponents) TokenStream() TokenStream { return c.stream }
