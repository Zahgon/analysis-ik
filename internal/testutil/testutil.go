// Package testutil builds the configuration the test suites segment against.
// It lives outside _test.go files because Go tests in one package cannot import
// helpers from another package's tests, and both the core and lucene suites
// need this one.
package testutil

import (
	"os"
	"path/filepath"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/dic"
)

// CreateFakeConfigurationSub returns a configuration pointing at the
// repository's own config directory, and initialises the dictionary from it.
func CreateFakeConfigurationSub(useSmart bool) cfg.Configuration {
	configurationSub := &fakeConfigurationSub{Settings: cfg.NewSettings()}
	configurationSub.SetUseSmartFlag(useSmart)
	dic.Initial(configurationSub)
	return configurationSub
}

// fakeConfigurationSub stands in for the plugin's configuration. A plugin would
// point at the search engine's config directory; pointing at the project's own
// config directory instead keeps the tests independent of any installation.
type fakeConfigurationSub struct {
	cfg.Settings
}

func (c *fakeConfigurationSub) ConfDir() string { return configDir() }

func (c *fakeConfigurationSub) ConfigInPluginDir() string { return configDir() }

func (c *fakeConfigurationSub) Path(first string, more ...string) string {
	return filepath.Join(append([]string{first}, more...)...)
}

func (c *fakeConfigurationSub) SetUseSmart(useSmart bool) cfg.Configuration {
	c.SetUseSmartFlag(useSmart)
	return c
}

func (c *fakeConfigurationSub) SetEnableLowercase(enableLowercase bool) cfg.Configuration {
	c.SetEnableLowercaseFlag(enableLowercase)
	return c
}

// configDirName is the directory the project keeps its dictionaries in.
const configDirName = "config"

// configDir resolves the project's config directory relative to the package the
// test is running in, which is one level below the repository root.
func configDir() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return configDirName
	}
	return filepath.Join(filepath.Dir(workingDir), configDirName)
}
