package log

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	Init()
}

func Init() {
	config := zap.NewProductionConfig()
	profile := viper.GetString("PROFILE")
	if profile == "prod" {
		config = zap.NewProductionConfig()
	}

	config.Level = zap.NewAtomicLevelAt(GetLogLevel())
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)

	logger, _ := config.Build(zap.AddStacktrace(zap.PanicLevel))
	zap.ReplaceGlobals(logger)
}
