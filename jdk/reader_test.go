package jdk

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestStringReaderReportsEOFAsACount(t *testing.T) {
	r := NewStringReader("ab")
	buf := make([]Char, 4)

	n, err := ReadInto(r, buf)
	if err != nil || n != 2 {
		t.Fatalf("first read = %d, %v; want 2, nil", n, err)
	}
	if !slices.Equal(buf[:2], []Char{'a', 'b'}) {
		t.Errorf("read %04X", buf[:2])
	}
	// End of input is a count of -1, not an error: AnalyzeContext does
	// arithmetic on the result.
	if n, err := ReadInto(r, buf); n != EOF || err != nil {
		t.Errorf("read past end = %d, %v; want %d, nil", n, err, EOF)
	}
}

func TestStringReaderHonoursOffsetAndLength(t *testing.T) {
	r := NewStringReader("abcdef")
	buf := make([]Char, 6)

	n, err := r.Read(buf, 2, 3)
	if err != nil || n != 3 {
		t.Fatalf("read = %d, %v; want 3, nil", n, err)
	}
	if !slices.Equal(buf, []Char{0, 0, 'a', 'b', 'c', 0}) {
		t.Errorf("buffer = %04X", buf)
	}
	if n, err := r.Read(buf, 0, 0); n != 0 || err != nil {
		t.Errorf("zero-length read = %d, %v; want 0, nil", n, err)
	}
}

func TestStringReaderCloseRejectsFurtherReads(t *testing.T) {
	r := NewStringReader("abc")
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInto(r, make([]Char, 4)); !errors.Is(err, ErrClosed) {
		t.Errorf("read after close = %v, want %v", err, ErrClosed)
	}
}

func TestUnitsReaderCopiesItsInput(t *testing.T) {
	units := []Char{'a', 'b'}
	r := NewUnitsReader(units)
	units[0] = 'z'

	buf := make([]Char, 2)
	if _, err := ReadInto(r, buf); err != nil {
		t.Fatal(err)
	}
	if buf[0] != 'a' {
		t.Errorf("reader saw a later mutation: %04X", buf)
	}
}

// A tokenizer holds this reader until reset() installs the real input, so
// reading from it has to be an error rather than a silent empty stream.
func TestIllegalStateReaderFailsEveryRead(t *testing.T) {
	r := IllegalStateReader()
	if _, err := ReadInto(r, make([]Char, 4)); !errors.Is(err, ErrNoReader) {
		t.Errorf("read = %v, want %v", err, ErrNoReader)
	}
	if err := r.Close(); err != nil {
		t.Errorf("close = %v, want nil", err)
	}
}

func TestNewIOReaderDecodesUTF8(t *testing.T) {
	r, err := NewIOReader(strings.NewReader("中华"))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]Char, 4)
	n, err := ReadInto(r, buf)
	if err != nil || n != 2 {
		t.Fatalf("read = %d, %v; want 2, nil", n, err)
	}
	if !slices.Equal(buf[:2], []Char{0x4E2D, 0x534E}) {
		t.Errorf("read %04X", buf[:2])
	}
}
