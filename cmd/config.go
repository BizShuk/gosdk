// Package cmd provides ready-made cobra commands for applications built on the
// SDK. Nothing here is executable on its own; the hosting application registers
// what it needs on its own root command.
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/cobra"
)

// LOCAL_SETTINGS_FILE is the only file ConfigCmd ever writes. It is the last
// source config.Default() merges, so a value written here overrides every other
// config file.
const LOCAL_SETTINGS_FILE = "settings.local.json"

// configLayers are the loaders config.Default() merges, in the same order.
// Reading through them keeps this command from re-deriving the config file
// names or the search path — the config package stays the single source of
// truth for both.
//
// Each loader merges its own base and .local file internally, so a value can be
// attributed to a layer but not to one of its two files.
var configLayers = []struct {
	name string
	load func() config.Config
}{
	{"env", config.NewEnvConfig},
	{"yaml", config.NewYamlConfig},
	{"json", config.NewJsonConfig},
}

// ChangeKind reports whether a config mutation adds, updates, or deletes a key.
type ChangeKind string

const (
	ChangeAdded   ChangeKind = "add"
	ChangeUpdated ChangeKind = "update"
	ChangeDeleted ChangeKind = "delete"
)

// Change is one mutation the command applied to settings.local.json.
// Old is set on update/delete; New is set on add/update.
type Change struct {
	Kind ChangeKind
	Key  string
	Old  any
	New  any
}

// ChangeReport bundles the mutations a single update or delete call applied,
// the file they were written to, and any non-fatal notices about them (for
// example, env vars that still outrank the file).
type ChangeReport struct {
	Changes  []Change
	Path     string
	Warnings []string
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

// ConfigCmd is the "config" command for the hosting application to register:
//
//	root.AddCommand(cmd.ConfigCmd)
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show and modify application configuration",
	Long: `Show and modify application configuration.

With no flags, prints every configuration key merged across the layers the
config package loads, with APP_ environment variables applied on top:

    env    .env, .env.local
    yaml   config.yaml, config.local.yaml
    json   settings.json, settings.local.json

Keys use viper's dotted path syntax; "." is the nesting delimiter:

    a.b.c   ->  {"a": {"b": {"c": "xyz"}}}

"_" is an ordinary character, so a_b_c is a separate flat key from a.b.c. Only
flat keys can be overridden by an APP_ environment variable, because the config
package installs no viper EnvKeyReplacer.

--update, --add and --delete write to ` + LOCAL_SETTINGS_FILE + ` only. That is
the last file in the merge order, so a value written there overrides every other
config file. It does not override an APP_ environment variable.`,
	Example: `  # show the merged configuration
  app config

  # show which layer each value came from
  app config --source

  # set a nested field (creates intermediate levels)
  app config --update server.host=0.0.0.0

  # values that parse as JSON keep their type; quote to force a string
  app config --update server.port=8080
  app config --update build.number='"1234"'

  # remove a field
  app config --delete server.host`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         RunConfig,
}

// Flag values bound in init(). They are package-level because ConfigCmd is a
// single shared command rather than a per-call instance; a process that runs
// ConfigCmd more than once must clear them in between.
var (
	configSource  bool
	configUpdates []string
	configAdds    []string
	configDeletes []string
)

func init() {
	f := ConfigCmd.Flags()
	f.BoolVar(&configSource, "source", false,
		"annotate every key with the layer it came from and the values it overrode")
	f.StringArrayVar(&configUpdates, "update", nil, "set a field, as a.b.c=value (repeatable)")
	f.StringArrayVar(&configAdds, "add", nil, "alias for --update (repeatable)")
	f.StringArrayVar(&configDeletes, "delete", nil, "remove a field, as a.b.c (repeatable)")
}

// RunConfig executes ConfigCmd using the values bound to its flags.
//
// The CLI layer owns two responsibilities: dispatching to the right public
// logic function, and rendering its structured result into human-readable
// output. The logic functions themselves never write to stdout or build
// display strings, so they can be reused by a future API surface (HTTP, gRPC,
// library) without dragging the table renderer along.
func RunConfig(c *cobra.Command, args []string) error {
	// --add is an alias for --update; concatenating keeps the two flags
	// independent instead of aliasing one slice variable.
	updates := append(append([]string{}, configUpdates...), configAdds...)

	output, err := dispatchConfig(configSource, updates, configDeletes)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(c.OutOrStdout(), output)
	return err
}

