package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultConfig(t *testing.T) {
	viper.Reset()
	os.Setenv("CONFIG_DIR", ".")
	defer os.Unsetenv("CONFIG_DIR")
	defer viper.Reset()

	Default()

	dir := GetConfigDir()
	if dir != "." {
		t.Errorf("Expected config dir ., got %s", dir)
	}
}

func TestConfigDirDefault(t *testing.T) {
	viper.Reset()
	os.Unsetenv("CONFIG_DIR")
	defer viper.Reset()

	Default()

	dir := GetConfigDir()
	if dir != "." {
		t.Errorf("Expected default config dir '.', got %s", dir)
	}
}
