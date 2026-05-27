package config

import (
	"path/filepath"

	"github.com/bizshuk/gosdk/utils"
)

type configOptions struct {
	configPath   string
	defaultValue string
}

// ConfigOption defines the functional option type for config loading.
type ConfigOption func(*configOptions)

// WithConfigPath sets a custom configuration directory.
func WithConfigPath(path string) ConfigOption {
	return func(o *configOptions) {
		o.configPath = path
	}
}

// WithDefaultValue specifies a default JSON configuration string to write
// if the config file does not exist. This only applies to jsonConfig (settings.json).
func WithDefaultValue(defaultValue string) ConfigOption {
	return func(o *configOptions) {
		o.defaultValue = defaultValue
	}
}

// applyOptions processes the functional options.
func applyOptions(opts ...ConfigOption) *configOptions {
	o := &configOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// If WithConfigPath option is provided, expand home symbol
	if o.configPath != "" {
		o.configPath = ExpandHome(o.configPath)
	}

	// If both configPath and defaultValue are set, ensure settings.json exists at that path
	if o.configPath != "" && o.defaultValue != "" {
		jsonPath := filepath.Join(o.configPath, "settings.json")
		_ = utils.CreateIfNotExist(jsonPath, o.defaultValue)
	}

	return o
}
