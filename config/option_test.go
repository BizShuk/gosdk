package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultWithConfigPathOption(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tempDir, err := os.MkdirTemp("", "config_opt_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	Default(WithConfigPath(tempDir))

	dir := GetConfigDir()
	if dir != tempDir {
		t.Errorf("Expected config dir %s, got %s", tempDir, dir)
	}
}

func TestDefaultWithValueOption(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tempDir, err := os.MkdirTemp("", "config_val_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	defaultJSON := `{"name": "test-app", "version": "1.2.3"}`

	// Execute with custom path and default value
	Default(
		WithConfigPath(tempDir),
		WithDefaultValue(defaultJSON),
	)

	// Verify settings.json was created
	settingsPath := filepath.Join(tempDir, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	if string(data) != defaultJSON {
		t.Errorf("Expected content %s, got %s", defaultJSON, string(data))
	}

	// Verify Viper successfully read the config
	name := viper.GetString("name")
	if name != "test-app" {
		t.Errorf("Expected config name 'test-app', got '%s'", name)
	}

	version := viper.GetString("version")
	if version != "1.2.3" {
		t.Errorf("Expected config version '1.2.3', got '%s'", version)
	}
}
