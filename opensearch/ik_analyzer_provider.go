package opensearch

import "github.com/infinilabs/analysis-ik/lucene"

// IkAnalyzerProvider hands out the IK analyzer for one named analyzer.
type IkAnalyzerProvider struct {
	name     string
	analyzer *lucene.IKAnalyzer
}

// NewIkAnalyzerProvider builds a provider from a settings block.
func NewIkAnalyzerProvider(env Environment, name string, settings Settings, useSmart bool) *IkAnalyzerProvider {
	// The analyzer settings may override case folding on top of whatever the
	// shared configuration read.
	enableLowercase := settings.GetAsBoolean("enable_lowercase", true)

	configuration := NewConfigurationSub(env, settings)
	configuration.SetUseSmart(useSmart)
	configuration.SetEnableLowercase(enableLowercase)
	return &IkAnalyzerProvider{name: name, analyzer: lucene.NewIKAnalyzer(configuration)}
}

// GetIkSmartAnalyzerProvider builds the ik_smart analyzer provider.
func GetIkSmartAnalyzerProvider(env Environment, name string, settings Settings) *IkAnalyzerProvider {
	return NewIkAnalyzerProvider(env, name, settings, true)
}

// GetIkAnalyzerProvider builds the ik_max_word analyzer provider.
func GetIkAnalyzerProvider(env Environment, name string, settings Settings) *IkAnalyzerProvider {
	return NewIkAnalyzerProvider(env, name, settings, false)
}

// Name is the name this provider was registered under.
func (p *IkAnalyzerProvider) Name() string { return p.name }

// Get returns the analyzer.
func (p *IkAnalyzerProvider) Get() *lucene.IKAnalyzer { return p.analyzer }
