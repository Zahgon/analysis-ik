package opensearch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/dic"
	"github.com/infinilabs/analysis-ik/jdk"
)

// stubEnvironment writes the dictionaries the loader insists on into a config
// directory laid out the way an OpenSearch node lays one out.
func stubEnvironment(t *testing.T) Environment {
	t.Helper()
	root := t.TempDir()
	confDir := filepath.Join(root, PluginName)
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		// Without the config file the loader falls back to the directory beside
		// the binary and looks for its dictionaries there instead.
		"IKAnalyzer.cfg.xml": `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE properties SYSTEM "http://java.sun.com/dtd/properties.dtd">
<properties><comment>IK Analyzer 扩展配置</comment></properties>`,
		"main.dic":        "中华人民共和国\n国歌\n",
		"surname.dic":     "",
		"quantifier.dic":  "年\n",
		"suffix.dic":      "",
		"preposition.dic": "",
		"stopword.dic":    "the\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(confDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Environment{ConfigDir: root}
}

func TestPluginRegistersBothNamesAsAnalyzerAndTokenizer(t *testing.T) {
	plugin := AnalysisIkPlugin{}

	tokenizers := plugin.Tokenizers()
	analyzers := plugin.Analyzers()
	for _, name := range []string{AnalyzerIkSmart, AnalyzerIkMaxWord} {
		if _, ok := tokenizers[name]; !ok {
			t.Errorf("tokenizer %q is not registered", name)
		}
		if _, ok := analyzers[name]; !ok {
			t.Errorf("analyzer %q is not registered", name)
		}
	}
	if len(tokenizers) != 2 || len(analyzers) != 2 {
		t.Errorf("registered %d tokenizers and %d analyzers, want 2 of each",
			len(tokenizers), len(analyzers))
	}
	if PluginName != "analysis-ik" {
		t.Errorf("PluginName = %q", PluginName)
	}
}

// ik_smart resolves ambiguity; ik_max_word does not. Getting this backwards
// would change every index built with the plugin.
func TestRegisteredNamesSelectTheirSegmentationMode(t *testing.T) {
	env := stubEnvironment(t)
	plugin := AnalysisIkPlugin{}

	smart := plugin.Tokenizers()[AnalyzerIkSmart](env, AnalyzerIkSmart, Settings{})
	if !smart.configuration.UseSmart() {
		t.Error("ik_smart must segment smartly")
	}
	if got := smart.Name(); got != AnalyzerIkSmart {
		t.Errorf("Name = %q, want %q", got, AnalyzerIkSmart)
	}
	if smart.Create() == nil {
		t.Error("the factory should build a tokenizer")
	}

	maxWord := plugin.Tokenizers()[AnalyzerIkMaxWord](env, AnalyzerIkMaxWord, Settings{})
	if maxWord.configuration.UseSmart() {
		t.Error("ik_max_word must not segment smartly")
	}

	maxWordAnalyzer := plugin.Analyzers()[AnalyzerIkMaxWord](env, AnalyzerIkMaxWord, Settings{})
	if maxWordAnalyzer.Get() == nil {
		t.Error("the provider should hold an analyzer")
	}

	smartAnalyzer := plugin.Analyzers()[AnalyzerIkSmart](env, AnalyzerIkSmart, Settings{})
	if smartAnalyzer.Get() == nil {
		t.Error("the provider should hold an analyzer")
	}
	if got := smartAnalyzer.Name(); got != AnalyzerIkSmart {
		t.Errorf("Name = %q, want %q", got, AnalyzerIkSmart)
	}
}

