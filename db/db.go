// Package db 提供由設定決定的 gorm 連線服務(service)。
//
// 一個服務只連一個資料庫:driver 與位址全由兩個扁平 viper key 決定,
// 程式碼裡沒有「哪個 key 有值就用哪個」的分支。
//
//	DB_DRIVER = mysql | sqlite
//	DB_DSN    = driver 自己的 DSN 字串
//
// 需要額外的資料庫連線時(必須是明確的決定,不是預設),以 suffix 區分:
//
//	DB_DSN_<NAME>    = 該連線的 DSN
//	DB_DRIVER_<NAME> = 該連線的 driver;未設定時沿用 DB_DRIVER
//
// 典型用法:
//
//	config.Default(config.WithAppName("myapp"))
//	if err := db.Init(); err != nil { /* 處理錯誤 */ }
//	gormDB := db.Default.DB()
//
//	trifecta, err := db.Open("trifecta") // 讀 DB_DSN_TRIFECTA
package db

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	KEY_DRIVER = "DB_DRIVER"
	KEY_DSN    = "DB_DSN"

	DRIVER_MYSQL  = "mysql"
	DRIVER_SQLITE = "sqlite"
)

// Service 是一條 gorm 連線。
//
// 欄位刻意不匯出:外部只能透過 Init() / Open() 建立,
// 再以 DB() 與 Close() 操作。
type Service struct {
	db     *gorm.DB
	name   string
	driver string
}

// DB 取得底層 *gorm.DB。
func (s *Service) DB() *gorm.DB { return s.db }

// Driver 回傳這條連線的 driver (mysql 或 sqlite)。
func (s *Service) Driver() string { return s.driver }

// Close 關閉連線。
func (s *Service) Close() error {
	if s.db == nil {
		return fmt.Errorf("db.Service(%s): underlying gorm.DB is nil", s.label())
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("db.Service(%s): get sql.DB: %w", s.label(), err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("db.Service(%s): close: %w", s.label(), err)
	}
	return nil
}

func (s *Service) label() string {
	if s.name == "" {
		return "default"
	}
	return s.name
}

// Default 是服務主資料庫的全域 singleton,由 Init() 設定;初始化前為 nil。
var Default *Service

// Init 依 DB_DRIVER / DB_DSN 開啟主資料庫並設定 Default。
// 重複呼叫回傳 error;失敗時 Default 維持 nil。
//
// 必須在 config.Default() 之後呼叫。
func Init() error {
	if Default != nil {
		return fmt.Errorf("db: already initialized")
	}
	svc, err := Open("")
	if err != nil {
		return err
	}
	Default = svc
	return nil
}

// Open 開啟一條連線並交給呼叫端持有。
//
// name 為空時讀 DB_DRIVER / DB_DSN,與 Init() 相同但不設定 Default;
// 否則讀 DB_DSN_<NAME> 與 DB_DRIVER_<NAME>(未設定時沿用 DB_DRIVER)。
// NAME 為 name 轉大寫, "-" 換成 "_"。
//
// DSN 為空時:主連線且 driver 為 sqlite,推導為 <app config>/data/<app_name>.db;
// 其餘情況一律報錯。
func Open(name string) (*Service, error) {
	driverKey, dsnKey := Keys(name)

	driver := strings.TrimSpace(viper.GetString(driverKey))
	if driver == "" && name != "" {
		driver = strings.TrimSpace(viper.GetString(KEY_DRIVER))
	}
	if driver == "" {
		return nil, fmt.Errorf("db.Open: %s not set (want %s|%s)", driverKey, DRIVER_MYSQL, DRIVER_SQLITE)
	}

	dsn := strings.TrimSpace(viper.GetString(dsnKey))
	if dsn == "" && name == "" && driver == DRIVER_SQLITE {
		path, err := defaultSQLitePath()
		if err != nil {
			return nil, fmt.Errorf("db.Open: %s not set: %w", dsnKey, err)
		}
		dsn = path
	}
	if dsn == "" {
		return nil, fmt.Errorf("db.Open: %s not set", dsnKey)
	}

	dialector, err := dialectorFor(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %s: %w", driverKey, err)
	}

	slog.Debug("db.Open: connecting", "name", name, "driver", driver)
	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("db.Open: %s open: %w", driver, err)
	}
	return &Service{db: gormDB, name: name, driver: driver}, nil
}

// Keys 回傳某條連線讀取的 driver 與 DSN key。name 為空即主連線。
func Keys(name string) (driverKey, dsnKey string) {
	if name == "" {
		return KEY_DRIVER, KEY_DSN
	}
	suffix := "_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	return KEY_DRIVER + suffix, KEY_DSN + suffix
}

func dialectorFor(driver, dsn string) (gorm.Dialector, error) {
	switch driver {
	case DRIVER_MYSQL:
		return mysql.Open(dsn), nil
	case DRIVER_SQLITE:
		return sqlite.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported driver %q (want %s|%s)", driver, DRIVER_MYSQL, DRIVER_SQLITE)
	}
}

// defaultSQLitePath 回傳 <app config>/data/<app_name>.db,並確保 data/ 存在。
func defaultSQLitePath() (string, error) {
	if config.GetAppConfigDir() == "" || config.GetAppName() == "" {
		return "", fmt.Errorf("no app config dir to derive a sqlite path (call config.Default with an app name)")
	}
	dir := config.GetAppDataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return filepath.Join(dir, config.GetAppName()+".db"), nil
}
