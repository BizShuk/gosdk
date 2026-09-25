package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/viper"
)

// resetDB 還原 Default singleton 與本套件讀取的 viper key。
func resetDB(t *testing.T, keys ...string) {
	t.Helper()
	t.Cleanup(func() {
		if Default != nil {
			_ = Default.Close()
			Default = nil
		}
		for _, k := range append([]string{KEY_DRIVER, KEY_DSN}, keys...) {
			viper.Set(k, "")
		}
	})
}

func TestInit_SQLiteFromDSN(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_DRIVER, DRIVER_SQLITE)
	viper.Set(KEY_DSN, ":memory:")

	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if Default == nil || Default.Driver() != DRIVER_SQLITE {
		t.Fatalf("Default = %+v, want sqlite service", Default)
	}
	sqlDB, err := Default.DB().DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Errorf("ping: %v", err)
	}
}

func TestInit_RefusesDoubleInit(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_DRIVER, DRIVER_SQLITE)
	viper.Set(KEY_DSN, ":memory:")

	if err := Init(); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	first := Default
	if err := Init(); err == nil || !strings.Contains(err.Error(), "already initialized") {
		t.Fatalf("second Init err = %v, want already initialized", err)
	}
	if Default != first {
		t.Error("Default was replaced by the second Init")
	}
}

func TestInit_MissingDriverFails(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_DSN, ":memory:")

	err := Init()
	if err == nil || !strings.Contains(err.Error(), "DB_DRIVER not set") {
		t.Fatalf("err = %v, want DB_DRIVER not set", err)
	}
	if Default != nil {
		t.Error("Default should stay nil on failure")
	}
}

func TestInit_UnsupportedDriverFails(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_DRIVER, "postgres")
	viper.Set(KEY_DSN, "host=localhost")

	if err := Init(); err == nil || !strings.Contains(err.Error(), `unsupported driver "postgres"`) {
		t.Fatalf("err = %v, want unsupported driver", err)
	}
}

func TestInit_MySQLEmptyDSNFails(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_DRIVER, DRIVER_MYSQL)

	if err := Init(); err == nil || !strings.Contains(err.Error(), "DB_DSN not set") {
		t.Fatalf("err = %v, want DB_DSN not set", err)
	}
}

func TestInit_MySQLInvalidDSNFails(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_DRIVER, DRIVER_MYSQL)
	viper.Set(KEY_DSN, "this is not a valid DSN")

	if err := Init(); err == nil {
		t.Fatal("expected error for invalid mysql DSN")
	}
	if Default != nil {
		t.Error("Default should stay nil when gorm.Open fails")
	}
}

func TestInit_SQLiteEmptyDSNDerivesAppDataPath(t *testing.T) {
	resetDB(t)
	dir := t.TempDir()
	config.SetAppName("dbtest")
	config.SetConfigDir(dir)
	t.Cleanup(func() {
		config.SetAppName("")
		config.SetConfigDir("")
	})
	viper.Set(KEY_DRIVER, DRIVER_SQLITE)

	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := Default.DB().Exec("CREATE TABLE t (id INTEGER)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "dbtest.db")); err != nil {
		t.Errorf("derived sqlite file missing: %v", err)
	}
}

func TestOpen_NamedReadsSuffixedDSNAndInheritsDriver(t *testing.T) {
	resetDB(t, "DB_DSN_TRIFECTA")
	viper.Set(KEY_DRIVER, DRIVER_SQLITE)
	viper.Set("DB_DSN_TRIFECTA", ":memory:")

	svc, err := Open("trifecta")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if svc.Driver() != DRIVER_SQLITE {
		t.Errorf("driver = %q, want inherited sqlite", svc.Driver())
	}
	if Default != nil {
		t.Error("Open must not set Default")
	}
}

func TestOpen_NamedDriverOverridesDefault(t *testing.T) {
	resetDB(t, "DB_DRIVER_OTHER_SRC", "DB_DSN_OTHER_SRC")
	viper.Set(KEY_DRIVER, DRIVER_MYSQL)
	viper.Set("DB_DRIVER_OTHER_SRC", DRIVER_SQLITE)
	viper.Set("DB_DSN_OTHER_SRC", ":memory:")

	svc, err := Open("other-src")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if svc.Driver() != DRIVER_SQLITE {
		t.Errorf("driver = %q, want sqlite", svc.Driver())
	}
}

func TestOpen_NamedEmptyDSNFailsEvenForSQLite(t *testing.T) {
	resetDB(t, "DB_DSN_TRIFECTA")
	viper.Set(KEY_DRIVER, DRIVER_SQLITE)

	if _, err := Open("trifecta"); err == nil || !strings.Contains(err.Error(), "DB_DSN_TRIFECTA not set") {
		t.Fatalf("err = %v, want DB_DSN_TRIFECTA not set", err)
	}
}

func TestKeys(t *testing.T) {
	for _, tc := range []struct{ name, driver, dsn string }{
		{"", "DB_DRIVER", "DB_DSN"},
		{"trifecta", "DB_DRIVER_TRIFECTA", "DB_DSN_TRIFECTA"},
		{"vid-note", "DB_DRIVER_VID_NOTE", "DB_DSN_VID_NOTE"},
	} {
		d, s := Keys(tc.name)
		if d != tc.driver || s != tc.dsn {
			t.Errorf("Keys(%q) = %s, %s; want %s, %s", tc.name, d, s, tc.driver, tc.dsn)
		}
	}
}

func TestService_CloseClosesAndNilDBFails(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_DRIVER, DRIVER_SQLITE)
	viper.Set(KEY_DSN, ":memory:")
	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := Default.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	sqlDB, _ := Default.DB().DB()
	if err := sqlDB.Ping(); err == nil {
		t.Error("ping should fail after Close")
	}
	Default = nil

	if err := (&Service{}).Close(); err == nil || !strings.Contains(err.Error(), "underlying gorm.DB is nil") {
		t.Errorf("nil db Close err = %v", err)
	}
}
