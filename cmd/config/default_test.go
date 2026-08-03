package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkconfig "github.com/bizshuk/gosdk/config"
)

// runDefault drives Default the way the CLI does — flags in, rendered text out
// — so these tests assert on exactly what a user sees, without dragging cobra
// into this package.
func runDefault(t *testing.T, args ...string) (string, error) {
	t.Helper()

	file, mode, dryRun, local := DEFAULT_CONFIG_FILE, DefaultModeSkip, false, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			i++
			file = args[i]
		case "--merge":
			mode = DefaultModeMerge
		case "--force":
			mode = DefaultModeForce
		case "--dry-run":
			dryRun = true
		case "--local":
			local = true
		default:
			t.Fatalf("runDefault: unknown flag %q", args[i])
		}
	}

	report, err := Default(file, mode, dryRun, local)
	if err != nil {
		return "", err
	}
	return RenderDefaultReport(report), nil
}

func TestConfigDefaultWritesIntoAppConfigDir(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{"server":{"port":8080}}`))

	out, err := runDefault(t)
	if err != nil {
		t.Fatalf("config default: %v (output %q)", err, out)
	}

	path := filepath.Join(dir, "settings.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded file: %v", err)
	}
	if got, want := string(body), `{"server":{"port":8080}}`; got != want {
		t.Fatalf("seeded content = %q, want %q", got, want)
	}
	if !strings.Contains(out, "wrote "+path) {
		t.Fatalf("output %q does not report the written path", out)
	}
}

func TestConfigDefaultKeepsExistingFileUnlessForced(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{"seed":true}`))

	path := filepath.Join(dir, "settings.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"mine":1}`), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	out, err := runDefault(t)
	if err != nil {
		t.Fatalf("config default: %v", err)
	}
	if !strings.Contains(out, "already exists") {
		t.Fatalf("output %q does not warn about the existing file", out)
	}
	if !strings.Contains(out, "--merge") {
		t.Fatalf("output %q does not point at --merge as the upgrade path", out)
	}
	if body, _ := os.ReadFile(path); string(body) != `{"mine":1}` {
		t.Fatalf("existing file was overwritten: %s", body)
	}

	if _, err := runDefault(t, "--force"); err != nil {
		t.Fatalf("config default --force: %v", err)
	}
	if body, _ := os.ReadFile(path); string(body) != `{"seed":true}` {
		t.Fatalf("--force did not overwrite: %s", body)
	}
}

