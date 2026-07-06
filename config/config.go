package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config interface {
	Load() *viper.Viper
	GetConfigName() string
}

var appName string

// ExpandHome 將路徑中的 "~" 或 "~/..." 展開為使用者家目錄。
// 如果路徑不以 "~" 開頭，則原樣返回。
func ExpandHome(path string) string {
	if path == "~" || (len(path) > 2 && path[:2] == "~/") {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

// Default 載入預設設定檔。
//
// 設定檔搜尋目錄固定為：目前工作目錄 (.)、./conf，以及應用程式專屬目錄
// ~/.config/<appName>（需透過 config.Default(config.WithAppName("myapp")) 啟用，
// 並可透過 GetAppConfigDir() 取得路徑）。
func Default(opts ...ConfigOption) {
	o := applyOptions(opts...)

	if o.appName != "" {
		appName = o.appName
	}

	if o.appConfigDir != "" {
		viper.Set("APP_CONFIG_DIR", o.appConfigDir)
	}

	slog.Debug("Load Configure...",
		"APP_CONFIG_DIR", GetAppConfigDir(),
	)

	// --- Load & merge all configs ---
	err := loadAllConfigs()
	if err != nil {
		slog.Debug("config load warning", "err", err)
	}

	// --- 4. 環境變數設定 (Environment Variables) ---
	// 讓 Viper 知道要自動尋找以 APP 開頭的環境變數
	// 例如：環境變數 APP_SERVER_PORT 會自動對應到配置鍵 server.port
	viper.SetEnvPrefix("APP")
	// 啟用環境變數的綁定 環境變數中的底線 '_' 會被視為點號 '.'
	viper.AutomaticEnv()

	// 步驟 1: 匯出所有設定 (AllSettings)
	settings := viper.AllSettings()

	// 步驟 2 & 3: 轉換成 Inline JSON 字串
	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		fmt.Printf("轉換 JSON 失敗: %v\n", err)
		return
	}

	// 輸出結果
	inlineJSON := string(jsonBytes)
	slog.Debug("Config Loaded", "settings", inlineJSON)

	// --- Start file watcher if requested ---
	if o.watch {
		if err := StartWatch(); err != nil {
			slog.Warn("failed to start config watcher", "err", err)
		}
	}
}

// loadAllConfigs 載入所有設定格式並合併至全域 viper。
// 供 Default() 首次載入與 watcher reload 共用。
func loadAllConfigs() error {
	v1 := NewEnvConfig().Load()
	viper.MergeConfigMap(v1.AllSettings())
	v2 := NewYamlConfig().Load()
	viper.MergeConfigMap(v2.AllSettings())
	v3 := NewJsonConfig().Load()
	viper.MergeConfigMap(v3.AllSettings())
	return nil
}

func GetAppConfigDir() string {
	if appName == "" {
		return ""
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", appName)
}

func GetAppName() string {
	return appName
}

// GetAppLogDir returns the application log directory: ~/.config/appName/log
func GetAppLogDir() string {
	return filepath.Join(GetAppConfigDir(), "log")
}

// GetAppDataDir returns the application data directory: ~/.config/appName/data
func GetAppDataDir() string {
	return filepath.Join(GetAppConfigDir(), "data")
}

func SetAppName(name string) {
	appName = name
}
