package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/pflag"
)

// fixture builds a temp working directory holding the given config files.
// config.GetAppConfigDir() is neutralised so the search path stays "." + "conf"
// and the tests never touch the developer's real ~/.config.
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

	config.SetAppName("")
	t.Cleanup(func() { config.SetAppName("") })

	return dir
}

// run executes the shared ConfigCmd and returns its combined output.
//
// ConfigCmd is a package-level command, so parsed flag state survives between
// calls: pflag's slice values append instead of replacing once a flag has been
// set. Clearing the bound variables first makes every call independent.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()

	configSource = false
	configUpdates = nil
	configAdds = nil
	configDeletes = nil
	ConfigCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

	var buf bytes.Buffer
	ConfigCmd.SetOut(&buf)
	ConfigCmd.SetErr(&buf)
	ConfigCmd.SetArgs(args)
	err := ConfigCmd.Execute()
	return buf.String(), err
}

// readLocal parses the settings.local.json the command wrote.
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

func TestPublicConfigFunctions(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"keep":"old","remove":"value"}`,
	})

	updateReport, err := RunConfigUpdate([]string{"keep=new"})
	if err != nil {
		t.Fatalf("RunConfigUpdate failed: %v", err)
	}
	deleteReport, err := RunConfigDelete([]string{"remove"})
	if err != nil {
		t.Fatalf("RunConfigDelete failed: %v", err)
	}
	entries := RunConfigShow()

	// Each public function exposes its work as data; the CLI layer is
	// responsible for turning that data into human-readable output, so these
	// assertions check the structured result, not a formatted string.
	if len(updateReport.Changes) != 1 {
		t.Errorf("RunConfigUpdate changes = %+v, want exactly one change", updateReport.Changes)
	} else {
		ch := updateReport.Changes[0]
		if ch.Kind != ChangeUpdated || ch.Key != "keep" || ch.Old != "old" || ch.New != "new" {
			t.Errorf("RunConfigUpdate change = %+v, want {Update, keep, old, new}", ch)
		}
	}
	if len(deleteReport.Changes) != 1 {
		t.Errorf("RunConfigDelete changes = %+v, want exactly one change", deleteReport.Changes)
	} else {
		ch := deleteReport.Changes[0]
		if ch.Kind != ChangeDeleted || ch.Key != "remove" || ch.Old != "value" || ch.New != nil {
			t.Errorf("RunConfigDelete change = %+v, want {Delete, remove, value, nil}", ch)
		}
	}
	var foundKeep bool
	for _, e := range entries {
		if e.Key == "keep" && e.Value == "new" {
			foundKeep = true
			break
		}
	}
	if !foundKeep {
		t.Errorf("RunConfigShow did not return updated keep=new entry: %+v", entries)
	}

	m := readLocal(t, dir)
	if _, ok := m["remove"]; ok {
		t.Errorf("RunConfigDelete left removed key in %#v", m)
	}
}

func TestShowMergesEveryLayer(t *testing.T) {
	fixture(t, map[string]string{
		".env":                "SHARED=1-env\nONLY_ENV=env\n",
		".env.local":          "SHARED=2-env-local\n",
		"config.yaml":         "shared: 3-yaml\nserver:\n  port: 8080\n",
		"config.local.yaml":   "shared: 4-yaml-local\n",
		"settings.json":       `{"shared":"5-json"}`,
		"settings.local.json": `{"shared":"6-json-local"}`,
	})

	out, err := run(t)
	if err != nil {
		t.Fatalf("show failed: %v\n%s", err, out)
	}

	// The highest-precedence value wins, and nested keys render as dotted paths.
	for _, want := range []string{"shared", "6-json-local", "only_env", "server.port", "8080"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}
	// Overridden values stay hidden without --source.
	if strings.Contains(out, "2-env-local") {
		t.Errorf("show without --source leaked an overridden value:\n%s", out)
	}
}

func TestShowSourceAnnotatesLayer(t *testing.T) {
	fixture(t, map[string]string{
		".env":              "SHARED=1-env\n",
		"config.local.yaml": "shared: 4-yaml-local\n",
	})

	out, err := run(t, "--source")
	if err != nil {
		t.Fatalf("show --source failed: %v\n%s", err, out)
	}

	// Source is reported per layer, not per file: each config loader merges its
	// own base and .local file internally, so finer attribution is unavailable.
	for _, want := range []string{"SOURCE", "yaml", "4-yaml-local", "1-env", "overridden"} {
		if !strings.Contains(out, want) {
			t.Errorf("--source output missing %q:\n%s", want, out)
		}
	}
}

func TestShowWithNoConfigFiles(t *testing.T) {
	fixture(t, nil)

	out, err := run(t)
	if err != nil {
		t.Fatalf("show failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no configuration found") {
		t.Errorf("expected an empty-config notice, got:\n%s", out)
	}
}

func TestShowAppliesEnvOverride(t *testing.T) {
	t.Setenv("APP_LOG_LEVEL", "from-envvar")
	fixture(t, map[string]string{".env": "LOG_LEVEL=from-file\n"})

	out, err := run(t, "--source")
	if err != nil {
		t.Fatalf("show failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "from-envvar") || !strings.Contains(out, "APP_LOG_LEVEL") {
		t.Errorf("expected the env var to win and be named as the source:\n%s", out)
	}
	if !strings.Contains(out, "from-file") {
		t.Errorf("expected the overridden file value to still be listed:\n%s", out)
	}
}

func TestUpdateCreatesNestedPath(t *testing.T) {
	dir := fixture(t, nil)

	out, err := run(t, "--update", "a.b.c=xyz")
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}

	m := readLocal(t, dir)
	a, ok := m["a"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map at %q, got %#v", "a", m["a"])
	}
	b, ok := a["b"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map at %q, got %#v", "a.b", a["b"])
	}
	if b["c"] != "xyz" {
		t.Errorf("a.b.c = %#v, want %q", b["c"], "xyz")
	}
}

func TestUpdateOverwritesExistingValue(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"server":{"host":"old","port":80}}`,
	})

	out, err := run(t, "--update", "server.host=new")
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "old -> new") {
		t.Errorf("expected the old value in the report, got:\n%s", out)
	}

	server := readLocal(t, dir)["server"].(map[string]any)
	if server["host"] != "new" {
		t.Errorf("server.host = %#v, want %q", server["host"], "new")
	}
	// Sibling keys survive the rewrite.
	if server["port"] == nil {
		t.Error("server.port was dropped by the rewrite")
	}
}

