package log

import (
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

// GetLogLevel 從 viper 讀取 LOG_LEVEL 並回傳對應的 slog.Level。
//
// 合法值 (case-insensitive): debug / info / warn / error。
// 空字串、未設定或不合法值皆 fallback 為 slog.LevelInfo（預設值）。
func GetLogLevel() slog.Level {
	return parseLevel(viper.GetString("LOG_LEVEL"))
}

// parseLevel 解析 LOG_LEVEL 字串到 slog.Level。
//
// 同時接受 "warn" 與 "warning"，兩者皆映射到 slog.LevelWarn。
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// parseFormat 解析 LOG_FORMAT 字串為 "text" 或 "json"。
//
// 預設為 "text"。空字串、未知值或大小寫差異皆會 fallback 到 "text"。
func parseFormat(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return "json"
	default:
		return "text"
	}
}
