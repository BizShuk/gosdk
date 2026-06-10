package db

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Postgres 是 PostgreSQL 儲存服務,實作了 Service 介面。
//
// 欄位刻意不匯出:外部呼叫端只能透過 DB() 與 Close() 與 singleton 互動,
// 無法直接繞過 InitPostgres() 操作 db / dsn。
//
// DSN 格式:接受 URL 形式 ("postgres://user:pass@host:port/dbname?sslmode=...")
// 或關鍵字形式 ("host=... port=... user=... password=... dbname=... sslmode=..."),
// 由 gorm.io/driver/postgres 自動判斷。
type Postgres struct {
	db  *gorm.DB
	dsn string
}

// DB 取得底層 *gorm.DB。
func (p *Postgres) DB() *gorm.DB { return p.db }

// Close 關閉 PostgreSQL 連線。
// 若 PostgreSQL 連線尚未透過 InitPostgres() 初始化,或底層 sql.DB 已被關閉,會回傳錯誤。
func (p *Postgres) Close() error {
	if p.db == nil {
		return fmt.Errorf("db.Postgres: underlying gorm.DB is nil")
	}
	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("db.Postgres: get sql.DB: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("db.Postgres: close: %w", err)
	}
	return nil
}

// DefaultPostgres 是 PostgreSQL 服務的全域 singleton。
// 必須透過 InitPostgres() 初始化後才能使用;在初始化前為 nil。
var DefaultPostgres *Postgres

// InitPostgres 從 viper 讀取 POSTGRES_DSN,開啟連線,並設定 DefaultPostgres。
//
// 守護規則與 InitSQLite / InitMySQL 對稱:
//   - DefaultPostgres 已被設定時 → 回傳 error 拒絕重複初始化
//   - POSTGRES_DSN 未設定或為空字串 → 回傳 error
//   - gorm.Open 失敗 → 回傳錯誤,且 DefaultPostgres 維持 nil
//
// 注意:必須在 config.Default() 呼叫後(讓 viper 載入設定)再呼叫此函式。
func InitPostgres() error {
	if DefaultPostgres != nil {
		return fmt.Errorf("db.Postgres already initialized")
	}

	dsn := viper.GetString("POSTGRES_DSN")
	if dsn == "" {
		return fmt.Errorf("db.Postgres: POSTGRES_DSN not set")
	}

	zap.S().Infof("db.Postgres: connecting")
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("db.Postgres: open: %w", err)
	}

	DefaultPostgres = &Postgres{db: gormDB, dsn: dsn}
	return nil
}
