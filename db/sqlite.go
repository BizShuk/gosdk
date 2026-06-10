package db

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SQLite 是 SQLite 儲存服務,實作了 Service 介面。
//
// 欄位刻意不匯出:外部呼叫端只能透過 DB() 與 Close() 與 singleton 互動,
// 無法直接繞過 InitSQLite() 操作 db / path,符合「singleton 不可繞過建構式」的精神。
type SQLite struct {
	db   *gorm.DB
	path string
}

// DB 取得底層 *gorm.DB。
func (s *SQLite) DB() *gorm.DB { return s.db }

// Close 關閉 SQLite 連線。
// 若 SQLite 連線尚未透過 InitSQLite() 初始化,或底層 sql.DB 已被關閉,會回傳錯誤。
func (s *SQLite) Close() error {
	if s.db == nil {
		return fmt.Errorf("db.SQLite: underlying gorm.DB is nil")
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("db.SQLite: get sql.DB: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("db.SQLite: close: %w", err)
	}
	return nil
}

// DefaultSQLite 是 SQLite 服務的全域 singleton。
//
// 「Default」前綴強調這是慣例存取點,而非「唯一存取點」——
// 仍然不鼓勵建立第二個 *SQLite(違背 micro-service 概念)。
// 必須透過 InitSQLite() 初始化後才能使用;在初始化前為 nil。
var DefaultSQLite *SQLite

// InitSQLite 從 viper 讀取 SQLITE_PATH,開啟連線,並設定 DefaultSQLite。
//
// 守護規則:
//   - DefaultSQLite 已被設定時(已經 Init 過)→ 回傳 error 拒絕重複初始化
//   - SQLITE_PATH 未設定或為空字串 → 回傳 error
//   - gorm.Open 失敗 → 回傳錯誤,且 DefaultSQLite 維持 nil(不留下半毀 singleton)
//
// 注意:必須在 config.Default() 呼叫後(讓 viper 載入設定)再呼叫此函式。
func InitSQLite() error {
	if DefaultSQLite != nil {
		return fmt.Errorf("db.SQLite already initialized")
	}

	path := viper.GetString("SQLITE_PATH")
	if path == "" {
		return fmt.Errorf("db.SQLite: SQLITE_PATH not set")
	}

	zap.S().Infof("db.SQLite: opening %s", path)
	gormDB, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("db.SQLite: open %s: %w", path, err)
	}

	DefaultSQLite = &SQLite{db: gormDB, path: path}
	return nil
}
