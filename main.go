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

	// 2. Connect DB(僅在對應 viper key 有設定時才初始化)
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

// initDB conditionally initialises each storage backend whose viper key is
// set. The errors are wrapped with `fmt.Errorf` to preserve the underlying
// failure context.
func initDB() error {
	if viper.IsSet("SQLITE_PATH") {
		if err := db.InitSQLite(); err != nil {
			return fmt.Errorf("sqlite: %w", err)
		}
		slog.Debug("SQLite connected successfully.")
	}
	if viper.IsSet("MYSQL_DSN") {
		if err := db.InitMySQL(); err != nil {
			return fmt.Errorf("mysql: %w", err)
		}
		slog.Debug("MySQL connected successfully.")
	}
	if viper.IsSet("POSTGRES_DSN") {
		if err := db.InitPostgres(); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		slog.Debug("PostgreSQL connected successfully.")
	}
	return nil
}
