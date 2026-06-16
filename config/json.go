package config

import (
	"log/slog"

	"github.com/spf13/viper"
)

type JsonConfig struct{}

func NewJsonConfig() Config {
	return JsonConfig{}
}

// Load reads settings.json and merges settings.local.json
func (c JsonConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetAppConfigDir())

	// Step 1: Load base settings.json
	v.SetConfigName("settings")
	v.SetConfigType("json")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Debug("Fatal error reading settings.json", "err", err)
		}
	}

	// Step 2: Merge settings.local.json (local overrides)
	v.SetConfigName("settings.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Debug("Fatal error reading settings.local.json", "err", err)
		}
	}

	return v
}

func (c JsonConfig) GetConfigName() string {
	return "settings"
}