// dispatchConfig picks the right public logic function for the flags the user
// passed and renders its result. Errors come from the logic layer; rendering
// never produces an error here.
func dispatchConfig(withSource bool, updates, deletes []string) (string, error) {
	switch {
	case len(updates) > 0 && len(deletes) > 0:
		// Keep mixed changes atomic: validate and apply both sets in memory,
		// then rewrite settings.local.json once.
		report, err := runConfigChanges(updates, deletes)
		if err != nil {
			return "", err
		}
		return renderChangeReport(report), nil
	case len(updates) > 0:
		report, err := RunConfigUpdate(updates)
		if err != nil {
			return "", err
		}
		return renderChangeReport(report), nil
	case len(deletes) > 0:
		report, err := RunConfigDelete(deletes)
		if err != nil {
			return "", err
		}
		return renderChangeReport(report), nil
	default:
		return renderShowTable(RunConfigShow(), withSource), nil
	}
}

// --- public logic -------------------------------------------------------

// RunConfigShow returns the merged configuration entries, sorted by key. Each
// entry carries the winning value, the layer it came from, and the values it
// overrode (lowest precedence first); the renderer decides what to display.
//
// The returned slice is always non-nil; an empty config yields an empty slice.
func RunConfigShow() []Entry {
	return mergedEntries()
}

// RunConfigUpdate applies every update and rewrites LOCAL_SETTINGS_FILE once.
// A failed update leaves the file untouched.
func RunConfigUpdate(updates []string) (ChangeReport, error) {
	return runConfigChanges(updates, nil)
}

// RunConfigDelete applies every delete and rewrites LOCAL_SETTINGS_FILE once.
// A failed delete leaves the file untouched.
func RunConfigDelete(deletes []string) (ChangeReport, error) {
	return runConfigChanges(nil, deletes)
}

// --- shared logic -------------------------------------------------------

// runConfigChanges applies updates and deletes to settings.local.json
// atomically and returns the resulting report. RunConfig routes update/delete
// and the combined case through here so the rewrite happens exactly once and
// the reported path is always the one that was actually written.
func runConfigChanges(updates, deletes []string) (ChangeReport, error) {
	path := localSettingsPath()
	settings, err := readLocalSettings(path)
	if err != nil {
		return ChangeReport{}, err
	}

	var changes []Change
	for _, spec := range updates {
		key, raw, ok := strings.Cut(spec, "=")
		if !ok {
			return ChangeReport{}, fmt.Errorf("invalid update %q: expected a.b.c=value", spec)
		}
		segs, err := splitKey(key)
		if err != nil {
			return ChangeReport{}, err
		}
		value := parseValue(raw)
		old, existed := lookupPath(settings, segs)
		if err := setPath(settings, segs, value); err != nil {
			return ChangeReport{}, err
		}
		if existed {
			changes = append(changes, Change{Kind: ChangeUpdated, Key: key, Old: old, New: value})
		} else {
			changes = append(changes, Change{Kind: ChangeAdded, Key: key, New: value})
		}
	}

	for _, key := range deletes {
		segs, err := splitKey(key)
		if err != nil {
			return ChangeReport{}, err
		}
		old, _ := lookupPath(settings, segs)
		if err := deletePath(settings, segs); err != nil {
			return ChangeReport{}, err
		}
		changes = append(changes, Change{Kind: ChangeDeleted, Key: key, Old: old})
	}

	if err := writeLocalSettings(path, settings); err != nil {
		return ChangeReport{}, err
	}

	return ChangeReport{
		Changes:  changes,
		Path:     path,
		Warnings: envShadowWarnings(updates, deletes),
	}, nil
}

