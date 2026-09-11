// Package dic holds the dictionary: a trie of UTF-16 code units, the singleton
// that loads it from config/, and the remote-dictionary monitor.
package dic

// Hit state bits. A hit is "unmatched" only when no bit is set, so a code unit
// that both completes a word and starts a longer one reports match and prefix
// at the same time.
const (
	unmatch = 0x00000000
	match   = 0x00000001
	prefix  = 0x00000010
)

// Hit describes one dictionary lookup: whether the span matched a word, whether
// it is the prefix of a longer one, and where the search should resume.
type Hit struct {
	hitState           int
	matchedDictSegment *dictSegment
	begin              int
	end                int
}

// IsMatch reports whether the span is a complete word.
func (h *Hit) IsMatch() bool { return h.hitState&match > 0 }

// SetMatch marks the span as a complete word.
func (h *Hit) SetMatch() { h.hitState |= match }

// IsPrefix reports whether the span is the prefix of a longer word.
func (h *Hit) IsPrefix() bool { return h.hitState&prefix > 0 }

// SetPrefix marks the span as the prefix of a longer word.
func (h *Hit) SetPrefix() { h.hitState |= prefix }

// IsUnmatch reports whether the span is neither a word nor a prefix.
func (h *Hit) IsUnmatch() bool { return h.hitState == unmatch }

// SetUnmatch clears every state bit.
func (h *Hit) SetUnmatch() { h.hitState = unmatch }

// Begin is the index the matched span starts at.
func (h *Hit) Begin() int { return h.begin }

// SetBegin moves the start of the matched span.
func (h *Hit) SetBegin(begin int) { h.begin = begin }

// End is the index the last lookup stopped at.
func (h *Hit) End() int { return h.end }

// SetEnd records where the last lookup stopped.
func (h *Hit) SetEnd(end int) { h.end = end }
