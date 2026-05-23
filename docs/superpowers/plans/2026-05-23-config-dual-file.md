# Config Dual-File Loading Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the PROFILE environment variable concept and implement a fixed dual-file loading system where each config format (YAML, JSON, Dotenv) always loads a base file and a local override file.

**Architecture:** Each loader (`EnvConfig`, `YamlConfig`, `JsonConfig`) internally handles loading its base file (e.g., `config.yaml`) and merging its local override file (e.g., `config.local.yaml`). This replaces the previous dynamic PROFILE-based selection. The loading precedence remains: Env → YAML → JSON → APP_* environment variables.

**Tech Stack:** Viper 1.17.0, Go 1.24.5, standard Go testing

---

## Task 1: Update EnvConfig to Load Both .env and .env.local

**Files:**
- Modify: `config/env.go`

- [ ] **Step 1: Read the current EnvConfig.Load() implementation**

Current behavior:
- Loads `.env` (base)
- Merges `.env.<profile>` based on GetProfile()
- Logs the file used

New behavior:
- Load `.env` (base)
- Merge `.env.local` (always, no profile)
- Same logging

- [ ] **Step 2: Update EnvConfig.Load() to load both files**

```go
// config/env.go
package config

import (
	"github.com/bizshuk/gosdk/log"
	"github.com/spf13/viper"
)

func NewEnvConfig() Config {
	return EnvConfig{}
}

type EnvConfig struct{}

// Load reads .env and merges .env.local
func (c EnvConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetConfigDir())

	// Step 1: Load base .env
	v.SetConfigName(".env")
	v.SetConfigType("dotenv")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info(".env not found. Using defaults and env variables.")
		} else {
			log.Fatalf("Fatal error reading .env: %s", err)
		}
	}

	// Step 2: Merge .env.local (local overrides)
	v.SetConfigName(".env.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info(".env.local not found. Skipping local overrides.")
		} else {
			log.Fatalf("Fatal error reading .env.local: %s", err)
		}
	}

	log.Infof("EnvConfig used: %s", v.ConfigFileUsed())
	return v
}

// GetConfigName is no longer used but kept for interface compatibility
func (c EnvConfig) GetConfigName() string {
	return ""
}
```

- [ ] **Step 3: Run the current tests to verify they still pass**

```bash
cd /Users/shuk/projects/gosdk
go test -v ./config -run TestDefaultConfig
```

Expected: Test may fail because GetProfile() no longer exists. We'll fix tests in Task 5.

- [ ] **Step 4: Commit the change**

```bash
git add config/env.go
git commit -m "refactor: update EnvConfig to load .env and .env.local directly"
```

---

## Task 2: Update YamlConfig to Load Both config.yaml and config.local.yaml

**Files:**
- Modify: `config/yaml.go`

- [ ] **Step 1: Review current YamlConfig implementation**

Current:
- Loads `config.<profile>.yaml` based on GetProfile()

New:
- Load `config.yaml` (base)
- Merge `config.local.yaml` (always)

- [ ] **Step 2: Update YamlConfig.Load()**

```go
// config/yaml.go
package config

import (
	"github.com/bizshuk/gosdk/log"
	"github.com/spf13/viper"
)

type YamlConfig struct{}

func NewYamlConfig() Config {
	return &YamlConfig{}
}

// Load reads config.yaml and merges config.local.yaml
func (c *YamlConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetConfigDir())

	// Step 1: Load base config.yaml
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info("config.yaml not found. Using defaults and env variables.")
		} else {
			log.Fatalf("Fatal error reading config.yaml: %s", err)
		}
	}

	// Step 2: Merge config.local.yaml (local overrides)
	v.SetConfigName("config.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info("config.local.yaml not found. Skipping local overrides.")
		} else {
			log.Fatalf("Fatal error reading config.local.yaml: %s", err)
		}
	}

	log.Infof("YamlConfig used: %s", v.ConfigFileUsed())
	return v
}

// GetConfigName is no longer used but kept for interface compatibility
func (c *YamlConfig) GetConfigName() string {
	return ""
}
```

- [ ] **Step 3: Commit the change**

```bash
git add config/yaml.go
git commit -m "refactor: update YamlConfig to load config.yaml and config.local.yaml directly"
```

---

## Task 3: Update JsonConfig to Load Both settings.json and settings.local.json

**Files:**
- Modify: `config/json.go`

