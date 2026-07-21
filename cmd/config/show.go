package config

import (
	"os"
	"sort"
	"strings"

	sdkconfig "github.com/bizshuk/gosdk/config"
)

// layers are the loaders sdkconfig.Default() merges, in the same order.
// Reading through them keeps this package from re-deriving the config file
// names or the search path — the config package stays the single source of
// truth for both.
//
// Each loader merges its own base and .local file internally, so a value can be
// attributed to a layer but not to one of its two files.
var layers = []struct {
	name string
	load func() sdkconfig.Config
}{
	{"env", sdkconfig.NewEnvConfig},
	{"yaml", sdkconfig.NewYamlConfig},
	{"json", sdkconfig.NewJsonConfig},
}

// ShadowedValue is a value lost to a higher-precedence layer.
type ShadowedValue struct {
	Value  any
	Source string
}

// Entry is one key in the merged view: the winning value plus the values it
// overrode, listed low precedence first.
type Entry struct {
	Key      string
	Value    any
	Source   string
	Shadowed []ShadowedValue
}

// Show returns the merged configuration entries, sorted by key. Each entry
// carries the winning value, the layer it came from, and the values it
// overrode (lowest precedence first); the renderer decides what to display.
//
// The returned slice is always non-nil; an empty config yields an empty slice.
func Show() []Entry {
	byKey := map[string]*Entry{}

	for _, layer := range layers {
		flat := map[string]any{}
		flattenSettings(layer.load().Load().AllSettings(), "", flat)

		for k, v := range flat {
			if prev, seen := byKey[k]; seen {
				prev.Shadowed = append(prev.Shadowed, ShadowedValue{Value: prev.Value, Source: prev.Source})
				prev.Value, prev.Source = v, layer.name
				continue
			}
			byKey[k] = &Entry{Key: k, Value: v, Source: layer.name}
		}
	}

	// APP_ environment variables outrank every config file.
	for k, e := range byKey {
		name, value, ok := envOverride(k)
		if !ok {
			continue
		}
		e.Shadowed = append(e.Shadowed, ShadowedValue{Value: e.Value, Source: e.Source})
		e.Value, e.Source = value, name
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

// envOverride reports the APP_ environment variable shadowing a key, if any.
//
// sdkconfig.Default() calls SetEnvPrefix("APP") and AutomaticEnv() but installs
// no EnvKeyReplacer, so viper looks up the literal name "APP_" + upper(key). A
// key holding a "." therefore maps to a name no shell can export, which is why
// nested keys cannot be overridden from the environment.
func envOverride(key string) (name, value string, ok bool) {
	name = "APP_" + strings.ToUpper(key)
	value, ok = os.LookupEnv(name)
	return name, value, ok
}
