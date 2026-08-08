package encode

import (
	"encoding/json"

	"github.com/tidwall/jsonc"
)

// JSONCCodec decodes JSONC and encodes strict JSON.
//
// The method set matches viper.Codec, so a config loader can register it for
// format "json" without this package importing viper:
//
//	reg.RegisterCodec("json", encode.JSONCCodec{})
//
// Decode accepts // and /* */ comments and trailing commas. Encode always
// emits standard JSON (comments are not preserved).
type JSONCCodec struct{}

// Decode fills v from JSONC bytes.
func (JSONCCodec) Decode(b []byte, v map[string]any) error {
	return json.Unmarshal(jsonc.ToJSON(b), &v)
}

// Encode serialises v as indented standard JSON.
func (JSONCCodec) Encode(v map[string]any) ([]byte, error) {
	return json.MarshalIndent(v, "", "    ")
}

// ToJSON strips JSONC comments and trailing commas, returning standard JSON
// bytes suitable for encoding/json.
func ToJSON(b []byte) []byte {
	return jsonc.ToJSON(b)
}
