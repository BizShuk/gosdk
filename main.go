package main

import (
	"github.com/bizshuk/gin_default/config"
	"github.com/bizshuk/gin_default/router"
	log "github.com/sirupsen/logrus"
)

func main() {
	config.Default()
	r := router.Default()
	err := r.Run("localhost:8080") // localhost:8080
	if err != nil {
		log.Fatal("Server failed to start...")
	}
}
