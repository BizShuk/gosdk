package db

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MySQL 是 MySQL 儲存服務,實作了 Service 介面。
//
// 欄位刻意不匯出:外部呼叫端只能透過 DB() 與 Close() 與 singleton 互動,
// 無法直接繞過 InitMySQL() 操作 db / dsn。
type MySQL struct {
	db  *gorm.DB
	dsn string
}

// DB 取得底層 *gorm.DB。
func (m *MySQL) DB() *gorm.DB { return m.db }

// Close 關閉 MySQL 連線。
// 若 MySQL 連線尚未透過 InitMySQL() 初始化,或底層 sql.DB 已被關閉,會回傳錯誤。
func (m *MySQL) Close() error {
	if m.db == nil {
		return fmt.Errorf("db.MySQL: underlying gorm.DB is nil")
	}
	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("db.MySQL: get sql.DB: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("db.MySQL: close: %w", err)
	}
	return nil
}

// DefaultMySQL 是 MySQL 服務的全域 singleton。
// 必須透過 InitMySQL() 初始化後才能使用;在初始化前為 nil。
var DefaultMySQL *MySQL

// InitMySQL 從 viper 讀取 MYSQL_DSN,開啟連線,並設定 DefaultMySQL。
//
// 守護規則與 InitSQLite 對稱:
//   - DefaultMySQL 已被設定時 → 回傳 error 拒絕重複初始化
//   - MYSQL_DSN 未設定或為空字串 → 回傳 error
//   - gorm.Open 失敗 → 回傳錯誤,且 DefaultMySQL 維持 nil
//
// 注意:必須在 config.Default() 呼叫後(讓 viper 載入設定)再呼叫此函式。
func InitMySQL() error {
	if DefaultMySQL != nil {
		return fmt.Errorf("db.MySQL already initialized")
	}

	dsn := viper.GetString("MYSQL_DSN")
	if dsn == "" {
		return fmt.Errorf("db.MySQL: MYSQL_DSN not set")
	}

	slog.Debug("db.MySQL: connecting")
	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("db.MySQL: open: %w", err)
	}

	DefaultMySQL = &MySQL{db: gormDB, dsn: dsn}
	return nil
}
