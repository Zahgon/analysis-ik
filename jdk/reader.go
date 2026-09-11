package jdk

import (
	"errors"
	"io"
)

// EOF is the count java.io.Reader.read returns once the input is exhausted.
// AnalyzeContext does arithmetic on it, so it is a count and not an error.
const EOF = -1

// Reader is a source of UTF-16 code units with java.io.Reader's contract: Read
// fills at most length units starting at off and returns how many it produced,
// or EOF once nothing is left. A short read is legal and does not mean the end.
type Reader interface {
	Read(cbuf []Char, off, length int) (int, error)
	Close() error
}

// ReadInto is the one-argument java.io.Reader.read(char[]).
func ReadInto(r Reader, cbuf []Char) (int, error) { return r.Read(cbuf, 0, len(cbuf)) }

// ErrClosed is reported by a reader that has been closed.
var ErrClosed = errors.New("jdk: stream closed")

// ErrNoReader is reported by the placeholder a Tokenizer holds before Reset
// installs the real input, mirroring Lucene's ILLEGAL_STATE_READER.
var ErrNoReader = errors.New(
	"jdk: TokenStream contract violation: reset()/close() call missing, " +
		"reset() called multiple times, or subclass does not call super.reset()")

type stringReader struct {
	units  []Char
	next   int
	closed bool
}

// NewStringReader reads s as UTF-16 code units, like java.io.StringReader.
func NewStringReader(s string) Reader { return &stringReader{units: EncodeUTF16(s)} }

// NewUnitsReader reads an already-encoded slice of code units.
func NewUnitsReader(units []Char) Reader {
	return &stringReader{units: append([]Char(nil), units...)}
}

func (r *stringReader) Read(cbuf []Char, off, length int) (int, error) {
	if r.closed {
		return 0, ErrClosed
	}
	if length == 0 {
		return 0, nil
	}
	if r.next >= len(r.units) {
		return EOF, nil
	}
	n := min(length, len(r.units)-r.next)
	copy(cbuf[off:off+n], r.units[r.next:r.next+n])
	r.next += n
	return n, nil
}

func (r *stringReader) Close() error {
	r.closed = true
	return nil
}

type illegalStateReader struct{}

// IllegalStateReader stands in for an input that has not been set yet. Every
// operation on it fails, which is how Lucene surfaces a missing reset().
func IllegalStateReader() Reader { return illegalStateReader{} }

func (illegalStateReader) Read([]Char, int, int) (int, error) { return 0, ErrNoReader }
func (illegalStateReader) Close() error                       { return nil }

// NewIOReader adapts an ordinary byte stream of UTF-8 text.
func NewIOReader(r io.Reader) (Reader, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return NewStringReader(string(data)), nil
}
