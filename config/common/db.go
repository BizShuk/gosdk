package common

import (
	"fmt"

	"github.com/bizshuk/gosdk/log"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func NewDBConfig(confKey string) DBConfig {
	confKey = "db." + confKey
	dbConfig := DBConfig{}
	if err := viper.UnmarshalKey(confKey, &dbConfig); err != nil {
		log.Fatalf("Unable to unmarshal db key: %v", err)
	}
	log.Infof("Load DBConfig: %+v", dbConfig)
	return dbConfig
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
