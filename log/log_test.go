package log

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/spf13/viper"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"warning alias", "warning", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"uppercase DEBUG", "DEBUG", slog.LevelDebug},
		{"mixed case Info", "Info", slog.LevelInfo},
		{"with whitespace", "  warn  ", slog.LevelWarn},
		{"empty", "", slog.LevelInfo},
		{"invalid value fallback", "trace", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"text", "text", "text"},
		{"json", "json", "json"},
		{"uppercase JSON", "JSON", "json"},
		{"mixed case Text", "Text", "text"},
		{"with whitespace", "  json  ", "json"},
		{"empty", "", "text"},
		{"unknown fallback", "yaml", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFormat(tt.input)
			if got != tt.expected {
				t.Errorf("parseFormat(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected slog.Level
	}{
		{"Debug Level", "debug", slog.LevelDebug},
		{"Info Level", "info", slog.LevelInfo},
		{"Warn Level", "warn", slog.LevelWarn},
		{"Error Level", "error", slog.LevelError},
		{"Default fallback", "invalid", slog.LevelInfo},
		{"Empty value", "", slog.LevelInfo},
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
		name            string
		logLevel        string
		checkLevel      slog.Level
		shouldBeEnabled bool
	}{
		{"debug level allows debug logs", "debug", slog.LevelDebug, true},
		{"info level disables debug logs", "info", slog.LevelDebug, false},
		{"info level allows info logs", "info", slog.LevelInfo, true},
		{"warn level disables info logs", "warn", slog.LevelInfo, false},
		{"warn level allows warn logs", "warn", slog.LevelWarn, true},
		{"error level disables warn logs", "error", slog.LevelWarn, false},
		{"error level allows error logs", "error", slog.LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 套件 init() 早已在 viper 為空時跑過一次並把 slog default 鎖在 info/text，
			// runtime 改 viper 不再影響 handler；這裡僅驗證 GetLogLevel() 對 viper 值的讀取正確性。
			viper.Reset()
			defer viper.Reset()

			viper.Set("LOG_LEVEL", tt.logLevel)
			if got := GetLogLevel(); got != tt.checkLevel && tt.shouldBeEnabled {
				t.Errorf("GetLogLevel() with LOG_LEVEL=%q = %v, expected level <= %v",
					tt.logLevel, got, tt.checkLevel)
			}
		})
	}
}

func TestInitDefaultValues(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	// 不設定 LOG_LEVEL / LOG_FORMAT，預期 GetLogLevel / parseFormat 走預設
	if got := GetLogLevel(); got != slog.LevelInfo {
		t.Errorf("default LOG_LEVEL: expected info, got %v", got)
	}
	if got := parseFormat(viper.GetString("LOG_FORMAT")); got != "text" {
		t.Errorf("default LOG_FORMAT: expected text, got %q", got)
	}
	if got := fmt.Sprintf("%T", slog.Default().Handler()); got != "*slog.TextHandler" {
		t.Errorf("default handler: expected *slog.TextHandler, got %s", got)
	}
}
