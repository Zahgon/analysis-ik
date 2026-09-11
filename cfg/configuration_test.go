package cfg

import "testing"

func TestNewSettingsDefaults(t *testing.T) {
	s := NewSettings()
	if s.UseSmart() {
		t.Error("useSmart should default to false")
	}
	if s.EnableRemoteDict() {
		t.Error("enableRemoteDict should default to false")
	}
	if !s.EnableLowercase() {
		t.Error("enableLowercase should default to true")
	}
}

// The permission hook is optional: a configuration that wants one implements
// Checker, and Check leaves every other configuration alone.
func TestCheckRunsOnlyForAConfigurationThatWantsIt(t *testing.T) {
	plain := &stubConfiguration{Settings: NewSettings()}
	Check(plain)

	hooked := &checkingConfiguration{stubConfiguration: stubConfiguration{Settings: NewSettings()}}
	Check(hooked)
	Check(hooked)
	if hooked.checked != 2 {
		t.Errorf("the hook ran %d times, want 2", hooked.checked)
	}

	var _ Checker = hooked
}

type checkingConfiguration struct {
	stubConfiguration
	checked int
}

func (c *checkingConfiguration) Check() { c.checked++ }

func TestSettingsFlags(t *testing.T) {
	s := NewSettings()
	s.SetUseSmartFlag(true)
	s.SetEnableRemoteDictFlag(true)
	s.SetEnableLowercaseFlag(false)

	if !s.UseSmart() || !s.EnableRemoteDict() || s.EnableLowercase() {
		t.Errorf("flags not applied: smart=%t remote=%t lowercase=%t",
			s.UseSmart(), s.EnableRemoteDict(), s.EnableLowercase())
	}
}

// A concrete configuration embeds Settings and supplies the directories, which
// is how the plugin and the test helper both build one.
func TestSettingsSatisfiesAConcreteConfiguration(t *testing.T) {
	var configuration Configuration = &stubConfiguration{Settings: NewSettings()}
	if configuration.ConfDir() != "conf" || configuration.ConfigInPluginDir() != "plugin" {
		t.Error("the concrete configuration supplies its own directories")
	}
	if got := configuration.Path("a", "b"); got != "a/b" {
		t.Errorf("Path = %q, want a/b", got)
	}
	if !configuration.SetUseSmart(true).UseSmart() {
		t.Error("SetUseSmart should return a configuration carrying the new value")
	}
	if configuration.SetEnableLowercase(false).EnableLowercase() {
		t.Error("SetEnableLowercase should return a configuration carrying the new value")
	}
}

type stubConfiguration struct {
	Settings
}

func (c *stubConfiguration) ConfDir() string           { return "conf" }
func (c *stubConfiguration) ConfigInPluginDir() string { return "plugin" }
func (c *stubConfiguration) Path(first string, more ...string) string {
	for _, part := range more {
		first += "/" + part
	}
	return first
}
func (c *stubConfiguration) SetUseSmart(useSmart bool) Configuration {
	c.SetUseSmartFlag(useSmart)
	return c
}
func (c *stubConfiguration) SetEnableLowercase(enableLowercase bool) Configuration {
	c.SetEnableLowercaseFlag(enableLowercase)
	return c
}
