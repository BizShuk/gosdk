package db

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func NewDBConfig(confKey string) (*DBConfig, error) {
	confKey += "db." + confKey
	fmt.Println("DBConfig:", viper.Get(confKey))
	dbConfig := DBConfig{}
	if err := viper.UnmarshalKey(confKey, &dbConfig); err != nil {
		log.Fatalf("Unable to unmarshal server key: %v", err)
	}
	fmt.Println(dbConfig)
	return &dbConfig, nil
}

type DBConfig struct {
	Driver string `mapstructure:"driver"`
	URL    string `mapstructure:"url"` // 這裡通常是完整的 DSN 字串
}

func (d DBConfig) Create() (*gorm.DB, error) {
	return DatabaseFactory(d)
}

func DatabaseFactory(cfg DBConfig) (*gorm.DB, error) {
	switch cfg.Driver {
	case "sqlite":
		return NewSQLite(cfg)
	case "mysql":
		return NewMySQL(cfg)
	default:
		return nil, fmt.Errorf("不支持的資料庫驅動: %s", cfg.Driver)
	}
}
