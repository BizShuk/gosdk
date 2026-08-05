package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cfgcmd "github.com/bizshuk/gosdk/cmd/config"
	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/pflag"
)

// ConfigCmd is a shell over cmd/config. The behaviour of a mutation — what a
// merge does to a nested key, which file shadows which — is covered there.
// What is only testable here is the wiring: that every flag reaches the right
// argument, that --add really is an alias, and that the rendered report goes
// to the command's own writer rather than the process stdout.

// fixture builds a temp working directory holding the given config files.
// config.GetAppConfigDir() is neutralised so the search path stays "." +
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
	configFiles = false
	configUpdates = nil
	configAdds = nil
	configDeletes = nil
	configAppends = nil
	configRemoves = nil
	configFile = cfgcmd.LOCAL_SETTINGS_FILE
	configLocal = false
	ConfigCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

	var buf bytes.Buffer
	ConfigCmd.SetOut(&buf)
	ConfigCmd.SetErr(&buf)
	ConfigCmd.SetArgs(args)
	t.Cleanup(func() { ConfigCmd.SetArgs(nil) })

	err := ConfigCmd.Execute()
	return buf.String(), err
}

// readTarget parses the JSON file the command wrote.
func readTarget(t *testing.T, dir, name string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return m
}

func TestShowRendersMergedTable(t *testing.T) {
	fixture(t, map[string]string{
		".env":          "from_env=1\n",
		"config.yaml":   "from_yaml: 2\n",
		"settings.json": `{"from_json":3}`,
	})

	out, err := run(t)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !strings.HasPrefix(out, "KEY") {
		t.Fatalf("output is not the show table:\n%s", out)
	}
	for _, key := range []string{"from_env", "from_yaml", "from_json"} {
		if !strings.Contains(out, key) {
			t.Errorf("show output is missing %q:\n%s", key, out)
		}
	}
}

// The SOURCE column is unconditional: the file a value was read from is part
// of the merged view, not an opt-in. --source adds the overridden values.
func TestShowAlwaysNamesTheSourceFile(t *testing.T) {
	fixture(t, map[string]string{"settings.json": `{"a":1}`})
	// Resolve the working directory the same way the renderer does; on macOS
	// the temp dir is reachable under two paths and only one of them matches.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	plain, err := run(t)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !strings.Contains(plain, "SOURCE") {
		t.Errorf("SOURCE column missing without --source:\n%s", plain)
	}
	if want := filepath.Join(wd, "settings.json"); !strings.Contains(plain, want) {
		t.Errorf("show output does not name %s:\n%s", want, plain)
	}
}

func TestShowSourceFlagListsOverriddenValues(t *testing.T) {
	fixture(t, map[string]string{
		"settings.json": `{"a":"from-json"}`,
		".env":          "a=from-env\n",
	})

	plain, err := run(t)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if strings.Contains(plain, "from-json") {
		t.Errorf("overridden value leaked without --source:\n%s", plain)
	}

	sourced, err := run(t, "--source")
	if err != nil {
		t.Fatalf("config --source: %v", err)
	}
	if !strings.Contains(sourced, "from-json") || !strings.Contains(sourced, "overridden") {
		t.Errorf("--source did not list the overridden json value:\n%s", sourced)
	}
}

