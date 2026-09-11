package lucene

import (
	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/jdk"
)

// IKAnalyzer produces token streams over text, using the IK segmentation
// engine.
type IKAnalyzer struct {
	configuration cfg.Configuration
}

// NewIKAnalyzer builds an analyzer with the given configuration.
func NewIKAnalyzer(configuration cfg.Configuration) *IKAnalyzer {
	return &IKAnalyzer{configuration: configuration}
}

// CreateComponents builds a tokenizer for a field.
func (a *IKAnalyzer) CreateComponents(fieldName string) *TokenStreamComponents {
	return NewTokenStreamComponents(NewIKTokenizer(a.configuration))
}

// TokenStream returns a stream over text. The caller must Reset it before
// reading, and End it afterwards.
func (a *IKAnalyzer) TokenStream(fieldName, text string) TokenStream {
	components := a.CreateComponents(fieldName)
	components.SetReader(jdk.NewStringReader(text))
	return components.TokenStream()
}

// Close releases the analyzer. Streams own their own input, so there is nothing
// left to release here.
func (a *IKAnalyzer) Close() error { return nil }
