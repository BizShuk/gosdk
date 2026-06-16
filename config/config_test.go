package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultRuns(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	// Default() 應在無 option、無設定檔的情況下正常執行，不 panic。
	Default()
}