- [ ] **Step 1: Review current JsonConfig implementation**

Current:
- Loads `settings.json` (no profile variant, already uses fixed name)

New:
- Load `settings.json` (base)
- Merge `settings.local.json` (local override)

- [ ] **Step 2: Update JsonConfig.Load()**

```go
// config/json.go
package config

import (
	"github.com/bizshuk/gosdk/log"
	"github.com/spf13/viper"
)

type JsonConfig struct{}

func NewJsonConfig() Config {
	return &JsonConfig{}
}

// Load reads settings.json and merges settings.local.json
func (c *JsonConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetConfigDir())

	// Step 1: Load base settings.json
	v.SetConfigName("settings")
	v.SetConfigType("json")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info("settings.json not found. Using defaults and env variables.")
		} else {
			log.Fatalf("Fatal error reading settings.json: %s", err)
		}
	}

	// Step 2: Merge settings.local.json (local overrides)
	v.SetConfigName("settings.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info("settings.local.json not found. Skipping local overrides.")
		} else {
			log.Fatalf("Fatal error reading settings.local.json: %s", err)
		}
	}

	log.Infof("JsonConfig used: %s", v.ConfigFileUsed())
	return v
}

// GetConfigName is no longer used but kept for interface compatibility
func (c *JsonConfig) GetConfigName() string {
	return ""
}
```

- [ ] **Step 3: Commit the change**

```bash
git add config/json.go
git commit -m "refactor: update JsonConfig to load settings.json and settings.local.json directly"
```

---

## Task 4: Simplify config.go — Remove PROFILE and GlobalConfig

**Files:**
- Modify: `config/config.go`

- [ ] **Step 1: Review what needs to be removed**

Items to remove:
- `GlobalConfig` variable
- `Profile` field from `ConfigSchema`
- `viper.BindEnv("PROFILE", "PROFILE")` and related PROFILE bindings
- `viper.SetDefault("PROFILE", "local")`
- `GetProfile()` function

Items to keep:
- `ServerConfig`, `DBConnConfig` structs
- `ConfigSchema` struct (minus Profile field)
- `Config` interface
- `Default()` function (simplified)
- `DefaultWithDir(configDir string)` function
- `GetConfigDir()` function
- `embedFS` and `CONFIG_DIR` support

- [ ] **Step 2: Update ConfigSchema to remove Profile**

```go
// config/config.go
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
	// REMOVED: Profile  string
	LogLevel string                  `mapstructure:"log_level"`
	Server   ServerConfig            `mapstructure:"server"`
	DB       map[string]DBConnConfig `mapstructure:"db"`
}

// REMOVED: var GlobalConfig ConfigSchema

type Config interface {
	Load() *viper.Viper
	GetConfigName() string
}
```

- [ ] **Step 3: Simplify Default() function**

```go
// Default 載入預設設定檔。
//
// 備註 (Method 1)：應用程式層 (application layer) 可以在呼叫 Default() 之前，
// 透過設定管理工具 (configuration management tool, Viper) 直接指定設定檔目錄：
//
//	viper.Set("CONFIG_DIR", "/path/to/config")
func Default() {
	viper.BindEnv("CONFIG_DIR", "CONFIG_DIR")
	viper.SetDefault("CONFIG_DIR", ".")

	zap.L().Info("Load Configure...",
		zap.String("CONFIG_DIR", GetConfigDir()),
	)

	v1 := NewEnvConfig().Load()
	viper.MergeConfigMap(v1.AllSettings())
	v2 := NewYamlConfig().Load()
	viper.MergeConfigMap(v2.AllSettings())
	v3 := NewJsonConfig().Load()
	viper.MergeConfigMap(v3.AllSettings())

	// --- Environment Variables Override ---
	// 讓 Viper 知道要自動尋找以 APP 開頭的環境變數
	// 例如：環境變數 APP_SERVER_PORT 會自動對應到配置鍵 server.port
	viper.SetEnvPrefix("APP")
	// 啟用環境變數的綁定 環境變數中的底線 '_' 會被視為點號 '.'
	viper.AutomaticEnv()
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

// REMOVED: GetProfile() function

func GetConfigDir() string {
	return viper.GetString("CONFIG_DIR")
}
```

- [ ] **Step 4: Verify the complete config.go compiles**

```bash
cd /Users/shuk/projects/gosdk
go build ./config
```

