package config

import (
	"os"
	"path/filepath"
	"testing"
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
