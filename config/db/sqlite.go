package db

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewSQLite(cfg DBConfig) (*gorm.DB, error) {
	fmt.Printf("Construct SQLite Connection:%s\n", cfg.URL)

	db, err := gorm.Open(sqlite.Open(cfg.URL), &gorm.Config{})

	if err != nil {
		return nil, fmt.Errorf("MySQL 連接失敗: %w", err)
	}

	return db, nil
}
