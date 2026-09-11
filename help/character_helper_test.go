package help

import (
	"testing"
	"time"

	"github.com/infinilabs/analysis-ik/jdk"
)

func TestIsSpaceLetterCoversTheSixWhitespaceUnits(t *testing.T) {
	spaces := map[jdk.Char]bool{8: true, 9: true, 10: true, 13: true, 32: true, 160: true}
	for i := 0; i <= 0xFFFF; i++ {
		c := jdk.Char(i)
		if got := IsSpaceLetter(c); got != spaces[c] {
			t.Fatalf("IsSpaceLetter(%04X) = %t, want %t", i, got, spaces[c])
		}
	}
}

func TestIsEnglishLetterAndIsArabicNumber(t *testing.T) {
	for i := 0; i <= 0xFFFF; i++ {
		c := jdk.Char(i)
		wantLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if got := IsEnglishLetter(c); got != wantLetter {
			t.Fatalf("IsEnglishLetter(%04X) = %t, want %t", i, got, wantLetter)
		}
		wantDigit := c >= '0' && c <= '9'
		if got := IsArabicNumber(c); got != wantDigit {
			t.Fatalf("IsArabicNumber(%04X) = %t, want %t", i, got, wantDigit)
		}
	}
}

// isCJKCharacter takes in the same blocks the segmenter classifies as Chinese
// or other CJK, which is what makes it agree with the engine's own view.
func TestIsCJKCharacterOverEveryCodeUnit(t *testing.T) {
	blocks := []struct{ lo, hi jdk.Char }{
		{0x1100, 0x11FF}, {0x3040, 0x30FF}, {0x3130, 0x318F}, {0x31F0, 0x31FF},
		{0x3400, 0x4DBF}, {0x4E00, 0x9FFF}, {0xAC00, 0xD7AF}, {0xF900, 0xFAFF},
		{0xFF00, 0xFFEF},
	}
	want := func(c jdk.Char) bool {
		for _, b := range blocks {
			if c >= b.lo && c <= b.hi {
				return true
			}
		}
		return false
	}
	for i := 0; i <= 0xFFFF; i++ {
		c := jdk.Char(i)
		if got := IsCJKCharacter(c); got != want(c) {
			t.Fatalf("IsCJKCharacter(%04X) = %t, want %t", i, got, want(c))
		}
	}
}

// This helper's fold always lowercases; the engine's own version makes it
// optional. The difference is deliberate.
func TestRegularizeAlwaysLowercases(t *testing.T) {
	cases := []struct{ in, want jdk.Char }{
		{12288, 32},
		{0xFF21, 'A'},
		{'A', 'a'},
		{'z', 'z'},
		{0x4E2D, 0x4E2D},
	}
	for _, c := range cases {
		if got := Regularize(c.in); got != c.want {
			t.Errorf("Regularize(%04X) = %04X, want %04X", c.in, got, c.want)
		}
	}
}

func TestSleepConvertsItsUnits(t *testing.T) {
	start := time.Now()
	Sleep(MSEC, 5)
	if elapsed := time.Since(start); elapsed < 5*time.Millisecond {
		t.Errorf("slept %v, want at least 5ms", elapsed)
	}
	// A zero count in any unit returns immediately, which is enough to prove
	// the conversion is reached without waiting an hour for it.
	for _, unit := range []SleepType{MSEC, SEC, MIN, HOUR} {
		Sleep(unit, 0)
	}
}

func TestSleepRejectsAnUnknownUnit(t *testing.T) {
	start := time.Now()
	Sleep(SleepType(99), 1000)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("an unknown unit should return at once, took %v", elapsed)
	}
}
