package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// registeredDefault is one seed file plus where it was registered from. The
// origin is only ever used to make a duplicate-registration error actionable.
type registeredDefault struct {
	content []byte
	origin  string
}

// defaultConfigs holds the seed content the hosting application registered,
// keyed by target file name.
//
// The key matters. A single package-level slot would make the outcome depend
// on init() ordering across packages: whichever dependency registered last
// would silently win. Keying by file name lets independent packages seed
// different files, and turns two packages seeding the *same* file into an
// error the host has to resolve rather than a race.
//
// Note that the *destination* is not part of the key: sdkconfig.GetAppConfigDir()
// is a process-wide singleton derived from sdkconfig.Default(WithAppName(...)),
// so a library cannot make this write into some other application's config
// directory.
var defaultConfigs = map[string]registeredDefault{}

// RegisterDefault registers embedded seed content for a config file. Use it
// from the hosting application, with the content supplied by an embed
// directive:
//
//	//go:embed settings.example.json
//	var defaultSettings []byte
//
//	func init() {
//		config.MustRegisterDefault("settings.json", defaultSettings)
//	}
//
// Registering the same file twice is an error: two packages disagreeing about
// what the default should be is a conflict only the host can settle. Use
// SetDefault when overriding a dependency's seed is the intent.
func RegisterDefault(file string, content []byte) error {
	name, err := normalizeFile(file, DEFAULT_CONFIG_FILE)
	if err != nil {
		return err
	}
	if err := validateDefaultContent(name, content); err != nil {
		return err
	}
	if prev, exists := defaultConfigs[name]; exists {
		return fmt.Errorf("default config for %s already registered at %s: "+
			"use config.SetDefault to override it deliberately", name, prev.origin)
	}
	defaultConfigs[name] = registeredDefault{content: content, origin: callerOrigin()}
	return nil
}

// MustRegisterDefault is RegisterDefault for use in init(), where a
// conflicting registration is a build-time mistake rather than a runtime
// condition. It panics on error.
func MustRegisterDefault(file string, content []byte) {
	if err := RegisterDefault(file, content); err != nil {
		panic(err)
	}
}

// SetDefault registers seed content, replacing any previous registration for
// the same file. It is the deliberate last-writer-wins counterpart to
// RegisterDefault — the escape hatch for a host that has to override a seed
// shipped by one of its dependencies.
func SetDefault(file string, content []byte) error {
	name, err := normalizeFile(file, DEFAULT_CONFIG_FILE)
	if err != nil {
		return err
	}
	if err := validateDefaultContent(name, content); err != nil {
		return err
	}
	defaultConfigs[name] = registeredDefault{content: content, origin: callerOrigin()}
	return nil
}

// RegisteredDefaults lists the file names that currently have seed content,
// sorted. Useful for diagnostics and for the command's error message.
func RegisteredDefaults() []string {
	names := make([]string, 0, len(defaultConfigs))
	for name := range defaultConfigs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultMode decides what happens when the target file already exists. It is
// a mode rather than a pair of booleans because the three outcomes are
// mutually exclusive, and a (force, merge) tuple can express a fourth state
// that has no meaning.
type DefaultMode string

const (
	// DefaultModeSkip leaves an existing file untouched and reports it.
	DefaultModeSkip DefaultMode = "skip"
	// DefaultModeMerge adds the fields the seed has and the file lacks,
	// leaving every value already in the file alone. This is the upgrade
	// path: a new release introduces settings, existing installs pick them
	// up without losing local edits.
	DefaultModeMerge DefaultMode = "merge"
	// DefaultModeForce replaces an existing file wholesale.
	DefaultModeForce DefaultMode = "force"
)

// DefaultReport describes one Default call: the file that was asked for, where
// it resolved to, and what happened to it.
//
// Registered is false when the application shipped no default for this file.
// That is a no-op, not a failure: File is set, every other field is zero, and
// the caller decides whether to care.
//
// Bytes is the size of the content written wholesale (create and force); it is
// 0 in merge mode, where Changes carries the meaningful result — one
// ChangeAdded per key the merge introduced, sorted by dotted path.
//
// Changes uses the same currency as ChangeReport on purpose: seeding a default
// and running --update are the same pipeline (decide the values, resolve the
// path, merge into the target), so the two reports describe their result the
// same way and the renderers share their change lines.
type DefaultReport struct {
	File       string
	Path       string
	Mode       DefaultMode
	Registered bool
	Bytes      int
	Written    bool
	Changes    []Change
	DryRun     bool
	Warnings   []string
}

// Default writes the seed content registered for file into the target config
// file.
//
// The target is resolved by FilePath, exactly as a --update is: the app config
// dir when the host set an app name, the working directory when local is true
// or no app name was set. That shared resolution is what keeps "seed the
// default" and "edit a value" pointing at the same file.
//
// When the file does not exist yet it is created from the seed and mode is
// irrelevant. When it does exist, mode decides: skip leaves it alone, merge
// adds only the fields it is missing, force replaces it.
//
// An untouched existing file is reported as a warning with Written false
// rather than as an error, so a first-run bootstrap can call this
// unconditionally.
func Default(file string, mode DefaultMode, dryRun, local bool) (DefaultReport, error) {
	name, err := normalizeFile(file, DEFAULT_CONFIG_FILE)
	if err != nil {
		return DefaultReport{}, err
	}

	// An application that wires up the command without shipping a default is
	// a supported configuration, not a mistake: the other config subcommands
	// still work. Report it and stop, so an installer or first-run script can
	// call it unconditionally without having to know whether this particular
	// build ships a seed.
	seed, ok := defaultConfigs[name]
	if !ok {
		return DefaultReport{File: name, Mode: mode, DryRun: dryRun}, nil
	}

	path := FilePath(name, local)
	report := DefaultReport{File: name, Path: path, Mode: mode, DryRun: dryRun, Registered: true}

	exists := true
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return DefaultReport{}, fmt.Errorf("stat %s: %w", path, err)
		}
		exists = false
	}

	if exists && mode == DefaultModeMerge {
		return mergeDefaultInto(path, name, seed.content, report)
	}

	if exists && mode != DefaultModeForce {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"%s already exists; pass --merge to add new fields or --force to replace it", path))
		return report, nil
	}

	// Create and force both write the seed verbatim rather than through the
	// format encoder, so the file on disk is byte-identical to what the
	// application shipped — comments and key order included.
	report.Bytes = len(seed.content)
	if dryRun {
		return report, nil
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return DefaultReport{}, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, seed.content, 0o644); err != nil {
		return DefaultReport{}, fmt.Errorf("write %s: %w", path, err)
	}
	report.Written = true
	return report, nil
}

