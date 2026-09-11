package lucene

import (
	"fmt"
	"strings"
	"testing"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/internal/testutil"
	"github.com/infinilabs/analysis-ik/jdk"
)

// loneHighSurrogate is a high surrogate with no partner. Java strings can hold
// one, so the analyzer can be handed one, and it emits it as a single-character
// lexeme of its own.
var loneHighSurrogate = jdk.DecodeUTF16([]jdk.Char{0xD800})

// astralChar is one non-BMP character, spelled by a surrogate pair.
const astralChar = "\U000F112E"

// astralHigh is the high half of astralChar on its own.
var astralHigh = jdk.DecodeUTF16([]jdk.Char{0xDB84})

// token is one row of the expected token stream.
type token struct {
	start, end int
	tokenType  string
	posIncr    int
	term       string
	length     int
}

func dump(t *testing.T, configuration cfg.Configuration, text string) []token {
	t.Helper()

	analyzer := NewIKAnalyzer(configuration)
	defer func() {
		if err := analyzer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	stream := analyzer.TokenStream("content", text)
	if err := stream.Reset(); err != nil {
		t.Fatal(err)
	}
	term, off := stream.CharTermAttribute(), stream.OffsetAttribute()
	typ, pos := stream.TypeAttribute(), stream.PositionIncrementAttribute()

	var tokens []token
	for {
		more, err := stream.IncrementToken()
		if err != nil {
			t.Fatal(err)
		}
		if !more {
			break
		}
		tokens = append(tokens, token{
			start: off.StartOffset(), end: off.EndOffset(), tokenType: typ.Type(),
			posIncr: pos.PositionIncrement(), term: term.String(), length: term.Length(),
		})
	}
	if err := stream.End(); err != nil {
		t.Fatal(err)
	}
	// Closing releases the input; a stream is finished with, not abandoned.
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	return tokens
}

func assertStream(t *testing.T, got, want []token) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d:\n%s", len(got), len(want), render(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func render(tokens []token) string {
	var b strings.Builder
	for _, tk := range tokens {
		fmt.Fprintf(&b, "  %d|%d|%s|%d|%q|%d\n", tk.start, tk.end, tk.tokenType, tk.posIncr, tk.term, tk.length)
	}
	return b.String()
}

// A surrogate whose partner never arrives is still a token, typed CN_WORD, and
// it must not swallow the characters around it.
func TestUnpairedSurrogatesBecomeSingleCharacterTokens(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)

	assertStream(t, dump(t, configuration, loneHighSurrogate), []token{
		{0, 1, "CN_WORD", 1, loneHighSurrogate, 1},
	})
	assertStream(t, dump(t, configuration, "中"+loneHighSurrogate+"文"), []token{
		{0, 1, "CN_CHAR", 1, "中", 1},
		{1, 2, "CN_WORD", 1, loneHighSurrogate, 1},
		{2, 3, "CN_CHAR", 1, "文", 1},
	})
	// A complete pair followed by a dangling high surrogate.
	assertStream(t, dump(t, configuration, astralChar+astralHigh), []token{
		{0, 2, "CN_CHAR", 1, astralChar, 2},
		{2, 3, "CN_WORD", 1, astralHigh, 1},
	})
}

// Chinese numerals are their own lexeme type, and under ik_smart a numeral
// merges with the quantifier that follows it.
func TestChineseNumeralsAndQuantifiers(t *testing.T) {
	maxWord := testutil.CreateFakeConfigurationSub(false)
	smart := testutil.CreateFakeConfigurationSub(true)

	assertStream(t, dump(t, maxWord, "三十五个人"), []token{
		{0, 3, "TYPE_CNUM", 1, "三十五", 3},
		{1, 4, "CN_WORD", 1, "十五个", 3},
		{1, 3, "CN_WORD", 1, "十五", 2},
		{3, 5, "CN_WORD", 1, "个人", 2},
	})
	assertStream(t, dump(t, smart, "三十五个人"), []token{
		{0, 3, "TYPE_CNUM", 1, "三十五", 3},
		{3, 5, "CN_WORD", 1, "个人", 2},
	})
	assertStream(t, dump(t, maxWord, "一百零八将"), []token{
		{0, 4, "TYPE_CNUM", 1, "一百零八", 4},
		{4, 5, "CN_CHAR", 1, "将", 1},
	})

	// Under ik_max_word the number and its quantifier stay apart; under
	// ik_smart they compound into one TYPE_CQUAN token.
	assertStream(t, dump(t, maxWord, "2023年"), []token{
		{0, 4, "ARABIC", 1, "2023", 4},
		{4, 5, "COUNT", 1, "年", 1},
	})
	assertStream(t, dump(t, smart, "2023年"), []token{
		{0, 5, "TYPE_CQUAN", 1, "2023年", 5},
	})
}

// A comma or a full stop inside a run of digits does not end the number, but it
// does end the mixed letter/digit token — so the two views of the same input
// disagree, and both are emitted.
func TestNumberConnectorsKeepDigitsTogether(t *testing.T) {
	maxWord := testutil.CreateFakeConfigurationSub(false)
	smart := testutil.CreateFakeConfigurationSub(true)

	assertStream(t, dump(t, maxWord, "1,234,567.89"), []token{
		{0, 12, "ARABIC", 1, "1,234,567.89", 12},
		{0, 1, "LETTER", 1, "1", 1},
		{2, 5, "LETTER", 1, "234", 3},
		{6, 12, "LETTER", 1, "567.89", 6},
	})
	assertStream(t, dump(t, smart, "1,234,567.89"), []token{
		{0, 12, "ARABIC", 1, "1,234,567.89", 12},
	})
	assertStream(t, dump(t, maxWord, "3.14159"), []token{
		{0, 7, "ARABIC", 1, "3.14159", 7},
	})
}
