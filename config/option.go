package config

import (
	"strings"
)

type configOptions struct {
	defaultValue string
	appName      string
	configDir    string
	appConfigDir string
	watch        bool
}

// ConfigOption defines the functional option type for config loading.
type ConfigOption func(*configOptions)

// WithDefaultValue sets the lowest configuration layer: the JSON is merged
// under every config file on each Default() call, so a key missing from an
// older or empty settings.json still reads its shipped value. It never writes
// a file — copying the example into the config directory is the job of
// "config default" (cmd/config.MustRegisterDefault). Pass both the same
// embedded bytes:
//
//	//go:embed example/settings.example.json
//	var defaultSettings []byte
//
//	func init() { cfgcmd.MustRegisterDefault("settings.json", defaultSettings) }
//
//	config.Default(config.WithAppName("myapp"), config.WithDefaultValue(string(defaultSettings)))
func WithDefaultValue(defaultValue string) ConfigOption {
	return func(o *configOptions) {
		o.defaultValue = defaultValue
	}
}

// WithAppName overrides the configuration directory to os.UserConfigDir() + appName.
func WithAppName(appName string) ConfigOption {
	return func(o *configOptions) {
		o.appName = appName
	}
}

// WithConfigDir forces the application config directory to dir, overriding the
// path WithAppName derives (<XDG_CONFIG_HOME or ~/.config>/<appName>). A
// leading "~" is expanded to the user's home directory.
//
//	config.Default(
//		config.WithAppName("myapp"),
//		config.WithConfigDir("~/.config/myapp"),  // home, whatever XDG says
//	)
//
// The app name is untouched: GetAppName() keeps reporting it, so log lines and
// anything else keyed by name stay the same — only the directory moves. Use
// this when the application must own its config location rather than inherit
// it from the environment: a machine that exports XDG_CONFIG_HOME for some
// other tool would otherwise silently relocate this application's settings.
//
// Everything derived from the config directory follows: GetAppDataDir,
// GetAppLogsDir, the loader search path, and the settings.json seeded by
// WithDefaultValue.
func WithConfigDir(dir string) ConfigOption {
	return func(o *configOptions) {
		o.configDir = dir
	}
}

// applyOptions processes the functional options.
func applyOptions(opts ...ConfigOption) *configOptions {
	o := &configOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// If WithAppName is provided, compute the user-specific appConfigDir.
	// Resolution goes through appConfigDirFor so the seed file written below
	// always lands in the directory GetAppConfigDir() later reads from —
	// on a machine with XDG_CONFIG_HOME set, computing the base twice would
	// mean seeding one directory and loading from another.
	if dir := appConfigDirFor(o.appName); dir != "" {
		o.appConfigDir = dir
	}

	// WithConfigDir wins over the derived path — that is the whole point of
	// the option. It is applied here, before the seed is written, so the file
	// lands in the directory Default() then installs as GetAppConfigDir().
	if dir := strings.TrimSpace(o.configDir); dir != "" {
		o.appConfigDir = ExpandHome(dir)
	}

	return o
}
