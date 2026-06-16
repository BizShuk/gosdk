// Package db 提供 gorm 連線的儲存服務(service)。
//
// 每種儲存型態(目前支援 SQLite 與 MySQL)都是一個獨立的 service,
// 有自己的型別、自己的全域 singleton (例如 DefaultSQLite / DefaultMySQL)
// 與自己的 viper key (例如 SQLITE_PATH / MYSQL_DSN)。
//
// micro-service 概念下,一個 process 內不應該存在兩個同型態的 service。
// 因此 InitSQLite() / InitMySQL() 會在第二次呼叫時回傳 error,守護 singleton 不變性。
//
// 典型用法:
//
//	config.Default()
//
//	if viper.IsSet("SQLITE_PATH") {
//	    if err := db.InitSQLite(); err != nil { /* 處理錯誤 */ }
//	}
//	if viper.IsSet("MYSQL_DSN") {
//	    if err := db.InitMySQL(); err != nil { /* 處理錯誤 */ }
//	}
//
//	// 之後任何地方透過 singleton 取用 *gorm.DB:
//	gormDB := db.DefaultSQLite.DB()
package db

import "gorm.io/gorm"

// Service 是儲存服務的通用介面。
//
// 任何實作了 DB() 與 Close() 的儲存型別都可視為 db service。
// DB() 取得底層 *gorm.DB;Close() 釋放連線資源。
type Service interface {
	DB() *gorm.DB
	Close() error
}