// mergeDefaultInto adds the seed's missing fields to an existing file.
//
// Both sides are decoded through the same parser the config loaders use, the
// missing keys are grafted in, and the result is re-encoded. That round trip
// is what makes the merge format-agnostic — and also what loses comments and
// key order, which is why the caller is warned.
func mergeDefaultInto(path, name string, content []byte, report DefaultReport) (DefaultReport, error) {
	seedData, err := parseConfigBytes(content, name)
	if err != nil {
		return DefaultReport{}, fmt.Errorf("parse the registered default for %s: %w", name, err)
	}
	current, err := readConfigFile(path, name)
	if err != nil {
		return DefaultReport{}, err
	}

	var added []Change
	mergeMissing(current, seedData, "", &added)
	report.Changes = added

	if len(added) == 0 {
		return report, nil
	}
	if fileFormat(name) != "json" {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"comments and key order in %s are not preserved by --merge", name))
	}
	if report.DryRun {
		return report, nil
	}
	if err := writeConfigFile(path, name, current); err != nil {
		return DefaultReport{}, err
	}
	report.Written = true
	return report, nil
}

// mergeMissing copies into dst every key the seed has and dst lacks, recursing
// into nested maps, and records one ChangeAdded per addition.
//
// Values already in dst are never touched — that is the whole contract. A key
// whose shape changed between releases (a scalar in the file where the seed
// now has a map, or the reverse) counts as present and is left alone: the
// operator's value is real, and guessing how to reconcile it would discard it.
//
// A subtree missing entirely is one change naming the subtree, not one per
// leaf inside it: "feature was added", not "feature.beta, feature.gamma, …".
func mergeMissing(dst, seed configData, prefix string, added *[]Change) {
	keys := make([]string, 0, len(seed))
	for k := range seed {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic ordering across runs

	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		// Match case-insensitively: viper folds keys, so a file spelling a
		// key differently must not gain a duplicate.
		existing, found := matchKey(dst, key)
		if !found {
			dst[key] = seed[key]
			*added = append(*added, Change{Kind: ChangeAdded, Key: path, New: seed[key]})
			continue
		}

		seedChild, seedIsMap := seed[key].(configData)
		dstChild, dstIsMap := dst[existing].(configData)
		if seedIsMap && dstIsMap {
			mergeMissing(dstChild, seedChild, path, added)
		}
	}
}

// validateDefaultContent rejects seed content that the matching loader could
// never parse. Only JSON is checked: encoding/json is already a dependency
// here, and a malformed settings.json is the failure that actually bites —
// viper reports it at Debug level and the application starts with no config.
func validateDefaultContent(name string, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("default config for %s is empty", name)
	}
	if !strings.HasSuffix(name, ".json") {
		return nil
	}
	if !json.Valid(content) {
		return fmt.Errorf("default config for %s is not valid JSON", name)
	}
	return nil
}

// callerOrigin reports the file:line that called into the exported registration
// function, so a duplicate registration names both sides of the conflict.
func callerOrigin() string {
	if _, file, line, ok := runtime.Caller(2); ok {
		return fmt.Sprintf("%s:%d", file, line)
	}
	return "unknown location"
}

// resetDefaultConfigs clears the registry. Test-only; a real process registers
// its seeds once at init().
func resetDefaultConfigs() {
	defaultConfigs = map[string]registeredDefault{}
}