// --files answers "where does this application read config from" without
// needing a key to exist first: every file in the chain, found or not.
func TestFilesFlagListsSearchedFiles(t *testing.T) {
	fixture(t, map[string]string{"settings.json": `{"a":1}`})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	out, err := run(t, "--files")
	if err != nil {
		t.Fatalf("config --files: %v", err)
	}
	if !strings.Contains(out, filepath.Join(wd, "settings.json")) {
		t.Errorf("--files does not resolve settings.json under %s:\n%s", wd, out)
	}
	for _, name := range []string{"config.yaml", "settings.local.json", ".env.local"} {
		if !strings.Contains(out, name) {
			t.Errorf("--files omits %q from the searched set:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "(not found)") {
		t.Errorf("--files does not mark the missing files:\n%s", out)
	}
}

func TestUpdateWritesAndReports(t *testing.T) {
	dir := fixture(t, nil)

	out, err := run(t, "--update", "server.host=0.0.0.0")
	if err != nil {
		t.Fatalf("config --update: %v\n%s", err, out)
	}
	if !strings.Contains(out, "add    server.host: 0.0.0.0") {
		t.Errorf("change line missing from output:\n%s", out)
	}
	if !strings.Contains(out, "written to ") {
		t.Errorf("target path missing from output:\n%s", out)
	}

	m := readTarget(t, dir, cfgcmd.LOCAL_SETTINGS_FILE)
	server, _ := m["server"].(map[string]any)
	if server["host"] != "0.0.0.0" {
		t.Errorf("server.host not written: %#v", m)
	}
}

// --add is documented as an alias, so it must land in the same argument as
// --update rather than in a slice of its own.
func TestAddIsAliasForUpdate(t *testing.T) {
	dir := fixture(t, nil)

	if _, err := run(t, "--add", "a=1", "--update", "b=2"); err != nil {
		t.Fatalf("config --add --update: %v", err)
	}
	m := readTarget(t, dir, cfgcmd.LOCAL_SETTINGS_FILE)
	if m["a"] == nil || m["b"] == nil {
		t.Errorf("--add and --update did not both apply: %#v", m)
	}
}

func TestDeleteReachesLogicLayer(t *testing.T) {
	dir := fixture(t, map[string]string{
		cfgcmd.LOCAL_SETTINGS_FILE: `{"keep":1,"drop":2}`,
	})

	out, err := run(t, "--delete", "drop")
	if err != nil {
		t.Fatalf("config --delete: %v\n%s", err, out)
	}
	m := readTarget(t, dir, cfgcmd.LOCAL_SETTINGS_FILE)
	if _, still := m["drop"]; still {
		t.Errorf("--delete did not remove the key: %#v", m)
	}
	if m["keep"] == nil {
		t.Errorf("--delete removed more than it was asked to: %#v", m)
	}
}

// --append and --remove-from must compose with the key-level flags in a single
// write; the CLI's job is to pass all four sets to one Apply call.
func TestAppendAndRemoveFromComposeInOneWrite(t *testing.T) {
	dir := fixture(t, map[string]string{
		cfgcmd.LOCAL_SETTINGS_FILE: `{"tags":["a","b"]}`,
	})

	out, err := run(t, "--append", "tags=c", "--remove-from", "tags=a", "--update", "name=x")
	if err != nil {
		t.Fatalf("combined flags: %v\n%s", err, out)
	}

	m := readTarget(t, dir, cfgcmd.LOCAL_SETTINGS_FILE)
	tags, _ := m["tags"].([]any)
	if len(tags) != 2 || tags[0] != "b" || tags[1] != "c" {
		t.Errorf("tags = %#v, want [b c]", tags)
	}
	if m["name"] != "x" {
		t.Errorf("--update was lost in the combined write: %#v", m)
	}
	// One write means one "written to" line, not three.
	if n := strings.Count(out, "written to "); n != 1 {
		t.Errorf("got %d write lines, want exactly 1:\n%s", n, out)
	}
}

// --file must select the target; a YAML target proves the format dispatch is
// reached rather than the JSON path being hardcoded.
func TestFileFlagSelectsTarget(t *testing.T) {
	dir := fixture(t, nil)

	if _, err := run(t, "--file", "config.yaml", "--update", "server.port=8080"); err != nil {
		t.Fatalf("config --file config.yaml: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("--file did not reach the logic layer: %v", err)
	}
	if !strings.Contains(string(body), "port: 8080") {
		t.Errorf("config.yaml not written as YAML: %s", body)
	}
	if _, err := os.Stat(filepath.Join(dir, cfgcmd.LOCAL_SETTINGS_FILE)); !os.IsNotExist(err) {
		t.Errorf("the default target was written despite --file")
	}
}

func TestFileFlagRejectsUnknownName(t *testing.T) {
	fixture(t, nil)

	if _, err := run(t, "--file", "whatever.toml", "--update", "a=1"); err == nil {
		t.Fatal("expected an unsupported-file error")
	}
}

// --local anchors at the working directory even when an app name is set.
func TestLocalFlagAnchorsAtWorkingDirectory(t *testing.T) {
	dir := fixture(t, nil)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	config.SetAppName("localtest")
	t.Cleanup(func() { config.SetAppName("") })

	if _, err := run(t, "--local", "--update", "a.b=xyz"); err != nil {
		t.Fatalf("config --local: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, cfgcmd.LOCAL_SETTINGS_FILE)); err != nil {
		t.Fatalf("--local did not write to the working directory: %v", err)
	}
	appPath := filepath.Join(home, ".config", "localtest", cfgcmd.LOCAL_SETTINGS_FILE)
	if _, err := os.Stat(appPath); !os.IsNotExist(err) {
		t.Errorf("--local still wrote into the app config dir")
	}
}

// Without --local the app config dir wins over a file sitting in the cwd.
func TestWriteDefaultsToAppConfigDir(t *testing.T) {
	fixture(t, map[string]string{cfgcmd.LOCAL_SETTINGS_FILE: `{"legacy":"cwd"}`})

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	config.SetAppName("appdirtest")
	t.Cleanup(func() { config.SetAppName("") })

	if _, err := run(t, "--update", "a.b=xyz"); err != nil {
		t.Fatalf("config --update: %v", err)
	}
	appPath := filepath.Join(home, ".config", "appdirtest", cfgcmd.LOCAL_SETTINGS_FILE)
	if _, err := os.Stat(appPath); err != nil {
		t.Fatalf("write did not land in the app config dir: %v", err)
	}
}

// A logic-layer error must surface as a command error, not a rendered report.
func TestErrorsPropagateFromLogicLayer(t *testing.T) {
	fixture(t, nil)

	out, err := run(t, "--update", "no-equals-sign")
	if err == nil {
		t.Fatalf("expected an error for a malformed spec, got output:\n%s", out)
	}
	if !strings.Contains(err.Error(), "expected a.b.c=value") {
		t.Errorf("error text lost on the way up: %v", err)
	}
}
