package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// jsonNumber is the type parseValue returns for numeric JSON literals. Using
// the alias keeps test code honest about the in-memory contract: report
// values travel as json.Number, not float64.
type jsonNumber = json.Number

// Logic-layer tests call the public RunConfigShow / RunConfigUpdate /
// RunConfigDelete functions directly and assert on their structured return
// values. CLI-flow tests in config_test.go continue to verify the same
// behavior end-to-end through ConfigCmd; this file is the fast, layer-pure
// safety net for the public API.

func TestRunConfigShow_ReturnsEmptyNotNil(t *testing.T) {
	fixture(t, nil)

	entries := RunConfigShow()
	if entries == nil {
		t.Fatal("RunConfigShow returned nil; want an empty slice so callers can range without a nil check")
	}
	if len(entries) != 0 {
		t.Errorf("RunConfigShow with no config = %d entries, want 0", len(entries))
	}
}

func TestRunConfigShow_SortedByKey(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"z":1,"a":2,"m":3}`,
	})

	entries := RunConfigShow()
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Key >= entries[i].Key {
			t.Errorf("entries not sorted by key: %q before %q", entries[i-1].Key, entries[i].Key)
		}
	}
}

func TestRunConfigShow_LayerPriority(t *testing.T) {
	// Higher-precedence layers override lower: json > yaml > env.
	fixture(t, map[string]string{
		".env":          "shared=from-env\nonly_env=env\n",
		"config.yaml":   "shared: from-yaml\nonly_yaml: yaml\n",
		"settings.json": `{"shared":"from-json","only_json":"json"}`,
	})

	entries := RunConfigShow()
	byKey := make(map[string]string, len(entries))
	sourceByKey := make(map[string]string, len(entries))
	for _, e := range entries {
		s, ok := e.Value.(string)
		if !ok {
			t.Errorf("entry %q value = %#v (%T), want a string", e.Key, e.Value, e.Value)
			continue
		}
		byKey[e.Key] = s
		sourceByKey[e.Key] = e.Source
	}
	if byKey["shared"] != "from-json" {
		t.Errorf("shared = %q, want from-json (json beats yaml + env)", byKey["shared"])
	}
	if sourceByKey["shared"] != "json" {
		t.Errorf("shared source = %q, want json", sourceByKey["shared"])
	}
	// Each layer's sole-sourced key must still surface.
	for _, k := range []string{"only_env", "only_yaml", "only_json"} {
		if _, ok := byKey[k]; !ok {
			t.Errorf("missing layer-only key %q: %+v", k, entries)
		}
	}
}

func TestRunConfigShow_EnvOverrideCapturedAsShadowed(t *testing.T) {
	t.Setenv("APP_SHARED", "from-envvar")
	fixture(t, map[string]string{
		"settings.json": `{"shared":"from-json"}`,
	})

	entries := RunConfigShow()
	var found *Entry
	for i := range entries {
		if entries[i].Key == "shared" {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("shared entry not found: %+v", entries)
	}
	if found.Value != "from-envvar" {
		t.Errorf("shared value = %v, want from-envvar", found.Value)
	}
	if found.Source != "APP_SHARED" {
		t.Errorf("shared source = %q, want APP_SHARED", found.Source)
	}
	if len(found.Shadowed) != 1 {
		t.Fatalf("shared shadowed = %+v, want exactly one (the json value)", found.Shadowed)
	}
	if found.Shadowed[0].Value != "from-json" || found.Shadowed[0].Source != "json" {
		t.Errorf("shared shadowed[0] = %+v, want {from-json, json}", found.Shadowed[0])
	}
}

func TestRunConfigUpdate_ReportsAddForNewKey(t *testing.T) {
	dir := fixture(t, nil)

	report, err := RunConfigUpdate([]string{"fresh=value"})
	if err != nil {
		t.Fatalf("RunConfigUpdate failed: %v", err)
	}
	if len(report.Changes) != 1 {
		t.Fatalf("Changes = %+v, want exactly one", report.Changes)
	}
	ch := report.Changes[0]
	if ch.Kind != ChangeAdded {
		t.Errorf("Kind = %q, want add (key did not exist before)", ch.Kind)
	}
	if ch.Key != "fresh" || ch.Old != nil || ch.New != "value" {
		t.Errorf("Change = %+v, want {add, fresh, nil, value}", ch)
	}
	// Path must point at the actual write target, not be empty.
	if report.Path == "" {
		t.Error("Path is empty; want the settings.local.json path the file was written to")
	}
	// File actually written.
	if _, ok := readLocal(t, dir)["fresh"]; !ok {
		t.Errorf("new key did not land in %s: %+v", LOCAL_SETTINGS_FILE, readLocal(t, dir))
	}
}

func TestRunConfigUpdate_PreservesJSONTypes(t *testing.T) {
	// parseValue uses dec.UseNumber(), so numbers come back as json.Number
	// (a string-typed value that json.Marshal renders without scientific
	// notation). The report exposes the in-memory value; the file roundtrip
	// is what tests in config_test.go (TestUpdateKeepsJSONTypes) cover.
	fixture(t, nil)

	report, err := RunConfigUpdate([]string{
		"num=1234",
		"flag=true",
		"list=[1,2,3]",
	})
	if err != nil {
		t.Fatalf("RunConfigUpdate failed: %v", err)
	}
	got := make(map[string]any, len(report.Changes))
	for _, ch := range report.Changes {
		got[ch.Key] = ch.New
	}
	if _, ok := got["num"].(jsonNumber); !ok {
		t.Errorf("num = %#v (%T), want json.Number (in-memory number type)", got["num"], got["num"])
	}
	if got["flag"] != true {
		t.Errorf("flag = %#v, want true", got["flag"])
	}
	arr, ok := got["list"].([]any)
	if !ok || len(arr) != 3 {
		t.Errorf("list = %#v, want 3-element array", got["list"])
	}
}

func TestRunConfigUpdate_ProcessesSpecsInInputOrder(t *testing.T) {
	fixture(t, nil)

	// Three updates, each later one building on the previous so a single
	// combined call proves the iteration is sequential, not parallel.
	report, err := RunConfigUpdate([]string{"a=1", "b=2", "c=3"})
	if err != nil {
		t.Fatalf("RunConfigUpdate failed: %v", err)
	}
	wantKeys := []string{"a", "b", "c"}
	if len(report.Changes) != len(wantKeys) {
		t.Fatalf("Changes = %+v, want %d entries in input order", report.Changes, len(wantKeys))
	}
	for i, ch := range report.Changes {
		if ch.Key != wantKeys[i] {
			t.Errorf("Changes[%d].Key = %q, want %q (input order)", i, ch.Key, wantKeys[i])
		}
	}
}

func TestRunConfigUpdate_FailsOnMalformedSpec(t *testing.T) {
	fixture(t, nil)

	_, err := RunConfigUpdate([]string{"a.b.c"})
	if err == nil {
		t.Fatal("expected an error for a spec without '='")
	}
}

func TestRunConfigUpdate_FailsWhenScalarBlocksPath(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"a":{"b":"scalar"}}`,
	})

	_, err := RunConfigUpdate([]string{"a.b.c=xyz"})
	if err == nil {
		t.Fatal("expected an error when descending into a scalar")
	}
}

