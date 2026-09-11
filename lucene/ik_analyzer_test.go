package lucene

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/core"
	"github.com/infinilabs/analysis-ik/dic"
	"github.com/infinilabs/analysis-ik/internal/testutil"
	"github.com/infinilabs/analysis-ik/jdk"
)

// The two non-BMP characters the surrogate-pair tests use. Java spells them as
// the surrogate pairs 󱄮 and 󱂗.
const (
	surrogatePairA = "\U000F112E"
	surrogatePairB = "\U000F1097"
)

// issue1155Words are six overlapping eleven-character words. Adding all of them
// makes every position of issue1155Text the start of a long dictionary word,
// which is the dense crossing path the arbitrator has to bail out of.
var issue1155Words = []string{
	"这段文字用于测试分词器",
	"文字用于测试分词器偏移",
	"用于测试分词器偏移量计",
	"测试分词器偏移量计算在",
	"分词器偏移量计算在运行",
	"器偏移量计算在运行过程",
}

const issue1155Text = "这段文字用于测试分词器偏移量计算在运行过程后检验和确认结果"

// A single Chinese character plus one surrogate pair.
func TestTokenizeCase1Correctly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)
	values := tokenize(t, configuration, "菩"+surrogatePairA)
	assertTokens(t, values, []string{"菩", surrogatePairA})
}

// A single Chinese character, one surrogate pair, another Chinese character.
func TestTokenizeCase2Correctly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)
	values := tokenize(t, configuration, "菩"+surrogatePairA+"凤")
	assertTokens(t, values, []string{"菩", surrogatePairA, "凤"})
}

// Single Chinese characters interleaved with several surrogate pairs.
func TestTokenizeCase3Correctly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)
	values := tokenize(t, configuration, "菩"+surrogatePairA+"剃"+surrogatePairB)
	assertTokens(t, values, []string{"菩", surrogatePairA, "剃", surrogatePairB})
}

// A single Chinese character followed by consecutive surrogate pairs.
func TestTokenizeCase4Correctly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)
	values := tokenize(t, configuration, "菩"+surrogatePairA+surrogatePairB)
	assertTokens(t, values, []string{"菩", surrogatePairA, surrogatePairB})
}

// Surrogate pairs mixed with a word that is in the dictionary.
func TestTokenizeCase5Correctly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)
	values := tokenize(t, configuration, "菩"+surrogatePairA+"龟龙麟凤凤")
	assertTokens(t, values, []string{"菩", surrogatePairA, "龟龙麟凤", "凤"})
}

// Surrogate pairs that straddle the end of the analysis buffer.
func TestTokenizeCase6Correctly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)
	// '菩' + spaces + 41 surrogate pairs, arranged so the pairs land beyond the
	// 4096-code-unit buffer boundary.
	var sb strings.Builder
	sb.Grow(4006)
	sb.WriteString("菩")
	for i := 0; i < 3995; i++ {
		sb.WriteByte(' ')
	}
	for i := 0; i < 41; i++ {
		sb.WriteString(surrogatePairA + " ")
	}
	values := tokenize(t, configuration, sb.String())

	if values[0] != "菩" {
		t.Errorf("first token: expected 菩, got %q", values[0])
	}

	if len(values) != 42 {
		t.Fatalf("token count: expected 42, got %d", len(values))
	}

	for i := 1; i <= 41; i++ {
		if values[i] != surrogatePairA {
			t.Errorf("Token at index %d is not the expected surrogate pair", i)
		}
	}
}

// Segmentation with the ik_max_word analyzer.
func TestTokenizeMaxWordCorrectly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)
	values := tokenize(t, configuration, "中华人民共和国国歌")
	if len(values) < 9 {
		t.Fatalf("token count: expected at least 9, got %d", len(values))
	}
	for _, expected := range []string{
		"中华人民共和国", "中华人民", "中华", "华人",
		"人民共和国", "人民", "共和国", "共和", "国歌",
	} {
		if !slices.Contains(values, expected) {
			t.Errorf("expected tokens to contain %q, got %v", expected, values)
		}
	}
}

