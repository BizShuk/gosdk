package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultWithConfigDirOption(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tempDir, err := os.MkdirTemp("", "config_opt_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	Default(WithConfigDir(tempDir))

	dir := GetConfigDir()
	if dir != tempDir {
		t.Errorf("Expected config dir %s, got %s", tempDir, dir)
	}
}

func TestDeprecatedWithConfigPathOption(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tempDir, err := os.MkdirTemp("", "config_opt_deprecated_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Verify the deprecated WithConfigPath still functions correctly
	Default(WithConfigPath(tempDir))

	dir := GetConfigDir()
	if dir != tempDir {
		t.Errorf("Expected config dir %s, got %s", tempDir, dir)
	}
}

func TestDefaultWithValueOption(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("failed to get user config dir: %v", err)
	}

	appName := "test-val-app"
	expectedDir := filepath.Join(userConfigDir, appName)
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

func TestWithConfigDirDoesNotWriteDefaultValue(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tempDir, err := os.MkdirTemp("", "config_dir_no_write_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	defaultJSON := `{"name": "test-app"}`

	Default(
		WithConfigDir(tempDir),
		WithDefaultValue(defaultJSON),
	)

	settingsPath := filepath.Join(tempDir, "settings.json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Errorf("Expected settings.json NOT to exist for WithConfigDir, but it does")
	}
}

func TestDefaultWithAppNameOption(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	appName := "test-app-name-options"
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("failed to get user config dir: %v", err)
	}

	expectedDir := filepath.Join(userConfigDir, appName)
	// clean up if it already exists or was left over
	defer os.RemoveAll(expectedDir)

	Default(WithAppName(appName))

	dir := GetAppConfigDir()
	if dir != expectedDir {
		t.Errorf("Expected app config dir %s, got %s", expectedDir, dir)
	}
}

func TestDefaultWithBothConfigDirAndAppName(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	appName := "test-both-app"
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("failed to get user config dir: %v", err)
	}
	expectedAppDir := filepath.Join(userConfigDir, appName)
	defer os.RemoveAll(expectedAppDir)

	tempDir, err := os.MkdirTemp("", "config_both_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	Default(
		WithConfigDir(tempDir),
		WithAppName(appName),
	)

	configDir := GetConfigDir()
	if configDir != tempDir {
		t.Errorf("Expected config dir %s, got %s", tempDir, configDir)
	}

	appDir := GetAppConfigDir()
	if appDir != expectedAppDir {
		t.Errorf("Expected app config dir %s, got %s", expectedAppDir, appDir)
	}
}

