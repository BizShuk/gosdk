// sample/config 展示 github.com/bizshuk/gosdk/config 套件的常見用法。
//
// 執行方式 (在專案根目錄):
//
//	go run ./sample/config
//
// 用環境變數覆蓋 yaml 中的值:
//
//	APP_SERVER_PORT=9999 go run ./sample/config
//
// 用本地設定檔覆蓋共用設定 (建立 config.local.yaml 或 .env.local):
//
//	echo "server:" > config.local.yaml
//	echo "  port: 7777" >> config.local.yaml
//	go run ./sample/config
package main

import (
	"fmt"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/db"
	"github.com/spf13/viper"
)

// AppConfig 自訂設定結構，對應 yaml 中的所有設定區塊。
// 此結構用於示範，可根據實際需求調整欄位。
type AppConfig struct {
	Name     string              `mapstructure:"name"`
	Version  string              `mapstructure:"version"`
	LogLevel string              `mapstructure:"log_level"`
	Server   ServerConfig        `mapstructure:"server"`
	DB       map[string]DBConfig `mapstructure:"db"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DBConfig struct {
	Driver string `mapstructure:"driver"`
	URL    string `mapstructure:"url"`
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
	//  or config.DefaultWithDir("./conf")

	// --- 用法 1：直接從全域 viper 讀單一鍵值 ---
	fmt.Println("config dir       :", config.GetConfigDir())
	fmt.Println("APP_NAME         :", viper.GetString("APP_NAME")) // APP_NAME in .env
	fmt.Println("server.host      :", viper.GetString("server.host"))
	fmt.Println("server.port      :", viper.GetInt("server.port"))

	// --- 用法 2：Unmarshal 成自訂強型別結構 ---
	var appConfig AppConfig
	if err := viper.Unmarshal(&appConfig); err != nil {
		fmt.Println("unmarshal config failed:", err)
		return
	}
	fmt.Printf("app (struct)     : name=%s (settings.json), version=%s (settings.json), log_level=%s (.env.local)\n",
		appConfig.Name, appConfig.Version, appConfig.LogLevel)
	fmt.Printf("server (struct)   : host=%s (config.local.yaml) , port=%d (config.local.yaml) \n",
		appConfig.Server.Host, appConfig.Server.Port)

	// --- 用法 3：用 UnmarshalKey(".", &dest) 示範部分欄位取值 ---
	// 只取部分常用欄位，示範如何選擇性取值
	var appSettings AppSettings
	if err := viper.Unmarshal(&appSettings); err != nil {
		fmt.Println("unmarshal appSettings failed:", err)
		return
	}
	fmt.Printf("appSettings      : name=%s, version=%s, host=%s, port=%d\n",
		appSettings.AppName, appSettings.AppVer, appSettings.ServerHost, appSettings.ServerPort)

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
	fmt.Println("---- config sources (check logs above) ----")
	fmt.Println("  .env.local      -> loaded, see 'EnvConfig used:' log")
	fmt.Println("  config.local.yaml -> loaded, see 'YamlConfig used:' log")
	fmt.Println("  settings.json   ->", viper.ConfigFileUsed())

	// --- 用法 5： Unmarshal 特定區塊成強型別 struct ---
	var srv ServerConfig
	if err := viper.UnmarshalKey("server", &srv); err != nil {
		fmt.Println("unmarshal server failed:", err)
		return
	}
	fmt.Printf("server (partial)  : %+v\n", srv)

	// --- 用法 5：透過 db helper 從 viper 取出設定並建立 *gorm.DB ---
	// NewDBConfig("default") 會讀取 db.default.driver / db.default.url，
	// 並依 driver 字串委派給 sqlite / mysql 的具體實作。
	gormDB, err := db.NewDBConfig("default").Create()
	if err != nil {
		fmt.Println("db connect failed:", err)
		return
	}
	sqlDB, _ := gormDB.DB()
	defer sqlDB.Close()
	fmt.Println("db driver        :", gormDB.Name())

	// --- 用法 6：印出所有 key/value，方便除錯 ---
	fmt.Println("---- all settings ----")
	for _, k := range viper.AllKeys() {
		fmt.Printf("  %-24s = %v\n", k, viper.Get(k))
	}
}
