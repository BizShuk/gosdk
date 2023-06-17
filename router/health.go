package router

import (
	"github.com/gin-gonic/gin"
	healthcheck "github.com/tavsec/gin-healthcheck"
	"github.com/tavsec/gin-healthcheck/checks"
	"github.com/tavsec/gin-healthcheck/config"
)

func HealthRouterGroup(s *gin.Engine) {
	healthcheck.New(s, config.DefaultConfig(), []checks.Check{})
}