// mergedEntries merges the config layers exactly as config.Default() does and
// records where each winning value came from.
func mergedEntries() []Entry {
	byKey := map[string]*Entry{}

	for _, layer := range configLayers {
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
// An empty map is emitted as a leaf so a parent left behind by --delete is still
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
// config.Default() calls SetEnvPrefix("APP") and AutomaticEnv() but installs no
// EnvKeyReplacer, so viper looks up the literal name "APP_" + upper(key). A key
// holding a "." therefore maps to a name no shell can export, which is why
// nested keys cannot be overridden from the environment.
func envOverride(key string) (name, value string, ok bool) {
	name = "APP_" + strings.ToUpper(key)
	value, ok = os.LookupEnv(name)
	return name, value, ok
}

// envShadowWarnings returns keys whose new value will not take effect because
// an APP_ environment variable outranks every config file.
func envShadowWarnings(updates, deletes []string) []string {
	keys := make([]string, 0, len(updates)+len(deletes))
	for _, spec := range updates {
		key, _, _ := strings.Cut(spec, "=")
		keys = append(keys, key)
	}
	keys = append(keys, deletes...)

	warnings := make([]string, 0, len(keys))
	for _, key := range keys {
		name, value, ok := envOverride(strings.ToLower(key))
		if !ok {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("warning: %s is still overridden by %s=%s", key, name, value))
	}
	return warnings
}

// --- file I/O -----------------------------------------------------------

// localSettingsPath resolves the file to write.
//
// The JSON loader resolves settings.local.json through the config package's own
// search path, and reports it via ConfigFileUsed() whenever the file exists. If
// it does not exist yet, it is created in the app config dir when WithAppName
// was used, else ./conf when that directory exists, else the working directory.
func localSettingsPath() string {
	if used := config.NewJsonConfig().Load().ConfigFileUsed(); used != "" {
		return used
	}
	if dir := config.GetAppConfigDir(); dir != "" {
		return filepath.Join(dir, LOCAL_SETTINGS_FILE)
	}
	if st, err := os.Stat("conf"); err == nil && st.IsDir() {
		return filepath.Join("conf", LOCAL_SETTINGS_FILE)
	}
	return LOCAL_SETTINGS_FILE
}

// readLocalSettings loads settings.local.json, treating a missing or empty file
// as an empty object. Numbers are decoded as json.Number so a rewrite never
// turns an integer into scientific notation.
func readLocalSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	m := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

// writeLocalSettings rewrites settings.local.json, creating the directory when
// the file is new.
func writeLocalSettings(path string, m map[string]any) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// --- key/value helpers --------------------------------------------------

// splitKey turns "a.b.c" into its path segments. The "." is viper's key
// delimiter, so each segment is one nesting level in the JSON document.
func splitKey(key string) ([]string, error) {
	if key == "" {
		return nil, fmt.Errorf("empty key: expected a path such as a.b.c")
	}
	segs := strings.Split(key, ".")
	if slices.Contains(segs, "") {
		return nil, fmt.Errorf("invalid key %q: empty path segment", key)
	}
	return segs, nil
}

// matchKey finds the key in m equal to seg under viper's case-insensitive
// lookup, so a mutation reuses the existing spelling instead of adding a second
// key that differs only by case.
func matchKey(m map[string]any, seg string) (string, bool) {
	if _, ok := m[seg]; ok {
		return seg, true
	}
	for k := range m {
		if strings.EqualFold(k, seg) {
			return k, true
		}
	}
	return "", false
}

// lookupPath returns the value at the dotted path, if present.
func lookupPath(m map[string]any, segs []string) (any, bool) {
	cur := m
	for _, seg := range segs[:len(segs)-1] {
		k, ok := matchKey(cur, seg)
		if !ok {
			return nil, false
		}
		child, ok := cur[k].(map[string]any)
		if !ok {
			return nil, false
		}
		cur = child
	}
	k, ok := matchKey(cur, segs[len(segs)-1])
	if !ok {
		return nil, false
	}
	return cur[k], true
}

// setPath writes value at the dotted path, creating intermediate maps as needed.
// A scalar blocking the path is reported rather than silently replaced, so
// "--update a.b=1" followed by "--update a.b.c=2" fails loudly.
func setPath(m map[string]any, segs []string, value any) error {
	cur := m
	for i, seg := range segs[:len(segs)-1] {
		k, ok := matchKey(cur, seg)
		if !ok {
			child := map[string]any{}
			cur[seg] = child
			cur = child
			continue
		}
		child, ok := cur[k].(map[string]any)
		if !ok {
			return fmt.Errorf("cannot set %q: %q already holds a scalar value",
				strings.Join(segs, "."), strings.Join(segs[:i+1], "."))
		}
		cur = child
	}
	last := segs[len(segs)-1]
	if k, ok := matchKey(cur, last); ok {
		last = k
	}
	cur[last] = value
	return nil
}

// deletePath removes the leaf at the dotted path.
//
// Parents left empty are kept on purpose: only the key that was named is
// removed. Note the consequence — an empty map contributes no key to viper, so
// after deleting a.b.c the remaining "a": {"b": {}} is present in the file but
// invisible to viper.AllKeys().
func deletePath(m map[string]any, segs []string) error {
	full := strings.Join(segs, ".")
	cur := m
	for i, seg := range segs[:len(segs)-1] {
		k, ok := matchKey(cur, seg)
		if !ok {
			return fmt.Errorf("cannot delete %q: %q not found", full, strings.Join(segs[:i+1], "."))
		}
		child, ok := cur[k].(map[string]any)
		if !ok {
			return fmt.Errorf("cannot delete %q: %q holds a scalar value", full, strings.Join(segs[:i+1], "."))
		}
		cur = child
	}
	last, ok := matchKey(cur, segs[len(segs)-1])
	if !ok {
		return fmt.Errorf("cannot delete %q: key not found", full)
	}
	delete(cur, last)
	return nil
}

// parseValue interprets the right-hand side of key=value. A valid JSON literal
// keeps its type (1234 stays a number, true a boolean, [1,2] an array);
// anything else is stored as a plain string. Quote to force a string:
//
//	--update a.b.c='"1234"'
func parseValue(raw string) any {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return raw
	}
	if dec.More() {
		// Trailing content means raw was never a single JSON literal.
		return raw
	}
	return v
}
