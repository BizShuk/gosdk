package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	sdkconfig "github.com/bizshuk/gosdk/config"
)

// WriteOptions selects the file a mutation writes to and where that file
// lives. Empty File defaults to LOCAL_SETTINGS_FILE; Local toggles whether the
// file is anchored at the working directory (true) or the application config
// dir reported by sdkconfig.GetAppConfigDir() (false).
type WriteOptions struct {
	File  string
	Local bool
}

// ChangeKind reports whether a config mutation adds, updates, or deletes a key.
type ChangeKind string

const (
	ChangeAdded    ChangeKind = "add"
	ChangeUpdated  ChangeKind = "update"
	ChangeDeleted  ChangeKind = "delete"
	ChangeAppended ChangeKind = "append"
	ChangeRemoved  ChangeKind = "remove"
)

// Change is one mutation applied to the target file.
// Old is set on update/delete; New is set on add/update.
type Change struct {
	Kind ChangeKind
	Key  string
	Old  any
	New  any
}

// ChangeReport bundles the mutations a single call applied, the file they were
// written to, and any non-fatal notices about them (for example, env vars that
// still outrank the file).
type ChangeReport struct {
	Changes  []Change
	Path     string
	Warnings []string
}

// Update applies every update and rewrites the target file once. A failed
// update leaves the file untouched.
//
// opts.File selects the target file. Empty defaults to LOCAL_SETTINGS_FILE;
// any other value must be one of the names SupportedFiles reports. opts.Local
// toggles the target directory: false anchors at sdkconfig.GetAppConfigDir(),
// true at the working directory.
func Update(updates []string, opts WriteOptions) (ChangeReport, error) {
	return Apply(updates, nil, nil, nil, opts)
}

// Delete applies every delete and rewrites the target file once. See Update
// for the meaning of opts.
func Delete(deletes []string, opts WriteOptions) (ChangeReport, error) {
	return Apply(nil, deletes, nil, nil, opts)
}

// Append appends one or more string elements to array fields and rewrites the
// target file once. The array is created when missing. See Update for the
// meaning of opts.
//
// .env targets are rejected — env files are flat and cannot represent an
// array element.
func Append(appends []string, opts WriteOptions) (ChangeReport, error) {
	return Apply(nil, nil, appends, nil, opts)
}

// RemoveFrom removes the first element equal to value from each named array
// field. A missing value is a no-op (the array is left untouched); a missing
// key or non-array target is an error. See Update for the meaning of opts.
//
// .env targets are rejected — env files are flat and cannot represent an
// array element.
func RemoveFrom(removes []string, opts WriteOptions) (ChangeReport, error) {
	return Apply(nil, nil, nil, removes, opts)
}