func TestConfigDefaultDryRunWritesNothing(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("config.yaml", []byte("server:\n  port: 8080\n"))

	out, err := runDefault(t, "--file", "config.yaml", "--dry-run")
	if err != nil {
		t.Fatalf("config default --dry-run: %v", err)
	}
	if !strings.HasPrefix(out, "would write ") {
		t.Fatalf("output %q is not a dry-run report", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry run created the file (stat err = %v)", err)
	}
}

func TestConfigDefaultRejectsUnknownFile(t *testing.T) {
	appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{}`))

	if _, err := runDefault(t, "--file", "whatever.toml"); err == nil {
		t.Fatal("expected an error for an unsupported file name")
	}
}

// An application with no default registered is a supported configuration:
// the command reports it and succeeds, so an installer can call it blind.
func TestConfigDefaultWithoutRegistrationIsANoOp(t *testing.T) {
	dir := appDirFixture(t, "seedapp")

	out, err := runDefault(t)
	if err != nil {
		t.Fatalf("an unregistered file must not fail the command: %v", err)
	}
	if !strings.Contains(out, "no default configuration registered for settings.json") {
		t.Fatalf("output %q does not explain the no-op", out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the config directory was created for a no-op (stat err = %v)", err)
	}
}

// The likeliest cause is a --file typo, so the message lists what is available.
func TestConfigDefaultNoOpListsRegisteredFiles(t *testing.T) {
	appDirFixture(t, "seedapp")
	MustRegisterDefault("config.yaml", []byte("a: 1\n"))

	out, err := runDefault(t) // asks for settings.json
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "registered: config.yaml") {
		t.Fatalf("output %q does not list the registered files", out)
	}
}

// Default and Apply must resolve the same file name to the same path — they
// share FilePath precisely so that seeding a default and editing a value can
// never disagree about which file they mean.
//
// Whether an unset app name is acceptable is a separate question, and not this
// layer's: ConfigDefaultCmd refuses it, while a library caller may point
// Default anywhere it likes. This test exercises the library path.
func TestDefaultResolvesTheSameTargetAsApply(t *testing.T) {
	appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{"a":1}`))
	sdkconfig.SetAppName("")

	dir := t.TempDir()
	t.Chdir(dir)

	out, err := runDefault(t)
	if err != nil {
		t.Fatalf("Default without an app name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatalf("seed did not land in the working directory: %v\n%s", err, out)
	}

	// Apply must agree on the destination.
	if _, err := Update([]string{"b=2"}, WriteOptions{File: "settings.json"}); err != nil {
		t.Fatalf("Update without an app name: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	if !strings.Contains(string(body), `"b"`) {
		t.Fatalf("Default and Update disagreed about the target file: %s", body)
	}
}

// The registry is keyed by file name so independent packages can seed
// different files without one clobbering the other.
func TestRegisterDefaultConfigKeepsFilesIndependent(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{"json":true}`))
	MustRegisterDefault("config.yaml", []byte("yaml: true\n"))

	if _, err := runDefault(t); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}
	if _, err := runDefault(t, "--file", "config.yaml"); err != nil {
		t.Fatalf("seed config.yaml: %v", err)
	}

	for name, want := range map[string]string{
		"settings.json": `{"json":true}`,
		"config.yaml":   "yaml: true\n",
	} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(body) != want {
			t.Fatalf("%s = %q, want %q", name, body, want)
		}
	}
}

// A second registration for the same file is a conflict, not a silent
// last-writer-wins. SetDefault is the deliberate override.
func TestRegisterDefaultConfigReportsConflicts(t *testing.T) {
	appDirFixture(t, "seedapp")

	if err := RegisterDefault("settings.json", []byte(`{"first":true}`)); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	err := RegisterDefault("settings.json", []byte(`{"second":true}`))
	if err == nil || !strings.Contains(err.Error(), "already registered at") {
		t.Fatalf("error = %v, want a conflict naming the first registration", err)
	}
	if got := defaultConfigs["settings.json"].content; string(got) != `{"first":true}` {
		t.Fatalf("conflicting registration mutated the registry: %s", got)
	}

	if err := SetDefault("settings.json", []byte(`{"second":true}`)); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if got := defaultConfigs["settings.json"].content; string(got) != `{"second":true}` {
		t.Fatalf("SetDefault did not override: %s", got)
	}
}

func TestRegisterDefaultConfigValidatesContent(t *testing.T) {
	appDirFixture(t, "seedapp")

	if err := RegisterDefault("settings.json", nil); err == nil {
		t.Fatal("expected an error for empty content")
	}
	if err := RegisterDefault("settings.json", []byte("{oops")); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
	// Non-JSON formats are passed through unchecked.
	if err := RegisterDefault(".env", []byte("PORT=8080\n")); err != nil {
		t.Fatalf(".env registration: %v", err)
	}
	if got := RegisteredDefaults(); len(got) != 1 || got[0] != ".env" {
		t.Fatalf("RegisteredDefaults() = %v, want [.env]", got)
	}
}

// --- --merge (upgrade path) ---------------------------------------------

// The upgrade case: a new release adds fields to the shipped default, and the
// operator's existing file must gain them without losing a single local edit.
func TestConfigDefaultMergeAddsOnlyMissingFields(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{
		"log_level": "info",
		"server": {"host": "127.0.0.1", "port": 8080, "timeout": 30},
		"feature": {"beta": false}
	}`))

	path := filepath.Join(dir, "settings.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// What the operator has: an edited port, a key the new default dropped,
	// and no knowledge of server.timeout or the feature block.
	existing := `{"log_level":"debug","server":{"host":"0.0.0.0","port":9999},"legacy":"keep me"}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	out, err := runDefault(t, "--merge")
	if err != nil {
		t.Fatalf("config default --merge: %v (output %q)", err, out)
	}

	m := map[string]any{}
	body, _ := os.ReadFile(path)
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("merged file is not valid JSON: %v\n%s", err, body)
	}

	// Local edits survive untouched.
	if m["log_level"] != "debug" {
		t.Errorf("log_level = %v, want the operator's \"debug\"", m["log_level"])
	}
	if m["legacy"] != "keep me" {
		t.Errorf("legacy = %v, want it preserved", m["legacy"])
	}
	server, _ := m["server"].(map[string]any)
	if server["host"] != "0.0.0.0" {
		t.Errorf("server.host = %v, want the operator's \"0.0.0.0\"", server["host"])
	}
	if server["port"] != float64(9999) {
		t.Errorf("server.port = %v, want the operator's 9999", server["port"])
	}

	// New fields arrive, at every nesting level.
	if server["timeout"] != float64(30) {
		t.Errorf("server.timeout = %v, want 30 from the new default", server["timeout"])
	}
	feature, ok := m["feature"].(map[string]any)
	if !ok || feature["beta"] != false {
		t.Errorf("feature block missing after merge: %#v", m["feature"])
	}

	for _, want := range []string{"+ feature", "+ server.timeout"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not report %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "+ log_level") {
		t.Errorf("output claims to have added a key that already existed:\n%s", out)
	}
}

func TestConfigDefaultMergeIsIdempotent(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{"a":1,"b":{"c":2}}`))

	if _, err := runDefault(t); err != nil {
		t.Fatalf("seed: %v", err)
	}
	path := filepath.Join(dir, "settings.json")
	first, _ := os.ReadFile(path)

	out, err := runDefault(t, "--merge")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !strings.Contains(out, "already up to date") {
		t.Fatalf("second merge did not report a no-op:\n%s", out)
	}
	if second, _ := os.ReadFile(path); !bytes.Equal(first, second) {
		t.Fatalf("a no-op merge rewrote the file:\n%s\n---\n%s", first, second)
	}
}

