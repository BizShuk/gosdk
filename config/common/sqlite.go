package common

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewSQLite(cfg DBConfig) (*gorm.DB, error) {
	zap.S().Infof("Construct SQLite Connection:%s", cfg.URL)

	db, err := gorm.Open(sqlite.Open(cfg.URL), &gorm.Config{})

	if err != nil {
		return nil, fmt.Errorf("SQLite 連接失敗: %w", err)
	}

	return db, nil
}
