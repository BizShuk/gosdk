package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cfgcmd "github.com/bizshuk/gosdk/cmd/config"
	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/pflag"
)

// ConfigDefaultCmd is a shell over cfgcmd.Default; the behaviour of the write
// itself is covered in cmd/config. What is only testable here is the wiring:
// that each flag reaches the right argument, that cobra rejects the
// combination that has no meaning, and that the rendered report reaches
// stdout.

// seedFixture points config.GetAppConfigDir() at a temp home and installs seed
// content. SetDefault rather than RegisterDefault, because the registry is
// process-wide and these tests run after whatever else touched it.
func seedFixture(t *testing.T, file, content string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	config.SetAppName("clitest")
	t.Cleanup(func() { config.SetAppName("") })

	if err := cfgcmd.SetDefault(file, []byte(content)); err != nil {
		t.Fatalf("seed %s: %v", file, err)
	}

	return filepath.Join(home, ".config", "clitest")
}

// runDefault executes "config default" through the shared command tree.
func runDefault(t *testing.T, args ...string) (string, error) {
	t.Helper()

	configDefaultFile = cfgcmd.DEFAULT_CONFIG_FILE
	configDefaultForce = false
	configDefaultMerge = false
	configDefaultDryRun = false
	configDefaultLocal = false
	ConfigDefaultCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

	var buf bytes.Buffer
	ConfigCmd.SetOut(&buf)
	ConfigCmd.SetErr(&buf)
	ConfigCmd.SetArgs(append([]string{"default"}, args...))
	t.Cleanup(func() { ConfigCmd.SetArgs(nil) })

	err := ConfigCmd.Execute()
	return buf.String(), err
}

func TestConfigDefaultCmdWritesAndReports(t *testing.T) {
	dir := seedFixture(t, "settings.json", `{"server":{"port":8080}}`)

	out, err := runDefault(t)
	if err != nil {
		t.Fatalf("config default: %v (output %q)", err, out)
	}

	path := filepath.Join(dir, "settings.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded file: %v", err)
	}
	if string(body) != `{"server":{"port":8080}}` {
		t.Fatalf("seeded content = %s", body)
	}
	if !strings.Contains(out, "wrote "+path) {
		t.Fatalf("output %q does not report the written path", out)
	}
}

// --file must reach cfgcmd.Default, not be quietly ignored.
func TestConfigDefaultCmdHonoursFileFlag(t *testing.T) {
	dir := seedFixture(t, "config.yaml", "server:\n  port: 8080\n")

	if _, err := runDefault(t, "--file", "config.yaml"); err != nil {
		t.Fatalf("config default --file config.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("--file did not reach the logic layer: %v", err)
	}
}

// --dry-run must reach the logic layer as the dryRun argument.
func TestConfigDefaultCmdHonoursDryRun(t *testing.T) {
	dir := seedFixture(t, "settings.json", `{"a":1}`)

	out, err := runDefault(t, "--dry-run")
	if err != nil {
		t.Fatalf("config default --dry-run: %v", err)
	}
	if !strings.HasPrefix(out, "would write ") {
		t.Fatalf("output %q is not a dry-run report", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("dry run created the file (stat err = %v)", err)
	}
}

// --merge and --force select different modes; only the mapping is under test
// here, the merge semantics themselves live in cmd/config.
func TestConfigDefaultCmdMapsMergeAndForceToModes(t *testing.T) {
	dir := seedFixture(t, "settings.json", `{"a":1,"b":2}`)
	path := filepath.Join(dir, "settings.json")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"a":99}`), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	out, err := runDefault(t, "--merge")
	if err != nil {
		t.Fatalf("--merge: %v", err)
	}
	if !strings.Contains(out, "+ b") {
		t.Fatalf("--merge did not reach merge mode:\n%s", out)
	}
	if body, _ := os.ReadFile(path); !strings.Contains(string(body), `"a": 99`) {
		t.Fatalf("--merge overwrote an existing value: %s", body)
	}

	if _, err := runDefault(t, "--force"); err != nil {
		t.Fatalf("--force: %v", err)
	}
	if body, _ := os.ReadFile(path); string(body) != `{"a":1,"b":2}` {
		t.Fatalf("--force did not replace the file: %s", body)
	}
}

// Merging and replacing are opposite answers to the same question; cobra is
// configured to reject the pair rather than let one silently win.
func TestConfigDefaultCmdRejectsMergeWithForce(t *testing.T) {
	seedFixture(t, "settings.json", `{"a":1}`)

	if _, err := runDefault(t, "--merge", "--force"); err == nil {
		t.Fatal("expected --merge --force to be rejected")
	}
}

// Without an app name the command refuses instead of seeding the working
// directory; the target of a first-run default has to be deliberate.
func TestConfigDefaultCmdRefusesWithoutAppName(t *testing.T) {
	seedFixture(t, "settings.json", `{"a":1}`)
	config.SetAppName("")
	t.Chdir(t.TempDir())

	_, err := runDefault(t)
	if err == nil {
		t.Fatal("expected a refusal when no app name is set")
	}
	if !strings.Contains(err.Error(), "no application config directory") {
		t.Errorf("error does not name the cause: %v", err)
	}
	if !strings.Contains(err.Error(), "--local") {
		t.Errorf("error does not offer the escape hatch: %v", err)
	}
}

// --local is that deliberate choice, and it must still work with no app name.
func TestConfigDefaultCmdLocalWritesToWorkingDirectory(t *testing.T) {
	seedFixture(t, "settings.json", `{"a":1}`)
	config.SetAppName("")

	dir := t.TempDir()
	t.Chdir(dir)

	if _, err := runDefault(t, "--local"); err != nil {
		t.Fatalf("config default --local: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatalf("--local did not write to the working directory: %v", err)
	}
}
