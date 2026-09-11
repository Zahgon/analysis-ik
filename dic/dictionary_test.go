package dic

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/help"
	"github.com/infinilabs/analysis-ik/jdk"
)

// TestMain silences the loader's own log records. These tests build dozens of
// dictionaries out of deliberately broken directories, and the resulting
// records would bury the test output. help's own tests cover the format.
func TestMain(m *testing.M) {
	help.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// testConfiguration points the loader at a directory the test controls.
type testConfiguration struct {
	cfg.Settings
	confDir     string
	inPluginDir string
	checked     int
}

func (c *testConfiguration) ConfDir() string           { return c.confDir }
func (c *testConfiguration) ConfigInPluginDir() string { return c.inPluginDir }
func (c *testConfiguration) Path(first string, more ...string) string {
	return filepath.Join(append([]string{first}, more...)...)
}
func (c *testConfiguration) SetUseSmart(v bool) cfg.Configuration {
	c.SetUseSmartFlag(v)
	return c
}
func (c *testConfiguration) SetEnableLowercase(v bool) cfg.Configuration {
	c.SetEnableLowercaseFlag(v)
	return c
}
func (c *testConfiguration) Check() { c.checked++ }

// byteOrderMark is what the shipped dictionaries start with.
const byteOrderMark = "\uFEFF"

// The six dictionaries the loader always reads; three of them are fatal when
// missing even though nothing ever queries them.
var requiredDicts = []string{
	pathDicMain, pathDicSurname, pathDicQuantifier,
	pathDicSuffix, pathDicPrep, pathDicStop,
}

func writeConfigDir(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range requiredDicts {
		if _, ok := entries[name]; !ok {
			entries[name] = ""
		}
	}
	for name, content := range entries {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newTestDictionary(t *testing.T, dir string) *Dictionary {
	t.Helper()
	c := &testConfiguration{Settings: cfg.NewSettings(), confDir: dir, inPluginDir: dir}
	d := newDictionary(c)
	d.loadMainDict()
	d.loadSurnameDict()
	d.loadQuantifierDict()
	d.loadSuffixDict()
	d.loadPrepDict()
	d.loadStopWordDict()
	return d
}

// withSingleton installs a dictionary as the process singleton for one test.
func withSingleton(t *testing.T, d *Dictionary) {
	t.Helper()
	previous := singleton.Load()
	singleton.Store(d)
	t.Cleanup(func() { singleton.Store(previous) })
}

func TestGetSingletonRefusesToGuess(t *testing.T) {
	previous := singleton.Load()
	singleton.Store(nil)
	t.Cleanup(func() { singleton.Store(previous) })

	defer func() {
		want := "ik dict has not been initialized yet, please call initial method first."
		if r := recover(); r != want {
			t.Errorf("recover() = %v, want %q", r, want)
		}
	}()
	GetSingleton()
}

// Words arrive with a byte order mark on the first line only, CRLF endings,
// blank lines and surrounding whitespace. All of that has to be handled the way
// a BufferedReader over a UTF-8 stream did.
func TestLoadDictFileHandlesTheShippedFileShape(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		pathDicMain: byteOrderMark + "中华\r\n\r\n   数据库   \r\n人民共和国\rGoLang\n",
		pathDicStop: byteOrderMark + "the\r\n",
	})
	d := newTestDictionary(t, dir)

	for _, word := range []string{"中华", "数据库", "人民共和国", "GoLang"} {
		if !d.MatchAllInMainDict(jdk.EncodeUTF16(word)).IsMatch() {
			t.Errorf("%q should have loaded", word)
		}
	}
	// The mark belongs to the file, not to the word.
	if d.MatchAllInMainDict(jdk.EncodeUTF16(byteOrderMark + "中华")).IsMatch() {
		t.Error("the byte order mark should not be part of the first word")
	}
	if !d.IsStopWord(jdk.EncodeUTF16("the"), 0, 3) {
		t.Error("the should be a stop word")
	}
}

func TestLoadDictFileTreatsAMissingCriticalFileAsFatal(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{})
	if err := os.Remove(filepath.Join(dir, pathDicSurname)); err != nil {
		t.Fatal(err)
	}
	c := &testConfiguration{Settings: cfg.NewSettings(), confDir: dir, inPluginDir: dir}
	d := newDictionary(c)

	defer func() {
		r, ok := recover().(string)
		if !ok || !strings.Contains(r, "Surname not found!!!") {
			t.Errorf("recover() = %v, want the surname dictionary to be fatal", r)
		}
	}()
	d.loadSurnameDict()
}

// A missing optional dictionary is only logged, and a directory in a
// dictionary's place counts as missing rather than as an empty file.
func TestLoadDictFileToleratesAMissingOptionalFileAndADirectory(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{pathDicMain: "中华\n"})
	if err := os.Remove(filepath.Join(dir, pathDicQuantifier)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, pathDicStop)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, pathDicStop), 0o755); err != nil {
		t.Fatal(err)
	}
	d := newTestDictionary(t, dir)

	if !d.MatchAllInMainDict(jdk.EncodeUTF16("中华")).IsMatch() {
		t.Error("loading should have continued")
	}
	if d.IsStopWord(jdk.EncodeUTF16("the"), 0, 3) {
		t.Error("a directory yields no stop words")
	}
}

