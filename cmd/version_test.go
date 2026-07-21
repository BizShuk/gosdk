package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// versionFixture gives each case its own working directory, since VERSION_FILE
// is resolved relative to the process working directory.
func versionFixture(t *testing.T, contents string) {
	t.Helper()

	t.Chdir(t.TempDir())
	if contents == "" {
		return
	}
	if err := os.WriteFile(VERSION_FILE, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", VERSION_FILE, err)
	}
}

func TestVersionString(t *testing.T) {
	if got := (Version{Major: 1, Minor: 2, Patch: 3}).String(); got != "1.2.3" {
		t.Errorf("String() = %q, want %q", got, "1.2.3")
	}
}

func TestParseVersion(t *testing.T) {
	if got, err := ParseVersion("1.2.3"); err != nil || got != (Version{1, 2, 3}) {
		t.Errorf("ParseVersion(\"1.2.3\") = %v, %v", got, err)
	}

	for _, bad := range []string{"", "1.2", "1.2.3.4", "a.2.3", "1.b.3", "1.2.c"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) should have failed", bad)
		}
	}
}

func TestReadVersionMissingFileIsZero(t *testing.T) {
	versionFixture(t, "")

	v, err := ReadVersion()
	if err != nil {
		t.Fatalf("a missing VERSION file must not be an error, got: %v", err)
	}
	if v != (Version{}) {
		t.Errorf("ReadVersion() = %v, want the zero Version", v)
	}
}

func TestReadVersionRejectsGarbage(t *testing.T) {
	versionFixture(t, "not-a-version\n")

	if _, err := ReadVersion(); err == nil {
		t.Error("expected an error for an unparsable VERSION file")
	}
}

func TestWriteThenReadVersion(t *testing.T) {
	versionFixture(t, "")

	want := Version{Major: 2, Minor: 5, Patch: 9}
	if err := WriteVersion(want); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}
	// Surrounding whitespace is tolerated on read.
	if err := os.WriteFile(VERSION_FILE, []byte("  "+want.String()+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	got, err := ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %v, want %v", got, want)
	}
}

// TestBumpCommands exercises the three subcommands the SDK exposes. Each RunE
// is called directly: they take no flags, so the cobra plumbing adds nothing.
func TestBumpCommands(t *testing.T) {
	tests := []struct {
		name  string
		cmd   *cobra.Command
		start string
		want  string
	}{
		{"major", MajorCmd, "1.2.3", "2.0.0"},
		{"minor", MinorCmd, "1.2.3", "1.3.0"},
		{"patch", PatchCmd, "1.2.3", "1.2.4"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			versionFixture(t, tc.start)

			if err := tc.cmd.RunE(tc.cmd, nil); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}

			data, err := os.ReadFile(VERSION_FILE)
			if err != nil {
				t.Fatalf("read %s: %v", VERSION_FILE, err)
			}
			if string(data) != tc.want {
				t.Errorf("%s: VERSION = %q, want %q", tc.name, data, tc.want)
			}
		})
	}
}
