package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DBConnConfig struct {
	Driver string `mapstructure:"driver"`
	URL    string `mapstructure:"url"`
}

type ConfigSchema struct {
	Name     string                  `mapstructure:"name"`
	Version  string                  `mapstructure:"version"`
	Profile  string                  `mapstructure:"profile"`
	LogLevel string                  `mapstructure:"log_level"`
	Server   ServerConfig            `mapstructure:"server"`
	DB       map[string]DBConnConfig `mapstructure:"db"`
}

var GlobalConfig ConfigSchema

type Config interface {
	Load() *viper.Viper
	GetConfigName() string
}

// Default 載入預設設定檔。
//
// 備註 (Method 1)：應用程式層 (application layer) 可以在呼叫 Default() 之前，
// 透過設定管理工具 (configuration management tool, Viper) 直接指定設定檔目錄：
//
//	viper.Set("CONFIG_DIR", "/path/to/config")
func Default() {
	viper.BindEnv("CONFIG_DIR", "CONFIG_DIR")
	viper.BindEnv("PROFILE", "PROFILE")
	viper.SetDefault("CONFIG_DIR", ".")
	viper.SetDefault("PROFILE", "local")

	zap.L().Info("Load Configure...",
		zap.String("CONFIG_DIR", GetConfigDir()),
	)

	v1 := NewEnvConfig().Load()
	viper.MergeConfigMap(v1.AllSettings())
	v2 := NewYamlConfig().Load()
	viper.MergeConfigMap(v2.AllSettings())
	v3 := NewJsonConfig().Load()
	viper.MergeConfigMap(v3.AllSettings())

	// --- 4. 環境變數設定 (Environment Variables) ---
	// 讓 Viper 知道要自動尋找以 APP 開頭的環境變數
	// 例如：環境變數 APP_SERVER_PORT 會自動對應到配置鍵 server.port
	viper.SetEnvPrefix("APP")
	// 啟用環境變數的綁定 環境變數中的底線 '_' 會被視為點號 '.'
	viper.AutomaticEnv()

	// Unmarshal into strong-typed global configuration schema
	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		zap.L().Fatal("Failed to unmarshal configuration", zap.Error(err))
	}
}

// DefaultWithDir 允許自訂設定檔目錄，並載入設定。
//
// 備註 (Method 2)：應用程式層 (application layer) 可以直接呼叫此函式並傳入自訂目錄，
// 例如：config.DefaultWithDir("/path/to/config")
func DefaultWithDir(configDir string) {
	if configDir != "" {
		viper.Set("CONFIG_DIR", configDir)
	}
	Default()
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
