package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type JsonConfig struct {
	configName string
}

func NewJsonConfig(configName string) Config {
	return &JsonConfig{
		configName: configName,
	}
}

// Load reads the yaml config file and returns a viper instance.
func (c *JsonConfig) Load() *viper.Viper {
	v := viper.New()
	v.SetConfigName(c.GetConfigName())
	v.SetConfigType("json")
	v.AddConfigPath(".")
	v.AddConfigPath("conf")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Yaml Config file not found. Using defaults and env variables.")
		} else { // 如果是其他讀取錯誤，則終止程式
			log.Fatalf("Fatal error reading config file: %s \n", err)
		}
	}

	fmt.Println("JsonConfig used:", v.ConfigFileUsed())
	return v
}

func (c *JsonConfig) GetConfigName() string {
	if c.configName != "" {
		return c.configName
	}

	profile := GetProfile()
	return "config." + profile
}
