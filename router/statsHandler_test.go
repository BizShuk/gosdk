package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bizshuk/gosdk/mw"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestStatsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	viper.Set("Version", "1.0.0")
	viper.Set("PROFILE", "test")
	viper.Set("viper.file", "config.test.yaml")

	s := gin.New()
	s.Use(mw.CorrelationID())
	s.GET("/stats", StatsHandler)

	req, _ := http.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var stats Stats
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if stats.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", stats.Version)
	}
	if stats.Profile != "test" {
		t.Errorf("Expected profile test, got %s", stats.Profile)
	}
	if stats.ConfigFile != "config.test.yaml" {
		t.Errorf("Expected configFile config.test.yaml, got %s", stats.ConfigFile)
	}
	if stats.Status != "OK" {
		t.Errorf("Expected status OK, got %s", stats.Status)
	}
	if stats.CorrelationId == "" {
		t.Error("Expected CorrelationId to be generated, got empty string")
	}
}
