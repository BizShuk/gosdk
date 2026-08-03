package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestGetAppConfigDirHonorsXDGConfigHome(t *testing.T) {
	orig := GetAppName()
	t.Cleanup(func() { SetAppName(orig) })

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	SetAppName("xdg-test-app")

	got := GetAppConfigDir()
	want := filepath.Join(tmp, "xdg-test-app")
	if got != want {
		t.Fatalf("GetAppConfigDir() = %q, want %q", got, want)
	}
}

func TestGetAppConfigDirFallsBackToHomeConfig(t *testing.T) {
	orig := GetAppName()
	t.Cleanup(func() { SetAppName(orig) })

	t.Setenv("XDG_CONFIG_HOME", "")
	SetAppName("home-test-app")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory available: %v", err)
	}

	got := GetAppConfigDir()
	want := filepath.Join(homeDir, ".config", "home-test-app")
	if got != want {
		t.Fatalf("GetAppConfigDir() = %q, want %q", got, want)
	}
}

// An empty app name must keep resolving to "" — cmd/config's tests rely on
// that value to drop the app directory out of viper's search path, so a
// well-meaning default here would silently change their behaviour.
func TestGetAppConfigDirEmptyWithoutAppName(t *testing.T) {
	orig := GetAppName()
	t.Cleanup(func() { SetAppName(orig) })

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	SetAppName("")

	if got := GetAppConfigDir(); got != "" {
		t.Fatalf("GetAppConfigDir() = %q, want \"\"", got)
	}
}

// applyOptions must resolve the seed directory through the same base as
// GetAppConfigDir, or a machine with XDG_CONFIG_HOME set would write
// settings.json to ~/.config and then read config from $XDG_CONFIG_HOME.
func TestApplyOptionsSeedsTheDirectoryGetAppConfigDirReads(t *testing.T) {
	orig := GetAppName()
	t.Cleanup(func() { SetAppName(orig) })

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	o := applyOptions(WithAppName("seed-test-app"))
	SetAppName("seed-test-app")

	if o.appConfigDir != GetAppConfigDir() {
		t.Fatalf("applyOptions dir = %q, GetAppConfigDir() = %q — the two must not drift",
			o.appConfigDir, GetAppConfigDir())
	}
	if o.appConfigDir != filepath.Join(tmp, "seed-test-app") {
		t.Fatalf("applyOptions dir = %q, want it under XDG_CONFIG_HOME", o.appConfigDir)
	}
}

func TestGetAppDataAndLogsDirsFollowConfigDir(t *testing.T) {
	orig := GetAppName()
	t.Cleanup(func() { SetAppName(orig) })

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	SetAppName("dirs-test-app")

	base := filepath.Join(tmp, "dirs-test-app")
	if got, want := GetAppDataDir(), filepath.Join(base, "data"); got != want {
		t.Errorf("GetAppDataDir() = %q, want %q", got, want)
	}
	if got, want := GetAppLogsDir(), filepath.Join(base, "logs"); got != want {
		t.Errorf("GetAppLogsDir() = %q, want %q", got, want)
	}
}

// WithConfigDir must beat both halves of the derived path — the app name and
// the environment. A machine that exports XDG_CONFIG_HOME for an unrelated
// tool must not be able to relocate an application that pinned its directory.
func TestWithConfigDirOverridesAppNameAndXDG(t *testing.T) {
	viper.Reset()
	origName := GetAppName()
	t.Cleanup(func() {
		viper.Reset()
		SetAppName(origName)
		SetConfigDir("")
	})

	forced := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	Default(WithAppName("forced-dir-app"), WithConfigDir(forced))

	if got := GetAppConfigDir(); got != forced {
		t.Fatalf("GetAppConfigDir() = %q, want %q", got, forced)
	}
	// The app name is deliberately untouched: only the directory moves.
	if got := GetAppName(); got != "forced-dir-app" {
		t.Fatalf("GetAppName() = %q, want %q", got, "forced-dir-app")
	}
	if got, want := GetAppDataDir(), filepath.Join(forced, "data"); got != want {
		t.Errorf("GetAppDataDir() = %q, want %q", got, want)
	}
	if got, want := GetAppLogsDir(), filepath.Join(forced, "logs"); got != want {
		t.Errorf("GetAppLogsDir() = %q, want %q", got, want)
	}
}

// "~/..." is the spelling a host actually writes when it wants the home
// directory rather than whatever XDG points at, so the option has to expand it.
func TestWithConfigDirExpandsHome(t *testing.T) {
	viper.Reset()
	origName := GetAppName()
	t.Cleanup(func() {
		viper.Reset()
		SetAppName(origName)
		SetConfigDir("")
	})

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory available: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	Default(WithAppName("tilde-app"), WithConfigDir("~/.config/tilde-app"))

	want := filepath.Join(homeDir, ".config", "tilde-app")
	if got := GetAppConfigDir(); got != want {
		t.Fatalf("GetAppConfigDir() = %q, want %q", got, want)
	}
}

// The seed file and the loader must agree, exactly as they do for the derived
// path: WithDefaultValue writes where GetAppConfigDir later reads.
func TestWithConfigDirSeedsTheDirectoryItReadsFrom(t *testing.T) {
	viper.Reset()
	origName := GetAppName()
	t.Cleanup(func() {
		viper.Reset()
		SetAppName(origName)
		SetConfigDir("")
	})

	forced := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	Default(
		WithAppName("seeded-forced-app"),
		WithConfigDir(forced),
		WithDefaultValue(`{"name": "forced"}`),
	)

	if _, err := os.Stat(filepath.Join(forced, "settings.json")); err != nil {
		t.Fatalf("settings.json not seeded into the forced dir: %v", err)
	}
	if got := viper.GetString("name"); got != "forced" {
		t.Fatalf("viper.GetString(\"name\") = %q, want %q — the loader read a different directory", got, "forced")
	}
}

// A Default() call that does not ask for a forced directory must not inherit
// one from an earlier call, or a process reconfiguring itself would keep a
// path no call site mentions.
func TestDefaultClearsAPreviousForcedConfigDir(t *testing.T) {
	viper.Reset()
	origName := GetAppName()
	t.Cleanup(func() {
		viper.Reset()
		SetAppName(origName)
		SetConfigDir("")
	})

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	Default(WithAppName("reset-app"), WithConfigDir(t.TempDir()))
	Default(WithAppName("reset-app"))

	if got := GetConfigDir(); got != "" {
		t.Fatalf("GetConfigDir() = %q, want \"\" after a Default() without WithConfigDir", got)
	}
	if got, want := GetAppConfigDir(), appConfigDirFor("reset-app"); got != want {
		t.Fatalf("GetAppConfigDir() = %q, want the derived %q", got, want)
	}
}
