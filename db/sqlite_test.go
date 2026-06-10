package db

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// resetSQLite 把 SQLite singleton 與 SQLITE_PATH viper key 還原為測試前狀態。
// 用 t.Cleanup 確保測試結束後(無論 pass / fail)都會跑。
func resetSQLite(t *testing.T) {
	t.Helper()
	if DefaultSQLite != nil {
		_ = DefaultSQLite.Close()
		DefaultSQLite = nil
	}
	viper.Set("SQLITE_PATH", "")
}

func TestInitSQLite_ReadsSQLitePathFromViper(t *testing.T) {
	t.Cleanup(func() { resetSQLite(t) })

	viper.Set("SQLITE_PATH", ":memory:")
	if err := InitSQLite(); err != nil {
		t.Fatalf("InitSQLite failed: %v", err)
	}
	if DefaultSQLite == nil {
		t.Fatal("DefaultSQLite is nil after successful InitSQLite")
	}
	if DefaultSQLite.path != ":memory:" {
		t.Errorf("expected path :memory:, got %q", DefaultSQLite.path)
	}
}

func TestInitSQLite_EmptyPathFails(t *testing.T) {
	t.Cleanup(func() { resetSQLite(t) })

	viper.Set("SQLITE_PATH", "")
	err := InitSQLite()
	if err == nil {
		t.Fatal("expected error for empty SQLITE_PATH, got nil")
	}
	if !strings.Contains(err.Error(), "SQLITE_PATH not set") {
		t.Errorf("unexpected error message: %v", err)
	}
	if DefaultSQLite != nil {
		t.Error("DefaultSQLite should remain nil when InitSQLite fails")
	}
}

func TestInitSQLite_RefusesDoubleInit(t *testing.T) {
	t.Cleanup(func() { resetSQLite(t) })

	viper.Set("SQLITE_PATH", ":memory:")
	if err := InitSQLite(); err != nil {
		t.Fatalf("first InitSQLite failed: %v", err)
	}

	// 第二次 init:即使換 path 也不允許。
	viper.Set("SQLITE_PATH", ":memory:")
	err := InitSQLite()
	if err == nil {
		t.Fatal("expected error for double InitSQLite, got nil")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSQLite_DB_ReturnsWorkingGormDB(t *testing.T) {
	t.Cleanup(func() { resetSQLite(t) })

	viper.Set("SQLITE_PATH", ":memory:")
	if err := InitSQLite(); err != nil {
		t.Fatalf("InitSQLite failed: %v", err)
	}

	gormDB := DefaultSQLite.DB()
	if gormDB == nil {
		t.Fatal("DB() returned nil")
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("underlying sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Errorf("ping failed on fresh SQLite: %v", err)
	}
}

func TestSQLite_Close_ClosesUnderlyingDB(t *testing.T) {
	t.Cleanup(func() { resetSQLite(t) })

	viper.Set("SQLITE_PATH", ":memory:")
	if err := InitSQLite(); err != nil {
		t.Fatalf("InitSQLite failed: %v", err)
	}

	if err := DefaultSQLite.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Close 後,底層 sql.DB 應該視為已關閉,Ping 應該回傳錯誤。
	sqlDB, err := DefaultSQLite.DB().DB()
	if err != nil {
		t.Fatalf("get underlying sql.DB after Close: %v", err)
	}
	if err := sqlDB.Ping(); err == nil {
		t.Error("expected ping to fail after Close, got nil error")
	}
}

func TestSQLite_Close_NilDBReturnsError(t *testing.T) {
	s := &SQLite{db: nil}
	err := s.Close()
	if err == nil {
		t.Fatal("expected error when closing SQLite with nil db, got nil")
	}
	if !strings.Contains(err.Error(), "underlying gorm.DB is nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}
