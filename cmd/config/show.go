package config

import (
	"os"
	"sort"

	sdkconfig "github.com/bizshuk/gosdk/v2/config"
)

// LAYER_OS_ENV is the layer name for a value that came from an OS environment
// variable rather than a file. It is not one of sdkconfig's loader layers —
// nothing reads it off disk — and it sits under all of them (above only the
// WithDefaultValue layer), so the merged view needs a name for it.
const LAYER_OS_ENV = "os-env"

// ShadowedValue is a value lost to a higher-precedence file or variable.
type ShadowedValue struct {
	Value  any
	Layer  string // default | env | yaml | json | os-env
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

// Show returns the merged configuration entries, sorted by key, starting from
// the WithDefaultValue layer (source "default") under every file. Each entry
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

	// The WithDefaultValue layer sits under every file; it has no path, so its
	// source is the layer name.
	defaults := map[string]any{}
	flattenSettings(sdkconfig.DefaultValues(), "", defaults)
	for k, val := range defaults {
		byKey[k] = &Entry{Key: k, Value: val, Layer: sdkconfig.LAYER_DEFAULT, Source: sdkconfig.LAYER_DEFAULT}
	}

	// Files are read before anything is merged: the OS env layer sits between
	// the defaults and the files, and it can only fill keys that some layer
	// names, so the full key set has to be known first.
	type fileLayer struct {
		src  sdkconfig.Source
		flat map[string]any
	}
	var files []fileLayer
	keys := map[string]bool{}
	for k := range defaults {
		keys[k] = true
	}
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
		for k := range flat {
			keys[k] = true
		}
		files = append(files, fileLayer{src: src, flat: flat})
	}

	set := func(k string, val any, layer, source string) {
		if prev, seen := byKey[k]; seen {
			prev.Shadowed = append(prev.Shadowed,
				ShadowedValue{Value: prev.Value, Layer: prev.Layer, Source: prev.Source})
			prev.Value, prev.Layer, prev.Source = val, layer, source
			return
		}
		byKey[k] = &Entry{Key: k, Value: val, Layer: layer, Source: source}
	}

	// OS environment variables outrank only the defaults.
	for k := range keys {
		if name := sdkconfig.EnvName(k); name != "" {
			if value, ok := os.LookupEnv(name); ok {
				set(k, value, LAYER_OS_ENV, name)
			}
		}
	}

	for _, f := range files {
		for k, val := range f.flat {
			set(k, val, f.src.Layer, f.src.Path)
		}
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
