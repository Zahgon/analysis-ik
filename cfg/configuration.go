// Package cfg defines the settings and directory lookups the analyzer needs
// from whatever host it is embedded in.
package cfg

// Configuration is what the original declared as an abstract class: three
// switches with fixed defaults, plus three lookups only the host can answer.
type Configuration interface {
	// ConfDir is the directory holding IKAnalyzer.cfg.xml and the dictionaries.
	ConfDir() string
	// ConfigInPluginDir is the fallback config directory shipped with the plugin.
	ConfigInPluginDir() string
	// Path joins path elements the way the host's file system does.
	Path(first string, more ...string) string
	// UseSmart reports whether ambiguity arbitration runs (ik_smart).
	UseSmart() bool
	// SetUseSmart selects the segmentation mode and returns the receiver.
	SetUseSmart(useSmart bool) Configuration
	// EnableRemoteDict reports whether remote dictionaries are polled.
	EnableRemoteDict() bool
	// EnableLowercase reports whether A-Z is folded to a-z while segmenting.
	EnableLowercase() bool
	// SetEnableLowercase selects case folding and returns the receiver.
	SetEnableLowercase(enableLowercase bool) Configuration
}

// Checker is implemented by a Configuration whose host wants to assert its own
// permissions before the analyzer performs I/O on its behalf. The original
// declared this as a method with an empty default body, which an abstract class
// can inherit and Go cannot; an optional interface is the equivalent, and a
// configuration that does not want the hook simply omits it.
type Checker interface {
	// Check runs before the analyzer reaches the network on the host's behalf.
	Check()
}

// Check invokes configuration's permission hook when it has one.
func Check(configuration Configuration) {
	if checker, ok := configuration.(Checker); ok {
		checker.Check()
	}
}

// Settings holds the three flags every Configuration shares. Embed it in a
// concrete configuration and supply the directory lookups.
type Settings struct {
	useSmart         bool
	enableRemoteDict bool
	enableLowercase  bool
}

// NewSettings returns the defaults: no smart segmentation, no remote
// dictionaries, case folding on.
func NewSettings() Settings { return Settings{enableLowercase: true} }

// UseSmart reports whether ambiguity arbitration runs.
func (s *Settings) UseSmart() bool { return s.useSmart }

// SetUseSmartFlag records the segmentation mode. Concrete configurations wrap
// it so the setter can return the Configuration itself.
func (s *Settings) SetUseSmartFlag(useSmart bool) { s.useSmart = useSmart }

// EnableRemoteDict reports whether remote dictionaries are polled.
func (s *Settings) EnableRemoteDict() bool { return s.enableRemoteDict }

// SetEnableRemoteDictFlag records whether remote dictionaries are polled.
func (s *Settings) SetEnableRemoteDictFlag(enableRemoteDict bool) {
	s.enableRemoteDict = enableRemoteDict
}

// EnableLowercase reports whether A-Z is folded to a-z while segmenting.
func (s *Settings) EnableLowercase() bool { return s.enableLowercase }

// SetEnableLowercaseFlag records the case-folding choice.
func (s *Settings) SetEnableLowercaseFlag(enableLowercase bool) {
	s.enableLowercase = enableLowercase
}
