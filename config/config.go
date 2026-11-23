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

// LAYER_DEFAULT names the WithDefaultValue layer: the lowest one, read from
// the binary rather than from a file.
const LAYER_DEFAULT = "default"

// defaultValues is the parsed WithDefaultValue JSON of the last Default()
// call, nil when none was given.
var defaultValues map[string]any

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

	// The seed JSON is also the lowest layer on every start, not only the
	// content of a freshly created settings.json: a key added to the seed after
	// the file was written still has a value, and every file layer overrides it.
	defaultValues = nil
	if o.defaultValue != "" {
		defaults, err := ParseJSON([]byte(o.defaultValue))
		if err != nil {
			slog.Warn("invalid default config", "app", appName, "err", err, "config_dir", GetAppConfigDir())
		} else {
			defaultValues = defaults
			viper.MergeConfigMap(defaults)
		}
	}

	// --- Load & merge all configs ---
	err := loadAllConfigs()
	if err != nil {
		slog.Debug("config load warning", "err", err)
	}

	// --- OS 環境變數 (Environment Variables) ---
	// 每次 viper.Get*() 才查 OS env（不把整份 environ 載入 viper）。
	// viper 的查找順序是 override > env > config > default：設定檔放在 override
	// 層，WithDefaultValue 放在 config 層，所以 env 只補上檔案沒有的 key。
	//   1. flat key：env 名稱 = UPPER(key)，log_level → LOG_LEVEL
	//   2. nested key：AutomaticEnv 不涵蓋，bindAllEnvVars() 對每個 leaf
	//      BindEnv，server.port → SERVER_PORT
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

// loadAllConfigs 載入所有設定檔並寫入全域 viper 的 override 層。
//
// 整體優先權，低到高（後者覆蓋前者）：
//
//	WithDefaultValue < OS env < config.yaml < settings.json < .env.vault < .env
//
// 每種格式內 base 檔先載、.local 檔後載，所以 .env.local 是最高的一層。
//
// 設定檔寫進 override 層（viper.Set）而非 config 層，是讓檔案勝過 OS env 的
// 唯一方式：viper 查值固定 override > env > config > default，而 env 必須維持
// 讀值時才查——把整份 environ 載入 viper 會讓 AllSettings() 帶出每一個環境
// 變數，包括 shell 裡的 secret。代價：之後對同一個 key 呼叫 MergeConfigMap /
// SetDefault 都蓋不過設定檔，只有 viper.Set 可以。
//
// .env.vault 排在 .env 之下：vault 是`可提交`的檔案，代表整個團隊共用的值，
// 而 .env 是開發者自己機器上的覆寫。
//
// 供 Default() 首次載入與 watcher reload 共用。
func loadAllConfigs() error {
	for k, v := range fileSettings() {
		viper.Set(k, v)
	}
	return nil
}

// fileSettings 依優先權合併所有設定檔（yaml → json → vault → env）。
func fileSettings() map[string]any {
	files := viper.New()
	files.MergeConfigMap(NewYamlConfig().Load().AllSettings())
	files.MergeConfigMap(NewJsonConfig().Load().AllSettings())
	files.MergeConfigMap(NewVaultConfig().Load().AllSettings())
	files.MergeConfigMap(NewEnvConfig().Load().AllSettings())
	return files.AllSettings()
}

// bindAllEnvVars walks viper.AllSettings() recursively and calls viper.BindEnv
// for every leaf key. This works around viper's AutomaticEnv limitation where
// nested keys (e.g. "server.port") are NOT covered, even though flat keys are.
// A binding only matters for keys no config file sets: those sit in the
// override layer, which viper checks before env.
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
	_ = viper.BindEnv(key, EnvName(key))
}

// EnvName 回傳 viper 為 key 查找的 OS 環境變數名稱：UPPER(key)，"." 換成 "_"，無前綴。
//
//	"log_level"     → "LOG_LEVEL"
//	"server.port"   → "SERVER_PORT"
//	"db.mysql.host" → "DB_MYSQL_HOST"
func EnvName(key string) string {
	return strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
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

// DefaultValues returns the settings installed by WithDefaultValue in the last
// Default() call, keys normalized the way viper reads them; nil when there were
// none. The map is a fresh copy, safe to modify.
func DefaultValues() map[string]any {
	if defaultValues == nil {
		return nil
	}
	v := viper.New()
	if err := v.MergeConfigMap(defaultValues); err != nil {
		return nil
	}
	return v.AllSettings()
}
