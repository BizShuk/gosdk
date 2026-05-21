package main

import (
	"fmt"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/db"
	"github.com/bizshuk/gosdk/log"
	"github.com/bizshuk/gosdk/mw"
	"github.com/bizshuk/gosdk/router"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Load Configurations
	config.Default()

	// 2. Re-initialize log systems with configuration level
	log.Init()
	log.Info("Configurations loaded successfully.")

	// 3. Connect DB
	if len(config.GlobalConfig.DB) > 0 {
		_, err := db.NewDBConfig("default").Create()
		if err != nil {
			log.Errorf("Database connection failed: %v", err)
		} else {
			log.Info("Database connected successfully.")
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

	host := config.GlobalConfig.Server.Host
	port := config.GlobalConfig.Server.Port
	if port == 0 {
		port = 8080
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	log.Infof("Server starting on %s", addr)
	err := s.Run(addr)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
