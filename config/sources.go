package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// This file answers one question the loaders themselves cannot: *which file on
// this machine* did a value come from. YamlConfig/JsonConfig/EnvConfig each
// hand viper a search path and a config name and get back a merged view, so by
// the time Default() is done the origin of any single key is gone. Sources()
// resolves the same search the loaders run, one file at a time, and keeps the
// path — which is what a "show me the merged config and where each value came
// from" front end needs.

// Source is one config file the loader chain reads: the layer that loads it,
// the name it is searched under, and the path it resolved to.
//
// Path is empty when no search directory holds the file. A missing file is
// still worth listing: "nowhere" is a legitimate answer to "where would this
// value come from".
type Source struct {
	Layer string // env | yaml | json | vault — the loader that reads this file
	Name  string // file name as searched, e.g. "settings.local.json"
	Path  string // resolved absolute path, "" when the file was not found
}

// sourceCatalog lists every file loadAllConfigs merges, lowest precedence
// first. Keep it in sync with loadAllConfigs and the per-loader ReadInConfig /
// MergeInConfig calls in yaml.go, json.go and env.go — a file missing here is
// a value with no reported origin, and an order that disagrees with
// loadAllConfigs reports the wrong winner.
var sourceCatalog = []Source{
	{Layer: "yaml", Name: "config.yaml"},
	{Layer: "yaml", Name: "config.local.yaml"},
	{Layer: "json", Name: "settings.json"},
	{Layer: "json", Name: "settings.local.json"},
	{Layer: "vault", Name: VAULT_FILE},
	{Layer: "vault", Name: VAULT_LOCAL_FILE},
	{Layer: "env", Name: ".env"},
	{Layer: "env", Name: ".env.local"},
}

// sourceAlternates maps a catalog name to the other spelling viper would also
// accept for it. Only YAML has one, because only YAML has two extensions in
// common use.
var sourceAlternates = map[string]string{
	"config.yaml":       "config.yml",
	"config.local.yaml": "config.local.yml",
}

// configTypes maps a layer to the viper parser that reads it. dotenv is spelled
// differently from the layer name, which is the whole reason for the table.
var configTypes = map[string]string{
	"yaml":  "yaml",
	"json":  "json",
	"env":   "dotenv",
	"vault": VAULT_FORMAT,
}

// SearchPaths returns the directories the loaders search, in viper's order:
// the working directory, ./conf, then the application config directory.
//
// The app directory is omitted when GetAppConfigDir() is empty — no app name
// registered and no directory forced — which is exactly what viper does with
// an empty AddConfigPath.
func SearchPaths() []string {
	paths := []string{".", "conf"}
	if dir := GetAppConfigDir(); dir != "" {
		paths = append(paths, dir)
	}
	return paths
}

// Sources returns every config file the loader chain reads, lowest precedence
// first, each resolved the way its loader resolves it: the first directory
// holding the file wins, and every file is resolved independently — a base
// file found in the working directory does not stop its .local counterpart
// from being picked up in the app config directory.
//
// Only the canonical names are resolved. A loader hands viper a config name
// without an extension, so viper would also accept config.json for the yaml
// layer; that ambiguity is deliberately not reproduced here.
func Sources() []Source {
	dirs := SearchPaths()
	out := make([]Source, 0, len(sourceCatalog))
	for _, src := range sourceCatalog {
		names := []string{src.Name}
		if alt, ok := sourceAlternates[src.Name]; ok {
			names = append(names, alt)
		}
		src.Path = findInSearchPath(dirs, names)
		out = append(out, src)
	}
	return out
}

// mergeNamedFiles resolves each name against the search path and merges the
// ones that exist into v, in the order given — later names override earlier
// ones, which is what "base then .local" means.
//
// Resolving exact names is deliberate. Handing viper a config name instead lets
// it match any supported extension, so a loader asked for ".env" will happily
// read ".env.vault" once the vault format is registered. The loaders address
// files, not name stems, and this is where that is enforced.
func mergeNamedFiles(v *viper.Viper, configType string, names ...string) {
	dirs := SearchPaths()
	for _, name := range names {
		path := findInSearchPath(dirs, []string{name})
		if path == "" {
			continue
		}
		v.SetConfigFile(path)
		v.SetConfigType(configType)
		if err := v.MergeInConfig(); err != nil {
			slog.Warn("failed to read config file",
				"file", name, "path", path, "type", configType, "err", err)
		}
	}
}

// findInSearchPath returns the absolute path of the first name that exists,
// scanning directory by directory so an earlier directory always wins over a
// later one — viper's own order.
func findInSearchPath(dirs, names []string) string {
	for _, dir := range dirs {
		for _, name := range names {
			path := filepath.Join(dir, name)
			st, err := os.Stat(path)
			if err != nil || st.IsDir() {
				continue
			}
			if abs, err := filepath.Abs(path); err == nil {
				return abs
			}
			return path
		}
	}
	return ""
}

// LoadFile reads a single config file into its own viper instance, parsed as
// the given layer implies (env files as dotenv, and so on).
//
// Going through viper rather than the Parse* helpers keeps key normalisation
// identical to the loaders' — lowercased keys, "." nesting — so a caller can
// attribute a value in the merged view to the exact file it came from without
// the two views disagreeing about what the key is called.
//
// The json layer uses the JSONC codec registry, matching JsonConfig.Load, and
// the vault layer uses the same credential resolution as VaultConfig.Load
// (VAULT_TOKEN, else VAULT_PASSWORD) — with no credential the read fails,
// which is the same outcome the loader chain produces (no values from that
// file). Other layers use plain viper.New().
func LoadFile(path, layer string) (*viper.Viper, error) {
	v := viper.New()
	switch layer {
	case "json":
		v = viper.NewWithOptions(viper.WithCodecRegistry(codecRegistry))
	case "vault":
		vc, _ := NewVaultConfig().(VaultConfig)
		v = viper.NewWithOptions(viper.WithCodecRegistry(vc.registry()))
	}
	v.SetConfigFile(path)
	if t, ok := configTypes[layer]; ok {
		v.SetConfigType(t)
	}
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v, nil
}
