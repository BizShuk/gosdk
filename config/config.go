package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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
	// 啟用 AutomaticEnv + bindAllEnvVars 後，每次 viper.Get*() 都會動態查 APP_* 環境變數。
	//
	// ⚠️ viper 行為限制（必須知道）：
	//   1. 對 flat key（含底線）：env 名稱 = prefix + "_" + UPPER(key)
	//      viper.Get("app_name")  → 查 APP_APP_NAME（prefix APP + key APP_NAME）
	//      viper.Get("log_level") → 查 APP_LOG_LEVEL
	//      （注意：prefix 會跟已有底線的 key 疊加，不是去重）
	//   2. 對 nested key（如 "server.port"）：AutomaticEnv 預設不生效。
	//      bindAllEnvVars() 透過 reflection 走完所有 leaf 並呼叫 BindEnv，
	//      讓 nested key 也能被 APP_SERVER_PORT 等 OS env 覆寫。
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()
	bindAllEnvVars()

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
// 合併順序決定跨格式優先權（後者覆蓋前者）：
//   .env < config.yaml < settings.json （YAML 最低、ENV 最高）
//
// 供 Default() 首次載入與 watcher reload 共用。
func loadAllConfigs() error {
	v1 := NewYamlConfig().Load()
	viper.MergeConfigMap(v1.AllSettings())
	v2 := NewJsonConfig().Load()
	viper.MergeConfigMap(v2.AllSettings())
	v3 := NewEnvConfig().Load()
	viper.MergeConfigMap(v3.AllSettings())
	return nil
}

// bindAllEnvVars walks viper.AllSettings() recursively and calls viper.BindEnv
// for every leaf key. This works around viper's AutomaticEnv limitation where
// nested keys (e.g. "server.port") are NOT covered, even though flat keys are.
//
// Env name composition: "APP_" + UPPER(key with "." → "_")
//
//	"server.port"    → "APP_SERVER_PORT"
//	"db.mysql.host"  → "APP_DB_MYSQL_HOST"
//	"log_level"      → "APP_LOG_LEVEL"（與 AutomaticEnv 對 flat key 組出的名稱一致）
//
// Side effect: also re-binds flat top-level keys, which is harmless —
// BindEnv("app_name", "APP_APP_NAME") matches what AutomaticEnv would compute,
// so we don't change semantics, just guarantee coverage for nested keys.
func bindAllEnvVars() {
	for key, val := range viper.AllSettings() {
		bindNestedEnv(key, val)
	}
}

func bindNestedEnv(key string, val any) {
	if m, ok := val.(map[string]any); ok {
		for k, v := range m {
			bindNestedEnv(key+"."+k, v)
		}
		return
	}
	envName := "APP_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	_ = viper.BindEnv(key, envName)
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
func GetAppLogsDir() string {
	return filepath.Join(GetAppConfigDir(), "logs")
}

// GetAppDataDir returns the application data directory: ~/.config/appName/data
func GetAppDataDir() string {
	return filepath.Join(GetAppConfigDir(), "data")
}

func SetAppName(name string) {
	appName = name
}
