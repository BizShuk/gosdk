// Package cmd provides ready-made cobra commands for applications built on the
// SDK. Nothing here is executable on its own; the hosting application registers
// what it needs on its own root command.
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

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
	RunE:         runConfig,
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

func runConfig(c *cobra.Command, args []string) error {
	// --add is an alias for --update; concatenating keeps the two flags
	// independent instead of aliasing one slice variable.
	mutations := append(append([]string{}, configUpdates...), configAdds...)
	if len(mutations) > 0 || len(configDeletes) > 0 {
		return runConfigMutate(c.OutOrStdout(), mutations, configDeletes)
	}
	return runConfigShow(c.OutOrStdout(), configSource)
}

// --- show ---------------------------------------------------------------

// shadow is a value that lost to a higher-precedence layer.
type shadow struct {
	value  any
	source string
}

// entry is one key in the merged view: the winning value plus everything it
// overrode, listed low precedence first.
type entry struct {
	key      string
	value    any
	source   string
	shadowed []shadow
}

// mergedEntries merges the config layers exactly as config.Default() does and
// records where each winning value came from.
func mergedEntries() []entry {
	byKey := map[string]*entry{}

	for _, layer := range configLayers {
		flat := map[string]any{}
		flattenSettings(layer.load().Load().AllSettings(), "", flat)

		for k, v := range flat {
			if prev, seen := byKey[k]; seen {
				prev.shadowed = append(prev.shadowed, shadow{value: prev.value, source: prev.source})
				prev.value, prev.source = v, layer.name
				continue
			}
			byKey[k] = &entry{key: k, value: v, source: layer.name}
		}
	}

	// APP_ environment variables outrank every config file.
	for k, e := range byKey {
		name, value, ok := envOverride(k)
		if !ok {
			continue
		}
		e.shadowed = append(e.shadowed, shadow{value: e.value, source: e.source})
		e.value, e.source = value, name
	}

	entries := make([]entry, 0, len(byKey))
	for _, e := range byKey {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
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

func runConfigShow(w io.Writer, withSource bool) error {
	entries := mergedEntries()
	if len(entries) == 0 {
		fmt.Fprintln(w, "no configuration found")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if withSource {
		fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")
	} else {
		fmt.Fprintln(tw, "KEY\tVALUE")
	}

	for _, e := range entries {
		if !withSource {
			fmt.Fprintf(tw, "%s\t%s\n", e.key, formatValue(e.value))
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.key, formatValue(e.value), e.source)
		// Listed most recent first, so the layer that came closest to winning
		// is printed directly under the winner.
		for i := len(e.shadowed) - 1; i >= 0; i-- {
			s := e.shadowed[i]
			fmt.Fprintf(tw, "\t%s\t%s (overridden)\n", formatValue(s.value), s.source)
		}
	}
	return tw.Flush()
}

// formatValue renders a value for the show table. Non-strings go through JSON so
// numbers, booleans and the empty maps --delete leaves behind stay readable.
func formatValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return "<nil>"
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(b)
	}
}

// --- mutate -------------------------------------------------------------

// runConfigMutate applies every update and delete, then rewrites
// settings.local.json once. A failure leaves the file untouched.
func runConfigMutate(w io.Writer, updates, deletes []string) error {
	path := localSettingsPath()
	settings, err := readLocalSettings(path)
	if err != nil {
		return err
	}

	var report []string

	for _, spec := range updates {
		key, raw, ok := strings.Cut(spec, "=")
		if !ok {
			return fmt.Errorf("invalid update %q: expected a.b.c=value", spec)
		}
		segs, err := splitKey(key)
		if err != nil {
			return err
		}
		value := parseValue(raw)
		old, existed := lookupPath(settings, segs)
		if err := setPath(settings, segs, value); err != nil {
			return err
		}
		if existed {
			report = append(report, fmt.Sprintf("update %s: %s -> %s",
				key, formatValue(old), formatValue(value)))
		} else {
			report = append(report, fmt.Sprintf("add    %s: %s", key, formatValue(value)))
		}
	}

	for _, key := range deletes {
		segs, err := splitKey(key)
		if err != nil {
			return err
		}
		old, _ := lookupPath(settings, segs)
		if err := deletePath(settings, segs); err != nil {
			return err
		}
		report = append(report, fmt.Sprintf("delete %s: was %s", key, formatValue(old)))
	}

	if err := writeLocalSettings(path, settings); err != nil {
		return err
	}

	for _, line := range report {
		fmt.Fprintln(w, line)
	}
	fmt.Fprintf(w, "written to %s\n", path)

	warnEnvShadow(w, updates, deletes)
	return nil
}

// warnEnvShadow reports keys whose new value will not take effect because an
// APP_ environment variable outranks every config file.
func warnEnvShadow(w io.Writer, updates, deletes []string) {
	keys := make([]string, 0, len(updates)+len(deletes))
	for _, spec := range updates {
		key, _, _ := strings.Cut(spec, "=")
		keys = append(keys, key)
	}
	keys = append(keys, deletes...)

	for _, key := range keys {
		name, value, ok := envOverride(strings.ToLower(key))
		if !ok {
			continue
		}
		fmt.Fprintf(w, "warning: %s is still overridden by %s=%s\n", key, name, value)
	}
}

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
