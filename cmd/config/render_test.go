package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// Render-layer tests pin the public output contract of configRender.go.
// Every assertion is on a returned string, not on internal state, so the
// renderer can be swapped (e.g. for a JSON emitter) without breaking these
// tests provided the textual contract is preserved.

// --- formatValue --------------------------------------------------------

func TestFormatValue_StringPassesThrough(t *testing.T) {
	if got := formatValue("hello"); got != "hello" {
		t.Errorf("formatValue(string) = %q, want %q", got, "hello")
	}
}

func TestFormatValue_NilRendersAsSentinel(t *testing.T) {
	if got := formatValue(nil); got != "<nil>" {
		t.Errorf("formatValue(nil) = %q, want <nil>", got)
	}
}

func TestFormatValue_NumberBecomesJSON(t *testing.T) {
	// Numbers carried in the report are json.Number (parseValue uses
	// dec.UseNumber()); json.Marshal round-trips them as the literal digits.
	if got := formatValue(json.Number("1234")); got != "1234" {
		t.Errorf("formatValue(json.Number) = %q, want 1234", got)
	}
}

func TestFormatValue_BoolAndSliceAndMapBecomeJSON(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"bool", true, "true"},
		{"slice", []any{1, 2, 3}, "[1,2,3]"},
		{"empty_map", map[string]any{}, "{}"},
		{"nested_map", map[string]any{"a": "b"}, `{"a":"b"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatValue(c.in); got != c.want {
				t.Errorf("formatValue(%#v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFormatValue_FallsBackWhenMarshalFails(t *testing.T) {
	// Channels are unmarshalable, so the default branch must fall back to
	// fmt.Sprint rather than returning an empty string. fmt.Sprint formats
	// a channel as its hex pointer, so the only safe assertion is that the
	// fallback produced *something* non-empty.
	ch := make(chan int)
	got := formatValue(ch)
	if got == "" {
		t.Errorf("formatValue(chan) returned empty; want the fmt.Sprint fallback")
	}
}

// --- RenderShowTable ----------------------------------------------------

func TestRenderShowTable_EmptyReportsNotice(t *testing.T) {
	got := RenderShowTable(nil, false)
	if !strings.Contains(got, "no configuration found") {
		t.Errorf("empty show = %q, want the 'no configuration found' notice", got)
	}
}

func TestRenderShowTable_HeaderNoSourceHasOnlyTwoColumns(t *testing.T) {
	out := RenderShowTable([]Entry{{Key: "k", Value: "v"}}, false)
	// tabwriter pads; what matters is the header text and column count.
	if !strings.Contains(out, "KEY") || !strings.Contains(out, "VALUE") {
		t.Errorf("output missing KEY/VALUE header: %q", out)
	}
	if strings.Contains(out, "SOURCE") {
		t.Errorf("output without --source should not show SOURCE column: %q", out)
	}
}

func TestRenderShowTable_HeaderWithSourceHasThreeColumns(t *testing.T) {
	out := RenderShowTable([]Entry{{Key: "k", Value: "v", Source: "json"}}, true)
	for _, want := range []string{"KEY", "VALUE", "SOURCE"} {
		if !strings.Contains(out, want) {
			t.Errorf("withSource output missing %q: %q", want, out)
		}
	}
}

func TestRenderShowTable_RendersEachEntry(t *testing.T) {
	out := RenderShowTable([]Entry{
		{Key: "a", Value: "1"},
		{Key: "b", Value: "2"},
	}, false)
	for _, want := range []string{"a", "1", "b", "2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestRenderShowTable_WithSourceListsShadowedMostRecentFirst(t *testing.T) {
	// Order in Entry.Shadowed is "lowest precedence first" (per the struct
	// comment). The renderer must invert the order so the value that came
	// closest to winning sits directly under the winner.
	entry := Entry{
		Key:    "shared",
		Value:  "from-json",
		Source: "json",
		Shadowed: []ShadowedValue{
			{Value: "from-yaml", Source: "yaml"}, // lowest precedence
			{Value: "from-env", Source: "env"},   // just below the winner
		},
	}
	out := RenderShowTable([]Entry{entry}, true)
	// The env value must appear above the yaml value in the rendered table.
	envIdx := strings.Index(out, "from-env")
	yamlIdx := strings.Index(out, "from-yaml")
	if envIdx < 0 || yamlIdx < 0 {
		t.Fatalf("expected both shadowed values in output: %q", out)
	}
	if envIdx > yamlIdx {
		t.Errorf("env (most recent) should be listed before yaml (lowest): %q", out)
	}
	if !strings.Contains(out, "overridden") {
		t.Errorf("shadowed rows should be marked 'overridden': %q", out)
	}
}

func TestRenderShowTable_NoSourceHidesShadowed(t *testing.T) {
	// Without --source the user has not asked for provenance; shadowed
	// values must stay hidden so the table does not become a wall of noise.
	entry := Entry{
		Key:    "shared",
		Value:  "from-json",
		Source: "json",
		Shadowed: []ShadowedValue{
			{Value: "from-env", Source: "env"},
		},
	}
	out := RenderShowTable([]Entry{entry}, false)
	if strings.Contains(out, "from-env") {
		t.Errorf("shadowed value leaked without --source: %q", out)
	}
	if strings.Contains(out, "overridden") {
		t.Errorf("'overridden' marker leaked without --source: %q", out)
	}
}

// --- RenderChangeReport -------------------------------------------------

func TestRenderChangeReport_AddedLine(t *testing.T) {
	report := ChangeReport{
		Changes: []Change{{Kind: ChangeAdded, Key: "fresh", New: "value"}},
		Path:    "/tmp/settings.local.json",
	}
	out := RenderChangeReport(report)
	if !strings.Contains(out, "add    fresh") || !strings.Contains(out, "value") {
		t.Errorf("add line missing or malformed: %q", out)
	}
	// Add must not show "Old":
	if strings.Contains(out, "->") {
		t.Errorf("add line should not show '->' (no old value): %q", out)
	}
}

func TestRenderChangeReport_UpdatedLine(t *testing.T) {
	report := ChangeReport{
		Changes: []Change{{Kind: ChangeUpdated, Key: "k", Old: "old", New: "new"}},
		Path:    "/tmp/settings.local.json",
	}
	out := RenderChangeReport(report)
	if !strings.Contains(out, "update k") {
		t.Errorf("update line missing: %q", out)
	}
	if !strings.Contains(out, "old -> new") {
		t.Errorf("update line should show 'old -> new': %q", out)
	}
}

func TestRenderChangeReport_DeletedLine(t *testing.T) {
	report := ChangeReport{
		Changes: []Change{{Kind: ChangeDeleted, Key: "k", Old: "value"}},
		Path:    "/tmp/settings.local.json",
	}
	out := RenderChangeReport(report)
	if !strings.Contains(out, "delete k") {
		t.Errorf("delete line missing: %q", out)
	}
	if !strings.Contains(out, "was value") {
		t.Errorf("delete line should show 'was value': %q", out)
	}
}

func TestRenderChangeReport_AlwaysIncludesWrittenToLine(t *testing.T) {
	report := ChangeReport{
		Path: "/some/where/settings.local.json",
	}
	out := RenderChangeReport(report)
	if !strings.Contains(out, "written to /some/where/settings.local.json") {
		t.Errorf("output missing 'written to' line: %q", out)
	}
}

func TestRenderChangeReport_RendersAllWarnings(t *testing.T) {
	report := ChangeReport{
		Path:     "/tmp/settings.local.json",
		Warnings: []string{"warning: a is still overridden by APP_A=1", "warning: b is still overridden by APP_B=2"},
	}
	out := RenderChangeReport(report)
	if !strings.Contains(out, "APP_A=1") {
		t.Errorf("warning APP_A missing: %q", out)
	}
	if !strings.Contains(out, "APP_B=2") {
		t.Errorf("warning APP_B missing: %q", out)
	}
}

func TestRenderChangeReport_CombinedOutputPreservesOrder(t *testing.T) {
	// The renderer must produce changes-then-path-then-warnings in that
	// exact order, so a future "did the report print what I expected"
	// consumer can locate sections by content alone.
	report := ChangeReport{
		Changes:  []Change{{Kind: ChangeUpdated, Key: "k", Old: "old", New: "new"}},
		Path:     "/p",
		Warnings: []string{"w1"},
	}
	out := RenderChangeReport(report)
	changeIdx := strings.Index(out, "update k")
	pathIdx := strings.Index(out, "written to /p")
	warnIdx := strings.Index(out, "w1")
	if changeIdx < 0 || pathIdx < 0 || warnIdx < 0 {
		t.Fatalf("missing a section in output: %q", out)
	}
	if !(changeIdx < pathIdx && pathIdx < warnIdx) {
		t.Errorf("sections out of order (change=%d, path=%d, warn=%d): %q",
			changeIdx, pathIdx, warnIdx, out)
	}
}
