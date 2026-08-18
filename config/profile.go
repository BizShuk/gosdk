package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// PROFILE_KEY 是決定執行環境的設定鍵。
//
// 這裡的 PROFILE 只表達「現在跑在哪個環境」，與已廢除的 PROFILE 設定檔切換
// （config.<profile>.yaml）無關 —— 設定檔一律走 base + .local 雙檔載入。
const PROFILE_KEY = "PROFILE"

// 視為正式環境的 PROFILE 值（比對時不分大小寫）。
const (
	PROFILE_PRODUCTION       = "production"
	PROFILE_PRODUCTION_SHORT = "prod"
)

// currentProfile 回傳正規化（去空白、轉小寫）後的 PROFILE 值。
//
// 取值順序為 viper（涵蓋設定檔與 config.Default() 綁定的環境變數）→ OS 環境變數；
// 後者讓尚未呼叫 config.Default() 的呼叫端仍能取得正確結果。未設定時回傳空字串。
func currentProfile() string {
	profile := viper.GetString(PROFILE_KEY)
	if profile == "" {
		profile = os.Getenv(PROFILE_KEY)
	}

	return strings.ToLower(strings.TrimSpace(profile))
}

// IsProduction 回報目前是否為正式環境。
//
// 只有 PROFILE 明確等於 "production" 或 "prod"（不分大小寫）才回傳 true；
// 未設定或任何其他值一律視為非正式環境，讓「忘了設定」不會意外開啟正式環境行為。
func IsProduction() bool {
	switch currentProfile() {
	case PROFILE_PRODUCTION, PROFILE_PRODUCTION_SHORT:
		return true
	default:
		return false
	}
}
