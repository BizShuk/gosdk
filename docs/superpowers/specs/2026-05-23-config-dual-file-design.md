# Config Module Refactor: Remove PROFILE, Implement Dual-File Loading

**Date:** 2026-05-23  
**Author:** Claude Code  
**Status:** Design Phase  

---

## Overview

移除 `config` 模組中的 `PROFILE` 環境變數概念，改為每個載入器內部固定載入 **base 檔案 + local 檔案** 的雙檔案模式。local 檔案覆蓋 base，通常放在 `.gitignore` 作為本地開發環境的設定。

### Goals

1. 簡化設定管理，移除動態 PROFILE 切換邏輯
2. 統一三種格式（YAML、JSON、Dotenv）的載入行為
3. 保留 `CONFIG_DIR` 和 `embedFS` 功能供特殊場景使用
4. 移除全域 `GlobalConfig` 變數，只透過 Viper 存取設定

---

## Design

### File Loading Strategy

每個格式有兩個固定檔案，順序如下：

| Format | Base File        | Local File           | 說明                       |
|--------|------------------|----------------------|---------------------------|
| Dotenv | `.env`           | `.env.local`         | 載入 base，merge local     |
| YAML   | `config.yaml`    | `config.local.yaml`  | 載入 base，merge local     |
| JSON   | `settings.json`  | `settings.local.json`| 載入 base，merge local     |

### Loading Precedence

```
Lower Priority  ↓ (overridden by)
┌─────────────────────────────────┐
│  1. Default Values (Viper)      │
│  2. .env + .env.local           │
│  3. config.yaml + .local.yaml   │
│  4. settings.json + .local.json │
│  5. APP_* Environment Variables │
└─────────────────────────────────┘
Higher Priority ↑ (overrides)
```

### Loader Implementation Changes

#### **EnvConfig**
```go
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
            log.Info(".env not found, using defaults")
        } else {
            log.Fatalf("Error reading .env: %v", err)
        }
    }
    
    // Step 2: Merge .env.local
    v.SetConfigName(".env.local")
    if err := v.MergeInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); ok {
            log.Info(".env.local not found, skipping")
        } else {
            log.Fatalf("Error reading .env.local: %v", err)
        }
    }
    
    log.Infof("EnvConfig used: %s", v.ConfigFileUsed())
    return v
}

// No longer uses GetProfile()
func (c EnvConfig) GetConfigName() string {
    return "" // Not used; Load() handles both files
}
```

#### **YamlConfig** (similar pattern)
```go
type YamlConfig struct{}

func (c YamlConfig) Load() *viper.Viper {
    v := viper.New()
    v.AddConfigPath(".")
    v.AddConfigPath("conf")
    v.AddConfigPath(GetConfigDir())
    
    // Step 1: Load config.yaml
    v.SetConfigName("config")
    v.SetConfigType("yaml")
    if err := v.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); ok {
            log.Info("config.yaml not found, using defaults")
        } else {
            log.Fatalf("Error reading config.yaml: %v", err)
        }
    }
    
    // Step 2: Merge config.local.yaml
    v.SetConfigName("config.local")
    if err := v.MergeInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); ok {
            log.Info("config.local.yaml not found, skipping")
        } else {
            log.Fatalf("Error reading config.local.yaml: %v", err)
        }
    }
    
    log.Infof("YamlConfig used: %s", v.ConfigFileUsed())
    return v
}

// Removed GetConfigName() or simplified
```

#### **JsonConfig** (similar pattern)

### Core Config Changes

#### **config.go: Simplify Default()**

```go
var GlobalConfig ConfigSchema  // REMOVE THIS

type ConfigSchema struct {
    Name     string                  `mapstructure:"name"`
    Version  string                  `mapstructure:"version"`
    // REMOVE: Profile  string       
    LogLevel string                  `mapstructure:"log_level"`
    Server   ServerConfig            `mapstructure:"server"`
    DB       map[string]DBConnConfig `mapstructure:"db"`
}

// Simplified Default() — no PROFILE logic
func Default() {
    viper.BindEnv("CONFIG_DIR", "CONFIG_DIR")
    viper.SetDefault("CONFIG_DIR", ".")

    zap.L().Info("Load Configure...",
        zap.String("CONFIG_DIR", GetConfigDir()),
    )

    // Load in order
    v1 := NewEnvConfig().Load()
    viper.MergeConfigMap(v1.AllSettings())
    
    v2 := NewYamlConfig().Load()
    viper.MergeConfigMap(v2.AllSettings())
    
    v3 := NewJsonConfig().Load()
    viper.MergeConfigMap(v3.AllSettings())

    // APP_* env vars override everything
    viper.SetEnvPrefix("APP")
    viper.AutomaticEnv()
}

func DefaultWithDir(configDir string) {
    if configDir != "" {
        viper.Set("CONFIG_DIR", configDir)
    }
    Default()
}

// REMOVE GetProfile()
func GetConfigDir() string {
    return viper.GetString("CONFIG_DIR")
}
```

