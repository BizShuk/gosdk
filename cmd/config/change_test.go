package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// jsonNumber is the type parseValue returns for numeric JSON literals. Using
// the alias keeps test code honest about the in-memory contract: report
// values travel as json.Number, not float64.
type jsonNumber = json.Number

// Logic-layer tests call the public Show / Update /
// Delete functions directly and assert on their structured return
// values. CLI-flow tests in config_test.go continue to verify the same
// behavior end-to-end through ConfigCmd; this file is the fast, layer-pure
// safety net for the public API.

func TestShow_ReturnsEmptyNotNil(t *testing.T) {
	fixture(t, nil)

	entries := Show()
	if entries == nil {
		t.Fatal("Show returned nil; want an empty slice so callers can range without a nil check")
	}
	if len(entries) != 0 {
		t.Errorf("Show with no config = %d entries, want 0", len(entries))
	}
}

func TestShow_SortedByKey(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"z":1,"a":2,"m":3}`,
	})

	entries := Show()
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Key >= entries[i].Key {
			t.Errorf("entries not sorted by key: %q before %q", entries[i-1].Key, entries[i].Key)
		}
	}
}

func TestShow_LayerPriority(t *testing.T) {
	// Higher-precedence layers override lower: json > yaml > env.
	fixture(t, map[string]string{
		".env":          "shared=from-env\nonly_env=env\n",
		"config.yaml":   "shared: from-yaml\nonly_yaml: yaml\n",
		"settings.json": `{"shared":"from-json","only_json":"json"}`,
	})

	entries := Show()
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

func TestShow_EnvOverrideCapturedAsShadowed(t *testing.T) {
	t.Setenv("APP_SHARED", "from-envvar")
	fixture(t, map[string]string{
		"settings.json": `{"shared":"from-json"}`,
	})

	entries := Show()
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

func TestUpdate_ReportsAddForNewKey(t *testing.T) {
	dir := fixture(t, nil)

	report, err := Update([]string{"fresh=value"}, WriteOptions{})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
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

func TestUpdate_PreservesJSONTypes(t *testing.T) {
	// parseValue uses dec.UseNumber(), so numbers come back as json.Number
	// (a string-typed value that json.Marshal renders without scientific
	// notation). The report exposes the in-memory value; the file roundtrip
	// is what tests in config_test.go (TestUpdateKeepsJSONTypes) cover.
	fixture(t, nil)

	report, err := Update([]string{
		"num=1234",
		"flag=true",
		"list=[1,2,3]",
	}, WriteOptions{})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
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

func TestUpdate_ProcessesSpecsInInputOrder(t *testing.T) {
	fixture(t, nil)

	// Three updates, each later one building on the previous so a single
	// combined call proves the iteration is sequential, not parallel.
	report, err := Update([]string{"a=1", "b=2", "c=3"}, WriteOptions{})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
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

func TestUpdate_FailsOnMalformedSpec(t *testing.T) {
	fixture(t, nil)

	_, err := Update([]string{"a.b.c"}, WriteOptions{})
	if err == nil {
		t.Fatal("expected an error for a spec without '='")
	}
}

func TestUpdate_FailsWhenScalarBlocksPath(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"a":{"b":"scalar"}}`,
	})

	_, err := Update([]string{"a.b.c=xyz"}, WriteOptions{})
	if err == nil {
		t.Fatal("expected an error when descending into a scalar")
	}
}

func TestUpdate_CollectsEnvShadowWarnings(t *testing.T) {
	// APP_LOG_LEVEL shadows the flat key log_level; the report must surface
	// this so the CLI can print a warning after the change is applied.
	t.Setenv("APP_LOG_LEVEL", "debug")
	fixture(t, nil)

	report, err := Update([]string{"log_level=info"}, WriteOptions{})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("Warnings = %+v, want exactly one (APP_LOG_LEVEL shadows log_level)", report.Warnings)
	}
	w := report.Warnings[0]
	if !containsAll(w, "log_level", "APP_LOG_LEVEL", "warning") {
		t.Errorf("warning %q does not name the key, the env var, or the prefix", w)
	}
}

func TestDelete_ReportsMissingKey(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"keep":"here"}`,
	})

	_, err := Delete([]string{"nope"}, WriteOptions{})
	if err == nil {
		t.Fatal("expected an error deleting a key that does not exist")
	}
}

func TestDelete_FailureLeavesFileUntouched(t *testing.T) {
	// If any spec fails, none of the changes land — the rewrite happens
	// exactly once, after every spec has been validated in memory.
	const original = `{"keep":"here"}`
	dir := fixture(t, map[string]string{
		"settings.local.json": original,
	})

	_, err := Delete([]string{"keep", "never.existed"}, WriteOptions{})
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

// --- --append (Layer 2: element-level array ops) ------------------------

func TestAppend_CreatesArrayIfMissing(t *testing.T) {
	dir := fixture(t, nil)

	report, err := Append([]string{"tags=a", "tags=b"}, WriteOptions{})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if len(report.Changes) != 2 {
		t.Fatalf("Changes = %+v, want exactly two", report.Changes)
	}
	for i, ch := range report.Changes {
		if ch.Kind != ChangeAppended {
			t.Errorf("Changes[%d].Kind = %q, want %q", i, ch.Kind, ChangeAppended)
		}
		if ch.Key != "tags" {
			t.Errorf("Changes[%d].Key = %q, want %q", i, ch.Key, "tags")
		}
	}

	tags := readLocal(t, dir)["tags"].([]any)
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("tags = %#v, want [a b]", tags)
	}
}

func TestAppend_ToExistingArray(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"tags":["x"]}`,
	})

	if _, err := Append([]string{"tags=y"}, WriteOptions{}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	tags := readLocal(t, dir)["tags"].([]any)
	if len(tags) != 2 || tags[0] != "x" || tags[1] != "y" {
		t.Errorf("tags = %#v, want [x y]", tags)
	}
}

