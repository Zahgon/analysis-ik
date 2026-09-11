package elasticsearch

import (
	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/lucene"
)

// IkTokenizerFactory builds IK tokenizers for one named tokenizer.
type IkTokenizerFactory struct {
	name          string
	configuration cfg.Configuration
}

// NewIkTokenizerFactory builds a factory from a settings block.
func NewIkTokenizerFactory(env Environment, name string, settings Settings) *IkTokenizerFactory {
	return &IkTokenizerFactory{name: name, configuration: NewConfigurationSub(env, settings)}
}

// GetIkTokenizerFactory builds the ik_max_word tokenizer factory.
func GetIkTokenizerFactory(env Environment, name string, settings Settings) *IkTokenizerFactory {
	return NewIkTokenizerFactory(env, name, settings).SetSmart(false)
}

// GetIkSmartTokenizerFactory builds the ik_smart tokenizer factory.
func GetIkSmartTokenizerFactory(env Environment, name string, settings Settings) *IkTokenizerFactory {
	return NewIkTokenizerFactory(env, name, settings).SetSmart(true)
}

// Name is the name this factory was registered under.
func (f *IkTokenizerFactory) Name() string { return f.name }

// SetSmart selects the segmentation mode and returns the receiver.
func (f *IkTokenizerFactory) SetSmart(smart bool) *IkTokenizerFactory {
	f.configuration.SetUseSmart(smart)
	return f
}

// Create builds a tokenizer.
func (f *IkTokenizerFactory) Create() *lucene.IKTokenizer {
	return lucene.NewIKTokenizer(f.configuration)
}