func TestSettingsDefaults(t *testing.T) {
	env := stubEnvironment(t)

	// use_smart defaults to false, enable_lowercase and enable_remote_dict to true.
	blank := NewConfigurationSub(env, Settings{})
	if blank.UseSmart() {
		t.Error("use_smart should default to false")
	}
	if !blank.EnableLowercase() {
		t.Error("enable_lowercase should default to true")
	}
	if !blank.EnableRemoteDict() {
		t.Error("enable_remote_dict should default to true")
	}

	// Every one of them is a string compared against "true".
	explicit := NewConfigurationSub(env, Settings{
		"use_smart":          "true",
		"enable_lowercase":   "false",
		"enable_remote_dict": "false",
	})
	if !explicit.UseSmart() || explicit.EnableLowercase() || explicit.EnableRemoteDict() {
		t.Errorf("settings not applied: smart=%t lowercase=%t remote=%t",
			explicit.UseSmart(), explicit.EnableLowercase(), explicit.EnableRemoteDict())
	}

	// Anything that is not exactly "true" reads as false.
	loose := NewConfigurationSub(env, Settings{"use_smart": "TRUE", "enable_lowercase": "yes"})
	if loose.UseSmart() || loose.EnableLowercase() {
		t.Error("only the exact string \"true\" enables a setting")
	}
}

func TestSettingsAccessors(t *testing.T) {
	settings := Settings{"present": "value", "flag": "true"}
	if got := settings.Get("present", "fallback"); got != "value" {
		t.Errorf("Get = %q, want value", got)
	}
	if got := settings.Get("absent", "fallback"); got != "fallback" {
		t.Errorf("Get = %q, want fallback", got)
	}
	if !settings.GetAsBoolean("flag", false) {
		t.Error("GetAsBoolean should read the stored value")
	}
	if !settings.GetAsBoolean("absent", true) {
		t.Error("GetAsBoolean should fall back")
	}
	if settings.GetAsBoolean("present", true) {
		t.Error("a non-boolean value reads as false")
	}
}

// The analyzer provider reads enable_lowercase a second time, as a boolean, and
// applies it over whatever the shared configuration already decided.
func TestAnalyzerProviderAppliesLowercaseOnTop(t *testing.T) {
	env := stubEnvironment(t)

	provider := NewIkAnalyzerProvider(env, AnalyzerIkSmart, Settings{"enable_lowercase": "false"}, true)
	if provider.Get() == nil {
		t.Fatal("the provider should hold an analyzer")
	}

	configuration := NewConfigurationSub(env, Settings{"enable_lowercase": "false"})
	if configuration.EnableLowercase() {
		t.Error("enable_lowercase=false should reach the configuration")
	}
	if configuration.SetEnableLowercase(true).EnableLowercase() != true {
		t.Error("the setter should return a configuration with the new value")
	}
	if !configuration.SetUseSmart(true).UseSmart() {
		t.Error("the setter should return a configuration with the new value")
	}
}

func TestConfigurationDirectories(t *testing.T) {
	env := stubEnvironment(t)
	configuration := NewConfigurationSub(env, Settings{})

	if got, want := configuration.ConfDir(), filepath.Join(env.ConfigDir, PluginName); got != want {
		t.Errorf("ConfDir = %q, want %q", got, want)
	}
	// The in-plugin fallback sits beside the running binary.
	if got := configuration.ConfigInPluginDir(); !strings.HasSuffix(got, "config") {
		t.Errorf("ConfigInPluginDir = %q, want it to end in config", got)
	}
	if got, want := configuration.Path("a", "b", "c"), filepath.Join("a", "b", "c"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	// The plugin needs no permission hook, so it implements none.
	if _, ok := interface{}(configuration).(cfg.Checker); ok {
		t.Error("the plugin configuration should not implement Checker")
	}
}

func TestTheDictionaryIsLoadedFromTheNodeConfigDirectory(t *testing.T) {
	env := stubEnvironment(t)
	NewConfigurationSub(env, Settings{})

	if !dic.GetSingleton().MatchAllInMainDict(jdk.EncodeUTF16("中华人民共和国")).IsMatch() {
		t.Error("the node's main dictionary should have been loaded")
	}
}

func TestSetSmartReturnsTheFactory(t *testing.T) {
	env := stubEnvironment(t)
	factory := NewIkTokenizerFactory(env, AnalyzerIkMaxWord, Settings{})
	if factory.SetSmart(true) != factory {
		t.Error("SetSmart should return the receiver")
	}
	if !factory.configuration.UseSmart() {
		t.Error("SetSmart(true) should switch the mode")
	}
}
