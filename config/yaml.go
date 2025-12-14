package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type YamlConfig struct{}

func NewYamlConfig() Config {
	return &YamlConfig{}
}

// Load reads the yaml config file and returns a viper instance.
func (y *YamlConfig) Load() *viper.Viper {
	v := viper.New()
	v.SetConfigName(y.GetConfigName())
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("conf")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Yaml Config file not found. Using defaults and env variables.")
		} else { // 如果是其他讀取錯誤，則終止程式
			log.Fatalf("Fatal error reading config file: %s \n", err)
		}
	}

	fmt.Println(v.ConfigFileUsed())
	return v
}

func (y *YamlConfig) GetConfigName() string {
	profile := GetProfile()
	return "config-" + profile
}
