package log

import (
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

// init 在套件載入時即根據 viper 設定的 LOG_LEVEL 與 LOG_FORMAT 初始化 slog 全域 logger，
// 確保任何 import 此套件的模組都能立即使用套件層級 slog.* 函式。
//
// 設定來源：
//   - LOG_LEVEL  debug | info | warn | error (case-insensitive, 預設 info)
//   - LOG_FORMAT text  | json                (case-insensitive, 預設 text)
//
// 行為：
//   - 從 viper 讀取 LOG_LEVEL 與 LOG_FORMAT，若為空字串或未設定則使用預設值
//   - 構造 slog handler 後呼叫 slog.SetDefault() 註冊至標準全域
//   - 輸出目標依等級分流：Warn/Error 至 os.Stderr，Debug/Info 至 os.Stdout
//
// 注意：套件 init 在 import 時僅執行一次，且早於 config.Default() 載入 viper 設定。
// 若需讓 LOG_LEVEL / LOG_FORMAT 生效，必須在 import gosdk/log 之前以環境變數
// 或其他外部機制注入；runtime 不再支援重新套用 viper 設定。
func init() {
	level := GetLogLevel()
	format := parseFormat(viper.GetString("LOG_FORMAT"))

	opts := &slog.HandlerOptions{Level: level}

	var stdoutH, stderrH slog.Handler
	switch format {
	case "json":
		stdoutH = slog.NewJSONHandler(os.Stdout, opts)
		stderrH = slog.NewJSONHandler(os.Stderr, opts)
	default:
		stdoutH = slog.NewTextHandler(os.Stdout, opts)
		stderrH = slog.NewTextHandler(os.Stderr, opts)
	}

	splitHandler := &LevelSplitHandler{stdoutHandler: stdoutH, stderrHandler: stderrH}
	slog.SetDefault(slog.New(splitHandler))
}
