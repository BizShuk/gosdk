// Package db 提供由設定決定的 gorm 連線服務(service)。
//
// 一個服務只連一個資料庫:driver 與位址全由兩個扁平 viper key 決定,
// 程式碼裡沒有「哪個 key 有值就用哪個」的分支。
//
//	DB_DRIVER = mysql | sqlite
//	DB_DSN    = driver 自己的 DSN 字串
//
// gorm 行為同樣由設定開關:
//
//	DB_LOG             = false (預設,丟棄 gorm log) | true (gorm 預設 logger)
//	DB_TRANSLATE_ERROR = true (預設,driver 錯誤轉成 gorm.ErrDuplicatedKey 等) | false
//
// 需要額外的資料庫連線時(必須是明確的決定,不是預設),以 suffix _<NAME> 區分。
// 每個 key 的 <KEY>_<NAME> 未設定時沿用 <KEY>;唯一的例外是 DSN,
// DB_DSN_<NAME> 不會沿用主連線的位址:mysql 必填,sqlite 空值推導 data/default.db。
//
// 典型用法:
//
//	config.Default(config.WithAppName("myapp"))
//	if err := db.Init(); err != nil { /* 處理錯誤 */ }
//	gormDB := db.Default.DB()
//
//	cache, err := db.Open("cache") // DB_DRIVER_CACHE=sqlite, DSN 空值 → data/default.db
//	trifecta, err := db.Open("trifecta") // 讀 DB_DSN_TRIFECTA
package db

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	KEY_DRIVER          = "DB_DRIVER"
	KEY_DSN             = "DB_DSN"
	KEY_LOG             = "DB_LOG"
	KEY_TRANSLATE_ERROR = "DB_TRANSLATE_ERROR"

	DEFAULT_SQLITE_FILE = "default.db"

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
	target string
}

// DB 取得底層 *gorm.DB。
func (s *Service) DB() *gorm.DB { return s.db }

// Driver 回傳這條連線的 driver (mysql 或 sqlite)。
func (s *Service) Driver() string { return s.driver }

// Target 回報這條連線連到哪裡,供啟動 log 與畫面標示:
// mysql 為遮掉密碼的 DSN,sqlite 為實際檔案路徑(含推導出來的預設路徑)。
func (s *Service) Target() string { return s.target }

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
// name 為空時讀主連線的 key,與 Init() 相同但不設定 Default;
// 否則讀 <KEY>_<NAME>,除 DSN 外未設定時沿用 <KEY>。
// NAME 為 name 轉大寫, "-" 換成 "_"。
//
// DSN 為空時:driver 為 sqlite 則推導為 <app config>/data/default.db
// (主連線與具名連線相同);mysql 一律報錯。
func Open(name string) (*Service, error) {
	driverKey, dsnKey := Key(KEY_DRIVER, name), Key(KEY_DSN, name)

	driver := setting(KEY_DRIVER, name)
	if driver == "" {
		return nil, fmt.Errorf("db.Open: %s not set (want %s|%s)", driverKey, DRIVER_MYSQL, DRIVER_SQLITE)
	}

	dsn := strings.TrimSpace(viper.GetString(dsnKey))
	if dsn == "" && driver == DRIVER_SQLITE {
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
	gormConfig, err := gormConfigFor(name)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}

	target := targetOf(driver, dsn)
	slog.Debug("db.Open: connecting", "name", name, "driver", driver, "target", target)
	gormDB, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %s open %s: %w", driver, target, err)
	}
	return &Service{db: gormDB, name: name, driver: driver, target: target}, nil
}

// Key 回傳 base key 在 name 這條連線上的名字:name 為空即 base 本身,
// 否則為 <base>_<NAME>。呼叫端組錯誤訊息時用它,不要自己拼字串。
func Key(base, name string) string {
	if name == "" {
		return base
	}
	return base + "_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// setting 讀 name 這條連線的設定:先看 <base>_<NAME>,空值再沿用 <base>。
func setting(base, name string) string {
	if name != "" {
		if v := strings.TrimSpace(viper.GetString(Key(base, name))); v != "" {
			return v
		}
	}
	return strings.TrimSpace(viper.GetString(base))
}

// flag 以 setting 的沿用規則讀布林開關;未設定回傳 def,無法解析即報錯。
func flag(base, name string, def bool) (bool, error) {
	raw := setting(base, name)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s=%q is not a bool", Key(base, name), raw)
	}
	return v, nil
}

func gormConfigFor(name string) (*gorm.Config, error) {
	logOn, err := flag(KEY_LOG, name, false)
	if err != nil {
		return nil, err
	}
	translate, err := flag(KEY_TRANSLATE_ERROR, name, true)
	if err != nil {
		return nil, err
	}
	cfg := &gorm.Config{TranslateError: translate, Logger: logger.Discard}
	if logOn {
		cfg.Logger = logger.Default
	}
	return cfg, nil
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

// targetOf 產生可以進 log 的連線描述。mysql 的 go-sql-driver DSN
// (user:pass@tcp(host:port)/db?params) 只遮掉密碼;認不得的格式原樣回傳,
// 這裡只遮罩不驗證。
func targetOf(driver, dsn string) string {
	if driver != DRIVER_MYSQL {
		return dsn
	}
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return dsn
	}
	colon := strings.Index(dsn[:at], ":")
	if colon < 0 {
		return dsn
	}
	return dsn[:colon] + ":***" + dsn[at:]
}

// defaultSQLitePath 回傳 <app config>/data/default.db,並確保 data/ 存在。
func defaultSQLitePath() (string, error) {
	if config.GetAppConfigDir() == "" {
		return "", fmt.Errorf("no app config dir to derive a sqlite path (call config.Default with an app name)")
	}
	dir := config.GetAppDataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return filepath.Join(dir, DEFAULT_SQLITE_FILE), nil
}