func TestRunConfigUpdate_CollectsEnvShadowWarnings(t *testing.T) {
	// APP_LOG_LEVEL shadows the flat key log_level; the report must surface
	// this so the CLI can print a warning after the change is applied.
	t.Setenv("APP_LOG_LEVEL", "debug")
	fixture(t, nil)

	report, err := RunConfigUpdate([]string{"log_level=info"})
	if err != nil {
		t.Fatalf("RunConfigUpdate failed: %v", err)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("Warnings = %+v, want exactly one (APP_LOG_LEVEL shadows log_level)", report.Warnings)
	}
	w := report.Warnings[0]
	if !containsAll(w, "log_level", "APP_LOG_LEVEL", "warning") {
		t.Errorf("warning %q does not name the key, the env var, or the prefix", w)
	}
}

func TestRunConfigDelete_ReportsMissingKey(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"keep":"here"}`,
	})

	_, err := RunConfigDelete([]string{"nope"})
	if err == nil {
		t.Fatal("expected an error deleting a key that does not exist")
	}
}

func TestRunConfigDelete_FailureLeavesFileUntouched(t *testing.T) {
	// If any spec fails, none of the changes land — the rewrite happens
	// exactly once, after every spec has been validated in memory.
	const original = `{"keep":"here"}`
	dir := fixture(t, map[string]string{
		"settings.local.json": original,
	})

	_, err := RunConfigDelete([]string{"keep", "never.existed"})
	if err == nil {
		t.Fatal("expected the second delete to fail")
	}
	m := readLocal(t, dir)
	if m["keep"] != "here" {
		t.Errorf("keep = %#v, want \"here\" (a failing spec must not half-apply)", m["keep"])
	}
}

// containsAll reports whether s contains every non-empty needle.
func containsAll(s string, needles ...string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if !strings.Contains(s, n) {
			return false
		}
	}
	return true
}