Expected: Build succeeds with no errors

- [ ] **Step 5: Commit the changes**

```bash
git add config/config.go
git commit -m "refactor: remove PROFILE concept and GlobalConfig from config module"
```

---

## Task 5: Update config_test.go to Remove PROFILE Tests

**Files:**
- Modify: `config/config_test.go`

- [ ] **Step 1: Review the current test**

The current test:
- Sets PROFILE env var
- Calls GetProfile()
- Checks it returns "local"

New test should:
- Verify Default() loads config without errors
- Verify GetConfigDir() works
- Verify Viper has expected keys from config files

- [ ] **Step 2: Create test config files for testing**

Create test config directory with sample files:

```bash
mkdir -p /Users/shuk/projects/gosdk/.test-config
```

Create `/Users/shuk/projects/gosdk/.test-config/config.yaml`:
```yaml
name: test-app
version: "1.0.0"
log_level: info
server:
  host: localhost
  port: 8080
```

Create `/Users/shuk/projects/gosdk/.test-config/.env`:
```
TEST_VAR=base-value
```

Create `/Users/shuk/projects/gosdk/.test-config/.env.local`:
```
TEST_VAR=local-value
```

- [ ] **Step 3: Write new config_test.go**

```go
package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultConfigLoading(t *testing.T) {
	// Setup: point to test config directory
	os.Setenv("CONFIG_DIR", ".test-config")
	defer os.Unsetenv("CONFIG_DIR")
	
	// Clear viper to start fresh
	viper.Reset()

	// Call Default() to load configs
	Default()

	// Verify GetConfigDir() returns the test directory
	dir := GetConfigDir()
	if dir != ".test-config" {
		t.Errorf("Expected config dir .test-config, got %s", dir)
	}

	// Verify config.yaml was loaded
	name := viper.GetString("name")
	if name != "test-app" {
		t.Errorf("Expected name 'test-app', got %s", name)
	}

	// Verify server config was loaded
	port := viper.GetInt("server.port")
	if port != 8080 {
		t.Errorf("Expected server.port 8080, got %d", port)
	}
}

func TestEnvLocalOverridesBase(t *testing.T) {
	// Setup: point to test config directory
	os.Setenv("CONFIG_DIR", ".test-config")
	defer os.Unsetenv("CONFIG_DIR")
	
	// Clear viper to start fresh
	viper.Reset()

	// Load just env config
	v := NewEnvConfig().Load()
	
	// .env.local should override .env
	val := v.GetString("TEST_VAR")
	if val != "local-value" {
		t.Errorf("Expected TEST_VAR='local-value', got %s", val)
	}
}

func TestConfigDirDefault(t *testing.T) {
	// Unset CONFIG_DIR to test default
	os.Unsetenv("CONFIG_DIR")
	
	// Clear viper to start fresh
	viper.Reset()

	// Call Default() without CONFIG_DIR set
	Default()

	// Verify default is "."
	dir := GetConfigDir()
	if dir != "." {
		t.Errorf("Expected default config dir '.', got %s", dir)
	}
}
```

- [ ] **Step 4: Run the new tests to verify they pass**

```bash
cd /Users/shuk/projects/gosdk
go test -v ./config -run TestDefault
```

Expected: All three tests pass

- [ ] **Step 5: Clean up viper state between tests (optional refactor)**

Add a helper function to reset Viper state:

```go
// In config_test.go, add helper
func setupTest(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
	})
}
```

Update each test to call `setupTest(t)` at the start. This is optional but keeps tests isolated.

- [ ] **Step 6: Commit the test changes**

```bash
git add config/config_test.go
git commit -m "test: update config tests to remove PROFILE and verify dual-file loading"
```

---

## Task 6: Update sample/config/main.go to Remove PROFILE Usage

**Files:**
- Modify: `sample/config/main.go`

- [ ] **Step 1: Understand the sample usage**

Current sample:
- Sets PROFILE env var
- Calls GetProfile() to show current profile
- Shows how to override PROFILE for different profiles

New sample:
- No PROFILE env var
- No GetProfile() call
- Shows how to use config.local.yaml for local overrides
- Show APP_* env var override

- [ ] **Step 2: Update the sample/config/main.go**

