package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	sdkconfig "github.com/bizshuk/gosdk/config"
)

// fixture builds a temp working directory holding the given config files.
// sdkconfig.GetAppConfigDir() is neutralised so the search path stays "." +
// "conf" and the tests never touch the developer's real ~/.config.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	t.Chdir(dir)

	sdkconfig.SetAppName("")
	t.Cleanup(func() { sdkconfig.SetAppName("") })

	return dir
}

// appDirFixture points sdkconfig.GetAppConfigDir() at a temp home and empties
// the seed registry, so no test touches the developer's real ~/.config and no
// test inherits another's registrations.
func appDirFixture(t *testing.T, appName string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir() on Windows

	sdkconfig.SetAppName(appName)
	t.Cleanup(func() { sdkconfig.SetAppName("") })

	resetDefaultConfigs()
	t.Cleanup(resetDefaultConfigs)

	return filepath.Join(home, ".config", appName)
}

// readLocal parses the settings.local.json a mutation wrote.
func readLocal(t *testing.T, dir string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, LOCAL_SETTINGS_FILE))
	if err != nil {
		t.Fatalf("read %s: %v", LOCAL_SETTINGS_FILE, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse %s: %v", LOCAL_SETTINGS_FILE, err)
	}
	return m
}