// Apply applies updates, deletes, appends and removes to the target file
// atomically and returns the resulting report. Every mutation entry point
// routes through here so the rewrite happens exactly once and the reported
// path is always the one that was actually written.
//
// Element-level operations (appends, removes) are not meaningful for flat file
// formats: .env has no notion of an array, and any JSON literal flattened to
// A_B_C loses its shape. Both are rejected up front with a name-spanning
// error so the user can fix the target file before retrying.
func Apply(updates, deletes, appends, removes []string, opts WriteOptions) (ChangeReport, error) {
	fileName, err := normalizeFile(opts.File, LOCAL_SETTINGS_FILE)
	if err != nil {
		return ChangeReport{}, err
	}
	if (len(appends) > 0 || len(removes) > 0) && fileFormat(fileName) == "env" {
		return ChangeReport{}, fmt.Errorf(
			"--append/--remove-from is not supported for %s (env files are flat key=value pairs)", fileName)
	}
	path := FilePath(fileName, opts.Local)
	settings, err := readConfigFile(path, fileName)
	if err != nil {
		return ChangeReport{}, err
	}

	isEnv := fileFormat(fileName) == "env"
	var warnings []string

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
		displayKey := key
		if isEnv {
			flat := envFlattenKey(segs)
			if len(segs) > 1 {
				warnings = append(warnings, fmt.Sprintf(
					"note: %q written to %s as %q (env vars are flat, dots flattened to underscores)",
					key, fileName, flat))
			}
			segs = []string{flat}
		}
		value := parseValue(raw)
		old, existed := lookupPath(settings, segs)
		if err := setPath(settings, segs, value); err != nil {
			return ChangeReport{}, err
		}
		if existed {
			changes = append(changes, Change{Kind: ChangeUpdated, Key: displayKey, Old: old, New: value})
		} else {
			changes = append(changes, Change{Kind: ChangeAdded, Key: displayKey, New: value})
		}
	}

	for _, key := range deletes {
		segs, err := splitKey(key)
		if err != nil {
			return ChangeReport{}, err
		}
		displayKey := key
		if isEnv {
			segs = []string{envFlattenKey(segs)}
		}
		old, _ := lookupPath(settings, segs)
		if err := deletePath(settings, segs); err != nil {
			return ChangeReport{}, err
		}
		changes = append(changes, Change{Kind: ChangeDeleted, Key: displayKey, Old: old})
	}

	for _, spec := range appends {
		key, value, ok := strings.Cut(spec, "=")
		if !ok {
			return ChangeReport{}, fmt.Errorf("invalid append %q: expected a.b.c=value", spec)
		}
		segs, err := splitKey(key)
		if err != nil {
			return ChangeReport{}, err
		}
		if isEnv {
			// Defensive: Apply rejects env targets up front, but a future
			// caller that bypasses that check still gets the same error here,
			// never a silent flatten.
			return ChangeReport{}, fmt.Errorf(
				"--append is not supported for %s (env files are flat key=value pairs)", fileName)
		}
		// Snapshot before mutating: the helpers below write through the same
		// backing array this value points at.
		old, _ := lookupPath(settings, segs)
		old = snapshot(old)
		newArr, err := appendArrayElement(settings, segs, value)
		if err != nil {
			return ChangeReport{}, err
		}
		changes = append(changes, Change{
			Kind: ChangeAppended, Key: key, Old: old, New: snapshot(newArr)})
	}

	for _, spec := range removes {
		key, value, ok := strings.Cut(spec, "=")
		if !ok {
			return ChangeReport{}, fmt.Errorf("invalid remove-from %q: expected a.b.c=value", spec)
		}
		segs, err := splitKey(key)
		if err != nil {
			return ChangeReport{}, err
		}
		if isEnv {
			return ChangeReport{}, fmt.Errorf(
				"--remove-from is not supported for %s (env files are flat key=value pairs)", fileName)
		}
		old, _ := lookupPath(settings, segs)
		old = snapshot(old)
		newArr, err := removeArrayElement(settings, segs, value)
		if err != nil {
			return ChangeReport{}, err
		}
		// Skip the change record entirely when nothing matched — a no-op
		// removal would otherwise clutter the report with "remove x: <nil> -> <nil>"
		// and confuse the reader. The type assertion cannot fail here (the
		// path resolved to an array or removeArrayElement would have errored),
		// so a failed one yields nil and slices.Equal still answers correctly.
		if prev, _ := old.([]any); slices.Equal(prev, newArr) {
			continue
		}
		changes = append(changes, Change{
			Kind: ChangeRemoved, Key: key, Old: old, New: snapshot(newArr)})
	}

	if err := writeConfigFile(path, fileName, settings); err != nil {
		return ChangeReport{}, err
	}

	warnings = append(warnings, envShadowWarnings(updates, deletes, appends, removes)...)
	warnings = append(warnings, shadowedWriteWarning(fileName, path)...)

	return ChangeReport{
		Changes:  changes,
		Path:     path,
		Warnings: warnings,
	}, nil
}

// FilePath resolves the on-disk path for the given config file name.
//
// When local is false (the default), the app config dir reported by
// sdkconfig.GetAppConfigDir() is the canonical home for every config file.
// Anchoring there is what makes an update survive across shells: the same
// update from a project checkout and from an installed binary lands in the
// same place.
//
// When local is true, the app config dir is skipped and resolution falls back
// to ./conf/ if that directory exists, then the working directory. This is the
// legacy behaviour kept for per-shell experiments that should not leak into
// the global app config.
func FilePath(fileName string, local bool) string {
	if !local {
		if dir := sdkconfig.GetAppConfigDir(); dir != "" {
			return filepath.Join(dir, fileName)
		}
	}
	if st, err := os.Stat("conf"); err == nil && st.IsDir() {
		return filepath.Join("conf", fileName)
	}
	return fileName
}

// envShadowWarnings returns keys whose new value will not take effect because
// an APP_ environment variable outranks every config file.
func envShadowWarnings(updates, deletes, appends, removes []string) []string {
	keys := make([]string, 0, len(updates)+len(deletes)+len(appends)+len(removes))
	for _, spec := range updates {
		key, _, _ := strings.Cut(spec, "=")
		keys = append(keys, key)
	}
	keys = append(keys, deletes...)
	for _, spec := range appends {
		key, _, _ := strings.Cut(spec, "=")
		keys = append(keys, key)
	}
	for _, spec := range removes {
		key, _, _ := strings.Cut(spec, "=")
		keys = append(keys, key)
	}

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

// shadowedWriteWarning returns one warning when path sits in a directory that
// also contains a higher-precedence config file. The warning is the one that
// actually matters at runtime: a write to config.yaml is shadowed by
// settings.local.json (and the rest above it), not by .env.local alone.
//
// Only the first shadowing file is reported. Showing every file in the chain
// would be noise — once the user knows settings.local.json wins, the
// remainder is implied.
func shadowedWriteWarning(fileName, path string) []string {
	dir := filepath.Dir(path)
	for _, higher := range higherPrecedenceFiles(fileName) {
		higherPath := filepath.Join(dir, higher)
		if _, err := os.Stat(higherPath); err == nil {
			return []string{fmt.Sprintf(
				"warning: %s is shadowed by %s — value may not take effect at runtime",
				fileName, higher)}
		}
	}
	return nil
}
