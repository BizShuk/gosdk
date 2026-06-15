package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/db"
	"github.com/bizshuk/gosdk/log"
	"github.com/bizshuk/gosdk/mw"
	"github.com/bizshuk/gosdk/router"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func main() {
	// 1. Load Configurations via dual-file loading
	config.Default()

	// 2. Re-initialize log systems with configuration level
	log.Init()
	slog.Debug("Configurations loaded successfully.")

	// 3. Connect DB(僅在對應 viper key 有設定時才初始化)
	if viper.IsSet("SQLITE_PATH") {
		if err := db.InitSQLite(); err != nil {
			slog.Error("SQLite connection failed", "err", err)
		} else {
			slog.Debug("SQLite connected successfully.")
		}
	}
	if viper.IsSet("MYSQL_DSN") {
		if err := db.InitMySQL(); err != nil {
			slog.Error("MySQL connection failed", "err", err)
		} else {
			slog.Debug("MySQL connected successfully.")
		}
	}
	if viper.IsSet("POSTGRES_DSN") {
		if err := db.InitPostgres(); err != nil {
			slog.Error("PostgreSQL connection failed", "err", err)
		} else {
			slog.Debug("PostgreSQL connected successfully.")
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

	slog.Debug("Server starting", "addr", addr)
	err := s.Run(addr)
	if err != nil {
		slog.Error("Server failed to start", "err", err)
		os.Exit(1)
	}
}
