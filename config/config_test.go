package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultConfig(t *testing.T) {
	viper.Reset()
	os.Setenv("CONFIG_DIR", ".")
	defer os.Unsetenv("CONFIG_DIR")
	defer viper.Reset()

	Default()

	dir := viper.GetString("CONFIG_DIR")
	if dir != "." {
		t.Errorf("Expected config dir ., got %s", dir)
	}
}

func TestConfigDirDefault(t *testing.T) {
	viper.Reset()
	os.Unsetenv("CONFIG_DIR")
	defer viper.Reset()

	Default()

	dir := viper.GetString("CONFIG_DIR")
	if dir != "." {
		t.Errorf("Expected default config dir '.', got %s", dir)
	}
}

func TestDefaultWithConfigDirExpansion(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Relative path",
			input:    "./myconfig",
			expected: "./myconfig",
		},
		{
			name:     "Tilde only",
			input:    "~",
			expected: homeDir,
		},
		{
			name:     "Tilde prefix",
			input:    "~/myconfig",
			expected: filepath.Join(homeDir, "myconfig"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			defer viper.Reset()

			Default(WithConfigDir(tt.input))

			dir := viper.GetString("CONFIG_DIR")
			if dir != tt.expected {
				t.Errorf("For %s, expected config dir %s, got %s", tt.input, tt.expected, dir)
			}
		})
	}
}
