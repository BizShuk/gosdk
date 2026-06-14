package log

import (
	"testing"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected zapcore.Level
	}{
		{
			name:     "Debug Level",
			logLevel: "debug",
			expected: zap.DebugLevel,
		},
		{
			name:     "Info Level",
			logLevel: "info",
			expected: zap.InfoLevel,
		},
		{
			name:     "Warn Level",
			logLevel: "warn",
			expected: zap.WarnLevel,
		},
		{
			name:     "Error Level",
			logLevel: "error",
			expected: zap.ErrorLevel,
		},
		{
			name:     "Default fallback",
			logLevel: "invalid",
			expected: zap.InfoLevel,
		},
		{
			name:     "Empty value",
			logLevel: "",
			expected: zap.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			defer viper.Reset()

			viper.Set("LOG_LEVEL", tt.logLevel)
			actual := GetLogLevel()
			if actual != tt.expected {
				t.Errorf("expected level %v for LOG_LEVEL=%q, got %v", tt.expected, tt.logLevel, actual)
			}
		})
	}
}

func TestInitWithLogLevels(t *testing.T) {
	tests := []struct {
		name          string
		logLevel      string
		checkLevel    zapcore.Level
		shouldBeEnabled bool
	}{
		{
			name:            "Init with debug level allows debug logs",
			logLevel:        "debug",
			checkLevel:      zap.DebugLevel,
			shouldBeEnabled: true,
		},
		{
			name:            "Init with info level disables debug logs",
			logLevel:        "info",
			checkLevel:      zap.DebugLevel,
			shouldBeEnabled: false,
		},
		{
			name:            "Init with info level allows info logs",
			logLevel:        "info",
			checkLevel:      zap.InfoLevel,
			shouldBeEnabled: true,
		},
		{
			name:            "Init with warn level disables info logs",
			logLevel:        "warn",
			checkLevel:      zap.InfoLevel,
			shouldBeEnabled: false,
		},
		{
			name:            "Init with warn level allows warn logs",
			logLevel:        "warn",
			checkLevel:      zap.WarnLevel,
			shouldBeEnabled: true,
		},
		{
			name:            "Init with error level disables warn logs",
			logLevel:        "error",
			checkLevel:      zap.WarnLevel,
			shouldBeEnabled: false,
		},
		{
			name:            "Init with error level allows error logs",
			logLevel:        "error",
			checkLevel:      zap.ErrorLevel,
			shouldBeEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			defer viper.Reset()

			viper.Set("LOG_LEVEL", tt.logLevel)
			Init()

			enabled := zap.L().Core().Enabled(tt.checkLevel)
			if enabled != tt.shouldBeEnabled {
				t.Errorf("with LOG_LEVEL=%q, check for %v: expected enabled=%v, got %v",
					tt.logLevel, tt.checkLevel, tt.shouldBeEnabled, enabled)
			}
		})
	}
}
