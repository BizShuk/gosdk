package config

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func LoadViperConfig() {
	viper.SetConfigType("dotenv")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	// if err := viper.ReadInConfig(); err == nil {
	// 	log.Info("Using config file:", viper.ConfigFileUsed())
	// } else {
	// 	log.Fatal(err)
	// }

	// Add extra config from env, `.env.local` should not commit in git
	// .env.local .env.<idc> .env.<region> .env.<geo> [.env.dev|.env.stage|env.prod] .env
	profiles := []string{}
	profiles = append(profiles, "", GetProfile())

	configFiles := ConfigFileName(profiles)

	LoadEnvConfig(configFiles)
	LoadViperProfile() // profile in .env.<profile>
}

func LoadViperProfile() string {
	profile := viper.GetString("profile")
	log.Info("LoadViperProfile:", profile)
	return profile
}

// Load more config from `profile` in .env and manually added
func ConfigFileName(envs []string) []string {
	configFileNames := []string{}
	for _, env := range envs {
		if env == "" {
			env = ".env"
		} else {
			env = ".env." + env
		}

		configFileNames = append(configFileNames, env)
	}
	return configFileNames
}

func LoadEnvConfig(filenames []string) {
	log.Info("Config files loaded (default with .env):", filenames)
	for _, filename := range filenames {

		viper.SetConfigName(filename)
		_ = viper.MergeInConfig() // higher priority if load later
	}
}
