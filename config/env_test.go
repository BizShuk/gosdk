package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// setupConfigDir 為測試設定 CONFIG_DIR，確保全域 viper 能正確讀到。
// 使用 t.Setenv 自動還原，避免並行測試 race condition。
func setupConfigDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("CONFIG_DIR", dir)
	// 全域 viper 需要 BindEnv 才能讀取環境變數
	viper.BindEnv("CONFIG_DIR", "CONFIG_DIR")
	t.Cleanup(func() {
		viper.Reset()
	})
}

func TestEnvConfig_LoadBase(t *testing.T) {
	// 建立臨時目錄模擬設定環境
	dir := t.TempDir()

	// 建立 .env 基礎設定檔
	envContent := "APP_NAME=base-app\nAPP_PORT=8080\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to create .env: %v", err)
	}

	// 不建立 .env.local，測試僅載入 base
	setupConfigDir(t, dir)

	c := EnvConfig{}
	v := c.Load()

	if v.GetString("APP_NAME") != "base-app" {
		t.Errorf("expected APP_NAME=base-app, got %s", v.GetString("APP_NAME"))
	}
	if v.GetString("APP_PORT") != "8080" {
		t.Errorf("expected APP_PORT=8080, got %s", v.GetString("APP_PORT"))
	}
}

func TestEnvConfig_LoadLocalOverridesBase(t *testing.T) {
	// 建立臨時目錄
	dir := t.TempDir()

	// 建立 .env 基礎設定檔
	envContent := "APP_NAME=base-app\nAPP_PORT=8080\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to create .env: %v", err)
	}

	// 建立 .env.local，覆寫 APP_PORT
	localContent := "APP_PORT=9090\nAPP_DEBUG=true\n"
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte(localContent), 0644); err != nil {
		t.Fatalf("failed to create .env.local: %v", err)
	}

	setupConfigDir(t, dir)

	c := EnvConfig{}
	v := c.Load()

	// APP_NAME 來自 .env（.env.local 沒有覆寫）
	if v.GetString("APP_NAME") != "base-app" {
		t.Errorf("expected APP_NAME=base-app, got %s", v.GetString("APP_NAME"))
	}
	// APP_PORT 應被 .env.local 覆寫
	if v.GetString("APP_PORT") != "9090" {
		t.Errorf("expected APP_PORT=9090 (overridden by .env.local), got %s", v.GetString("APP_PORT"))
	}
	// APP_DEBUG 只在 .env.local 中
	if v.GetString("APP_DEBUG") != "true" {
		t.Errorf("expected APP_DEBUG=true, got %s", v.GetString("APP_DEBUG"))
	}
}

func TestEnvConfig_MissingBaseEnvContinues(t *testing.T) {
	// 建立空目錄（沒有 .env）
	dir := t.TempDir()

	setupConfigDir(t, dir)

	c := EnvConfig{}
	// 應不 panic，正常回傳空 viper instance
	v := c.Load()

	if v == nil {
		t.Error("expected non-nil viper instance when .env is missing")
	}
}

func TestEnvConfig_GetConfigNameReturnsEmpty(t *testing.T) {
	// GetConfigName() 應回傳空字串（不再依賴 profile）
	c := EnvConfig{}
	name := c.GetConfigName()
	if name != "" {
		t.Errorf("expected empty string from GetConfigName, got %s", name)
	}
}

func TestEnvConfig_NoProfileDependency(t *testing.T) {
	// 設定 PROFILE=staging，確認 .env.local 不受 PROFILE 影響
	dir := t.TempDir()

	envContent := "APP_NAME=base\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to create .env: %v", err)
	}

	localContent := "APP_NAME=local-override\n"
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte(localContent), 0644); err != nil {
		t.Fatalf("failed to create .env.local: %v", err)
	}

	// 即使 PROFILE=staging，仍應載入 .env.local 而非 .env.staging
	setupConfigDir(t, dir)
	t.Setenv("PROFILE", "staging")

	c := EnvConfig{}
	v := c.Load()

	if v.GetString("APP_NAME") != "local-override" {
		t.Errorf("expected APP_NAME=local-override from .env.local (not profile-based), got %s", v.GetString("APP_NAME"))
	}
}