```go
// sample/config 展示 github.com/bizshuk/gosdk/config 套件的常見用法。
//
// 執行方式 (在專案根目錄):
//
//	go run ./sample/config
//
// 用環境變數覆蓋 yaml 中的值:
//
//	APP_SERVER_PORT=9999 go run ./sample/config
//
// 用本地設定檔覆蓋共用設定 (建立 config.local.yaml):
//
//	echo "server:
//	  port: 7777" > config.local.yaml
//	go run ./sample/config
package main

import (
	"fmt"
	"os"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/db"
	"github.com/spf13/viper"
)

// ServerConfig 對應 yaml 中的 `server:` 區塊，
// 透過 mapstructure tag 由 viper.UnmarshalKey 反序列化。
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func main() {
	// CONFIG_DIR 告訴 viper 去哪個資料夾找設定檔；
	// 實務上建議由 shell export，這裡為了讓範例自包含才在程式內設定。
	if _, ok := os.LookupEnv("CONFIG_DIR"); !ok {
		os.Setenv("CONFIG_DIR", "./sample/config/conf")
	}

	// Default() 內部依序合併：
	//   1) .env              <- 共用
	//   2) .env.local        <- 本地覆蓋 (覆蓋 1)
	//   3) config.yaml       <- 共用 YAML 設定
	//   4) config.local.yaml <- 本地 YAML 覆蓋 (覆蓋 3)
	//   5) settings.json     <- 共用 JSON 設定
	//   6) settings.local.json <- 本地 JSON 覆蓋 (覆蓋 5)
	//   7) APP_* 環境變數    <- 最高優先序 (覆蓋 1-6)
	config.Default()

	// --- 用法 1：直接從全域 viper 讀單一鍵值 ---
	fmt.Println("config dir       :", config.GetConfigDir())
	fmt.Println("server.host      :", viper.GetString("server.host"))
	fmt.Println("server.port      :", viper.GetInt("server.port"))
	fmt.Println("db.default.url   :", viper.GetString("db.default.url"))

	// --- 用法 2：Unmarshal 成強型別 struct ---
	var srv ServerConfig
	if err := viper.UnmarshalKey("server", &srv); err != nil {
		fmt.Println("unmarshal server failed:", err)
		return
	}
	fmt.Printf("server (struct)  : %+v\n", srv)

	// --- 用法 3：透過 db helper 從 viper 取出設定並建立 *gorm.DB ---
	// NewDBConfig("default") 會讀取 db.default.driver / db.default.url，
	// 並依 driver 字串委派給 sqlite / mysql 的具體實作。
	gormDB, err := db.NewDBConfig("default").Create()
	if err != nil {
		fmt.Println("db connect failed:", err)
		return
	}
	sqlDB, _ := gormDB.DB()
	defer sqlDB.Close()
	fmt.Println("db driver        :", gormDB.Name())

	// --- 用法 4：印出所有 key/value，方便除錯 ---
	fmt.Println("---- all settings ----")
	for _, k := range viper.AllKeys() {
		fmt.Printf("  %-24s = %v\n", k, viper.Get(k))
	}
}
```

- [ ] **Step 3: Test the updated sample**

```bash
cd /Users/shuk/projects/gosdk
go run ./sample/config
```

Expected: Program runs and displays config values (server.host, server.port, db.default.url, etc.)

- [ ] **Step 4: Test with APP_* env var override**

```bash
APP_SERVER_PORT=9999 go run ./sample/config
```

Expected: Output shows `server.port: 9999`

- [ ] **Step 5: Test with local config override**

Create `config.local.yaml` in the sample config directory:

```bash
echo "server:
  port: 7777" > sample/config/conf/config.local.yaml
go run ./sample/config
```

Expected: Output shows `server.port: 7777`

Clean up:
```bash
rm sample/config/conf/config.local.yaml
```

- [ ] **Step 6: Commit the sample changes**

```bash
git add sample/config/main.go
git commit -m "docs: update sample/config to show dual-file loading without PROFILE"
```

---

## Task 7: Update CLAUDE.md with New Config Design

**Files:**
- Modify: `CLAUDE.md` (project-level, at repo root)

- [ ] **Step 1: Review the current Configuration section in CLAUDE.md**

Find the "Configuration" section that mentions PROFILE. Update it to reflect the new system.

- [ ] **Step 2: Update the Configuration section**

Replace the old PROFILE-based description with:

