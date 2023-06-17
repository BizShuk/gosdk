package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func PingRouterGroup(s *gin.Engine) {
	pingGroup := s.Group("ping")
	pingGroup.GET("/", ping)
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}
