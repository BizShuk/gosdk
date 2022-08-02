package router

import (
	"net/http"

	"github.com/bizshuk/gin_default/config"
	"github.com/gin-gonic/gin"

	"github.com/spf13/viper"
	// stats "github.com/semihalev/gin-stats"
)

func init() {
	r.GET("/health", StatsHandler) // http://localhost:8080/health
	r.GET("/stats", StatsHandler)  // http://localhost:8080/stats
	r.GET("/hello", HelloHandler)  // http://localhost:8080/hello

	// r.Use(stats.RequestStats())
	// r.GET("/stats", func(c *gin.Context) {
	// 	c.JSON(http.StatusOK, stats.Report())
	// })
}

type Stats struct {
	Version    string `json:"version"`
	Profile    string `json:"profile"`
	ConfigFile string `json:"configFile"`
	Status     string `json:"status"`
}

func StatsHandler(c *gin.Context) {
	stats := &Stats{
		Version:    config.Version,
		Profile:    config.Profile,
		ConfigFile: viper.GetString("viper.file"),
		Status:     GetStatus(),
	}

	c.JSON(http.StatusOK, stats)
}

func GetStatus() string {
	return "OK"
}

func HelloHandler(c *gin.Context) {
	c.String(200, "Hello!!")
}
