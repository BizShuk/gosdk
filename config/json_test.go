package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJsonConfig_LoadBaseOnly(t *testing.T) {
	// 建立臨時目錄，只有 settings.json
	dir := t.TempDir()

	baseContent := `{"name":"base-app","version":"1.0.0","server":{"port":8080}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create settings.json: %v", err)
	}

	// 不建立 settings.local.json，確認只讀 base
	setupConfigDir(t, dir)

	c := JsonConfig{}
	v := c.Load()

	if v.GetString("name") != "base-app" {
		t.Errorf("expected name=base-app, got %s", v.GetString("name"))
	}
	if v.GetString("version") != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", v.GetString("version"))
	}
	if v.GetInt("server.port") != 8080 {
		t.Errorf("expected server.port=8080, got %d", v.GetInt("server.port"))
	}
}

func TestJsonConfig_LocalOverridesBase(t *testing.T) {
	// 建立臨時目錄，同時有 settings.json 和 settings.local.json
	dir := t.TempDir()

	// base 提供 name / version / server.port
	baseContent := `{"name":"base-app","version":"1.0.0","server":{"port":8080,"host":"base.local"}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create settings.json: %v", err)
	}

	// local 覆寫 server.port（巢狀 deep merge），新增 debug
	localContent := `{"server":{"port":9090},"debug":true}`
	if err := os.WriteFile(filepath.Join(dir, "settings.local.json"), []byte(localContent), 0644); err != nil {
		t.Fatalf("failed to create settings.local.json: %v", err)
	}

	setupConfigDir(t, dir)

	c := JsonConfig{}
	v := c.Load()

	// name 來自 base（local 沒有覆寫）
	if v.GetString("name") != "base-app" {
		t.Errorf("expected name=base-app, got %s", v.GetString("name"))
	}
	// version 來自 base
	if v.GetString("version") != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", v.GetString("version"))
	}
	// server.port 被 settings.local.json 覆寫
	if v.GetInt("server.port") != 9090 {
		t.Errorf("expected server.port=9090 (overridden by settings.local.json), got %d", v.GetInt("server.port"))
	}
	// server.host 應保留 base 的值（MergeInConfig 是 deep merge，不會整個取代 server 物件）
	if v.GetString("server.host") != "base.local" {
		t.Errorf("expected server.host=base.local (preserved by deep merge), got %s", v.GetString("server.host"))
	}
	// debug 只在 settings.local.json 中
	if !v.GetBool("debug") {
		t.Errorf("expected debug=true from settings.local.json, got %v", v.GetBool("debug"))
	}
}

func TestJsonConfig_MissingBaseConfigContinues(t *testing.T) {
	// 建立空目錄（沒有 settings.json），應不 panic
	dir := t.TempDir()

	setupConfigDir(t, dir)

	c := JsonConfig{}
	// 不應 panic，正常回傳空 viper instance
	v := c.Load()

	if v == nil {
		t.Error("expected non-nil viper instance when settings.json is missing")
	}
}

func TestJsonConfig_MissingLocalConfigContinues(t *testing.T) {
	// 只有 settings.json，沒有 settings.local.json，應不 panic
	dir := t.TempDir()

	baseContent := `{"name":"base-app"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create settings.json: %v", err)
	}

	setupConfigDir(t, dir)

	c := JsonConfig{}
	v := c.Load()

	if v == nil {
		t.Error("expected non-nil viper instance")
	}
	if v.GetString("name") != "base-app" {
		t.Errorf("expected name=base-app, got %s", v.GetString("name"))
	}
}

func TestJsonConfig_GetConfigNameReturnsSettings(t *testing.T) {
	// GetConfigName() 在 JsonConfig 應回傳 "settings"（與 EnvConfig/YamlConfig 回傳空字串不同）
	c := JsonConfig{}
	name := c.GetConfigName()
	if name != "settings" {
		t.Errorf("expected GetConfigName=settings, got %s", name)
	}
}

func TestJsonConfig_NoProfileDependency(t *testing.T) {
	// 設定 PROFILE=staging，確認 settings.local.json 仍然載入，不受 PROFILE 影響
	dir := t.TempDir()

	baseContent := `{"name":"base"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create settings.json: %v", err)
	}

	localContent := `{"name":"local-override"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.local.json"), []byte(localContent), 0644); err != nil {
		t.Fatalf("failed to create settings.local.json: %v", err)
	}

	// 即使 PROFILE=staging，仍應載入 settings.local.json 而非 settings.staging.json
	setupConfigDir(t, dir)
	t.Setenv("PROFILE", "staging")

	c := JsonConfig{}
	v := c.Load()

	if v.GetString("name") != "local-override" {
		t.Errorf("expected name=local-override from settings.local.json (not profile-based), got %s", v.GetString("name"))
	}
}
