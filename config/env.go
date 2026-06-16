package config

import (
	"log/slog"

	"github.com/spf13/viper"
)

func NewEnvConfig() Config {
	return EnvConfig{}
}

type EnvConfig struct{}

// Load reads .env and merges .env.local
func (c EnvConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetAppConfigDir())

	// Step 1: Load base .env
	v.SetConfigName(".env")
	v.SetConfigType("dotenv")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Debug("Fatal error reading .env", "err", err)
		}
	}

	// Step 2: Merge .env.local (local overrides)
	v.SetConfigName(".env.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Debug("Fatal error reading .env.local", "err", err)
		}
	}

	return v
}

// GetConfigName is no longer used but kept for interface compatibility
func (c EnvConfig) GetConfigName() string {
	return ""
}
