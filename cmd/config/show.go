package config

import (
	"os"
	"sort"
	"strings"

	sdkconfig "github.com/bizshuk/gosdk/config"
)

// LAYER_OS_ENV is the layer name for a value that came from an OS environment
// variable rather than a file. It is not one of sdkconfig's loader layers —
// nothing reads it off disk — but it sits on top of all of them, so the merged
// view needs a name for it.
const LAYER_OS_ENV = "os-env"

// ShadowedValue is a value lost to a higher-precedence file or variable.
type ShadowedValue struct {
	Value  any
	Layer  string // env | yaml | json | os-env
	Source string // the file path it was read from, or the env var name
}

// Entry is one key in the merged view: the winning value, where it was read
// from, and the values it overrode, listed low precedence first.
//
// Source is the resolved path of the file the value came from — not the layer
// name — because "which file" is the question a merged view leaves open: the
// same settings.json can be the one in the checkout, the one in ./conf, or the
// one in the app config dir, and only the path tells them apart.
type Entry struct {
	Key      string
	Value    any
	Layer    string
	Source   string
	Shadowed []ShadowedValue
}

// Show returns the merged configuration entries, sorted by key. Each entry
// carries the winning value, the file or variable it came from, and the values
// it overrode (lowest precedence first); the renderer decides what to display.
//
// The merge walks sdkconfig.Sources() in the order the config package merges
// them, one file at a time, so this view cannot drift from the one the running
// application sees — the file list, the search path and the precedence order
// all stay owned by the config package.
//
// A file that fails to parse is skipped, matching the loaders: they log the
// failure and carry on with what they could read, and a view that errored out
// where the application starts fine would be reporting a different program.
//
// The returned slice is always non-nil; an empty config yields an empty slice.
func Show() []Entry {
	byKey := map[string]*Entry{}

	for _, src := range sdkconfig.Sources() {
		if src.Path == "" {
			continue
		}
		v, err := sdkconfig.LoadFile(src.Path, src.Layer)
		if err != nil {
			continue
		}

		flat := map[string]any{}
		flattenSettings(v.AllSettings(), "", flat)

		for k, val := range flat {
			if prev, seen := byKey[k]; seen {
				prev.Shadowed = append(prev.Shadowed,
					ShadowedValue{Value: prev.Value, Layer: prev.Layer, Source: prev.Source})
				prev.Value, prev.Layer, prev.Source = val, src.Layer, src.Path
				continue
			}
			byKey[k] = &Entry{Key: k, Value: val, Layer: src.Layer, Source: src.Path}
		}
	}

	// OS environment variables outrank every config file.
	for k, e := range byKey {
		name, value, ok := envOverride(k)
		if !ok {
			continue
		}
		e.Shadowed = append(e.Shadowed,
			ShadowedValue{Value: e.Value, Layer: e.Layer, Source: e.Source})
		e.Value, e.Layer, e.Source = value, LAYER_OS_ENV, name
	}

	entries := make([]Entry, 0, len(byKey))
	for _, e := range byKey {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries
}

// flattenSettings turns a nested settings map into dotted viper keys ("a.b.c").
// An empty map is emitted as a leaf so a parent left behind by Delete is still
// reported, even though viper itself exposes no key for it.
func flattenSettings(m map[string]any, prefix string, out map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if child, ok := v.(map[string]any); ok && len(child) > 0 {
			flattenSettings(child, key, out)
			continue
		}
		out[key] = v
	}
}

// envOverride reports the OS environment variable shadowing a key, if any.
//
// sdkconfig.Default() calls AutomaticEnv() and then binds every leaf key with
// bindAllEnvVars, so the name viper looks up is UPPER(key) with "." replaced by
// "_" and no prefix: log_level reads LOG_LEVEL, and the nested server.port
// reads SERVER_PORT. Composing the name the same way here is what keeps this
// view honest — a mismatch reports an override the application will not apply,
// or hides one it will.
func envOverride(key string) (name, value string, ok bool) {
	name = strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	value, ok = os.LookupEnv(name)
	return name, value, ok
}