func TestAppend_RejectsNonArrayTarget(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"tags":"scalar"}`,
	})

	_, err := Append([]string{"tags=a"}, WriteOptions{})
	if err == nil {
		t.Fatal("expected an error when target is not an array")
	}
	if !strings.Contains(err.Error(), "not an array") {
		t.Errorf("error should explain the type mismatch, got: %v", err)
	}
}

func TestAppend_RejectsEnvFile(t *testing.T) {
	// .env files are flat — there is no array to append to. The CLI must
	// fail loud instead of silently flattening or creating a malformed file.
	fixture(t, map[string]string{".env": "TAG=x\n"})

	_, err := Append([]string{"tags=a"}, WriteOptions{File: ".env"})
	if err == nil {
		t.Fatal("expected an error appending to a .env file")
	}
	if !strings.Contains(err.Error(), ".env") {
		t.Errorf("error should name the file format, got: %v", err)
	}
}

// --- --remove-from (Layer 2: element-level array ops) ------------------

func TestRemoveFrom_RemovesFirstMatch(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"tags":["a","b","a"]}`,
	})

	report, err := RemoveFrom([]string{"tags=a"}, WriteOptions{})
	if err != nil {
		t.Fatalf("RemoveFrom failed: %v", err)
	}
	if len(report.Changes) != 1 || report.Changes[0].Kind != ChangeRemoved {
		t.Fatalf("Changes = %+v, want one %q entry", report.Changes, ChangeRemoved)
	}

	tags := readLocal(t, dir)["tags"].([]any)
	if len(tags) != 2 || tags[0] != "b" || tags[1] != "a" {
		t.Errorf("tags = %#v, want [b a] (first match removed)", tags)
	}
}

func TestRemoveFrom_MissingValueIsNoop(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"tags":["a","b"]}`,
	})

	if _, err := RemoveFrom([]string{"tags=z"}, WriteOptions{}); err != nil {
		t.Fatalf("RemoveFrom failed: %v", err)
	}

	tags := readLocal(t, dir)["tags"].([]any)
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("tags = %#v, want unchanged [a b]", tags)
	}
}

func TestRemoveFrom_RejectsNonArrayTarget(t *testing.T) {
	fixture(t, map[string]string{
		"settings.local.json": `{"tags":"scalar"}`,
	})

	_, err := RemoveFrom([]string{"tags=a"}, WriteOptions{})
	if err == nil {
		t.Fatal("expected an error when target is not an array")
	}
	if !strings.Contains(err.Error(), "not an array") {
		t.Errorf("error should explain the type mismatch, got: %v", err)
	}
}

func TestRemoveFrom_RejectsEnvFile(t *testing.T) {
	fixture(t, map[string]string{".env": "TAG=x\n"})

	_, err := RemoveFrom([]string{"tags=a"}, WriteOptions{File: ".env"})
	if err == nil {
		t.Fatal("expected an error removing from a .env file")
	}
	if !strings.Contains(err.Error(), ".env") {
		t.Errorf("error should name the file format, got: %v", err)
	}
}

// --- mixed operations ---------------------------------------------------

// TestRunConfigMixed_UpdateThenAppend covers the full Apply path
// with both update and append specs in the same call. The order in the file
// (update first, then append) is the contract that makes the CLI's call-order
// semantics predictable: --update tags=["a"] --append tags=b lands as ["a","b"].
func TestRunConfigMixed_UpdateThenAppend(t *testing.T) {
	dir := fixture(t, nil)

	_, err := Update(
		[]string{`tags=["a"]`},
		WriteOptions{},
	)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	_, err = Append([]string{"tags=b"}, WriteOptions{})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	tags := readLocal(t, dir)["tags"].([]any)
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("tags = %#v, want [a b] (update applied before append)", tags)
	}
}

// TestRunConfigMixed_AppendThenRemoveFrom proves that two separate calls on
// the same key compose: append then remove-from leaves only the appended
// element. The CLI can do the same by stacking --append and --remove-from
// flags; each call hits Apply independently, both writing
// atomically.
func TestRunConfigMixed_AppendThenRemoveFrom(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"tags":["x"]}`,
	})

	if _, err := Append([]string{"tags=y"}, WriteOptions{}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := RemoveFrom([]string{"tags=x"}, WriteOptions{}); err != nil {
		t.Fatalf("remove-from failed: %v", err)
	}

	tags := readLocal(t, dir)["tags"].([]any)
	if len(tags) != 1 || tags[0] != "y" {
		t.Errorf("tags = %#v, want [y]", tags)
	}
}

