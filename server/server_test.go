package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestAssembleEngine_Ping(t *testing.T) {
	engine := assembleEngine()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping/", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["message"] != "pong" {
		t.Errorf("ping body: want pong, got %q", body["message"])
	}
}

func TestAssembleEngine_Health(t *testing.T) {
	engine := assembleEngine()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestAssembleEngine_Stats(t *testing.T) {
	engine := assembleEngine()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}
	if body["status"] != "OK" {
		t.Errorf("stats status field: want OK, got %v", body["status"])
	}
}

func TestAssembleEngine_NotFound(t *testing.T) {
	engine := assembleEngine()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/this-does-not-exist", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rec.Code)
	}
}

func TestRun_GracefulShutdown(t *testing.T) {
	// Pick an ephemeral port via :0 isn't directly supported by Run
	// (which reads server.port from viper), so we use 127.0.0.1:0 trick
	// indirectly: configure viper to a high random port and rely on Run
	// binding it. Since Run uses viper, we exercise Run against a
	// dedicated high port here.
	viper.Set("server.host", "127.0.0.1")
	viper.Set("server.port", 18080)
	t.Cleanup(func() {
		viper.Set("server.port", 8080)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- Run(ctx) }()

	// Give the listener time to bind.
	time.Sleep(150 * time.Millisecond)

	start := time.Now()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("shutdown took too long: %v", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of ctx cancellation")
	}
}
