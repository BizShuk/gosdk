package db

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// resetDB 還原 Default singleton 與本套件讀取的 viper key。
func resetDB(t *testing.T, keys ...string) {
	t.Helper()
	t.Cleanup(func() {
		if Default != nil {
			_ = Default.Close()
			Default = nil
		}
		for _, k := range append([]string{KEY_DRIVER, KEY_DSN, KEY_LOG, KEY_TRANSLATE_ERROR}, keys...) {
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
	want := filepath.Join(dir, "data", DEFAULT_SQLITE_FILE)
	if _, err := os.Stat(want); err != nil {
		t.Errorf("derived sqlite file missing: %v", err)
	}
	if Default.Target() != want {
		t.Errorf("Target() = %q, want %q", Default.Target(), want)
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

func TestKey(t *testing.T) {
	for _, tc := range []struct{ base, name, want string }{
		{KEY_DSN, "", "DB_DSN"},
		{KEY_DSN, "trifecta", "DB_DSN_TRIFECTA"},
		{KEY_DRIVER, "vid-note", "DB_DRIVER_VID_NOTE"},
		{KEY_LOG, "trifecta", "DB_LOG_TRIFECTA"},
	} {
		if got := Key(tc.base, tc.name); got != tc.want {
			t.Errorf("Key(%q, %q) = %q, want %q", tc.base, tc.name, got, tc.want)
		}
	}
}

func TestGormConfig_Defaults(t *testing.T) {
	resetDB(t)

	cfg, err := gormConfigFor("")
	if err != nil {
		t.Fatalf("gormConfigFor: %v", err)
	}
	if cfg.Logger != logger.Discard {
		t.Error("DB_LOG unset: want logger.Discard")
	}
	if !cfg.TranslateError {
		t.Error("DB_TRANSLATE_ERROR unset: want true")
	}
}

func TestGormConfig_SwitchesAndNamedFallback(t *testing.T) {
	resetDB(t, "DB_LOG_TRIFECTA")
	viper.Set(KEY_LOG, "true")
	viper.Set(KEY_TRANSLATE_ERROR, "false")
	viper.Set("DB_LOG_TRIFECTA", "false")

	primary, err := gormConfigFor("")
	if err != nil {
		t.Fatalf("gormConfigFor: %v", err)
	}
	if primary.Logger != logger.Default || primary.TranslateError {
		t.Errorf("primary = log %v translate %v, want default logger and false", primary.Logger, primary.TranslateError)
	}

	named, err := gormConfigFor("trifecta")
	if err != nil {
		t.Fatalf("gormConfigFor(trifecta): %v", err)
	}
	if named.Logger != logger.Discard {
		t.Error("DB_LOG_TRIFECTA=false should override DB_LOG=true")
	}
	if named.TranslateError {
		t.Error("DB_TRANSLATE_ERROR_TRIFECTA unset should inherit DB_TRANSLATE_ERROR=false")
	}
}

func TestGormConfig_InvalidBoolFails(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_LOG, "maybe")

	if _, err := gormConfigFor(""); err == nil || !strings.Contains(err.Error(), `DB_LOG="maybe"`) {
		t.Fatalf("err = %v, want DB_LOG not a bool", err)
	}
}

func TestInit_TranslateErrorReturnsDuplicatedKey(t *testing.T) {
	resetDB(t)
	viper.Set(KEY_DRIVER, DRIVER_SQLITE)
	viper.Set(KEY_DSN, ":memory:")
	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	type row struct {
		ID    uint
		Email string `gorm:"uniqueIndex"`
	}
	gdb := Default.DB()
	if err := gdb.AutoMigrate(&row{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := gdb.Create(&row{Email: "a@b"}).Error; err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := gdb.Create(&row{Email: "a@b"}).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("duplicate insert err = %v, want gorm.ErrDuplicatedKey", err)
	}
}

func TestTargetOf_RedactsMySQLPassword(t *testing.T) {
	for _, tc := range []struct{ driver, dsn, want string }{
		{DRIVER_MYSQL, "app:s3cr@t@tcp(mysql.local:3306)/app?tls=true", "app:***@tcp(mysql.local:3306)/app?tls=true"},
		{DRIVER_MYSQL, "app@tcp(mysql.local:3306)/app", "app@tcp(mysql.local:3306)/app"},
		{DRIVER_SQLITE, "/data/default.db", "/data/default.db"},
	} {
		if got := targetOf(tc.driver, tc.dsn); got != tc.want {
			t.Errorf("targetOf(%s, %q) = %q, want %q", tc.driver, tc.dsn, got, tc.want)
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
