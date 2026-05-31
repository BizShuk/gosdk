package main

import (
	"fmt"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/common"
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

	// 3. Connect DB
	dbConfigs := viper.GetStringMap("db")
	if len(dbConfigs) > 0 {
		_, err := common.NewDBConfig("default").Create()
		if err != nil {
			zap.S().Errorf("Database connection failed: %v", err)
		} else {
			zap.S().Info("Database connected successfully.")
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
