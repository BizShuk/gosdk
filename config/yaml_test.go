package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYamlConfig_LoadBaseOnly(t *testing.T) {
	// 建立臨時目錄，只有 config.yaml
	dir := t.TempDir()

	yamlContent := "name: base-app\nversion: \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	// 不建立 config.local.yaml，確認只讀 base
	setupConfigDir(t, dir)

	c := YamlConfig{}
	v := c.Load()

	if v.GetString("name") != "base-app" {
		t.Errorf("expected name=base-app, got %s", v.GetString("name"))
	}
	if v.GetString("version") != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", v.GetString("version"))
	}
}

func TestYamlConfig_LocalOverridesBase(t *testing.T) {
	// 建立臨時目錄，同時有 config.yaml 和 config.local.yaml
	dir := t.TempDir()

	baseContent := "name: base-app\nversion: \"1.0.0\"\nport: 8080\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	// config.local.yaml 覆寫 port，新增 debug
	localContent := "port: 9090\ndebug: true\n"
	if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte(localContent), 0644); err != nil {
		t.Fatalf("failed to create config.local.yaml: %v", err)
	}

	setupConfigDir(t, dir)

	c := YamlConfig{}
	v := c.Load()

	// name 來自 base（local 沒有覆寫）
	if v.GetString("name") != "base-app" {
		t.Errorf("expected name=base-app, got %s", v.GetString("name"))
	}
	// port 被 config.local.yaml 覆寫
	if v.GetInt("port") != 9090 {
		t.Errorf("expected port=9090 (overridden by config.local.yaml), got %d", v.GetInt("port"))
	}
	// debug 只在 config.local.yaml 中
	if !v.GetBool("debug") {
		t.Errorf("expected debug=true from config.local.yaml, got %v", v.GetBool("debug"))
	}
}

func TestYamlConfig_MissingBaseConfigContinues(t *testing.T) {
	// 建立空目錄（沒有 config.yaml），應不 panic
	dir := t.TempDir()

	setupConfigDir(t, dir)

	c := YamlConfig{}
	// 不應 panic，正常回傳空 viper instance
	v := c.Load()

	if v == nil {
		t.Error("expected non-nil viper instance when config.yaml is missing")
	}
}

func TestYamlConfig_MissingLocalConfigContinues(t *testing.T) {
	// 只有 config.yaml，沒有 config.local.yaml，應不 panic
	dir := t.TempDir()

	baseContent := "name: base-app\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	setupConfigDir(t, dir)

	c := YamlConfig{}
	v := c.Load()

	if v == nil {
		t.Error("expected non-nil viper instance")
	}
	if v.GetString("name") != "base-app" {
		t.Errorf("expected name=base-app, got %s", v.GetString("name"))
	}
}

func TestYamlConfig_GetConfigNameReturnsEmpty(t *testing.T) {
	// GetConfigName() 應回傳空字串（不再依賴 profile）
	c := YamlConfig{}
	name := c.GetConfigName()
	if name != "" {
		t.Errorf("expected empty string from GetConfigName, got %s", name)
	}
}

func TestYamlConfig_NoProfileDependency(t *testing.T) {
	// 設定 PROFILE=staging，確認 config.local.yaml 仍然載入，不受 PROFILE 影響
	dir := t.TempDir()

	baseContent := "name: base\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	localContent := "name: local-override\n"
	if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte(localContent), 0644); err != nil {
		t.Fatalf("failed to create config.local.yaml: %v", err)
	}

	// 即使 PROFILE=staging，仍應載入 config.local.yaml 而非 config.staging.yaml
	setupConfigDir(t, dir)
	t.Setenv("PROFILE", "staging")

	c := YamlConfig{}
	v := c.Load()

	if v.GetString("name") != "local-override" {
		t.Errorf("expected name=local-override from config.local.yaml (not profile-based), got %s", v.GetString("name"))
	}
}
