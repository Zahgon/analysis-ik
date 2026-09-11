package help

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func capture(t *testing.T, write func()) string {
	t.Helper()
	var buf bytes.Buffer
	SetOutput(&buf)
	t.Cleanup(func() { SetOutput(defaultOutput) })
	write()
	return buf.String()
}

func TestRecordsCarryLevelAndLoggerName(t *testing.T) {
	logger := GetLogger("org.wltea.analyzer.dic.Dictionary")
	if got := logger.Name(); got != "org.wltea.analyzer.dic.Dictionary" {
		t.Errorf("Name = %q", got)
	}
	if got := logger.Prefix(); got != "" {
		t.Errorf("Prefix = %q, want empty", got)
	}

	out := capture(t, func() {
		logger.Info("start to reload ik dict.")
		logger.Warn("[Ext Loading] file not found: {}", "/tmp/absent.dic")
		logger.Error("ik-analyzer", errors.New("boom"))
	})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("wrote %d lines: %q", len(lines), out)
	}
	for i, want := range []string{
		"INFO org.wltea.analyzer.dic.Dictionary start to reload ik dict.",
		"WARN org.wltea.analyzer.dic.Dictionary [Ext Loading] file not found: /tmp/absent.dic",
		"ERROR org.wltea.analyzer.dic.Dictionary ik-analyzer boom",
	} {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
}

// A prefix becomes a marker shared by every logger built with it, the way the
// original interned them in a weak map.
func TestPrefixMarkersAreInterned(t *testing.T) {
	before := markersSize()
	first := GetLoggerWithPrefix("analysis-ik", "a")
	second := GetLoggerWithPrefix("analysis-ik", "b")
	if first.Prefix() != "analysis-ik" || second.Prefix() != "analysis-ik" {
		t.Error("both loggers should carry the prefix")
	}
	if got := markersSize(); got != before+1 {
		t.Errorf("markers grew from %d to %d, want one new marker", before, got)
	}

	out := capture(t, func() { first.Info("hello") })
	if want := "INFO a [analysis-ik] hello\n"; out != want {
		t.Errorf("record = %q, want %q", out, want)
	}
	// An empty prefix means no marker at all.
	if got := GetLoggerWithPrefix("", "c").Prefix(); got != "" {
		t.Errorf("Prefix = %q, want empty", got)
	}
}

func TestFormatFollowsParameterizedMessageRules(t *testing.T) {
	cases := []struct {
		name    string
		message string
		params  []any
		want    string
	}{
		{"no parameters", "plain", nil, "plain"},
		{"one placeholder", "loading {}", []any{"main.dic"}, "loading main.dic"},
		{"two placeholders", "{} returned {}", []any{"/dict", 500}, "/dict returned 500"},
		{"placeholder without parameter", "loading {}", nil, "loading {}"},
		// A trailing error is the record's throwable and follows the message.
		{"trailing error", "ik-analyzer", []any{errors.New("boom")}, "ik-analyzer boom"},
		// Anything else left over is dropped, as it is by log4j2.
		{"surplus parameter", "getRemoteWords {} error", []any{errors.New("boom"), "/dict"},
			"getRemoteWords boom error"},
	}
	for _, c := range cases {
		if got := Format(c.message, c.params...); got != c.want {
			t.Errorf("%s: Format = %q, want %q", c.name, got, c.want)
		}
	}
}