func TestConfigDefaultMergeDryRunWritesNothing(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{"old":1,"new":2}`))

	path := filepath.Join(dir, "settings.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := []byte(`{"old":1}`)
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	out, err := runDefault(t, "--merge", "--dry-run")
	if err != nil {
		t.Fatalf("merge --dry-run: %v", err)
	}
	if !strings.Contains(out, "would merge") || !strings.Contains(out, "+ new") {
		t.Fatalf("dry run did not preview the addition:\n%s", out)
	}
	if body, _ := os.ReadFile(path); !bytes.Equal(body, existing) {
		t.Fatalf("dry run modified the file: %s", body)
	}
}

// A key the operator turned into a different shape is their business; the
// merge must not overwrite it just because the seed disagrees.
func TestConfigDefaultMergeLeavesTypeConflictsAlone(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{"server":{"host":"127.0.0.1"}}`))

	path := filepath.Join(dir, "settings.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"server":"localhost:8080"}`), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if _, err := runDefault(t, "--merge"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	m := map[string]any{}
	body, _ := os.ReadFile(path)
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("parse merged file: %v", err)
	}
	if m["server"] != "localhost:8080" {
		t.Fatalf("a scalar the operator set was replaced by the seed's map: %#v", m["server"])
	}
}

// .env has no nesting and no comment preservation; the merge still has to add
// missing keys and keep existing ones, and must say so.
func TestConfigDefaultMergeSupportsEnvAndYaml(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault(".env", []byte("PORT=8080\nLOG_LEVEL=info\n"))
	MustRegisterDefault("config.yaml", []byte("server:\n  host: 127.0.0.1\n  port: 8080\n"))

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PORT=9999\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("server:\n  host: 0.0.0.0\n"), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	out, err := runDefault(t, "--file", ".env", "--merge")
	if err != nil {
		t.Fatalf("merge .env: %v", err)
	}
	if !strings.Contains(out, "+ LOG_LEVEL") {
		t.Errorf(".env merge did not add LOG_LEVEL:\n%s", out)
	}
	if !strings.Contains(out, "comments and key order") {
		t.Errorf(".env merge did not warn about losing comments:\n%s", out)
	}
	envBody, _ := os.ReadFile(filepath.Join(dir, ".env"))
	if !strings.Contains(string(envBody), "PORT=9999") {
		t.Errorf(".env merge discarded the operator's PORT: %s", envBody)
	}
	if !strings.Contains(string(envBody), "LOG_LEVEL=info") {
		t.Errorf(".env merge did not write LOG_LEVEL: %s", envBody)
	}

	if out, err = runDefault(t, "--file", "config.yaml", "--merge"); err != nil {
		t.Fatalf("merge config.yaml: %v", err)
	}
	if !strings.Contains(out, "+ server.port") {
		t.Errorf("yaml merge did not add the nested server.port:\n%s", out)
	}
	yamlBody, _ := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if !strings.Contains(string(yamlBody), "0.0.0.0") {
		t.Errorf("yaml merge discarded the operator's host: %s", yamlBody)
	}
}

// --merge on a missing file creates it. The report has to say so: an installer
// that runs "config default --merge" unconditionally would otherwise be told
// "already up to date" on the very run that produced the file.
func TestConfigDefaultMergeReportsCreation(t *testing.T) {
	dir := appDirFixture(t, "seedapp")
	MustRegisterDefault("settings.json", []byte(`{"a":1}`))

	out, err := runDefault(t, "--merge")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatalf("settings.json was not created: %v", err)
	}
	if !strings.Contains(out, "wrote") {
		t.Fatalf("creation reported as something other than a write:\n%s", out)
	}
	if strings.Contains(out, "already up to date") {
		t.Fatalf("a file that was just created is reported as unchanged:\n%s", out)
	}
}
