package jdk

import (
	"slices"
	"testing"
)

func TestEncodeUTF16CountsCodeUnitsNotRunes(t *testing.T) {
	cases := []struct {
		text  string
		units []Char
	}{
		{"", nil},
		{"abc", []Char{'a', 'b', 'c'}},
		{"中华", []Char{0x4E2D, 0x534E}},
		{"\U000F112E", []Char{0xDB84, 0xDD2E}},
		{"菩\U000F112E凤", []Char{0x83E9, 0xDB84, 0xDD2E, 0x51E4}},
	}
	for _, c := range cases {
		got := EncodeUTF16(c.text)
		if !slices.Equal(got, c.units) {
			t.Errorf("EncodeUTF16(%q) = %04X, want %04X", c.text, got, c.units)
		}
		if back := DecodeUTF16(got); back != c.text {
			t.Errorf("DecodeUTF16 round trip of %q gave %q", c.text, back)
		}
	}
}

// A surrogate with no partner is what SurrogatePairSegmenter emits when a pair
// is broken, so it has to survive the trip through a Go string intact.
func TestUnpairedSurrogatesRoundTrip(t *testing.T) {
	cases := [][]Char{
		{0xD800},
		{0xDC00},
		{0xDBFF},
		{0xDFFF},
		{0x4E2D, 0xD800, 0x6587},
		{0xDC00, 0xD800},
		{0xDB84, 0xDD2E, 0xDB84},
	}
	for _, units := range cases {
		text := DecodeUTF16(units)
		if got := EncodeUTF16(text); !slices.Equal(got, units) {
			t.Errorf("round trip of %04X gave %04X", units, got)
		}
	}
}

func TestDecodeUTF16PairsSurrogates(t *testing.T) {
	if got, want := DecodeUTF16([]Char{0xDB84, 0xDD2E}), "\U000F112E"; got != want {
		t.Errorf("DecodeUTF16 = %q, want %q", got, want)
	}
	// A low surrogate first is not a pair, so both stay separate.
	if got := EncodeUTF16(DecodeUTF16([]Char{0xDD2E, 0xDB84})); len(got) != 2 {
		t.Errorf("expected two units, got %04X", got)
	}
}

func TestString(t *testing.T) {
	units := EncodeUTF16("菩\U000F112E凤")
	if got, want := String(units, 1, 2), "\U000F112E"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if got, want := String(units, 0, 0), ""; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

// Trim follows String.trim, which cuts every code unit at or below U+0020 —
// not the Unicode whitespace set strings.TrimSpace uses.
func TestTrimCutsControlCharactersButNotUnicodeSpaces(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  abc  ", "abc"},
		{"\t\r\n abc \n\r\t", "abc"},
		{"\x00\x01abc\x1f", "abc"},
		{"abc", "abc"},
		{"   ", ""},
		{"", ""},
		{" abc ", " abc "},
		{"　abc　", "　abc　"},
	}
	for _, c := range cases {
		if got := Trim(c.in); got != c.want {
			t.Errorf("Trim(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToLower(t *testing.T) {
	if got, want := ToLower("RS12345678901"), "rs12345678901"; got != want {
		t.Errorf("ToLower = %q, want %q", got, want)
	}
}
