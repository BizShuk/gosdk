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
// 切換 profile (會去讀 .env.prod 與 config.prod.yaml):
//
//	PROFILE=prod go run ./sample/config
package main

import (
	"fmt"
	"os"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/db"
	"github.com/spf13/viper"
)

// ServerConfig 對應 yaml 中的 `server:` 區塊，
// 透過 mapstructure tag 由 viper.UnmarshalKey 反序列化。
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func main() {
	// CONFIG_DIR 告訴 viper 去哪個資料夾找 .env / yaml；
	// PROFILE 決定要讀哪一份 profile 變體 (.env.<profile>, config.<profile>.yaml)。
	// 實務上建議由 shell export，這裡為了讓範例自包含才在程式內設定。
	if _, ok := os.LookupEnv("CONFIG_DIR"); !ok {
		os.Setenv("CONFIG_DIR", "./sample/config/conf")
	}
	if _, ok := os.LookupEnv("PROFILE"); !ok {
		os.Setenv("PROFILE", "local")
	}

	// Default() 內部依序合併：
	//   1) .env            <- 共用
	//   2) .env.<profile>  <- 環境差異 (覆蓋 1)
	//   3) config.<profile>.yaml
	//   4) APP_ 前綴的環境變數 (最高優先序)
	config.Default()

	// --- 用法 1：直接從全域 viper 讀單一鍵值 ---
	fmt.Println("profile          :", config.GetProfile())
	fmt.Println("config dir       :", config.GetConfigDir())
	fmt.Println("server.host      :", viper.GetString("server.host"))
	fmt.Println("server.port      :", viper.GetInt("server.port"))
	fmt.Println("db.default.url   :", viper.GetString("db.default.url"))

	// --- 用法 2：Unmarshal 成強型別 struct ---
	var srv ServerConfig
	if err := viper.UnmarshalKey("server", &srv); err != nil {
		fmt.Println("unmarshal server failed:", err)
		return
	}
	fmt.Printf("server (struct)  : %+v\n", srv)

	// --- 用法 3：透過 db helper 從 viper 取出設定並建立 *gorm.DB ---
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

	// --- 用法 4：印出所有 key/value，方便除錯 ---
	fmt.Println("---- all settings ----")
	for _, k := range viper.AllKeys() {
		fmt.Printf("  %-24s = %v\n", k, viper.Get(k))
	}
}
