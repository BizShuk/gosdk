package router

import (
	"github.com/bizshuk/gin_default/middleware"
	helmet "github.com/danielkov/gin-helmet"
	"github.com/gin-gonic/gin"
)

var r *gin.Engine = gin.Default()

func Default() *gin.Engine {
	r.Use(helmet.Default())
	r.Use(middleware.CorrelationID())

	r.GET("user", userHandler)
	r.GET("/stats", StatsHandler)  // http://localhost:8080/stats
	r.GET("/health", HelloHandler) // http://localhost:8080/health
	return r
}
