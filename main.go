package main

import (
	"fmt"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/db"
	"github.com/bizshuk/gosdk/log"
	"github.com/bizshuk/gosdk/mw"
	"github.com/bizshuk/gosdk/router"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	// 1. Load Configurations via dual-file loading
	config.Default()

	// 2. Re-initialize log systems with configuration level
	log.Init()
	zap.S().Info("Configurations loaded successfully.")

	// 3. Connect DB(僅在對應 viper key 有設定時才初始化)
	if viper.IsSet("SQLITE_PATH") {
		if err := db.InitSQLite(); err != nil {
			zap.S().Errorf("SQLite connection failed: %v", err)
		} else {
			zap.S().Info("SQLite connected successfully.")
		}
	}
	if viper.IsSet("MYSQL_DSN") {
		if err := db.InitMySQL(); err != nil {
			zap.S().Errorf("MySQL connection failed: %v", err)
		} else {
			zap.S().Info("MySQL connected successfully.")
		}
	}
	if viper.IsSet("POSTGRES_DSN") {
		if err := db.InitPostgres(); err != nil {
			zap.S().Errorf("PostgreSQL connection failed: %v", err)
		} else {
			zap.S().Info("PostgreSQL connected successfully.")
		}
	}

	// 4. Start HTTP Server
	HTTPServer()
}

func HTTPServer() {
	s := gin.Default()
	s.Use(mw.CorrelationID())
	s.Use(mw.Helmet())

	router.Default(s)
	router.HealthRouterGroup(s)
	router.PingRouterGroup(s)

	host := viper.GetString("server.host")
	port := viper.GetInt("server.port")
	if port == 0 {
		port = 8080
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	zap.S().Infof("Server starting on %s", addr)
	err := s.Run(addr)
	if err != nil {
		zap.S().Fatalf("Server failed to start: %v", err)
	}
}
