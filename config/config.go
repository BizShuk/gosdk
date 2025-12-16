package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config interface {
	Load() *viper.Viper
	GetConfigName() string
}

func Default() {
	viper.BindEnv("CONFIG_DIR", "CONFIG_DIR")
	viper.BindEnv("PROFILE", "PROFILE")
	viper.SetDefault("CONFIG_DIR", ".")
	viper.SetDefault("PROFILE", "local")

	v1 := NewEnvConfig().Load()
	viper.MergeConfigMap(v1.AllSettings())
	v2 := NewYamlConfig().Load()
	viper.MergeConfigMap(v2.AllSettings())

	zap.L().Info("Load Configure...",
		zap.String("CONFIG_DIR", GetConfigDir()),
		zap.String("CONFIG_DIR", "."),
		zap.String("CONFIG_DIR", "conf"),
	)

	// --- 4. 環境變數設定 (Environment Variables) ---
	// 讓 Viper 知道要自動尋找以 APP 開頭的環境變數
	// 例如：環境變數 APP_SERVER_PORT 會自動對應到配置鍵 server.port
	viper.SetEnvPrefix("APP")
	// 啟用環境變數的綁定 環境變數中的底線 '_' 會被視為點號 '.'
	viper.AutomaticEnv()
}

func GetProfile() string {
	profile := viper.GetString("PROFILE")
	if profile != "" {
		return profile
	}
	return "local"
}

func GetConfigDir() string {
	return viper.GetString("CONFIG_DIR")
}