func TestUpdateKeepsJSONTypes(t *testing.T) {
	dir := fixture(t, nil)

	out, err := run(t,
		"--update", "num=1234",
		"--update", "flag=true",
		"--update", "text=hello",
		"--update", `quoted="1234"`,
	)
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}

	m := readLocal(t, dir)
	if _, ok := m["num"].(float64); !ok {
		t.Errorf("num = %#v (%T), want a JSON number", m["num"], m["num"])
	}
	if m["flag"] != true {
		t.Errorf("flag = %#v, want true", m["flag"])
	}
	if m["text"] != "hello" {
		t.Errorf("text = %#v, want %q", m["text"], "hello")
	}
	// Quoting is the escape hatch for keeping a numeric-looking value a string.
	if m["quoted"] != "1234" {
		t.Errorf("quoted = %#v (%T), want the string %q", m["quoted"], m["quoted"], "1234")
	}
}

func TestAddIsAliasForUpdate(t *testing.T) {
	dir := fixture(t, nil)

	out, err := run(t, "--add", "a.b=one", "--update", "c.d=two")
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}

	m := readLocal(t, dir)
	if got := m["a"].(map[string]any)["b"]; got != "one" {
		t.Errorf("a.b = %#v, want %q", got, "one")
	}
	// Both flags must land; neither may reset the other's slice.
	if got := m["c"].(map[string]any)["d"]; got != "two" {
		t.Errorf("c.d = %#v, want %q", got, "two")
	}
}

func TestUpdateReusesExistingKeyCase(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"SQLITE_PATH":"./old.db"}`,
	})

	if out, err := run(t, "--update", "sqlite_path=./new.db"); err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}

	m := readLocal(t, dir)
	// viper looks keys up case-insensitively, so the original spelling is kept
	// rather than a second key being introduced.
	if m["SQLITE_PATH"] != "./new.db" {
		t.Errorf("SQLITE_PATH = %#v, want %q", m["SQLITE_PATH"], "./new.db")
	}
	if len(m) != 1 {
		t.Errorf("expected exactly one key, got %#v", m)
	}
}

func TestUpdateRejectsScalarInPath(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"a":{"b":"scalar"}}`,
	})

	out, err := run(t, "--update", "a.b.c=xyz")
	if err == nil {
		t.Fatalf("expected an error when descending into a scalar, got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "scalar") {
		t.Errorf("error should name the blocking scalar, got: %v", err)
	}
}

func TestUpdateRejectsMalformedSpec(t *testing.T) {
	fixture(t, nil)

	if _, err := run(t, "--update", "a.b.c"); err == nil {
		t.Error("expected an error for a spec with no '='")
	}
	if _, err := run(t, "--update", "a..c=xyz"); err == nil {
		t.Error("expected an error for an empty path segment")
	}
}

