package db

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// resetMySQL 把 MySQL singleton 與 MYSQL_DSN viper key 還原為測試前狀態。
func resetMySQL(t *testing.T) {
	t.Helper()
	if DefaultMySQL != nil {
		_ = DefaultMySQL.Close()
		DefaultMySQL = nil
	}
	viper.Set("MYSQL_DSN", "")
}

func TestInitMySQL_EmptyDSNFails(t *testing.T) {
	t.Cleanup(func() { resetMySQL(t) })

	viper.Set("MYSQL_DSN", "")
	err := InitMySQL()
	if err == nil {
		t.Fatal("expected error for empty MYSQL_DSN, got nil")
	}
	if !strings.Contains(err.Error(), "MYSQL_DSN not set") {
		t.Errorf("unexpected error message: %v", err)
	}
	if DefaultMySQL != nil {
		t.Error("DefaultMySQL should remain nil when InitMySQL fails")
	}
}

func TestInitMySQL_InvalidDSNFailsAndLeavesSingletonNil(t *testing.T) {
	t.Cleanup(func() { resetMySQL(t) })

	// 格式錯誤的 DSN:gorm.Open 會回傳錯誤,且 DefaultMySQL 應維持 nil。
	viper.Set("MYSQL_DSN", "this is not a valid DSN")
	err := InitMySQL()
	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}
	if DefaultMySQL != nil {
		t.Error("DefaultMySQL should remain nil when gorm.Open fails")
	}
}

func TestInitMySQL_RefusesDoubleInit(t *testing.T) {
	t.Cleanup(func() { resetMySQL(t) })

	// 模擬一個已初始化的狀態:直接設定 singleton(白箱測試,同 package 可存取)。
	// 這讓我們能在沒有真實 MySQL 的情況下,單獨驗證「already initialized」守衛。
	DefaultMySQL = &MySQL{db: nil, dsn: "user:pass@tcp(host:3306)/db"}

	viper.Set("MYSQL_DSN", "user:pass@tcp(other:3306)/db")
	err := InitMySQL()
	if err == nil {
		t.Fatal("expected error for double InitMySQL, got nil")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("unexpected error message: %v", err)
	}
	// 守衛應該在讀 DSN 之前就觸發,所以 dsn 不應被覆寫。
	if DefaultMySQL.dsn != "user:pass@tcp(host:3306)/db" {
		t.Errorf("DSN was unexpectedly overwritten to %q", DefaultMySQL.dsn)
	}
}

func TestMySQL_Close_NilDBReturnsError(t *testing.T) {
	m := &MySQL{db: nil}
	err := m.Close()
	if err == nil {
		t.Fatal("expected error when closing MySQL with nil db, got nil")
	}
	if !strings.Contains(err.Error(), "underlying gorm.DB is nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}
