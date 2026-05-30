// config/sample 展示 github.com/bizshuk/gosdk/config 套件的常見用法。
//
// 執行方式 (在專案根目錄):
//
//	go run ./config/sample
//
// 用環境變數覆蓋 yaml 中的值:
//
//	APP_SERVER_PORT=9999 go run ./config/sample
//
// 用本地設定檔覆蓋共用設定 (建立 config.local.yaml 或 .env.local):
//
//	echo "server:" > config.local.yaml
//	echo "  port: 7777" >> config.local.yaml
//	go run ./config/sample
package main

import (
	"fmt"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/common"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// AppConfig 自訂設定結構，對應 yaml 中的所有設定區塊。
// 此結構用於示範，可根據實際需求調整欄位。
type AppConfig struct {
	Name     string                 `mapstructure:"name"`
	Version  string                 `mapstructure:"version"`
	LogLevel string                 `mapstructure:"log_level"`
	Server   ServerConfig           `mapstructure:"server"`
	DB       map[string]common.DBConfig `mapstructure:"db"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// AppSettings 展示用的自訂結構，對應 yaml 中部分常用欄位。
type AppSettings struct {
	AppName    string `mapstructure:"name"`
	AppVer     string `mapstructure:"version"`
	ServerHost string `mapstructure:"server.host"`
	ServerPort int    `mapstructure:"server.port"`
}

func main() {
	// Default() 內部依序合併：
	//   1) .env              <- 共用
	//   2) .env.local        <- 本地覆蓋 (覆蓋 1)
	//   3) config.yaml       <- 共用 YAML 設定
	//   4) config.local.yaml <- 本地 YAML 覆蓋 (覆蓋 3)
	//   5) settings.json     <- 共用 JSON 設定
	//   6) settings.local.json <- 本地 JSON 覆蓋 (覆蓋 5)
	//   7) APP_* 環境變數    <- 最高優先序 (覆蓋 1-6)
	viper.SetDefault("CONFIG_DIR", "./conf")
	config.Default()
	// or config.DefaultWithDir("./conf")

	// --- 用法 1：直接從全域 viper 讀單一鍵值 ---
	zap.L().Info("config dir", zap.String("value", config.GetConfigDir()))
	zap.L().Info("APP_NAME", zap.String("value", viper.GetString("APP_NAME")))
	zap.L().Info("server.host", zap.String("value", viper.GetString("server.host")))
	zap.L().Info("server.port", zap.Int("value", viper.GetInt("server.port")))

	// --- 用法 2：Unmarshal 成自訂強型別結構 ---
	var appConfig AppConfig
	if err := viper.Unmarshal(&appConfig); err != nil {
		fmt.Println("unmarshal config failed:", err)
		return
	}

	zap.L().Info("unmarshal appConfig", zap.Any("value", appConfig))

	// --- 用法 3：用 UnmarshalKey(".", &dest) 示範部分欄位取值 ---
	// 只取部分常用欄位，示範如何選擇性取值
	var appSettings AppSettings
	if err := viper.Unmarshal(&appSettings); err != nil {
		fmt.Println("unmarshal appSettings failed:", err)
		return
	}

	zap.L().Info("unmarshal appSettings", zap.Any("value", appSettings))

	// === 設定載入流程說明 ===
	// 每次執行 config.Default() 後，日誌會顯示實際載入的設定檔：
	//   EnvConfig used: /path/to/.env.local      (.env 被 .env.local 覆蓋)
	//   YamlConfig used: /path/to/config.local.yaml  (config.yaml 被 config.local.yaml 覆蓋)
	//   JsonConfig used: /path/to/settings.json  (settings.json 被 settings.local.json 覆蓋)
	// 若某個 base 檔案不存在，日誌會顯示 "not found. Using defaults"
	//
	// 設定優先序（後者覆蓋前者）：
	//   1) .env + .env.local
	//   2) config.yaml + config.local.yaml
	//   3) settings.json + settings.local.json
	//   4) APP_* 環境變數 (最高優先)
	// zap.L().Info("---- config sources ----")
	// zap.L().Info(".env.local", zap.String("note", "see 'EnvConfig used:' log"))
	// zap.L().Info("config.local.yaml", zap.String("note", "see 'YamlConfig used:' log"))
	// zap.L().Info("settings.json", zap.String("file", viper.ConfigFileUsed()))

	// --- 用法 5： Unmarshal 特定區塊成強型別 struct ---
	var serverConfig ServerConfig
	if err := viper.UnmarshalKey("server", &serverConfig); err != nil {
		zap.L().Error("unmarshal server failed", zap.Error(err))
		return
	}
	zap.L().Info("unmarshal server", zap.Any("value", serverConfig))

	// --- 用法 5：透過 db helper 從 viper 取出設定並建立 *gorm.DB ---
	// NewDBConfig("default") 會讀取 db.default.driver / db.default.url，
	// 並依 driver 字串委派給 sqlite / mysql 的具體實作。
	gormDB, err := common.NewDBConfig("default").Create()
	if err != nil {
		zap.L().Error("db connect failed", zap.Error(err))
		return
	}
	sqlDB, _ := gormDB.DB()
	defer sqlDB.Close()
	zap.L().Info("db driver", zap.String("name", gormDB.Name()))

	// --- 用法 6：印出所有 key/value，方便除錯 ---
	zap.L().Info("---- all settings ----")
	for _, k := range viper.AllKeys() {
		zap.L().Info("config", zap.String("key", k), zap.Any("value", viper.Get(k)))
	}
}
