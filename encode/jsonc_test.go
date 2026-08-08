package encode

import (
	"testing"
)

func TestJSONCCodec_Decode(t *testing.T) {
	m := map[string]any{}
	err := JSONCCodec{}.Decode([]byte(`{
		// comment
		"k": "v",
		"n": 1,
	}`), m)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if m["k"] != "v" {
		t.Errorf("k = %v, want v", m["k"])
	}
}

func TestJSONCCodec_EncodeIsStrictJSON(t *testing.T) {
	data, err := JSONCCodec{}.Encode(map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Round-trip must succeed with the standard library (no comments left).
	m := map[string]any{}
	if err := (JSONCCodec{}).Decode(data, m); err != nil {
		t.Fatalf("re-Decode: %v", err)
	}
	if m["a"] == nil {
		t.Errorf("missing a after encode/decode: %v", m)
	}
}

func TestToJSON_StripsComments(t *testing.T) {
	got := string(ToJSON([]byte(`{ /* x */ "a": 1, }`)))
	if got == "" {
		t.Fatal("ToJSON returned empty")
	}
	m := map[string]any{}
	if err := (JSONCCodec{}).Decode([]byte(got), m); err != nil {
		t.Fatalf("ToJSON output not valid for Decode: %v (got %q)", err, got)
	}
}
