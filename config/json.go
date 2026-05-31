package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type JsonConfig struct{}

func NewJsonConfig() Config {
	return &JsonConfig{}
}

// Load reads settings.json and merges settings.local.json
func (c *JsonConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetAppConfigDir())
	v.AddConfigPath(GetConfigDir())

	// Step 1: Load base settings.json
	v.SetConfigName("settings")
	v.SetConfigType("json")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			zap.S().Warnf("Fatal error reading settings.json: %s", err)
		}
	}

	// Step 2: Merge settings.local.json (local overrides)
	v.SetConfigName("settings.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			zap.S().Warnf("Fatal error reading settings.local.json: %s", err)
		}
	}

	zap.S().Infof("JsonConfig used: %s", v.ConfigFileUsed())
	return v
}

func (c *JsonConfig) GetConfigName() string {
	return "settings"
}
