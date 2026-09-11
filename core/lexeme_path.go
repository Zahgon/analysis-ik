package core

import (
	"math"
	"strconv"
	"strings"
)

// lexemePath is one candidate segmentation: an ordered chain of lexemes plus
// the span it covers and how much of that span the lexemes actually carry.
type lexemePath struct {
	quickSortSet

	pathBegin     int
	pathEnd       int
	payloadLength int
}

func newLexemePath() *lexemePath {
	return &lexemePath{pathBegin: -1, pathEnd: -1}
}

// addCrossLexeme appends a lexeme that overlaps the path, growing the span.
func (p *lexemePath) addCrossLexeme(lexeme *Lexeme) bool {
	switch {
	case p.isEmpty():
		p.addLexeme(lexeme)
		p.pathBegin = lexeme.Begin()
		p.pathEnd = lexeme.Begin() + lexeme.Length()
		p.payloadLength += lexeme.Length()
		return true
	case p.checkCross(lexeme):
		p.addLexeme(lexeme)
		if lexeme.Begin()+lexeme.Length() > p.pathEnd {
			p.pathEnd = lexeme.Begin() + lexeme.Length()
		}
		p.payloadLength = p.pathEnd - p.pathBegin
		return true
	default:
		return false
	}
}

// addNotCrossLexeme appends a lexeme only when it does not overlap the path.
func (p *lexemePath) addNotCrossLexeme(lexeme *Lexeme) bool {
	switch {
	case p.isEmpty():
		p.addLexeme(lexeme)
		p.pathBegin = lexeme.Begin()
		p.pathEnd = lexeme.Begin() + lexeme.Length()
		p.payloadLength += lexeme.Length()
		return true
	case p.checkCross(lexeme):
		return false
	default:
		p.addLexeme(lexeme)
		p.payloadLength += lexeme.Length()
		head := p.peekFirst()
		p.pathBegin = head.Begin()
		tail := p.peekLast()
		p.pathEnd = tail.Begin() + tail.Length()
		return true
	}
}

// removeTail drops the last lexeme and shrinks the span to match.
func (p *lexemePath) removeTail() *Lexeme {
	tail := p.pollLast()
	if p.isEmpty() {
		p.pathBegin = -1
		p.pathEnd = -1
		p.payloadLength = 0
	} else {
		p.payloadLength -= tail.Length()
		newTail := p.peekLast()
		p.pathEnd = newTail.Begin() + newTail.Length()
	}
	return tail
}

// checkCross reports whether a lexeme overlaps the path's span — the definition
// of an ambiguous split.
func (p *lexemePath) checkCross(lexeme *Lexeme) bool {
	return (lexeme.Begin() >= p.pathBegin && lexeme.Begin() < p.pathEnd) ||
		(p.pathBegin >= lexeme.Begin() && p.pathBegin < lexeme.Begin()+lexeme.Length())
}

func (p *lexemePath) getPathBegin() int { return p.pathBegin }
func (p *lexemePath) getPathEnd() int   { return p.pathEnd }

// getPayloadLength is how many code units the chain's lexemes carry.
func (p *lexemePath) getPayloadLength() int { return p.payloadLength }

// getPathLength is the span the chain covers.
func (p *lexemePath) getPathLength() int { return p.pathEnd - p.pathBegin }

// getXWeight is the product of the lexeme lengths, each clamped to 10, and
// saturates at math.MaxInt32 rather than overflowing.
func (p *lexemePath) getXWeight() int {
	product := int64(1)
	c := p.getHead()
	for c != nil && c.getLexeme() != nil {
		length := c.getLexeme().Length()
		if length > 10 {
			length = 10
		}
		product *= int64(length)
		if product > math.MaxInt32 {
			return math.MaxInt32
		}
		c = c.getNext()
	}
	return int(product)
}

// getPWeight weights each lexeme's length by its 1-based position in the chain.
func (p *lexemePath) getPWeight() int {
	pWeight := 0
	position := 0
	c := p.getHead()
	for c != nil && c.getLexeme() != nil {
		position++
		pWeight += position * c.getLexeme().Length()
		c = c.getNext()
	}
	return pWeight
}

// copy clones the path, sharing the lexemes themselves.
func (p *lexemePath) copy() *lexemePath {
	theCopy := newLexemePath()
	theCopy.pathBegin = p.pathBegin
	theCopy.pathEnd = p.pathEnd
	theCopy.payloadLength = p.payloadLength
	c := p.getHead()
	for c != nil && c.getLexeme() != nil {
		theCopy.addLexeme(c.getLexeme())
		c = c.getNext()
	}
	return theCopy
}

// compareTo ranks two candidate segmentations, best first: more text covered,
// then fewer lexemes, then a wider span, then a later end — reverse
// segmentation is statistically more often right than forward — then more even
// lexeme lengths, then more weight towards the front.
func (p *lexemePath) compareTo(other *lexemePath) int {
	switch {
	case p.payloadLength > other.payloadLength:
		return -1
	case p.payloadLength < other.payloadLength:
		return 1
	}
	switch {
	case p.getSize() < other.getSize():
		return -1
	case p.getSize() > other.getSize():
		return 1
	}
	switch {
	case p.getPathLength() > other.getPathLength():
		return -1
	case p.getPathLength() < other.getPathLength():
		return 1
	}
	switch {
	case p.pathEnd > other.pathEnd:
		return -1
	case p.pathEnd < other.pathEnd:
		return 1
	}
	switch {
	case p.getXWeight() > other.getXWeight():
		return -1
	case p.getXWeight() < other.getXWeight():
		return 1
	}
	switch {
	case p.getPWeight() > other.getPWeight():
		return -1
	case p.getPWeight() < other.getPWeight():
		return 1
	}
	return 0
}

// String renders the path with the CRLF line endings the original used.
func (p *lexemePath) String() string {
	var b strings.Builder
	b.WriteString("pathBegin  : ")
	b.WriteString(strconv.Itoa(p.pathBegin))
	b.WriteString("\r\n")
	b.WriteString("pathEnd  : ")
	b.WriteString(strconv.Itoa(p.pathEnd))
	b.WriteString("\r\n")
	b.WriteString("payloadLength  : ")
	b.WriteString(strconv.Itoa(p.payloadLength))
	b.WriteString("\r\n")
	for head := p.getHead(); head != nil; head = head.getNext() {
		b.WriteString("lexeme : ")
		b.WriteString(head.getLexeme().String())
		b.WriteString("\r\n")
	}
	return b.String()
}
