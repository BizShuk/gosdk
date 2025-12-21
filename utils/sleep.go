package utils

import (
	"time"

	"github.com/spf13/viper"
)

func ConfigSleep(delayKey string) {
	delay := time.Duration(viper.GetInt(delayKey))
	if delay == 0 {
		delay = 1000
	}
	time.Sleep(delay * time.Millisecond)
}
