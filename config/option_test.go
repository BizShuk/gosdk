package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultWithValueOption(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get user home dir: %v", err)
	}

	appName := "test-val-app"
	expectedDir := filepath.Join(homeDir, ".config", appName)
	defer os.RemoveAll(expectedDir)

	defaultJSON := `{"name": "test-app", "version": "1.2.3"}`

	// Execute with AppName and default value
	Default(
		WithAppName(appName),
		WithDefaultValue(defaultJSON),
	)

	// Verify settings.json was created
	settingsPath := filepath.Join(expectedDir, "settings.json")
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

func TestWithoutAppNameDoesNotWriteDefaultValue(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	defaultJSON := `{"name": "test-app"}`

	// 沒有 WithAppName 時，WithDefaultValue 不應寫出任何 settings.json。
	Default(WithDefaultValue(defaultJSON))

	if _, err := os.Stat("settings.json"); !os.IsNotExist(err) {
		t.Errorf("Expected settings.json NOT to be written without WithAppName, but it exists")
	}
}

func TestDefaultWithAppNameOption(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	appName := "test-app-name-options"
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get user home dir: %v", err)
	}

	expectedDir := filepath.Join(homeDir, ".config", appName)
	// clean up if it already exists or was left over
	defer os.RemoveAll(expectedDir)

	Default(WithAppName(appName))

	dir := GetAppConfigDir()
	if dir != expectedDir {
		t.Errorf("Expected app config dir %s, got %s", expectedDir, dir)
	}

	if GetAppName() != appName {
		t.Errorf("Expected app name %s, got %s", appName, GetAppName())
	}
}

func TestSetGetAppName(t *testing.T) {
	orig := GetAppName()
	defer SetAppName(orig)

	name := "my-custom-test-app"
	SetAppName(name)
	if GetAppName() != name {
		t.Errorf("Expected app name %s, got %s", name, GetAppName())
	}
}


