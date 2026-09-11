// Package opensearch registers the IK analyzer under the names OpenSearch knows
// it by, and resolves its dictionaries out of an OpenSearch installation.
//
// OpenSearch loads plugins as JVM classes, which a Go build cannot produce, so
// the plugin bundle itself is out of scope. What is reproduced here is the part
// that decides behaviour: the two analyzer names, which of them segments
// smartly, how the settings block is read, and where the configuration
// directory is.
package opensearch

// PluginName is the name OpenSearch knows the plugin by. It is also the name of
// the configuration directory inside the node's config directory.
const PluginName = "analysis-ik"

// The two names the plugin registers, as both an analyzer and a tokenizer.
const (
	// AnalyzerIkSmart resolves ambiguity and emits the fewest sensible tokens.
	AnalyzerIkSmart = "ik_smart"
	// AnalyzerIkMaxWord emits every word the dictionary can find.
	AnalyzerIkMaxWord = "ik_max_word"
)

// AnalysisIkPlugin is the plugin's registration surface.
type AnalysisIkPlugin struct{}

// TokenizerFactoryProvider builds a tokenizer factory from a settings block.
type TokenizerFactoryProvider func(env Environment, name string, settings Settings) *IkTokenizerFactory

// AnalyzerProviderProvider builds an analyzer provider from a settings block.
type AnalyzerProviderProvider func(env Environment, name string, settings Settings) *IkAnalyzerProvider

// Tokenizers returns the tokenizers the plugin registers.
func (AnalysisIkPlugin) Tokenizers() map[string]TokenizerFactoryProvider {
	return map[string]TokenizerFactoryProvider{
		AnalyzerIkSmart:   GetIkSmartTokenizerFactory,
		AnalyzerIkMaxWord: GetIkTokenizerFactory,
	}
}

// Analyzers returns the analyzers the plugin registers.
func (AnalysisIkPlugin) Analyzers() map[string]AnalyzerProviderProvider {
	return map[string]AnalyzerProviderProvider{
		AnalyzerIkSmart:   GetIkSmartAnalyzerProvider,
		AnalyzerIkMaxWord: GetIkAnalyzerProvider,
	}
}
