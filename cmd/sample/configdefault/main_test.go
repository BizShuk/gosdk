package configdefault

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tempHome redirects os.UserHomeDir() so the sample never writes into the
// developer's real ~/.config/seedapp.
func tempHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return filepath.Join(home, ".config", APP_NAME)
}

func TestRunSeedsDefaultSettings(t *testing.T) {
	dir := tempHome(t)

	var out bytes.Buffer
	if err := run([]string{"config", "default"}, &out); err != nil {
		t.Fatalf("run: %v (output %q)", err, out.String())
	}

	path := filepath.Join(dir, "settings.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded settings.json: %v", err)
	}
	if !bytes.Equal(body, defaultSettings) {
		t.Fatalf("seeded content does not match the embedded file:\n%s", body)
	}
	if !strings.Contains(out.String(), path) {
		t.Fatalf("output %q does not report the written path", out.String())
	}

	var settings struct {
		Server struct {
			Port int `json:"port"`
		} `json:"server"`
	}
	if err := json.Unmarshal(body, &settings); err != nil {
		t.Fatalf("seeded file is not valid JSON: %v", err)
	}
	if settings.Server.Port != 8080 {
		t.Fatalf("server.port = %d, want 8080", settings.Server.Port)
	}
}

// Re-running the command must not discard a config the user has since edited.
func TestRunIsSafeToRepeat(t *testing.T) {
	dir := tempHome(t)

	if err := run([]string{"config", "default"}, new(bytes.Buffer)); err != nil {
		t.Fatalf("first run: %v", err)
	}

	path := filepath.Join(dir, "settings.json")
	edited := []byte(`{"server":{"port":9999}}`)
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("simulate a user edit: %v", err)
	}

	var out bytes.Buffer
	if err := run([]string{"config", "default"}, &out); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !strings.Contains(out.String(), "already exists") {
		t.Fatalf("output %q does not warn about the existing file", out.String())
	}
	if body, _ := os.ReadFile(path); !bytes.Equal(body, edited) {
		t.Fatalf("the user's edit was overwritten: %s", body)
	}
}