// Segmentation with the ik_smart analyzer.
func TestTokenizeSmartCorrectly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)
	values := tokenize(t, configuration, "中华人民共和国国歌")
	if len(values) != 2 {
		t.Fatalf("token count: expected 2, got %d (%v)", len(values), values)
	}
	for _, expected := range []string{"中华人民共和国", "国歌"} {
		if !slices.Contains(values, expected) {
			t.Errorf("expected tokens to contain %q, got %v", expected, values)
		}
	}
}

// Chinese quantifiers, with the ik_max_word analyzer.
func TestTokenizeCNQuantifierCorrectly(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false)
	text := "2023年人才"

	tokenInfos := tokenizeWithType(t, configuration, text)

	// Print every token and its type, which is what makes a failure diagnosable.
	for _, info := range tokenInfos {
		t.Logf("Token: %s, Type: %d", info.text, info.tokenType)
	}

	tokens := make([]string, 0, len(tokenInfos))
	for _, info := range tokenInfos {
		tokens = append(tokens, info.text)
	}
	for _, expected := range []string{"2023", "年", "人才"} {
		if !slices.Contains(tokens, expected) {
			t.Errorf("expected tokens to contain %q, got %v", expected, tokens)
		}
	}

	hasPersonAsCount := slices.ContainsFunc(tokenInfos, func(info tokenInfo) bool {
		return info.text == "人" && info.tokenType == core.TypeCount
	})
	if hasPersonAsCount {
		t.Error("'人'不应该被分割为COUNT类型")
	}

	hasYearAsCount := slices.ContainsFunc(tokenInfos, func(info tokenInfo) bool {
		return info.text == "年" && info.tokenType == core.TypeCount
	})
	if !hasYearAsCount {
		t.Error("'年'应该是COUNT类型")
	}
}

// Very long runs of a repeated character must stay fast under ik_smart: the
// test fails if segmentation takes more than five seconds.
func TestTokenizeSmartLongRepeatedWordsPerformance(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)

	var sb strings.Builder
	const repeatedWord = "哈哈哈哈哈哈哈哈哈哈"
	for i := 0; i < 1001; i++ {
		sb.WriteString(repeatedWord)
	}
	longRepeatedText := sb.String()

	startTime := time.Now()
	tokens := tokenize(t, configuration, longRepeatedText)
	duration := time.Since(startTime)

	if duration > 5000*time.Millisecond {
		t.Errorf("IK_SMART分词超长叠词耗时%dms，超过5秒限制", duration.Milliseconds())
	}
	if len(tokens) == 0 {
		t.Error("分词结果不能为空")
	}

	t.Logf("IK_SMART分词超长叠词耗时: %dms, 分词结果数量: %d", duration.Milliseconds(), len(tokens))
}

// Issue #1137: digit runs longer than ten characters were swallowed.
// https://github.com/infinilabs/analysis-ik/issues/1137
func TestTokenizeIssue1137LongDigitsWithPrefix(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true) // ik_smart
	tokens := tokenize(t, configuration, "RS12345678901")

	if !slices.Contains(tokens, "rs12345678901") {
		t.Errorf("Bug复现: 超过10位的数字被吞掉了！预期包含'rs12345678901'，实际 %v", tokens)
	}
}

// Issue #1137: a bare digit run longer than ten characters.
func TestTokenizeIssue1137PureLongDigits(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)
	tokens := tokenize(t, configuration, "12345678901234567890") // 20 digits

	if !slices.Contains(tokens, "12345678901234567890") {
		t.Errorf("Bug复现: 纯长数字也被吞掉了！预期包含'12345678901234567890'，实际 %v", tokens)
	}
}