### API Changes

**Removed:**
- `GetProfile()` function
- `Profile` field from `ConfigSchema`
- `viper.BindEnv("PROFILE", "PROFILE")`
- `viper.SetDefault("PROFILE", "local")`
- Global `GlobalConfig` variable

**Retained:**
- `Default()` function (simplified)
- `DefaultWithDir(configDir string)` function
- `GetConfigDir()` function
- `Config` interface (can remain for abstraction)
- `CONFIG_DIR` parameter for customization
- `embedFS` module for special scenarios

### Error Handling

**File Not Found:**
- Base files (`.env`, `config.yaml`, `settings.json`): Log warning, continue with defaults
- Local files (`.env.local`, `config.local.yaml`, `settings.local.json`): Log info, skip
- This matches current behavior where missing config files are non-fatal

**Read Errors (permissions, format errors):**
- Log fatal error and exit
- These are programming/deployment errors, not runtime config variations

**Environment Variable Conflicts:**
- `APP_*` environment variables have highest precedence
- Override all file-based configuration

---

## Migration Path

### For Applications Using PROFILE

**Before:**
```bash
# Switch profiles via env var
PROFILE=prod go run main.go
PROFILE=dev go run main.go
```

**After:**
```bash
# Always load fixed files
go run main.go

# Create config.local.yaml for local overrides (in .gitignore)
# Production uses config.yaml only
```

### For Code Reading Config

**Before:**
```go
if config.GetProfile() == "prod" {
    // prod-specific logic
}
```

**After:**
```go
// Query viper directly instead
if !viper.IsSet("debug_mode") {
    // assume production mode
}
```

---

## Testing Strategy

### Unit Tests
- Each loader (`EnvConfig`, `YamlConfig`, `JsonConfig`) should test:
  - Base file loading works
  - Local file merging works
  - Missing files handled gracefully
  - Precedence order correct (local overrides base)

### Integration Tests
- `Default()` loads files in correct order
- Final viper state has expected values

### Manual Testing
- Create sample `config.yaml` and `config.local.yaml`
- Verify local values override base
- Verify `APP_*` env vars override files

---

## Backward Compatibility

This is a **breaking change**. Code that depends on `PROFILE` or `GetProfile()` must be updated:

1. **Hardcoded profile checks** → Remove or refactor to query actual config values
2. **Profile-based feature flags** → Use config values directly
3. **Sample code** (`sample/config/main.go`) → Update to reflect new loading

---

## Implementation Phases

### Phase 1: Loader Refactoring
- Update `EnvConfig.Load()` to load both `.env` and `.env.local`
- Update `YamlConfig.Load()` to load both `config.yaml` and `config.local.yaml`
- Update `JsonConfig.Load()` to load both `settings.json` and `settings.local.json`
- Remove `GetConfigName()` usage or adapt per loader

### Phase 2: Core Config Cleanup
- Remove `PROFILE` from `ConfigSchema`
- Remove `GlobalConfig` variable
- Simplify `Default()` function
- Remove `GetProfile()` and related bindings

### Phase 3: Testing & Documentation
- Update or create tests for new loader behavior
- Update `sample/config/main.go` to reflect new pattern
- Update `README.md` and `CLAUDE.md` config documentation
- Commit design doc and implementation plan

---

## Open Questions & Decisions

1. ✓ All three formats supported (YAML, JSON, Dotenv)
2. ✓ Local overrides base
3. ✓ Keep `embedFS` and `CONFIG_DIR` for flexibility
4. ✓ Missing files are warnings, not fatal
5. ✓ Remove `GlobalConfig` entirely, use Viper only
6. ✓ Sequential loading: Env → YAML → JSON → APP_*

---

## References

- Current implementation: `/Users/shuk/projects/gosdk/config/`
- Viper docs: https://github.com/spf13/viper
- CLAUDE.md: See "Configuration" section for current design decisions
