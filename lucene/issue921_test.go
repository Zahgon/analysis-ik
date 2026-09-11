package lucene

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/dic"
	"github.com/infinilabs/analysis-ik/internal/testutil"
	"github.com/infinilabs/analysis-ik/jdk"
)

// Issue #921: a remote stop-word list made the fast vector highlighter mis-place
// highlights on multi-valued fields.
// https://github.com/infinilabs/analysis-ik/issues/921
//
// The cause: when every lexeme of one value was filtered out as a stop word,
// IKTokenizer.end() reported a final offset of 0 instead of the value's length,
// so every following value's offsets were shifted.
//
// Each scenario below runs under both ik_max_word and ik_smart, interleaved.

const finalOffsetMarker = "<FINAL_OFFSET>"

// extraStopwords are added on top of the shipped stop-word list so the
// scenarios have something to filter.
var extraStopwords = []string{"value", "hello", "world", "的"}

var (
	setUpOnce  sync.Once
	cfgMaxWord cfg.Configuration
	cfgSmart   cfg.Configuration
)

func setUp() {
	setUpOnce.Do(func() {
		cfgMaxWord = testutil.CreateFakeConfigurationSub(false)
		cfgSmart = testutil.CreateFakeConfigurationSub(true)
		lowered := make([]string, 0, len(extraStopwords))
		for _, word := range extraStopwords {
			lowered = append(lowered, jdk.ToLower(word))
		}
		dic.GetSingleton().AddStopWords(lowered)
	})
}

// tokenInfo921 is one token's term, span, position increment and type.
type tokenInfo921 struct {
	term         string
	startOffset  int
	endOffset    int
	posIncrement int
	tokenType    string
}

