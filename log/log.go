package log

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

func init() {
	Init()
}

func Init() {
	config := zap.NewProductionConfig()
	profile := viper.GetString("PROFILE")
	if profile != "prod" {
		config = zap.NewDevelopmentConfig()
	}

	config.Level = zap.NewAtomicLevelAt(GetLogLevel())
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)

	log, _ = config.Build()
	zap.ReplaceGlobals(log)
}

func Info(args ...interface{}) {
	log.Sugar().Info(args...)
}

func Infof(format string, args ...interface{}) {
	log.Sugar().Infof(format, args...)
}

func Fatal(args ...interface{}) {
	log.Sugar().Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	log.Sugar().Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	log.Sugar().Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	log.Sugar().Panicf(format, args...)
}

func Error(args ...interface{}) {
	log.Sugar().Error(args...)
}

func Errorf(format string, args ...interface{}) {
	log.Sugar().Errorf(format, args...)
}

func Debug(args ...interface{}) {
	log.Sugar().Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	log.Sugar().Debugf(format, args...)
}
