package time

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestConfigSleep(t *testing.T) {
	viper.Set("test.sleep.delay", 10)
	defer viper.Set("test.sleep.delay", nil)

	start := time.Now()
	ConfigSleep("test.sleep.delay")
	duration := time.Since(start)

	if duration < 10*time.Millisecond {
		t.Errorf("Expected sleep duration at least 10ms, got %v", duration)
	}
}
