package core

import (
	"testing"

	"github.com/infinilabs/analysis-ik/jdk"
)

// jdkCharTypes is the class the original assigns to every code unit, captured
// from the JDK it was built against. Classification used to come out of
// Character.UnicodeBlock; now it is this package's responsibility, so the whole
// range is checked rather than a handful of samples.
var jdkCharTypes = []struct {
	lo, hi   jdk.Char
	charType int
}{
	{0x0030, 0x0039, charArabic},
	{0x0041, 0x005A, charEnglish},
	{0x0061, 0x007A, charEnglish},
	{0x1100, 0x11FF, charOtherCJK},
	{0x3040, 0x30FF, charOtherCJK},
	{0x3130, 0x318F, charOtherCJK},
	{0x31F0, 0x31FF, charOtherCJK},
	{0x3400, 0x4DBF, charChinese},
	{0x4E00, 0x9FFF, charChinese},
	{0xAC00, 0xD7AF, charOtherCJK},
	{0xD800, 0xDFFF, charSurrogate},
	{0xF900, 0xFAFF, charChinese},
	{0xFF00, 0xFFEF, charOtherCJK},
}

func expectedCharType(c jdk.Char) int {
	for _, r := range jdkCharTypes {
		if c >= r.lo && c <= r.hi {
			return r.charType
		}
	}
	return charUseless
}

func TestIdentifyCharTypeOverEveryCodeUnit(t *testing.T) {
	for i := 0; i <= 0xFFFF; i++ {
		c := jdk.Char(i)
		if got, want := identifyCharType(c), expectedCharType(c); got != want {
			t.Fatalf("identifyCharType(%04X) = %d, want %d", i, got, want)
		}
	}
}

// charSurrogate is 0x16 — decimal 22, not the next free bit. It overlaps
// charEnglish and charChinese, which would matter if the classes were ever
// masked; the engine only compares them with equality, so it does not. The
// value is what the original defines and must not be tidied into 0x10.
func TestCharSurrogateKeepsItsOverlappingValue(t *testing.T) {
	if charSurrogate != 0x16 {
		t.Errorf("charSurrogate = %#x, want 0x16", charSurrogate)
	}
	if charSurrogate&charEnglish == 0 || charSurrogate&charChinese == 0 {
		t.Error("charSurrogate no longer overlaps charEnglish and charChinese")
	}
	if charSurrogate&charOtherCJK != 0 || charSurrogate&charArabic != 0 {
		t.Error("charSurrogate unexpectedly overlaps charOtherCJK or charArabic")
	}
}

func TestRegularizeOverEveryCodeUnit(t *testing.T) {
	expected := func(c jdk.Char, lowercase bool) jdk.Char {
		switch {
		case c == 12288:
			return 32
		case c > 65280 && c < 65375:
			return c - 65248
		case c >= 'A' && c <= 'Z' && lowercase:
			return c + 32
		}
		return c
	}
	for i := 0; i <= 0xFFFF; i++ {
		c := jdk.Char(i)
		for _, lowercase := range []bool{true, false} {
			if got, want := regularize(c, lowercase), expected(c, lowercase); got != want {
				t.Fatalf("regularize(%04X, %t) = %04X, want %04X", i, lowercase, got, want)
			}
		}
	}
}

func TestRegularizeFoldsTheDocumentedCases(t *testing.T) {
	cases := []struct {
		in        jdk.Char
		lowercase bool
		want      jdk.Char
	}{
		{12288, true, 32},      // ideographic space
		{0xFF21, true, 'A'},    // fullwidth A stays uppercase: the fold happens first
		{0xFF10, true, '0'},    // fullwidth zero
		{'A', true, 'a'},       // ASCII uppercase folds when asked
		{'A', false, 'A'},      // and does not when not
		{0xFF00, true, 0xFF00}, // just below the fullwidth window
		{0xFF5F, true, 0xFF5F}, // just above it
		{0x4E2D, true, 0x4E2D}, // Chinese is untouched
	}
	for _, c := range cases {
		if got := regularize(c.in, c.lowercase); got != c.want {
			t.Errorf("regularize(%04X, %t) = %04X, want %04X", c.in, c.lowercase, got, c.want)
		}
	}
}
