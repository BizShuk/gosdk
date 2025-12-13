package db

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newMySQLClient 專門建立 MySQL 連接
func NewMySQL(cfg DBConfig) (*gorm.DB, error) {
	// DSN 範例: "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	fmt.Printf("建立 MySQL 連接 (URL: %s)\n", cfg.URL)

	db, err := gorm.Open(mysql.Open(cfg.URL), &gorm.Config{})

	if err != nil {
		return nil, fmt.Errorf("failed to connect DB: %w", err)
	}

	return db, nil
}
