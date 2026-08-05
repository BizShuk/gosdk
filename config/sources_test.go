package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// sourceFixture builds a working directory, points the app config dir at a
// temp home, and restores the package-level app name afterwards so no test
// leaks its name into the next one or touches the developer's real ~/.config.
func sourceFixture(t *testing.T, appName string) (wd, appDir string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := t.TempDir()
	t.Chdir(dir)

	SetAppName(appName)
	SetConfigDir("")
	t.Cleanup(func() { SetAppName(""); SetConfigDir("") })

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if appName == "" {
		return cwd, ""
	}
	appDir = filepath.Join(home, ".config", appName)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", appDir, err)
	}
	return cwd, appDir
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func sourceFor(t *testing.T, name string) Source {
	t.Helper()
	for _, s := range Sources() {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("%q is not in the source catalog: %+v", name, Sources())
	return Source{}
}

func TestSearchPaths_OmitsAppDirWhenUnset(t *testing.T) {
	sourceFixture(t, "")

	if got := SearchPaths(); !slices.Equal(got, []string{".", "conf"}) {
		t.Errorf("SearchPaths() = %v, want [. conf] when no app name is registered", got)
	}
}

func TestSearchPaths_AppendsAppDirLast(t *testing.T) {
	_, appDir := sourceFixture(t, "probeapp")

	got := SearchPaths()
	want := []string{".", "conf", appDir}
	if !slices.Equal(got, want) {
		t.Errorf("SearchPaths() = %v, want %v", got, want)
	}
}

// The order Sources reports is the order values override each other, so it has
// to match loadAllConfigs: yaml, then json, then env.
func TestSources_ListedInMergeOrder(t *testing.T) {
	sourceFixture(t, "")

	var names []string
	for _, s := range Sources() {
		names = append(names, s.Name)
	}
	want := []string{
		"config.yaml", "config.local.yaml",
		"settings.json", "settings.local.json",
		".env", ".env.local",
	}
	if !slices.Equal(names, want) {
		t.Errorf("Sources() = %v, want %v", names, want)
	}
}

func TestSources_MissingFileHasEmptyPath(t *testing.T) {
	sourceFixture(t, "")

	if got := sourceFor(t, "settings.json"); got.Path != "" {
		t.Errorf("settings.json resolved to %q with no file on disk", got.Path)
	}
}

func TestSources_WorkingDirectoryWinsOverAppDir(t *testing.T) {
	wd, appDir := sourceFixture(t, "probeapp")
	want := writeFile(t, wd, "settings.json", `{"a":1}`)
	writeFile(t, appDir, "settings.json", `{"a":2}`)

	if got := sourceFor(t, "settings.json"); got.Path != want {
		t.Errorf("settings.json = %q, want %q (the working directory is searched first)", got.Path, want)
	}
}

// Each file is resolved on its own, exactly as the loaders resolve them: a
// base file in the working directory does not pin its .local counterpart to
// the same directory.
func TestSources_BaseAndLocalResolveIndependently(t *testing.T) {
	wd, appDir := sourceFixture(t, "probeapp")
	wantBase := writeFile(t, wd, "settings.json", `{"a":1}`)
	wantLocal := writeFile(t, appDir, "settings.local.json", `{"a":2}`)

	if got := sourceFor(t, "settings.json"); got.Path != wantBase {
		t.Errorf("settings.json = %q, want %q", got.Path, wantBase)
	}
	if got := sourceFor(t, "settings.local.json"); got.Path != wantLocal {
		t.Errorf("settings.local.json = %q, want %q", got.Path, wantLocal)
	}
}

func TestSources_ConfDirectoryBeatsAppDir(t *testing.T) {
	wd, appDir := sourceFixture(t, "probeapp")
	want := writeFile(t, wd, filepath.Join("conf", ".env"), "a=1\n")
	writeFile(t, appDir, ".env", "a=2\n")

	if got := sourceFor(t, ".env"); got.Path != want {
		t.Errorf(".env = %q, want %q (conf is searched before the app dir)", got.Path, want)
	}
}

func TestSources_AcceptsYmlSpelling(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	want := writeFile(t, wd, "config.yml", "a: 1\n")

	if got := sourceFor(t, "config.yaml"); got.Path != want {
		t.Errorf("config.yaml = %q, want the config.yml at %q", got.Path, want)
	}
}

func TestSources_ForcedConfigDirIsSearched(t *testing.T) {
	sourceFixture(t, "")
	forced := t.TempDir()
	SetConfigDir(forced)
	want := writeFile(t, forced, "settings.json", `{"a":1}`)

	if got := sourceFor(t, "settings.json"); got.Path != want {
		t.Errorf("settings.json = %q, want %q from the forced config dir", got.Path, want)
	}
}

// LoadFile has to normalise keys the way the loaders do, or a caller
// attributing a merged value to a file would be looking up a key that view
// does not have.
func TestLoadFile_NormalisesKeysLikeTheLoaders(t *testing.T) {
	wd, _ := sourceFixture(t, "")

	yamlPath := writeFile(t, wd, "config.yaml", "Server:\n  Port: 8080\n")
	v, err := LoadFile(yamlPath, "yaml")
	if err != nil {
		t.Fatalf("LoadFile(yaml): %v", err)
	}
	if got := v.GetInt("server.port"); got != 8080 {
		t.Errorf("server.port = %d, want 8080 (keys lowercased, nesting preserved)", got)
	}

	envPath := writeFile(t, wd, ".env", "LOG_LEVEL=debug\n")
	v, err = LoadFile(envPath, "env")
	if err != nil {
		t.Fatalf("LoadFile(env): %v", err)
	}
	if got := v.GetString("log_level"); got != "debug" {
		t.Errorf("log_level = %q, want debug — .env must be parsed as dotenv", got)
	}
}

func TestLoadFile_MalformedFileReportsError(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	path := writeFile(t, wd, "settings.json", `{"a":`)

	if _, err := LoadFile(path, "json"); err == nil {
		t.Error("LoadFile on a truncated document returned no error")
	}
}
