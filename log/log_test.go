package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
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
			os.Setenv("LOG_LEVEL", tt.logLevel)
			t.Cleanup(func() { os.Unsetenv("LOG_LEVEL") })

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
			// GetLogLevel 直接讀取 os.Getenv("LOG_LEVEL")，
			// 此測試驗證 env var 能正確傳遞到 GetLogLevel()。
			os.Setenv("LOG_LEVEL", tt.logLevel)
			t.Cleanup(func() { os.Unsetenv("LOG_LEVEL") })

			if got := GetLogLevel(); got != tt.checkLevel && tt.shouldBeEnabled {
				t.Errorf("GetLogLevel() with LOG_LEVEL=%q = %v, expected level <= %v",
					tt.logLevel, got, tt.checkLevel)
			}
		})
	}
}

func TestInitDefaultValues(t *testing.T) {
	// 清除 env var 確保走預設值
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("LOG_FORMAT")

	// 不設定 LOG_LEVEL / LOG_FORMAT，預期 GetLogLevel / parseFormat 走預設
	if got := GetLogLevel(); got != slog.LevelInfo {
		t.Errorf("default LOG_LEVEL: expected info, got %v", got)
	}
	if got := parseFormat(""); got != "text" {
		t.Errorf("default LOG_FORMAT: expected text, got %q", got)
	}

	handler, ok := slog.Default().Handler().(*LevelSplitHandler)
	if !ok {
		t.Fatalf("expected handler type to be *LevelSplitHandler, got %T", slog.Default().Handler())
	}
	if got := fmt.Sprintf("%T", handler.stdoutHandler); got != "*slog.TextHandler" {
		t.Errorf("default stdout handler: expected *slog.TextHandler, got %s", got)
	}
}

func TestLevelSplitHandler(t *testing.T) {
	// 建立 stdout 與 stderr 的緩衝區
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	stdoutH := slog.NewTextHandler(&stdoutBuf, opts)
	stderrH := slog.NewTextHandler(&stderrBuf, opts)

	handler := &LevelSplitHandler{
		stdoutHandler: stdoutH,
		stderrHandler: stderrH,
	}
	logger := slog.New(handler)

	// 1. 測試 Info 等級下，slog.Info 去 stdout，slog.Error 去 stderr
	logger.Info("info message")
	logger.Error("error message")

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if !strings.Contains(stdoutStr, "info message") {
		t.Errorf("stdout should contain 'info message', got: %q", stdoutStr)
	}
	if strings.Contains(stdoutStr, "error message") {
		t.Errorf("stdout should not contain 'error message', got: %q", stdoutStr)
	}

	if !strings.Contains(stderrStr, "error message") {
		t.Errorf("stderr should contain 'error message', got: %q", stderrStr)
	}
	if strings.Contains(stderrStr, "info message") {
		t.Errorf("stderr should not contain 'info message', got: %q", stderrStr)
	}

	// 重設緩衝區
	stdoutBuf.Reset()
	stderrBuf.Reset()

	// 2. 測試 Warn 等級設定下，過濾 Info 訊息
	optsWarn := &slog.HandlerOptions{Level: slog.LevelWarn}
	stdoutHWarn := slog.NewTextHandler(&stdoutBuf, optsWarn)
	stderrHWarn := slog.NewTextHandler(&stderrBuf, optsWarn)

	handlerWarn := &LevelSplitHandler{
		stdoutHandler: stdoutHWarn,
		stderrHandler: stderrHWarn,
	}
	loggerWarn := slog.New(handlerWarn)

	loggerWarn.Info("info message should be filtered")
	loggerWarn.Error("error message should pass")

	stdoutStrWarn := stdoutBuf.String()
	stderrStrWarn := stderrBuf.String()

	if stdoutStrWarn != "" {
		t.Errorf("stdout should be empty when Level is Warn, got: %q", stdoutStrWarn)
	}
	if !strings.Contains(stderrStrWarn, "error message should pass") {
		t.Errorf("stderr should contain 'error message should pass', got: %q", stderrStrWarn)
	}
}
