package testutil

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/infinilabs/analysis-ik/cfg"
)

// The helper resolves the project's config directory from the package the test
// is running in, which is one level below the repository root — the same
// relationship the original relied on when its build ran the suite from the
// core module.
func TestFakeConfigurationResolvesTheProjectConfigDirectory(t *testing.T) {
	configuration := &fakeConfigurationSub{Settings: cfg.NewSettings()}

	confDir := configuration.ConfDir()
	if !strings.HasSuffix(confDir, string(filepath.Separator)+"config") {
		t.Errorf("ConfDir = %q, want it to end in a config directory", confDir)
	}
	if got := configuration.ConfigInPluginDir(); got != confDir {
		t.Errorf("ConfigInPluginDir = %q, want the same directory as ConfDir (%q)", got, confDir)
	}
	if got, want := configuration.Path("a", "b"), filepath.Join("a", "b"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestFakeConfigurationSetters(t *testing.T) {
	configuration := &fakeConfigurationSub{Settings: cfg.NewSettings()}

	if !configuration.SetUseSmart(true).UseSmart() {
		t.Error("SetUseSmart should return a configuration carrying the new value")
	}
	if configuration.SetEnableLowercase(false).EnableLowercase() {
		t.Error("SetEnableLowercase should return a configuration carrying the new value")
	}
	// The helper needs no permission hook.
	if _, ok := interface{}(configuration).(cfg.Checker); ok {
		t.Error("the helper should not implement Checker")
	}
}
