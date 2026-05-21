package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	os.Setenv("CONFIG_DIR", ".")
	os.Setenv("PROFILE", "local")
	defer os.Unsetenv("CONFIG_DIR")
	defer os.Unsetenv("PROFILE")

	Default()

	profile := GetProfile()
	if profile != "local" {
		t.Errorf("Expected profile local, got %s", profile)
	}

	dir := GetConfigDir()
	if dir != "." {
		t.Errorf("Expected config dir ., got %s", dir)
	}
}
