package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// setupConfigDir 定義於 env_test.go（共用 helper）

func TestLoadAllConfigs_CrossFormatPriority(t *testing.T) {
	// 驗證跨格式優先權：.env > settings.json > config.yaml
	//（loadAllConfigs 內呼叫順序為 YAML → JSON → ENV，後者覆蓋前者）
	//
	// 注意：dotenv reader 不會把 SERVER_PORT 拆成 server.port（不像 AutomaticEnv 的 _ → .），
	// 所以這裡用三種格式都能表達的 flat key（app_name / app_debug）來驗證覆蓋優先權。
	// 若要驗證嵌套結構（server.port）的覆蓋，需在每個格式裡都明確指定 key。
	dir := t.TempDir()

	yamlContent := "app_name: yaml-app\napp_debug: false\napp_yaml_only: yaml-only\n"
	jsonContent := `{"app_name":"json-app","app_debug":true,"app_json_only":"json-only"}`
	envContent := "APP_NAME=env-app\nAPP_ENV_ONLY=env-only\n"

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to create settings.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to create .env: %v", err)
	}

	setupConfigDir(t, dir)

	// 重置 viper 並呼叫 loadAllConfigs（不走 Default() 以避免 side effects）
	viper.Reset()
	if err := loadAllConfigs(); err != nil {
		t.Fatalf("loadAllConfigs failed: %v", err)
	}

	// .env 最高優先：覆寫 app_name 為 env-app
	if got := viper.GetString("app_name"); got != "env-app" {
		t.Errorf("expected app_name=env-app (from .env, highest priority), got %s", got)
	}
	// JSON 蓋掉 YAML：app_debug=true（YAML 是 false）
	if got := viper.GetBool("app_debug"); !got {
		t.Errorf("expected app_debug=true (from settings.json, overrides YAML's false), got %v", got)
	}
	// YAML 是 base：app_yaml_only 只有 YAML 有，應保留
	if got := viper.GetString("app_yaml_only"); got != "yaml-only" {
		t.Errorf("expected app_yaml_only=yaml-only (from config.yaml, lowest priority but only source), got %s", got)
	}
	// JSON 補上 yaml 缺的：app_json_only
	if got := viper.GetString("app_json_only"); got != "json-only" {
		t.Errorf("expected app_json_only=json-only (from settings.json), got %s", got)
	}
	// .env 自己新增的 key：app_env_only
	if got := viper.GetString("app_env_only"); got != "env-only" {
		t.Errorf("expected app_env_only=env-only (from .env), got %s", got)
	}
}

func TestLoadAllConfigs_MissingFormatsContinues(t *testing.T) {
	// 三種格式都缺：不應 panic，正常載入空設定
	dir := t.TempDir()
	setupConfigDir(t, dir)

	viper.Reset()
	if err := loadAllConfigs(); err != nil {
		t.Fatalf("loadAllConfigs should not fail when all formats missing: %v", err)
	}

	if got := viper.AllSettings(); len(got) != 0 {
		t.Errorf("expected empty settings when no config files exist, got %v", got)
	}
}

func TestDefaultRuns(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	// Default() 應在無 option、無設定檔的情況下正常執行，不 panic。
	Default()
}

