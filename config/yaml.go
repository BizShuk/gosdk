package config

import (
	"github.com/bizshuk/gosdk/log"
	"github.com/spf13/viper"
)

type YamlConfig struct{}

func NewYamlConfig() Config {
	return &YamlConfig{}
}

// Load reads config.yaml and merges config.local.yaml
func (c *YamlConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetConfigDir())

	// Step 1: Load base config.yaml
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info("config.yaml not found. Using defaults and env variables.")
		} else {
			log.Fatalf("Fatal error reading config.yaml: %s", err)
		}
	}

	// Step 2: Merge config.local.yaml (local overrides)
	v.SetConfigName("config.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info("config.local.yaml not found. Skipping local overrides.")
		} else {
			log.Fatalf("Fatal error reading config.local.yaml: %s", err)
		}
	}

	log.Infof("YamlConfig used: %s", v.ConfigFileUsed())
	return v
}

// GetConfigName is no longer used but kept for interface compatibility
func (c *YamlConfig) GetConfigName() string {
	return ""
}