// TestRunConfigChanges_CombinedUpdateAppendRemoveAtomic exercises every
// operation class in a single Apply call through the public
// surface, asserting the report carries one change per spec and the file
// ends in the expected state.
func TestRunConfigChanges_CombinedUpdateAppendRemoveAtomic(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"tags":["keep"],"remove":"me"}`,
	})

	report, err := Apply(
		[]string{`tags=["reset"]`},
		[]string{"remove"},
		[]string{"tags=added"},
		[]string{"tags=reset"}, // matches the value update just wrote
		WriteOptions{},
	)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	// Expect three changes: update, delete, append, remove-from.
	// The order in the report mirrors the input order. A remove-from that
	// matches nothing would be skipped — that case is covered by
	// TestRemoveFrom_MissingValueIsNoop.
	wantOrder := []ChangeKind{ChangeUpdated, ChangeDeleted, ChangeAppended, ChangeRemoved}
	if len(report.Changes) != len(wantOrder) {
		t.Fatalf("Changes = %+v, want %d entries in order %v", report.Changes, len(wantOrder), wantOrder)
	}
	for i, ch := range report.Changes {
		if ch.Kind != wantOrder[i] {
			t.Errorf("Changes[%d].Kind = %q, want %q", i, ch.Kind, wantOrder[i])
		}
	}

	m := readLocal(t, dir)
	if _, present := m["remove"]; present {
		t.Errorf("remove key should be gone, got %#v", m)
	}
	tags := m["tags"].([]any)
	// After all four ops in order:
	//   tags starts at ["keep"],
	//   update sets it to ["reset"],
	//   append adds "added" → ["reset","added"],
	//   remove-from "reset" → ["added"].
	want := []any{"added"}
	if len(tags) != len(want) || tags[0] != want[0] {
		t.Errorf("tags = %#v, want %#v", tags, want)
	}
}

// The three public entry points, exercised together on one fixture.
func TestPublicEntryPoints(t *testing.T) {
	dir := fixture(t, map[string]string{
		"settings.local.json": `{"keep":"old","remove":"value"}`,
	})

	updateReport, err := Update([]string{"keep=new"}, WriteOptions{})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	deleteReport, err := Delete([]string{"remove"}, WriteOptions{})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	entries := Show()

	// Each public function exposes its work as data; the CLI layer is
	// responsible for turning that data into human-readable output, so these
	// assertions check the structured result, not a formatted string.
	if len(updateReport.Changes) != 1 {
		t.Errorf("Update changes = %+v, want exactly one change", updateReport.Changes)
	} else {
		ch := updateReport.Changes[0]
		if ch.Kind != ChangeUpdated || ch.Key != "keep" || ch.Old != "old" || ch.New != "new" {
			t.Errorf("Update change = %+v, want {Update, keep, old, new}", ch)
		}
	}
	if len(deleteReport.Changes) != 1 {
		t.Errorf("Delete changes = %+v, want exactly one change", deleteReport.Changes)
	} else {
		ch := deleteReport.Changes[0]
		if ch.Kind != ChangeDeleted || ch.Key != "remove" || ch.Old != "value" || ch.New != nil {
			t.Errorf("Delete change = %+v, want {Delete, remove, value, nil}", ch)
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
		t.Errorf("Show did not return updated keep=new entry: %+v", entries)
	}

	m := readLocal(t, dir)
	if _, ok := m["remove"]; ok {
		t.Errorf("Delete left removed key in %#v", m)
	}
}

// A report must describe what happened, not what the document looks like after
// later mutations. slices.Delete shifts elements inside the existing backing
// array, so an --append recorded before a --remove-from on the same key used
// to have its New value rewritten under it.
func TestApply_ReportSurvivesLaterMutationsOfTheSameKey(t *testing.T) {
	fixture(t, nil)

	report, err := Apply(nil, nil,
		[]string{"tags=a", "tags=b"},
		[]string{"tags=a"},
		WriteOptions{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Changes) != 3 {
		t.Fatalf("got %d changes, want 3: %+v", len(report.Changes), report.Changes)
	}

	second, _ := report.Changes[1].New.([]any)
	if len(second) != 2 || second[0] != "a" || second[1] != "b" {
		t.Errorf("the second append reads %#v after the later remove; want [a b]", second)
	}

	if before, _ := report.Changes[2].Old.([]any); len(before) != 2 || before[0] != "a" || before[1] != "b" {
		t.Errorf("remove records Old as %#v; want the pre-removal [a b]", before)
	}

	final, _ := report.Changes[2].New.([]any)
	if len(final) != 1 || final[0] != "b" {
		t.Errorf("remove result = %#v, want [b]", final)
	}
}
