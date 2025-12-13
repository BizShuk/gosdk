package router

import (
	"net/http"

	"github.com/bizshuk/gosdk/mw"
	"github.com/gin-gonic/gin"

	"github.com/spf13/viper"
	// stats "github.com/semihalev/gin-stats"
)

type Stats struct {
	Version       string `json:"version"`
	Profile       string `json:"profile"`
	ConfigFile    string `json:"configFile"`
	Status        string `json:"status"`
	CorrelationId string `json:"correlationId"`
}

func StatsHandler(c *gin.Context) {
	stats := &Stats{
		Version:       viper.GetString("Version"),
		Profile:       viper.GetString("PROFILE"),
		ConfigFile:    viper.GetString("viper.file"),
		Status:        GetStatus(),
		CorrelationId: mw.GetCorrelationID(c),
	}

	c.JSON(http.StatusOK, stats)
}

func GetStatus() string {
	return "OK"
}
