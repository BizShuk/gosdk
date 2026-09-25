package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/db"
	"github.com/bizshuk/gosdk/server"
	"github.com/spf13/viper"
)

func main() {
	// 1. Load Configurations via dual-file loading
	config.Default()

	// 2. Connect DB(僅在 DB_DRIVER 有設定時才初始化)
	//
	// 註：log 套件在 import 時即已透過 init() 完成 slog 設定，
	// LOG_LEVEL / LOG_FORMAT 需在 import 前注入，這裡不再重新初始化。
	if err := initDB(); err != nil {
		slog.Error("DB init failed", "err", err)
		os.Exit(1)
	}

	// 3. Start HTTP Server (blocks until ctx is cancelled by SIGINT/SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx); err != nil {
		slog.Error("Server exited with error", "err", err)
		os.Exit(1)
	}
}

// initDB opens the primary database when DB_DRIVER is configured; the
// sample server runs without a database otherwise.
func initDB() error {
	if viper.GetString(db.KEY_DRIVER) == "" {
		return nil
	}
	if err := db.Init(); err != nil {
		return fmt.Errorf("db: %w", err)
	}
	slog.Debug("DB connected successfully.", "driver", db.Default.Driver())
	return nil
}