func TestNewDictionaryFallsBackToThePluginConfigDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	fallback := writeConfigDir(t, map[string]string{
		pathDicMain: "中华\n",
		fileName: `<?xml version="1.0" encoding="UTF-8"?>
<properties><entry key="ext_dict">extra.dic</entry></properties>`,
		"extra.dic": "量子纠缠态\n",
	})

	c := &testConfiguration{Settings: cfg.NewSettings(), confDir: missing, inPluginDir: fallback}
	d := newDictionary(c)
	d.loadMainDict()

	// The fallback directory becomes the dictionary root, not just the config
	// file's home.
	if got := d.getDictRoot(); !strings.HasPrefix(got, fallback) {
		t.Errorf("dict root = %q, want it under %q", got, fallback)
	}
	if !d.MatchAllInMainDict(jdk.EncodeUTF16("量子纠缠态")).IsMatch() {
		t.Error("the extension dictionary named in the fallback config should have loaded")
	}
}

func TestExtensionDictionariesAreSplitTrimmedAndWalked(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		pathDicMain: "中华\n",
		fileName: `<?xml version="1.0" encoding="UTF-8"?>
<properties>
  <entry key="ext_dict">custom/one.dic; custom/nested ;;custom/absent.dic</entry>
  <entry key="ext_stopwords">custom/stop.dic</entry>
</properties>`,
		"custom/one.dic":         "量子纠缠态\n",
		"custom/nested/deep.dic": "深度学习框架\n",
		"custom/stop.dic":        "噪声\n",
	})
	d := newTestDictionary(t, dir)

	for _, word := range []string{"量子纠缠态", "深度学习框架"} {
		if !d.MatchAllInMainDict(jdk.EncodeUTF16(word)).IsMatch() {
			t.Errorf("%q should have loaded", word)
		}
	}
	if !d.IsStopWord(jdk.EncodeUTF16("噪声"), 0, 2) {
		t.Error("the extension stop word should have loaded")
	}
}

func TestAddAndDisableWordsAtRuntime(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{pathDicMain: "数据库\n数据\n"})
	d := newTestDictionary(t, dir)

	d.AddWords([]string{"  量子纠缠态  ", "深度学习框架"})
	if !d.MatchAllInMainDict(jdk.EncodeUTF16("量子纠缠态")).IsMatch() {
		t.Error("added words are trimmed before insertion")
	}

	d.DisableWords([]string{"数据"})
	if d.MatchAllInMainDict(jdk.EncodeUTF16("数据")).IsMatch() {
		t.Error("数据 should be disabled")
	}
	if !d.MatchAllInMainDict(jdk.EncodeUTF16("数据库")).IsMatch() {
		t.Error("数据库 should survive its prefix being disabled")
	}

	d.AddStopWords([]string{"噪声"})
	if !d.IsStopWord(jdk.EncodeUTF16("噪声"), 0, 2) {
		t.Error("噪声 should be a stop word")
	}
}

func TestMatchHelpersSpanTheThreeDictionaries(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		pathDicMain:       "数据库\n",
		pathDicQuantifier: "年\n年代\n",
		pathDicStop:       "the\n",
	})
	d := newTestDictionary(t, dir)
	withSingleton(t, d)

	units := jdk.EncodeUTF16("x数据库y")
	if !d.MatchInMainDict(units, 1, 3).IsMatch() {
		t.Error("数据库 should match inside a larger buffer")
	}

	quantifiers := jdk.EncodeUTF16("年代")
	hit := d.MatchInQuantifierDict(quantifiers, 0, 1)
	if !hit.IsMatch() || !hit.IsPrefix() {
		t.Error("年 is both a quantifier and the prefix of 年代")
	}
	if !d.MatchWithHit(quantifiers, 1, hit).IsMatch() {
		t.Error("年代 should match when the hit is carried forward")
	}
	if !d.IsStopWord(jdk.EncodeUTF16("the"), 0, 3) {
		t.Error("the should be a stop word")
	}
}

func TestReLoadMainDictPicksUpChangedFiles(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{pathDicMain: "中华\n", pathDicStop: "the\n"})
	d := newTestDictionary(t, dir)
	withSingleton(t, d)

	d.AddWords([]string{"运行期加入"})
	if err := os.WriteFile(filepath.Join(dir, pathDicMain), []byte("量子纠缠态\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, pathDicStop), []byte("noise\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d.reLoadMainDict()

	if !d.MatchAllInMainDict(jdk.EncodeUTF16("量子纠缠态")).IsMatch() {
		t.Error("the new file should be live")
	}
	// A reload builds a fresh trie, so runtime additions are lost with the old one.
	if d.MatchAllInMainDict(jdk.EncodeUTF16("运行期加入")).IsMatch() {
		t.Error("runtime additions do not survive a reload")
	}
	if !d.IsStopWord(jdk.EncodeUTF16("noise"), 0, 5) {
		t.Error("the new stop words should be live")
	}
}

func TestScanJavaLinesSplitsOnEveryLineTerminator(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a\nb\n", []string{"a", "b"}},
		{"a\r\nb\r\n", []string{"a", "b"}},
		{"a\rb\r", []string{"a", "b"}},
		{"a\r\nb", []string{"a", "b"}},
		{"a", []string{"a"}},
		{"", nil},
		{"\n", []string{""}},
	}
	for _, c := range cases {
		var got []string
		advance, token, err := 0, []byte(nil), error(nil)
		data := []byte(c.in)
		for {
			advance, token, err = scanJavaLines(data, true)
			if err != nil {
				t.Fatal(err)
			}
			if advance == 0 && token == nil {
				break
			}
			got = append(got, string(token))
			data = data[advance:]
		}
		if len(got) != len(c.want) {
			t.Errorf("scan(%q) = %q, want %q", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("scan(%q) = %q, want %q", c.in, got, c.want)
				break
			}
		}
	}
}
