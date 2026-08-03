package config

import (
	"path/filepath"
	"strings"

	"github.com/bizshuk/gosdk/utils"
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

// WithDefaultValue specifies a default JSON configuration string to write
// if the config file does not exist. This only applies to jsonConfig (settings.json).
//
// How to set up: Provide a valid JSON string literal. Example for a settings.json
// with "host" and "port" fields:
//
//	  // file locaation: config/default_settings.json
//	  ```json
//	  {}
//	  ```
//
//	  //go:embed default_settings.json
//	  var defaultConfigJSON string
//
//		 config.Default(
//			  WithAppName("myapp"),
//			  WithDefaultValue(defaultConfigJSON),
//		 )
//
// When the settings.json file does not exist at appConfigDir, it will be created
// with this JSON content automatically.
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

	// Only automatically create settings.json if it is using the appName config directory
	if o.appConfigDir != "" && o.defaultValue != "" {
		jsonPath := filepath.Join(o.appConfigDir, "settings.json")
		_ = utils.CreateIfNotExist(jsonPath, o.defaultValue)
	}

	return o
}
