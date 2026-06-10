package db

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// resetPostgres 把 PostgreSQL singleton 與 POSTGRES_DSN viper key 還原為測試前狀態。
func resetPostgres(t *testing.T) {
	t.Helper()
	if DefaultPostgres != nil {
		_ = DefaultPostgres.Close()
		DefaultPostgres = nil
	}
	viper.Set("POSTGRES_DSN", "")
}

func TestInitPostgres_EmptyDSNFails(t *testing.T) {
	t.Cleanup(func() { resetPostgres(t) })

	viper.Set("POSTGRES_DSN", "")
	err := InitPostgres()
	if err == nil {
		t.Fatal("expected error for empty POSTGRES_DSN, got nil")
	}
	if !strings.Contains(err.Error(), "POSTGRES_DSN not set") {
		t.Errorf("unexpected error message: %v", err)
	}
	if DefaultPostgres != nil {
		t.Error("DefaultPostgres should remain nil when InitPostgres fails")
	}
}

func TestInitPostgres_InvalidDSNFailsAndLeavesSingletonNil(t *testing.T) {
	t.Cleanup(func() { resetPostgres(t) })

	// pgx 在 DSN 解析階段對無效格式會回傳錯誤;若解析通過但連線失敗,
	// 也會在 gorm.Open 階段回傳錯誤。任一情況都應維持 DefaultPostgres 為 nil。
	viper.Set("POSTGRES_DSN", "this is not a valid DSN")
	err := InitPostgres()
	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}
	if DefaultPostgres != nil {
		t.Error("DefaultPostgres should remain nil when gorm.Open fails")
	}
}

func TestInitPostgres_RefusesDoubleInit(t *testing.T) {
	t.Cleanup(func() { resetPostgres(t) })

	// 模擬一個已初始化的狀態:直接設定 singleton(白箱測試,同 package 可存取)。
	DefaultPostgres = &Postgres{db: nil, dsn: "host=localhost user=postgres dbname=test"}

	viper.Set("POSTGRES_DSN", "host=other user=postgres dbname=other")
	err := InitPostgres()
	if err == nil {
		t.Fatal("expected error for double InitPostgres, got nil")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("unexpected error message: %v", err)
	}
	// 守衛應該在讀 DSN 之前就觸發,所以 dsn 不應被覆寫。
	if DefaultPostgres.dsn != "host=localhost user=postgres dbname=test" {
		t.Errorf("DSN was unexpectedly overwritten to %q", DefaultPostgres.dsn)
	}
}

func TestPostgres_Close_NilDBReturnsError(t *testing.T) {
	p := &Postgres{db: nil}
	err := p.Close()
	if err == nil {
		t.Fatal("expected error when closing Postgres with nil db, got nil")
	}
	if !strings.Contains(err.Error(), "underlying gorm.DB is nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}
