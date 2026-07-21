package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type YamlConfig struct{}

func NewYamlConfig() Config {
	return &YamlConfig{}
}

// Load reads config.yaml and merges config.local.yaml
func (c *YamlConfig) Load() *viper.Viper {
	v := viper.New()
	v.AddConfigPath(".")
	v.AddConfigPath("conf")
	v.AddConfigPath(GetAppConfigDir())

	// Step 1: Load base config.yaml
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Debug("Warning reading config.yaml", "err", err)
		}
	}

	// Step 2: Merge config.local.yaml (local overrides)
	v.SetConfigName("config.local")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Debug("Warning reading config.local.yaml", "err", err)
		}
	}

	return v
}

// GetConfigName is no longer used but kept for interface compatibility
func (c *YamlConfig) GetConfigName() string {
	return ""
}

// --- document encoding ----------------------------------------------------
//
// YamlConfig above loads config.yaml into viper for the running application.
// The two functions below work on the same format one level lower: they decode
// and encode a config document as a plain map, which is what a tool that edits
// the file (rather than consuming it) needs.

// ParseYAML decodes a config document. Empty or whitespace-only input is an
// empty document, not an error.
func ParseYAML(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	m := map[string]any{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// WriteYAML encodes a config document to path.
//
// Comments and the original key order are not preserved: the document arrives
// here as a map, which carries neither. Callers that rewrite a file an
// operator maintains by hand should say so before doing it.
func WriteYAML(path string, m map[string]any) error {
	data, err := yaml.Marshal(denumber(m))
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// denumber replaces every json.Number in a document with a plain Go number.
//
// A document can reach this encoder from a JSON source — ParseJSON decodes
// numbers as json.Number so a JSON round trip never rewrites 8080 as 8.08e3.
// json.Number is a string type, though, so handing one to the YAML encoder
// emits a quoted "8080" and the value silently changes type. Converting on the
// way out keeps the two formats interchangeable, which is the whole point of
// sharing one in-memory document across them.
func denumber(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = denumber(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = denumber(val)
		}
		return out
	}
	return v
}