// Issue #1137: exactly ten digits, which always worked.
func TestTokenizeIssue1137Exactly10Digits(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)
	tokens := tokenize(t, configuration, "1234567890") // exactly 10

	if !slices.Contains(tokens, "1234567890") {
		t.Errorf("10位数字应该正常分词，实际 %v", tokens)
	}
}

// Issue #1137: eleven digits.
func TestTokenizeIssue1137_11Digits(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)
	tokens := tokenize(t, configuration, "12345678901") // 11 digits

	if !slices.Contains(tokens, "12345678901") {
		t.Errorf("Bug复现: 11位数字被吞掉了！预期包含'12345678901'，实际 %v", tokens)
	}
}

// Issue #1137: a phone number.
func TestTokenizeIssue1137PhoneNumber(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)
	tokens := tokenize(t, configuration, "13800138000") // phone number

	if !slices.Contains(tokens, "13800138000") {
		t.Errorf("手机号应该完整保留！预期包含'13800138000'，实际 %v", tokens)
	}
}

// Issue #1137: an order identifier with a letter prefix.
func TestTokenizeIssue1137OrderID(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)
	tokens := tokenize(t, configuration, "ORD202603180001") // order id

	if !slices.Contains(tokens, "ord202603180001") {
		t.Errorf("订单号应该完整保留！预期包含'ord202603180001'，实际 %v", tokens)
	}
}

// Issue #1137: the same input under ik_max_word.
func TestTokenizeIssue1137IkMaxWord(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(false) // ik_max_word
	tokens := tokenize(t, configuration, "RS12345678901")

	if !slices.Contains(tokens, "rs12345678901") {
		t.Errorf("Bug复现: ik_max_word模式下长数字也被吞掉了！实际 %v", tokens)
	}
}

// Issue #1155: the fallback for a complex crossing path must not emit offsets
// that go backwards.
// https://github.com/infinilabs/analysis-ik/issues/1155
func TestTokenizeIssue1155SmartComplexCrossPathOffsetsDoNotGoBackwards(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)
	dic.GetSingleton().AddWords(issue1155Words)

	assertOffsetsNeverGoBackwards(t, configuration, issue1155Text)
}

// Issue #1155: repeated complex crossing paths must stay bounded. This is a
// coarse performance guard, not a micro benchmark.
// https://github.com/infinilabs/analysis-ik/issues/1155
func TestTokenizeIssue1155SmartComplexCrossPathPerformance(t *testing.T) {
	configuration := testutil.CreateFakeConfigurationSub(true)
	dic.GetSingleton().AddWords(issue1155Words)

	var sb strings.Builder
	sb.Grow(len(issue1155Text) * 200)
	for i := 0; i < 200; i++ {
		sb.WriteString(issue1155Text)
		sb.WriteString("。")
	}

	startTime := time.Now()
	assertOffsetsNeverGoBackwards(t, configuration, sb.String())
	duration := time.Since(startTime)

	if duration > 5000*time.Millisecond {
		t.Errorf("IK_SMART分词Issue #1155复杂crossPath耗时%dms，超过5秒限制", duration.Milliseconds())
	}

	t.Logf("IK_SMART分词Issue #1155复杂crossPath耗时: %dms", duration.Milliseconds())
}

