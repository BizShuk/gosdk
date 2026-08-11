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

// forcedConfigDir is the directory installed by WithConfigDir. Empty means
// "derive from appName", which is the default behaviour.
var forcedConfigDir string

// ExpandHome 將路徑中的 "~" 或 "~/..." 展開為使用者家目錄。
// 如果路徑不以 "~" 開頭，則原樣返回。
func ExpandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
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

	// WithConfigDir is installed before anything reads GetAppConfigDir(): the
	// loaders below take it as a search path, and cmd/config resolves every
	// write target through it. Calling Default() without the option clears a
	// previous override — Default() decides the config location outright, so a
	// forced directory surviving a call that did not ask for one would be a
	// setting no call site admits to.
	SetConfigDir(o.configDir)

	if o.appConfigDir != "" {
		viper.Set("APP_CONFIG_DIR", o.appConfigDir)
	}

	slog.Debug(
		"Load Configure...",
		"APP_CONFIG_DIR", GetAppConfigDir(),
	)

	// --- Load & merge all configs ---
	err := loadAllConfigs()
	if err != nil {
		slog.Debug("config load warning", "err", err)
	}

	// --- 4. 環境變數設定 (Environment Variables) ---
	// 啟用 AutomaticEnv + bindAllEnvVars 後，每次 viper.Get*() 都會動態查 OS 環境變數。
	//
	// ⚠️ viper 行為與機制：
	//   1. 對 flat key（含底線）：env 名稱 = UPPER(key)
	//      viper.Get("app_name")  → 查 APP_NAME
	//      viper.Get("log_level") → 查 LOG_LEVEL
	//   2. 對 nested key（如 "server.port"）：AutomaticEnv 預設不生效。
	//      bindAllEnvVars() 透過 reflection 走完所有 leaf 並呼叫 BindEnv，
	//      讓 nested key 也能被 SERVER_PORT 等 OS env 覆寫。
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
//
//	config.yaml < settings.json < .env.vault < .env （YAML 最低、ENV 最高）
//
// .env.vault 排在 .env 之下：vault 是`可提交`的檔案，代表整個團隊共用的值，
// 而 .env 是開發者自己機器上的覆寫。已提交的設定不應該蓋掉未提交的本機設定。
//
// 供 Default() 首次載入與 watcher reload 共用。
func loadAllConfigs() error {
	v1 := NewYamlConfig().Load()
	viper.MergeConfigMap(v1.AllSettings())
	v2 := NewJsonConfig().Load()
	viper.MergeConfigMap(v2.AllSettings())
	v3 := NewVaultConfig().Load()
	viper.MergeConfigMap(v3.AllSettings())
	v4 := NewEnvConfig().Load()
	viper.MergeConfigMap(v4.AllSettings())
	return nil
}

// bindAllEnvVars walks viper.AllSettings() recursively and calls viper.BindEnv
// for every leaf key. This works around viper's AutomaticEnv limitation where
// nested keys (e.g. "server.port") are NOT covered, even though flat keys are.
//
// Env name composition: UPPER(key with "." → "_")
//
//	"server.port"    → "SERVER_PORT"
//	"db.mysql.host"  → "DB_MYSQL_HOST"
//	"log_level"      → "LOG_LEVEL"
//
// Side effect: also re-binds flat top-level keys, guaranteeing coverage for
// both flat and nested keys.
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
	envName := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	_ = viper.BindEnv(key, envName)
}

// configBaseDir returns the directory that holds every application's config
// folder: XDG_CONFIG_HOME when set, otherwise ~/.config. Returns "" when the
// home directory cannot be determined and XDG_CONFIG_HOME is unset.
//
// This is the single place the base is decided — applyOptions resolves the
// same path when it seeds settings.json, and the two must not drift, or a
// machine with XDG_CONFIG_HOME set would write its seed file to one
// directory and read its config from another.
func configBaseDir() string {
	if base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); base != "" {
		return base
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config")
}

// appConfigDirFor returns the config directory for name, or "" when the name
// is empty or no base directory can be resolved.
func appConfigDirFor(name string) string {
	if name == "" {
		return ""
	}
	base := configBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, name)
}

// GetAppConfigDir returns the application config directory:
// <config-base>/appName, where <config-base> is XDG_CONFIG_HOME when set and
// ~/.config otherwise.
//
// WithConfigDir / SetConfigDir override that derivation wholesale, app name
// and environment included — the override is the single answer to "where does
// this application keep its config", which is why it is checked first.
//
// It returns "" when no app name has been registered (Default / SetAppName
// not called) and no directory was forced. Callers and tests rely on that
// empty value meaning "no app directory in the search path" — do not
// substitute a default here.
func GetAppConfigDir() string {
	if forcedConfigDir != "" {
		return forcedConfigDir
	}
	return appConfigDirFor(appName)
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

// GetConfigDir returns the directory forced by WithConfigDir / SetConfigDir,
// or "" when the config directory is still derived from the app name. Use
// GetAppConfigDir to ask where config actually lives; this one answers the
// narrower question of whether the location was overridden.
func GetConfigDir() string {
	return forcedConfigDir
}

// SetConfigDir forces the application config directory, the imperative
// counterpart of WithConfigDir. A leading "~" is expanded; an empty string
// clears the override and restores the appName-derived path.
func SetConfigDir(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		forcedConfigDir = ""
		return
	}
	forcedConfigDir = ExpandHome(dir)
}
