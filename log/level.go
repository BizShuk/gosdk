package log

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func GetLogLevel() zapcore.Level {
	levelStr := viper.GetString("LOG_LEVEL")

	// Switch based on the string value to assign the zap Level
	switch levelStr {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel // Default fallback
	}

}
