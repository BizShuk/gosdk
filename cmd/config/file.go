package config

import (
	"fmt"
	"strings"
)

// DEFAULT_CONFIG_FILE is the file "config default" and "config" use when no
// --file is passed. It is the highest-precedence JSON file in the merge order,
// so a value written there overrides every other config file but never an
// APP_ environment variable.
const DEFAULT_CONFIG_FILE = "settings.json"

// LOCAL_SETTINGS_FILE is the layer-local override of the default JSON file.
// It is the last file the JSON loader merges, which makes it the natural home
// for ad-hoc overrides written from the CLI.
const LOCAL_SETTINGS_FILE = "settings.local.json"

// defaultConfigFiles is the set of file names the config command family
// (ConfigCmd, ConfigDefaultCmd) accept. It mirrors the files the config
// loaders actually read, so neither command can write a file config.Default()
// would never open.
var defaultConfigFiles = []string{
	".env", ".env.local",
	"config.yaml", "config.local.yaml",
	"settings.json", "settings.local.json",
}

// configFilePrecedence is the order, lowest to highest, in which the loader
// chain merges files. A write to a file lower in the list can be overridden by
// every file above it (and by APP_ environment variables, which sit on top
// of the whole list).
//
// Keep this in sync with config/config.go:loadAllConfigs and the per-loader
// MergeInConfig calls — any new file added there needs a slot here.
var configFilePrecedence = []string{
	"config.yaml",
	"config.local.yaml",
	"settings.json",
	"settings.local.json",
	".env",
	".env.local",
}

// normalizeFile validates a file name against the set the config loaders read
// and returns the canonical spelling. Comparison is case-insensitive because
// the caller is typing a file name, not a viper key.
//
// An empty name resolves to fallback, which differs per caller: a mutation
// defaults to the highest-precedence local file, seeding a default to the
// shipped one. Passing it in keeps that choice at the call site instead of
// duplicating this function once per default.
func normalizeFile(file, fallback string) (string, error) {
	name := strings.TrimSpace(file)
	if name == "" {
		return fallback, nil
	}
	for _, known := range defaultConfigFiles {
		if strings.EqualFold(name, known) {
			return known, nil
		}
	}
	return "", fmt.Errorf("unsupported config file %q: expected one of %s",
		file, strings.Join(defaultConfigFiles, ", "))
}

// fileFormat reports the wire format of a config file by its name.
// Unknown names return ""; callers handle that as an error.
func fileFormat(name string) string {
	switch {
	case strings.HasPrefix(name, ".env"):
		return "env"
	case strings.HasSuffix(name, ".yaml"), strings.HasSuffix(name, ".yml"):
		return "yaml"
	case strings.HasSuffix(name, ".json"):
		return "json"
	}
	return ""
}

// higherPrecedenceFiles lists the supported files that would shadow a write
// to name. The list is in precedence order (lowest shadow first); the caller
// decides whether to surface one warning or all of them.
func higherPrecedenceFiles(name string) []string {
	idx := -1
	for i, n := range configFilePrecedence {
		if n == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil
	}
	out := make([]string, 0, len(configFilePrecedence)-idx-1)
	for _, n := range configFilePrecedence[idx+1:] {
		out = append(out, n)
	}
	return out
}

// envFlattenKey converts a dotted viper-style key into a flat env var name
// by joining segments with "_" and uppercasing. a.b.c becomes A_B_C; the
// result is the only thing .env can express, since .env has no notion of
// nesting.
func envFlattenKey(segments []string) string {
	return strings.ToUpper(strings.Join(segments, "_"))
}

// SupportedFiles returns the config file names any of this package's write
// operations accept, in declaration order. The slice is a copy: the catalog is
// package state and a caller building flag help text must not be able to
// reorder or truncate it.
func SupportedFiles() []string {
	return append([]string(nil), defaultConfigFiles...)
}
