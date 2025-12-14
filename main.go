package main

import (
	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/router"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func main() {
	config.Default()
}

func HTTPServer() {
	s := gin.Default()
	router.Default(s)
	err := s.Run("localhost:8080") // localhost:8080
	if err != nil {
		log.Fatal("Server failed to start...")
	}
}
