package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
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
	v.AddConfigPath(GetConfigDir())

	// Step 1: Load base .env
	v.SetConfigName(".env")
	v.SetConfigType("dotenv")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			zap.S().Warnf("Fatal error reading .env: %s", err)
		}
	}

	// Step 2: Merge .env.local (local overrides)
	v.SetConfigName(".env.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			zap.S().Warnf("Fatal error reading .env.local: %s", err)
		}
	}

	zap.S().Infof("EnvConfig used: %s", v.ConfigFileUsed())
	return v
}

// GetConfigName is no longer used but kept for interface compatibility
func (c EnvConfig) GetConfigName() string {
	return ""
}