func TestDeleteKeepsEmptyParent(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"a":{"b":{"c":"xyz"},"d":"keep"}}`,
	})

	out, err := run(t, "--delete", "a.b.c")
	if err != nil {
		t.Fatalf("delete failed: %v\n%s", err, out)
	}

	a := readLocal(t, dir)["a"].(map[string]any)
	b, ok := a["b"].(map[string]any)
	if !ok {
		t.Fatalf("parent %q should survive as a map, got %#v", "a.b", a["b"])
	}
	if len(b) != 0 {
		t.Errorf("a.b should be empty, got %#v", b)
	}
	if a["d"] != "keep" {
		t.Errorf("sibling a.d was lost, got %#v", a["d"])
	}
}

func TestDeleteRejectsMissingKey(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"a":{"b":{"c":"xyz"}}}`,
	})

	if _, err := run(t, "--delete", "a.b.nope"); err == nil {
		t.Error("expected an error deleting a key that does not exist")
	}
	if _, err := run(t, "--delete", "nope.deeper"); err == nil {
		t.Error("expected an error deleting through a missing parent")
	}
}

func TestUpdateAndDeleteFailureLeavesFileUntouched(t *testing.T) {
	const original = `{"keep":"original"}`
	dir := fixture(t, map[string]string{
		"settings.local.json": original,
	})

	out, err := run(t, "--update", "added=value", "--delete", "missing")
	if err == nil {
		t.Fatal("expected the missing delete to fail")
	}
	if strings.Contains(out, "add    added") || strings.Contains(out, "written to") {
		t.Errorf("failed changes wrote success output before the adapter returned successfully: %q", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, LOCAL_SETTINGS_FILE))
	if err != nil {
		t.Fatalf("read %s: %v", LOCAL_SETTINGS_FILE, err)
	}
	if string(data) != original {
		t.Errorf("failed changes rewrote %s:\ngot:  %s\nwant: %s", LOCAL_SETTINGS_FILE, data, original)
	}
}

func TestDeleteOnlyTouchesLocalSettings(t *testing.T) {
	dir := fixture(t, map[string]string{
		"config.yaml":         "server:\n  host: from-yaml\n",
		"settings.local.json": `{"server":{"host":"from-json-local"}}`,
	})

	if out, err := run(t, "--delete", "server.host"); err != nil {
		t.Fatalf("delete failed: %v\n%s", err, out)
	}

	// config.yaml is never rewritten, so its value resurfaces as the winner.
	yaml, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if !strings.Contains(string(yaml), "from-yaml") {
		t.Errorf("config.yaml was modified: %s", yaml)
	}

	out, err := run(t, "--source")
	if err != nil {
		t.Fatalf("show failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "from-yaml") {
		t.Errorf("expected the yaml value to win after the delete:\n%s", out)
	}
}

func TestUpdateWarnsWhenEnvVarShadows(t *testing.T) {
	t.Setenv("APP_LOG_LEVEL", "debug")
	fixture(t, nil)

	out, err := run(t, "--update", "log_level=info")
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "APP_LOG_LEVEL") || !strings.Contains(out, "warning") {
		t.Errorf("expected a warning that the env var still wins:\n%s", out)
	}
}

func TestUpdateNoWarningForNestedKey(t *testing.T) {
	// APP_SERVER_HOST maps to the flat key server_host, never to server.host,
	// because the config package installs no EnvKeyReplacer.
	t.Setenv("APP_SERVER_HOST", "1.2.3.4")
	fixture(t, nil)

	out, err := run(t, "--update", "server.host=0.0.0.0")
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "warning") {
		t.Errorf("nested key cannot be shadowed by an env var, got:\n%s", out)
	}
}

func TestUpdatePrefersConfDirWhenPresent(t *testing.T) {
	dir := fixture(t, map[string]string{
		"conf/settings.json": `{"existing":"value"}`,
	})

	if out, err := run(t, "--update", "a.b=xyz"); err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(dir, "conf", LOCAL_SETTINGS_FILE)); err != nil {
		t.Errorf("expected the file to be created in ./conf, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, LOCAL_SETTINGS_FILE)); !os.IsNotExist(err) {
		t.Error("settings.local.json should not have been created in the working directory")
	}
}

// TestUpdateReusesLocationFoundByConfigPackage pins the fact that the write
// target is resolved through config.NewJsonConfig(), not a search path this
// command maintains itself.
func TestUpdateReusesLocationFoundByConfigPackage(t *testing.T) {
	dir := fixture(t, map[string]string{
		"conf/settings.local.json": `{"existing":"value"}`,
	})

	if out, err := run(t, "--update", "a.b=xyz"); err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}

	data, err := os.ReadFile(filepath.Join(dir, "conf", LOCAL_SETTINGS_FILE))
	if err != nil {
		t.Fatalf("read conf/%s: %v", LOCAL_SETTINGS_FILE, err)
	}
	if !strings.Contains(string(data), "existing") {
		t.Errorf("the existing file was replaced instead of updated: %s", data)
	}
}
