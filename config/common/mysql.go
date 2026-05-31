package common

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// NewMySQL 專門建立 MySQL 連接 newMySQLClient
func NewMySQL(cfg DBConfig) (*gorm.DB, error) {
	// DSN 範例: "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	zap.S().Infof("建立 MySQL 連接 (URL: %s)", cfg.URL)

	db, err := gorm.Open(mysql.Open(cfg.URL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect DB: %w", err)
	}

	return db, nil
}
