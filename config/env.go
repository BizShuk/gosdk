package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

func NewEnvConfig() Config {
	return EnvConfig{}
}

type EnvConfig struct{}

// Load reads .env and merges .env.local.
//
// Both files are resolved by exact name rather than handed to viper as a
// config name: viper would treat ".env" as a stem and match any supported
// extension, which since the vault format was registered includes the
// encrypted ".env.vault" sitting in the same directory.
func (c EnvConfig) Load() *viper.Viper {
	v := viper.New()
	mergeNamedFiles(v, "dotenv", ".env", ".env.local")
	return v
}

// GetConfigName is no longer used but kept for interface compatibility
func (c EnvConfig) GetConfigName() string {
	return ""
}

// --- document encoding ----------------------------------------------------
//
// EnvConfig above loads .env into viper for the running application. The
// functions below work on the same format one level lower: they decode and
// encode a dotenv document as a flat map, which is what a tool that edits the
// file (rather than consuming it) needs.
//
// dotenv has no nesting. A caller holding a multi-segment key must flatten it
// to a single name before it can be written here.

// ParseEnv decodes a dotenv document into a flat map. Keys are the env var
// names as written (case preserved); values have surrounding double quotes
// stripped. Blank lines and lines starting with "#" are skipped. A line
// without "=" is skipped rather than treated as an error: one malformed line
// should not stop a caller from reading back the keys it cares about.
func ParseEnv(data []byte) (map[string]any, error) {
	m := map[string]any{}
	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		m[strings.TrimSpace(key)] = unquoteEnvValue(strings.TrimSpace(value))
	}
	return m, nil
}

// WriteEnv encodes a flat map as a dotenv document at path. Keys are written
// in sorted order so repeated runs are byte-stable.
//
// Comments in an existing file are not preserved, and a nested value has no
// dotenv representation — it is rendered as JSON, which viper will read back
// as a string. Flatten before calling if that matters.
func WriteEnv(path string, m map[string]any) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, renderEnvValue(m[k]))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// unquoteEnvValue strips a matching pair of surrounding double quotes, with no
// other escape handling. That covers the common case where a value contains
// whitespace; anything fancier (\n, $VAR) is deliberately out of scope.
func unquoteEnvValue(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// renderEnvValue converts an in-memory value to its dotenv string form.
// Strings are quoted when they contain characters a parser would treat as
// structure; everything else goes through JSON for a stable textual form.
func renderEnvValue(v any) string {
	switch x := v.(type) {
	case string:
		if strings.ContainsAny(x, " \t#\"'") {
			return `"` + strings.ReplaceAll(x, `"`, `\"`) + `"`
		}
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
