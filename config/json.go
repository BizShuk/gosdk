package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

type JsonConfig struct{}

func NewJsonConfig() Config {
	return JsonConfig{}
}

// Load reads settings.json and merges settings.local.json
func (c JsonConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetAppConfigDir())

	// Step 1: Load base settings.json
	v.SetConfigName("settings")
	v.SetConfigType("json")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Debug("Fatal error reading settings.json", "err", err)
		}
	}

	// Step 2: Merge settings.local.json (local overrides)
	v.SetConfigName("settings.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Debug("Fatal error reading settings.local.json", "err", err)
		}
	}

	return v
}

func (c JsonConfig) GetConfigName() string {
	return "settings"
}

// --- document encoding ----------------------------------------------------
//
// JsonConfig above loads settings.json into viper for the running application.
// The two functions below work on the same format one level lower: they decode
// and encode a settings document as a plain map, which is what a tool that
// edits the file (rather than consuming it) needs. Keeping both halves in this
// file means the JSON format has exactly one home in the SDK.

// ParseJSON decodes a settings document. Empty or whitespace-only input is an
// empty document, not an error — an operator truncating a file is a normal
// state, not a corruption.
//
// Numbers are decoded as json.Number so a decode/encode round trip never turns
// an integer into scientific notation.
func ParseJSON(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	m := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// WriteJSON encodes a settings document to path with four-space indentation
// and a trailing newline, matching the style of a hand-written settings.json.
//
// Go maps have no order, so the output is key-sorted by encoding/json. A
// rewrite therefore normalises key order — acceptable for JSON, which has no
// comments to lose either way.
func WriteJSON(path string, m map[string]any) error {
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
