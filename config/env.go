package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

func NewEnvConfig() Config {
	return EnvConfig{}
}

type EnvConfig struct{}

// Add extra config from env, `.env.local` should not commit in git
// .env.local .env.<idc> .env.<region> .env.<geo> [.env.dev|.env.stage|env.prod] .env
func (c EnvConfig) Load() *viper.Viper {
	v := viper.New()
	v.SetConfigType("dotenv")
	v.AddConfigPath(".")
	v.AddConfigPath("conf")

	v.SetConfigFile(".env")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Config file not found. Using defaults and env variables.")
		} else { // 如果是其他讀取錯誤，則終止程式
			log.Fatalf("Fatal error reading config file: %s \n", err)
		}
	}

	v.SetConfigFile(c.GetConfigName())
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Config file not found. Using defaults and env variables.")
		} else { // 如果是其他讀取錯誤，則終止程式
			log.Fatalf("Fatal error reading config file: %s \n", err)
		}
	}

	fmt.Println(v.ConfigFileUsed())
	return v
}

func (c EnvConfig) GetConfigName() string {
	return ".env." + GetProfile()
}