func TestDefault_EnvVarOverridesAllFiles(t *testing.T) {
	// 驗證 4 層優先權：OS env (APP_*) > .env > settings.json > config.yaml
	//
	// ⚠️ viper AutomaticEnv 行為限制（必須用 flat key 測，nested key 不適用）：
	//   1. 對 flat key：env 名稱 = prefix + "_" + UPPER(key)
	//      viper.Get("app_name") → 查 APP_APP_NAME（prefix "APP" + key "APP_NAME"）
	//      → 若 OS env 是 APP_NAME=... 則**不會**被讀到（要 APP_APP_NAME=...）
	//   2. 對 nested key（如 server.port）：AutomaticEnv 完全不生效，
	//      必須用 viper.BindEnv("server.port", "APP_SERVER_PORT") 明確綁定才會讀 OS env。
	//      → 反映在這個測試：我們只驗 flat key（app_name / app_debug），nested key 走檔案鏈。
	dir := t.TempDir()

	// config.yaml（最低）
	yamlContent := "app_name: from-yaml\napp_debug: false\nserver:\n  port: 7000\n  host: yaml-host\n"
	// settings.json（中）
	jsonContent := `{"app_name":"from-json","app_debug":true}`
	// .env（檔案層最高）
	envContent := "APP_NAME=from-env-file\n"

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to create settings.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to create .env: %v", err)
	}

	t.Chdir(dir)
	t.Cleanup(func() { viper.Reset() })

	// OS env 設最高優先權。注意：因為 key 是 app_name（flat with underscore），
	// viper 組出的 env 名稱是 APP_APP_NAME（prefix APP + key APP_NAME），不是 APP_NAME。
	t.Setenv("APP_APP_NAME", "from-os-env")

	Default()

	// 1. OS env 最高：覆寫 .env 的 from-env-file
	if got := viper.GetString("app_name"); got != "from-os-env" {
		t.Errorf("expected app_name=from-os-env (AutomaticEnv wins), got %s", got)
	}
	// 2. 沒設 OS env 的 key：仍走檔案優先權鏈（JSON > YAML）
	if got := viper.GetBool("app_debug"); !got {
		t.Errorf("expected app_debug=true (from settings.json, no OS env set), got %v", got)
	}
	// 3. 沒被任何檔案覆蓋的 YAML 鍵仍保留
	if got := viper.GetString("server.host"); got != "yaml-host" {
		t.Errorf("expected server.host=yaml-host (from config.yaml, no override), got %s", got)
	}
	// 4. 沒設 OS env 時，server.port 仍由 config.yaml 提供（7000）。
	//    bindAllEnvVars() 雖對 nested key 做 BindEnv，但沒 OS env 就不會覆寫。
	//    （見 TestDefault_NestedKeyEnvOverride 驗證 OS env 設了時能覆寫）
	if got := viper.GetInt("server.port"); got != 7000 {
		t.Errorf("expected server.port=7000 (no OS env set, config.yaml wins), got %d", got)
	}
}

// TestDefault_NestedKeyEnvOverride 驗證 bindAllEnvVars() 透過 reflection BindEnv
// 所有 leaf 後，OS env 能覆寫 nested key（如 APP_SERVER_PORT → server.port）。
// 這是對 viper AutomaticEnv 不覆蓋 nested key 的 workaround。
func TestDefault_NestedKeyEnvOverride(t *testing.T) {
	dir := t.TempDir()

	yamlContent := "server:\n  port: 7000\n  host: yaml-host\n  db:\n    mysql:\n      host: yaml-db-host\n      port: 3306\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	t.Chdir(dir)
	t.Cleanup(func() { viper.Reset() })

	// 三個不同深度的 nested key，驗證 reflection 確實走完整棵樹
	t.Setenv("APP_SERVER_PORT", "9090")             // depth 2
	t.Setenv("APP_SERVER_HOST", "env-host")         // depth 2
	t.Setenv("APP_SERVER_DB_MYSQL_HOST", "env-db")  // depth 4

	Default()

	// depth-2 nested key 覆寫
	if got := viper.GetInt("server.port"); got != 9090 {
		t.Errorf("expected server.port=9090 (from APP_SERVER_PORT via BindEnv), got %d", got)
	}
	if got := viper.GetString("server.host"); got != "env-host" {
		t.Errorf("expected server.host=env-host (from APP_SERVER_HOST via BindEnv), got %s", got)
	}
	// depth-4 nested key 覆寫（reflection 必須遞迴到深層）
	if got := viper.GetString("server.db.mysql.host"); got != "env-db" {
		t.Errorf("expected server.db.mysql.host=env-db (from APP_SERVER_DB_MYSQL_HOST via BindEnv), got %s", got)
	}
	// 沒被 OS env 覆寫的同層 leaf 仍由 config.yaml 提供
	if got := viper.GetInt("server.db.mysql.port"); got != 3306 {
		t.Errorf("expected server.db.mysql.port=3306 (no OS env, config.yaml wins), got %d", got)
	}
}