// tokenize reads every token's text out of the term buffer, taking its length
// from the offsets the way a consumer of the stream would.
func tokenize(t *testing.T, configuration cfg.Configuration, s string) []string {
	t.Helper()
	tokens := []string{}

	analyzer := NewIKAnalyzer(configuration)
	defer func() {
		if err := analyzer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	tokenStream := analyzer.TokenStream("text", s)
	if err := tokenStream.Reset(); err != nil {
		t.Fatal(err)
	}
	for {
		more, err := tokenStream.IncrementToken()
		if err != nil {
			t.Fatal(err)
		}
		if !more {
			break
		}
		charTermAttribute := tokenStream.CharTermAttribute()
		offsetAttribute := tokenStream.OffsetAttribute()
		length := offsetAttribute.EndOffset() - offsetAttribute.StartOffset()
		chars := make([]jdk.Char, length)
		copy(chars, charTermAttribute.Buffer()[:length])
		tokens = append(tokens, jdk.DecodeUTF16(chars))
	}
	return tokens
}

// tokenInfo is one token's text and its type constant.
type tokenInfo struct {
	text      string
	tokenType int
}

// tokenizeWithType reads every token along with its type.
func tokenizeWithType(t *testing.T, configuration cfg.Configuration, s string) []tokenInfo {
	t.Helper()
	tokenInfos := []tokenInfo{}

	analyzer := NewIKAnalyzer(configuration)
	defer func() {
		if err := analyzer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	tokenStream := analyzer.TokenStream("text", s)
	if err := tokenStream.Reset(); err != nil {
		t.Fatal(err)
	}

	charTermAttribute := tokenStream.CharTermAttribute()
	offsetAttribute := tokenStream.OffsetAttribute()
	typeAttribute := tokenStream.TypeAttribute()

	for {
		more, err := tokenStream.IncrementToken()
		if err != nil {
			t.Fatal(err)
		}
		if !more {
			break
		}
		length := offsetAttribute.EndOffset() - offsetAttribute.StartOffset()
		chars := make([]jdk.Char, length)
		copy(chars, charTermAttribute.Buffer()[:length])
		tokenInfos = append(tokenInfos, tokenInfo{
			text:      jdk.DecodeUTF16(chars),
			tokenType: mapTypeStringToInt(typeAttribute.Type()),
		})
	}
	return tokenInfos
}

func assertOffsetsNeverGoBackwards(t *testing.T, configuration cfg.Configuration, s string) {
	t.Helper()

	analyzer := NewIKAnalyzer(configuration)
	defer func() {
		if err := analyzer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	tokenStream := analyzer.TokenStream("text", s)
	if err := tokenStream.Reset(); err != nil {
		t.Fatal(err)
	}

	offsetAttribute := tokenStream.OffsetAttribute()
	lastStartOffset := 0
	sawToken := false

	for {
		more, err := tokenStream.IncrementToken()
		if err != nil {
			t.Fatal(err)
		}
		if !more {
			break
		}
		startOffset := offsetAttribute.StartOffset()
		endOffset := offsetAttribute.EndOffset()

		if startOffset < 0 {
			t.Fatal("startOffset must be non-negative")
		}
		if endOffset < startOffset {
			t.Fatal("endOffset must be >= startOffset")
		}
		if startOffset < lastStartOffset {
			t.Fatalf("offsets must not go backwards: startOffset=%d,lastStartOffset=%d",
				startOffset, lastStartOffset)
		}

		lastStartOffset = startOffset
		sawToken = true
	}

	if err := tokenStream.End(); err != nil {
		t.Fatal(err)
	}
	if !sawToken {
		t.Error("分词结果不能为空")
	}
}

// mapTypeStringToInt turns a token type name back into its constant.
func mapTypeStringToInt(typeStr string) int {
	switch typeStr {
	case "ENGLISH":
		return core.TypeEnglish
	case "ARABIC":
		return core.TypeArabic
	case "LETTER":
		return core.TypeLetter
	case "CN_WORD":
		return core.TypeCNWord
	case "CN_CHAR":
		return core.TypeCNChar
	case "OTHER_CJK":
		return core.TypeOtherCJK
	case "COUNT":
		return core.TypeCount
	case "TYPE_CNUM":
		return core.TypeCNum
	case "TYPE_CQUAN":
		return core.TypeCQuan
	default:
		return core.TypeUnknown
	}
}

func assertTokens(t *testing.T, values, expected []string) {
	t.Helper()
	if len(values) != len(expected) {
		t.Fatalf("token count: expected %d, got %d (%s)", len(expected), len(values), describe(values))
	}
	for i, want := range expected {
		if values[i] != want {
			t.Errorf("token %d: expected %q, got %q", i, want, values[i])
		}
	}
}

func describe(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}
