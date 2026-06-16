package log

import (
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

// init 在套件載入時即以預設值（LOG_LEVEL=info、LOG_FORMAT=text）初始化 slog
// 全域 logger，確保任何 import 此套件的模組都能立即使用 slog。設定載入後
// （例如 config.Default() 之後）可再次呼叫 Init() 以套用最新的 LOG_LEVEL / LOG_FORMAT。
func init() {
	Init()
}

// Init 根據 viper 設定的 LOG_LEVEL 與 LOG_FORMAT 初始化 slog 全域 logger。
//
// 設定來源：
//   - LOG_LEVEL  debug | info | warn | error (case-insensitive, 預設 info)
//   - LOG_FORMAT text  | json                (case-insensitive, 預設 text)
//
// 行為：
//   - 從 viper 讀取 LOG_LEVEL 與 LOG_FORMAT，若為空字串或未設定則使用預設值
//   - 構造 slog handler 後呼叫 slog.SetDefault() 註冊至標準全域
//   - 輸出目標固定為 os.Stdout
//
// 可在 config.Default() 載入 viper 設定後再呼叫一次，以套用最新的 LOG_LEVEL / LOG_FORMAT。
func Init() {
	level := GetLogLevel()
	format := parseFormat(viper.GetString("LOG_FORMAT"))

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
