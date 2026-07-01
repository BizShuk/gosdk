// Package server hosts the default HTTP server entry point for gosdk.
// The Run function is designed to be invokable from any host application
// (including external Go projects that import this module) and supports
// graceful shutdown via the supplied context.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bizshuk/gosdk/mw"
	"github.com/bizshuk/gosdk/router"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const (
	defaultShutdownTimeout = 10 * time.Second
	defaultServerPort      = 8080
)

// Run starts the default HTTP server and blocks until the supplied context
// is cancelled. On cancellation it triggers a graceful shutdown with a
// 10-second deadline. The returned error is non-nil if the listener
// fails to bind, returns a non-ErrServerClosed error, or shutdown itself
// fails.
func Run(ctx context.Context) error {
	engine := assembleEngine()

	addr := fmt.Sprintf("%s:%d",
		viper.GetString("server.host"),
		serverPort(),
	)

	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	listenErr := make(chan error, 1)
	go func() {
		slog.Debug("Server starting", "addr", addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
			return
		}
		listenErr <- nil
	}()

	select {
	case err := <-listenErr:
		// Listener exited before ctx cancellation (e.g. port already in
		// use). Nothing to shut down.
		if err != nil {
			return fmt.Errorf("server listener: %w", err)
		}
		return nil
	case <-ctx.Done():
		slog.Debug("Server shutdown requested", "reason", ctx.Err())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	// Drain the listener goroutine result (always nil at this point).
	<-listenErr
	slog.Debug("Server stopped")
	return nil
}

// assembleEngine builds the gin.Engine with the standard middleware stack
// and the documented router groups (/stats, /healthz, /ping).
//
// Kept package-private so external callers cannot depend on the specific
// Engine layout; tests use the same package to access it.
func assembleEngine() *gin.Engine {
	s := gin.Default()
	s.Use(mw.CorrelationID())
	s.Use(mw.Helmet())

	router.Default(s)
	router.HealthRouterGroup(s)
	router.PingRouterGroup(s)

	return s
}

func serverPort() int {
	port := viper.GetInt("server.port")
	if port == 0 {
		return defaultServerPort
	}
	return port
}
