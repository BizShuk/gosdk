package mw

import (
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

const (
	CorrelationHeader = "X-Correlation-Id"
)

func CorrelationID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		cid := ctx.Request.Header.Get(CorrelationHeader)
		if len(cid) == 0 {
			u := uuid.NewV4().String()
			ctx.Request.Header.Add(CorrelationHeader, u)
		}
	}
}

func GetCorrelationID(ctx *gin.Context) string {
	return ctx.Request.Header.Get(CorrelationHeader)
}