func tokenizeWithDetails(t *testing.T, configuration cfg.Configuration, text string) []tokenInfo921 {
	t.Helper()
	tokens := []tokenInfo921{}

	analyzer := NewIKAnalyzer(configuration)
	defer func() {
		if err := analyzer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	tokenStream := analyzer.TokenStream("text", text)
	if err := tokenStream.Reset(); err != nil {
		t.Fatal(err)
	}

	charTermAttr := tokenStream.CharTermAttribute()
	offsetAttr := tokenStream.OffsetAttribute()
	posIncrAttr := tokenStream.PositionIncrementAttribute()
	typeAttr := tokenStream.TypeAttribute()

	for {
		more, err := tokenStream.IncrementToken()
		if err != nil {
			t.Fatal(err)
		}
		if !more {
			break
		}
		tokens = append(tokens, tokenInfo921{
			term:         charTermAttr.String(),
			startOffset:  offsetAttr.StartOffset(),
			endOffset:    offsetAttr.EndOffset(),
			posIncrement: posIncrAttr.PositionIncrement(),
			tokenType:    typeAttr.Type(),
		})
	}
	if err := tokenStream.End(); err != nil {
		t.Fatal(err)
	}

	finalOffset := offsetAttr.StartOffset()
	tokens = append(tokens, tokenInfo921{
		term: finalOffsetMarker, startOffset: finalOffset, endOffset: finalOffset,
		posIncrement: 0, tokenType: "META",
	})
	return tokens
}

func getFinalOffset(t *testing.T, configuration cfg.Configuration, text string) int {
	t.Helper()

	analyzer := NewIKAnalyzer(configuration)
	defer func() {
		if err := analyzer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	tokenStream := analyzer.TokenStream("text", text)
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
	}
	if err := tokenStream.End(); err != nil {
		t.Fatal(err)
	}
	return tokenStream.OffsetAttribute().StartOffset()
}

func printHeader(t *testing.T, scenario, mode, description string) {
	t.Helper()
	t.Log("")
	t.Log("┌─────────────────────────────────────────────────────────")
	t.Log("│ 场景: " + scenario + " [" + mode + "]")
	t.Log("│ 说明: " + description)
	t.Log("└─────────────────────────────────────────────────────────")
}

func printInput(t *testing.T, text string) {
	t.Helper()
	t.Logf("  输入: %q (长度=%d)", text, len(jdk.EncodeUTF16(text)))
}

func printTokenTable(t *testing.T, tokens []tokenInfo921, originalText string) {
	t.Helper()

	realTokens := make([]tokenInfo921, 0, len(tokens))
	var finalOffsetToken *tokenInfo921
	for i, token := range tokens {
		if token.term == finalOffsetMarker {
			if finalOffsetToken == nil {
				finalOffsetToken = &tokens[i]
			}
			continue
		}
		realTokens = append(realTokens, token)
	}

	t.Log("  ┌────────────────────┬──────────┬──────────┬──────────┬────────────┐")
	t.Log("  │ term               │ start    │ end      │ posIncr  │ type       │")
	t.Log("  ├────────────────────┼──────────┼──────────┼──────────┼────────────┤")
	for _, token := range realTokens {
		term := token.term
		if units := jdk.EncodeUTF16(term); len(units) > 18 {
			term = jdk.String(units, 0, 15) + "..."
		}
		t.Logf("  │ %-18s │ %8d │ %8d │ %8d │ %-10s │",
			term, token.startOffset, token.endOffset, token.posIncrement, token.tokenType)
	}
	t.Log("  └────────────────────┴──────────┴──────────┴──────────┴────────────┘")

	if originalText != "" {
		t.Log("  原文:       \"" + originalText + "\"")

		units := jdk.EncodeUTF16(originalText)
		covered := make([]bool, len(units))
		for _, token := range realTokens {
			for i := token.startOffset; i < token.endOffset && i < len(units); i++ {
				covered[i] = true
			}
		}
		var mapLine strings.Builder
		for i := range units {
			if covered[i] {
				mapLine.WriteString("^")
			} else {
				mapLine.WriteString(" ")
			}
		}
		t.Log("  词元覆盖:                 " + mapLine.String())

		terms := make([]string, 0, len(realTokens))
		for _, token := range realTokens {
			terms = append(terms, "\""+token.term+"\"")
		}
		t.Logf("  分词结果:   [%s] (%d个词元)", strings.Join(terms, ", "), len(realTokens))

		// Report the spans no token covered — stop words and whitespace.
		uncoveredStart := -1
		gaps := []string{}
		for i := range units {
			if !covered[i] {
				if uncoveredStart == -1 {
					uncoveredStart = i
				}
				continue
			}
			if uncoveredStart != -1 {
				gaps = append(gaps, "\""+jdk.Trim(jdk.String(units, uncoveredStart, i-uncoveredStart))+"\"")
				uncoveredStart = -1
			}
		}
		if uncoveredStart != -1 {
			remaining := jdk.Trim(jdk.String(units, uncoveredStart, len(units)-uncoveredStart))
			if remaining != "" {
				gaps = append(gaps, "\""+remaining+"\"")
			}
		}
		if len(gaps) > 0 {
			t.Log("  过滤区间:   " + strings.Join(gaps, ", ") + " (停用词/空白)")
		}
	}

	if finalOffsetToken != nil {
		t.Logf("  finalOffset: %d", finalOffsetToken.startOffset)
	}
}

func printAssertion(t *testing.T, label string, expected, actual any, pass bool) {
	t.Helper()
	icon := "✗"
	if pass {
		icon = "✓"
	}
	t.Logf("  %s %s: 期望=%v, 实际=%v", icon, label, expected, actual)
}

func printMultiValueResult(t *testing.T, values []string, expectedLengths, actualOffsets []int, cumulativeOffset int) {
	t.Helper()

	expectedTotal := 0
	for _, length := range expectedLengths {
		expectedTotal += length
	}

	t.Log("  ┌─────┬────────────────────┬──────┬──────────┬──────────┬───────┐")
	t.Log("  │  #  │ 值                 │ 长度 │ 期望FO   │ 实际FO   │ 结果  │")
	t.Log("  ├─────┼────────────────────┼──────┼──────────┼──────────┼───────┤")
	for i, value := range values {
		result := "  ✗  "
		if actualOffsets[i] == expectedLengths[i] {
			result = "  ✓  "
		}
		units := jdk.EncodeUTF16(value)
		shown := value
		if len(units) > 18 {
			shown = jdk.String(units, 0, 15) + "..."
		}
		t.Logf("  │ %3d │ %-18s │ %4d │ %8d │ %8d │ %s │",
			i+1, shown, len(units), expectedLengths[i], actualOffsets[i], result)
	}
	t.Log("  ├─────┼────────────────────┼──────┼──────────┼──────────┼───────┤")
	totalResult := "  ✗  "
	if cumulativeOffset == expectedTotal {
		totalResult = "  ✓  "
	}
	t.Logf("  │     │ 累积 offset        │      │ %8d │ %8d │ %s │",
		expectedTotal, cumulativeOffset, totalResult)
	t.Log("  └─────┴────────────────────┴──────┴──────────┴──────────┴───────┘")
}

// runBothModes runs one verification under ik_max_word and then ik_smart.
func runBothModes(t *testing.T, scenario, description string, verifier func(configuration cfg.Configuration, mode string)) {
	t.Helper()
	printHeader(t, scenario, "ik_max_word", description)
	verifier(cfgMaxWord, "maxWord")
	printHeader(t, scenario, "ik_smart", description)
	verifier(cfgSmart, "smart")
}

// Scenario 1: a value that is entirely stop words, and a run of consecutive
// stop words.
func TestAllStopwords(t *testing.T) {
	setUp()
	runBothModes(t, "全停用词 + 连续停用词",
		"全部被过滤时 finalOffset 正确；连续停用词 posIncrement 累加",
		func(configuration cfg.Configuration, mode string) {
			// A single value that is nothing but a stop word.
			text := "value"
			tokens := tokenizeWithDetails(t, configuration, text)
			printInput(t, text)
			printTokenTable(t, tokens, text)

			realTokens := []string{}
			for _, token := range tokens {
				if token.term != finalOffsetMarker {
					realTokens = append(realTokens, token.term)
				}
			}
			noTokens := len(realTokens) == 0
			printAssertion(t, "无 token 产出（停用词全部过滤）", true, noTokens, noTokens)
			if !noTokens {
				t.Fatalf("[%s] \"value\" 是停用词，不应产生 token", mode)
			}

			finalOffset := finalOffsetOf(tokens)
			printAssertion(t, "finalOffset = 文本长度", 5, finalOffset, finalOffset == 5)
			if finalOffset != 5 {
				t.Fatalf("[%s] finalOffset 应为 5，实际为 %d", mode, finalOffset)
			}

			// Consecutive stop words.
			text2 := "hello a the world test"
			tokens2 := tokenizeWithDetails(t, configuration, text2)
			printInput(t, text2)
			printTokenTable(t, tokens2, text2)

			testToken := findToken(tokens2, "test")
			if testToken == nil {
				t.Fatalf("[%s] 'test' token 缺失", mode)
			}
			posCorrect := testToken.posIncrement == 5
			printAssertion(t, "'test' posIncrement（跳过 hello/a/the/world 4个停用词）", 5, testToken.posIncrement, posCorrect)
			if !posCorrect {
				t.Fatalf("[%s] 'test' posIncrement 应为 5，实际为 %d", mode, testToken.posIncrement)
			}

			finalOffset2 := finalOffsetOf(tokens2)
			printAssertion(t, "finalOffset = 文本长度", 22, finalOffset2, finalOffset2 == 22)
			if finalOffset2 != 22 {
				t.Fatalf("[%s] finalOffset 应为 22", mode)
			}
		})
}

// Scenario 2: stop-word filtering in English and in Chinese.
func TestStopwordFiltering(t *testing.T) {
	setUp()
	runBothModes(t, "停用词过滤（英文 offset + 中文停用词）",
		"英文停用词被过滤后 offset/posIncrement 正确；中文 '的' 被过滤",
		func(configuration cfg.Configuration, mode string) {
			// English offsets and position increments.
			text := "foo value bar"
			tokens := tokenizeWithDetails(t, configuration, text)
			printInput(t, text)
			printTokenTable(t, tokens, text)

			fooToken := findToken(tokens, "foo")
			if fooToken == nil {
				t.Fatalf("[%s] 'foo' token 缺失", mode)
			}
			printAssertion(t, "'foo' startOffset", 0, fooToken.startOffset, fooToken.startOffset == 0)
			printAssertion(t, "'foo' endOffset", 3, fooToken.endOffset, fooToken.endOffset == 3)
			printAssertion(t, "'foo' posIncrement", 1, fooToken.posIncrement, fooToken.posIncrement == 1)
			if fooToken.startOffset != 0 || fooToken.endOffset != 3 || fooToken.posIncrement != 1 {
				t.Fatalf("[%s] 'foo' 期望 [0,3) posIncrement 1，实际 [%d,%d) posIncrement %d",
					mode, fooToken.startOffset, fooToken.endOffset, fooToken.posIncrement)
			}

			barToken := findToken(tokens, "bar")
			if barToken == nil {
				t.Fatalf("[%s] 'bar' token 缺失", mode)
			}
			printAssertion(t, "'bar' startOffset（跳过 'value ' 共6字符）", 10, barToken.startOffset, barToken.startOffset == 10)
			printAssertion(t, "'bar' endOffset", 13, barToken.endOffset, barToken.endOffset == 13)
			printAssertion(t, "'bar' posIncrement（跳过 value 1个停用词）", 2, barToken.posIncrement, barToken.posIncrement == 2)
			if barToken.startOffset != 10 || barToken.endOffset != 13 || barToken.posIncrement != 2 {
				t.Fatalf("[%s] 'bar' 期望 [10,13) posIncrement 2，实际 [%d,%d) posIncrement %d",
					mode, barToken.startOffset, barToken.endOffset, barToken.posIncrement)
			}

			// A Chinese stop word.
			text2 := "我的数据库"
			tokens2 := tokenizeWithDetails(t, configuration, text2)
			printInput(t, text2)
			printTokenTable(t, tokens2, text2)

			cnTerms := []string{}
			for _, token := range tokens2 {
				if token.term != finalOffsetMarker {
					cnTerms = append(cnTerms, token.term)
				}
			}
			printAssertion(t, "'的' 不出现（停用词过滤）", false, contains(cnTerms, "的"), !contains(cnTerms, "的"))
			printAssertion(t, "'我' 存在", true, contains(cnTerms, "我"), contains(cnTerms, "我"))
			printAssertion(t, "'数据库' 存在", true, contains(cnTerms, "数据库"), contains(cnTerms, "数据库"))
			if contains(cnTerms, "的") {
				t.Fatalf("[%s] '的' 不应出现", mode)
			}
			if !contains(cnTerms, "我") || !contains(cnTerms, "数据库") {
				t.Fatalf("[%s] 期望包含 '我' 和 '数据库'，实际 %v", mode, cnTerms)
			}
		})
}

// Scenario 3: the multi-valued field from the original issue.
func TestOriginalIssueScenario(t *testing.T) {
	setUp()
	runBothModes(t, "原始 issue #921 多值场景",
		"模拟 ES 多值字段 [\"RS\",\"复称\",\"value\",\"数据\",\"采集\",\"232\",\"485\",\"数据库\",\"数据库服务器\"]，其中 \"value\" 是停用词",
		func(configuration cfg.Configuration, mode string) {
			values := []string{"RS", "复称", "value", "数据", "采集", "232", "485", "数据库", "数据库服务器"}
			expectedLengths := []int{2, 2, 5, 2, 2, 3, 3, 3, 6}
			actualOffsets := make([]int, len(values))

			cumulativeOffset := 0
			for i, value := range values {
				actualOffsets[i] = getFinalOffset(t, configuration, value)
				cumulativeOffset += actualOffsets[i]
				if actualOffsets[i] != expectedLengths[i] {
					t.Fatalf("[%s] 值 %d finalOffset: 期望 %d，实际 %d",
						mode, i+1, expectedLengths[i], actualOffsets[i])
				}
			}

			expectedTotal := 0
			for _, length := range expectedLengths {
				expectedTotal += length
			}
			printMultiValueResult(t, values, expectedLengths, actualOffsets, cumulativeOffset)

			if cumulativeOffset != expectedTotal {
				t.Fatalf("[%s] 总累积 offset: 期望 %d，实际 %d", mode, expectedTotal, cumulativeOffset)
			}

			t.Log("  ※ 值3 \"value\" 全部是停用词，修复前 finalOffset=0，修复后 finalOffset=5")
			t.Log("  ※ 修复前累积 offset 差 5 字符，导致后续所有值的高亮偏移")
		})
}

// Scenario 4: ordinary text, to prove the fix changed nothing else.
func TestNoStopwordRegression(t *testing.T) {
	setUp()
	runBothModes(t, "无停用词回归测试",
		"确保修复不影响正常分词场景",
		func(configuration cfg.Configuration, mode string) {
			text := "中华人民共和国"
			tokens := tokenizeWithDetails(t, configuration, text)
			printInput(t, text)
			printTokenTable(t, tokens, text)

			terms := []string{}
			for _, token := range tokens {
				if token.term != finalOffsetMarker {
					terms = append(terms, token.term)
				}
			}
			if len(terms) == 0 {
				t.Fatalf("[%s] 应有分词结果", mode)
			}

			lastRealToken := tokens[len(tokens)-2]
			finalToken := tokens[len(tokens)-1]
			pass := finalToken.startOffset >= lastRealToken.endOffset
			printAssertion(t, "finalOffset >= 最后 token endOffset",
				fmt.Sprintf(">=%d", lastRealToken.endOffset), finalToken.startOffset, pass)
			if !pass {
				t.Fatalf("[%s] finalOffset 应 >= 最后一个 token 的 endOffset", mode)
			}
		})
}

// Scenario 5: a multi-valued field where one value contains a stop word.
func TestMultiValueWithPartialStopword(t *testing.T) {
	setUp()
	runBothModes(t, "多值+值内含停用词",
		"多值字段 [\"RS\",\"foo value bar\",\"数据库\"]，\"value\" 被过滤但 finalOffset 仍正确",
		func(configuration cfg.Configuration, mode string) {
			values := []string{"RS", "foo value bar", "数据库"}
			expectedLengths := []int{2, 13, 3}

			cumulativeOffset := 0
			actualOffsets := make([]int, len(values))

			for i, value := range values {
				tokens := tokenizeWithDetails(t, configuration, value)
				actualFinalOffset := finalOffsetOf(tokens)
				actualOffsets[i] = actualFinalOffset
				cumulativeOffset += actualFinalOffset
				if actualFinalOffset != expectedLengths[i] {
					t.Fatalf("[%s] 值 %d finalOffset: 期望 %d，实际 %d",
						mode, i+1, expectedLengths[i], actualFinalOffset)
				}
			}

			printMultiValueResult(t, values, expectedLengths, actualOffsets, cumulativeOffset)

			expectedTotal := 0
			for _, length := range expectedLengths {
				expectedTotal += length
			}
			if cumulativeOffset != expectedTotal {
				t.Fatalf("[%s] 总累积 offset: 期望 %d，实际 %d", mode, expectedTotal, cumulativeOffset)
			}

			// The middle value in detail.
			t.Log("  ── \"foo value bar\" 内部分词详情 ──")
			midTokens := tokenizeWithDetails(t, configuration, "foo value bar")
			printTokenTable(t, midTokens, "foo value bar")

			barToken := findToken(midTokens, "bar")
			if barToken == nil {
				t.Fatalf("[%s] 'bar' token 缺失", mode)
			}
			offsetPass := barToken.startOffset == 10
			posPass := barToken.posIncrement == 2
			printAssertion(t, "'bar' startOffset（'value ' 被过滤，offset 不跳跃）", 10, barToken.startOffset, offsetPass)
			printAssertion(t, "'bar' posIncrement（跳过 'value' 1个停用词）", 2, barToken.posIncrement, posPass)
			if !offsetPass || !posPass {
				t.Fatalf("[%s] 'bar' 期望 startOffset 10 posIncrement 2，实际 %d / %d",
					mode, barToken.startOffset, barToken.posIncrement)
			}

			t.Log("  ※ \"foo value bar\" 中 'value' 被过滤，但 offset 仍基于原始文本位置 [0,13]")
		})
}

func finalOffsetOf(tokens []tokenInfo921) int {
	for _, token := range tokens {
		if token.term == finalOffsetMarker {
			return token.startOffset
		}
	}
	return -1
}

func findToken(tokens []tokenInfo921, term string) *tokenInfo921 {
	for i, token := range tokens {
		if token.term == term {
			return &tokens[i]
		}
	}
	return nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
