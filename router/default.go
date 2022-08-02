package router

import (
	"net/http"

	helmet "github.com/danielkov/gin-helmet"
	"github.com/gin-gonic/gin"
)

var r *gin.Engine = gin.Default()

func Default() *gin.Engine {

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.Use(helmet.Default())
	return r
}
