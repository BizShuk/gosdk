package router

import "github.com/gin-gonic/gin"

func init() {
	r.GET("user", userHandler)
}

func userHandler(c *gin.Context) { // localhost:8080/user
}
