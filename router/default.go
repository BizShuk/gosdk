package router

import (
	"github.com/gin-gonic/gin"
)

func Default(s *gin.Engine) {
	s.GET("/stats", StatsHandler) // http://localhost:8080/stats
}