```markdown
### Configuration: Dual-File Loading

The `config/` module loads configuration from three formats in order of precedence:

1. **Dotenv**: `.env` (shared) + `.env.local` (local overrides, in `.gitignore`)
2. **YAML**: `config.yaml` (shared) + `config.local.yaml` (local overrides, in `.gitignore`)
3. **JSON**: `settings.json` (shared) + `settings.local.json` (local overrides, in `.gitignore`)
4. **Environment Variables**: `APP_*` prefix (highest precedence, overrides all files)

**Entry Point:** `config.Default()` loads all configured files and merges them into a global `viper.Viper` instance. Use `viper.GetString()`, `viper.GetInt()`, etc. to read values.

**Configuration Discovery:** Files are searched in:
- Current working directory (`.`)
- `conf/` subdirectory
- Directory specified by `CONFIG_DIR` environment variable (default: `.`)

**Local Overrides (Development):** Create `.local` variants (`.env.local`, `config.local.yaml`, `settings.local.json`) in `.gitignore` for machine-specific settings. These override base files but are never committed.

**No PROFILE Variable:** Removed the `PROFILE` environment variable. The system always loads the fixed base + local files. For multi-environment deployments (dev/stage/prod), use container environment variables or ConfigMaps (Kubernetes) with `APP_*` prefixes.

**GlobalConfig Removal:** The global `GlobalConfig` variable was removed in favor of direct Viper access. Always use `viper.Get*()` functions to read config values, or `viper.UnmarshalKey()` to deserialize into structs.
```

- [ ] **Step 3: Update module mapping table if present**

If there's a module mapping table, update the entry for config:

```markdown
| 設定管理              | `config/`, `config/db/`                 | `config.Default()` (simplified, no PROFILE) |
```

- [ ] **Step 4: Update any examples in CLAUDE.md that reference PROFILE**

Search for "PROFILE" in CLAUDE.md and remove/update references.

- [ ] **Step 5: Verify CLAUDE.md is still consistent**

Check that the config description aligns with the actual code and other references.

- [ ] **Step 6: Commit the documentation update**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md config documentation for dual-file loading"
```

---

## Task 8: Run Full Test Suite and Integration Check

**Files:**
- No files modified, verification only

- [ ] **Step 1: Run all config package tests**

```bash
cd /Users/shuk/projects/gosdk
go test -v ./config
```

Expected: All tests pass (TestDefaultConfigLoading, TestEnvLocalOverridesBase, TestConfigDirDefault)

- [ ] **Step 2: Run all package tests to ensure no regressions**

```bash
go test -v ./...
```

Expected: All tests pass across the entire project

- [ ] **Step 3: Build the project to ensure no compilation errors**

```bash
go build -v ./...
```

Expected: Build succeeds with no errors

- [ ] **Step 4: Run the sample to verify functionality**

```bash
go run ./sample/config
```

Expected: Sample runs and displays config values from loaded files

- [ ] **Step 5: Verify git log shows all commits in order**

```bash
git log --oneline -10
```

Expected: See commits from all tasks in reverse chronological order

- [ ] **Step 6: Create a summary commit (optional)**

```bash
git log --oneline -8 | head -8
```

If desired, create a summary message. Otherwise, the implementation is complete.

---

## Spec Coverage Checklist

- [x] **File Loading Strategy**: Each loader (Env, Yaml, Json) loads base + local files
- [x] **Loading Precedence**: Env → Yaml → Json → APP_* env vars
- [x] **Remove PROFILE**: GetProfile(), PROFILE env var, Profile field all removed
- [x] **Remove GlobalConfig**: GlobalConfig variable removed
- [x] **Keep CONFIG_DIR**: CONFIG_DIR parameter retained
- [x] **Keep embedFS**: embedFS module untouched
- [x] **Error Handling**: Missing base files log warning, missing local files log info, read errors are fatal
- [x] **Tests**: New tests verify dual-file loading and merging
- [x] **Sample Code**: Updated to show new pattern without PROFILE
- [x] **Documentation**: CLAUDE.md updated to reflect new system

---

## No Breaking External APIs

- `Default()` remains the entry point (signature unchanged)
- `DefaultWithDir(configDir)` remains available
- `GetConfigDir()` remains available
- `Config` interface remains compatible
- Viper API is unchanged

Breaking changes (internal to project):
- `GetProfile()` removed
- `GlobalConfig` variable removed
- `Profile` field from ConfigSchema removed
- PROFILE env var no longer recognized
