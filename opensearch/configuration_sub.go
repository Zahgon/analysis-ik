package opensearch

import (
	"os"
	"path/filepath"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/dic"
)

// Environment locates the OpenSearch installation the plugin runs inside.
type Environment struct {
	// ConfigDir is the node's config directory.
	ConfigDir string
}

// Settings is an analyzer or tokenizer settings block.
type Settings map[string]string

// Get returns the setting, or defaultValue when it is absent.
func (s Settings) Get(key, defaultValue string) string {
	if value, ok := s[key]; ok {
		return value
	}
	return defaultValue
}

// GetAsBoolean returns the setting parsed as a boolean, or defaultValue when it
// is absent.
func (s Settings) GetAsBoolean(key string, defaultValue bool) bool {
	if value, ok := s[key]; ok {
		return value == "true"
	}
	return defaultValue
}

// ConfigurationSub is the Configuration an OpenSearch node supplies.
type ConfigurationSub struct {
	cfg.Settings

	environment Environment
}

// NewConfigurationSub reads the settings block and initialises the dictionary.
func NewConfigurationSub(env Environment, settings Settings) *ConfigurationSub {
	c := &ConfigurationSub{Settings: cfg.NewSettings(), environment: env}
	c.SetUseSmartFlag(settings.Get("use_smart", "false") == "true")
	c.SetEnableLowercaseFlag(settings.Get("enable_lowercase", "true") == "true")
	c.SetEnableRemoteDictFlag(settings.Get("enable_remote_dict", "true") == "true")

	dic.Initial(c)
	return c
}

// ConfDir is the plugin's directory inside the node's config directory.
func (c *ConfigurationSub) ConfDir() string {
	return filepath.Join(c.environment.ConfigDir, PluginName)
}

// ConfigInPluginDir is the config directory shipped beside the plugin itself.
func (c *ConfigurationSub) ConfigInPluginDir() string {
	executable, err := os.Executable()
	if err != nil {
		return filepath.Join("config")
	}
	absolute, err := filepath.Abs(filepath.Join(filepath.Dir(executable), "config"))
	if err != nil {
		return filepath.Join(filepath.Dir(executable), "config")
	}
	return absolute
}

// Path joins path elements.
func (c *ConfigurationSub) Path(first string, more ...string) string {
	return filepath.Join(append([]string{first}, more...)...)
}

// SetUseSmart selects the segmentation mode.
func (c *ConfigurationSub) SetUseSmart(useSmart bool) cfg.Configuration {
	c.SetUseSmartFlag(useSmart)
	return c
}

// SetEnableLowercase selects case folding.
func (c *ConfigurationSub) SetEnableLowercase(enableLowercase bool) cfg.Configuration {
	c.SetEnableLowercaseFlag(enableLowercase)
	return c
}
