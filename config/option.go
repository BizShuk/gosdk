package config

import (
	"os"
	"path/filepath"

	"github.com/bizshuk/gosdk/utils"
)

type configOptions struct {
	defaultValue string
	appName      string
	appConfigDir string
}

// ConfigOption defines the functional option type for config loading.
type ConfigOption func(*configOptions)

// WithDefaultValue specifies a default JSON configuration string to write
// if the config file does not exist. This only applies to jsonConfig (settings.json).
//
// How to set up: Provide a valid JSON string literal. Example for a settings.json
// with "host" and "port" fields:
//
//   // file locaation: config/default_settings.json
//   ```json
//   {}
//   ```
//
//   //go:embed default_settings.json
//   var defaultConfigJSON string
//
//	 config.Default(
//		  WithAppName("myapp"),
//		  WithDefaultValue(defaultConfigJSON),
//	 )
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

// applyOptions processes the functional options.
func applyOptions(opts ...ConfigOption) *configOptions {
	o := &configOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// If WithAppName is provided, compute user-specific appConfigDir
	// Use ${HOME}/.config as base (cross-platform consistent path)
	if o.appName != "" {
		if homeDir, err := os.UserHomeDir(); err == nil {
			o.appConfigDir = filepath.Join(homeDir, ".config", o.appName)
		}
	}

	// Only automatically create settings.json if it is using the appName config directory
	if o.appConfigDir != "" && o.defaultValue != "" {
		jsonPath := filepath.Join(o.appConfigDir, "settings.json")
		_ = utils.CreateIfNotExist(jsonPath, o.defaultValue)
	}

	return o
}
