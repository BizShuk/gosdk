package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// splitKey turns "a.b.c" into its path segments. The "." is viper's key
// delimiter, so each segment is one nesting level in the document.
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

// walkToLeaf resolves the dotted path down to the map holding its last segment
// and returns that map together with the segment's existing spelling.
//
// Every mutation in this file needs the same walk and differs only in two
// things, which is exactly what the parameters carry: whether a missing level
// should be created (set and append create; delete and remove-from refuse),
// and the verb to use when the walk fails ("cannot set", "cannot delete", …).
// The returned leaf key is the spelling already in the document when one
// exists, so a mutation never adds a second key differing only by case.
func walkToLeaf(m map[string]any, segs []string, create bool, verb string) (map[string]any, string, error) {
	full := strings.Join(segs, ".")
	cur := m
	for i, seg := range segs[:len(segs)-1] {
		so_far := strings.Join(segs[:i+1], ".")
		k, ok := matchKey(cur, seg)
		if !ok {
			if !create {
				return nil, "", fmt.Errorf("cannot %s %q: %q not found", verb, full, so_far)
			}
			child := map[string]any{}
			cur[seg] = child
			cur = child
			continue
		}
		child, ok := cur[k].(map[string]any)
		if !ok {
			return nil, "", fmt.Errorf("cannot %s %q: %q holds a scalar value", verb, full, so_far)
		}
		cur = child
	}

	last := segs[len(segs)-1]
	if k, ok := matchKey(cur, last); ok {
		return cur, k, nil
	}
	if !create {
		return nil, "", fmt.Errorf("cannot %s %q: key not found", verb, full)
	}
	return cur, last, nil
}

// lookupPath returns the value at the dotted path, if present.
func lookupPath(m map[string]any, segs []string) (any, bool) {
	leaf, key, err := walkToLeaf(m, segs, false, "read")
	if err != nil {
		return nil, false
	}
	return leaf[key], true
}

// setPath writes value at the dotted path, creating intermediate maps as needed.
// A scalar blocking the path is reported rather than silently replaced, so
// setting a.b=1 followed by a.b.c=2 fails loudly.
func setPath(m map[string]any, segs []string, value any) error {
	leaf, key, err := walkToLeaf(m, segs, true, "set")
	if err != nil {
		return err
	}
	leaf[key] = value
	return nil
}

// deletePath removes the leaf at the dotted path.
//
// Parents left empty are kept on purpose: only the key that was named is
// removed. Note the consequence — an empty map contributes no key to viper, so
// after deleting a.b.c the remaining "a": {"b": {}} is present in the file but
// invisible to viper.AllKeys().
func deletePath(m map[string]any, segs []string) error {
	leaf, key, err := walkToLeaf(m, segs, false, "delete")
	if err != nil {
		return err
	}
	delete(leaf, key)
	return nil
}

// appendArrayElement appends a string element to the []any at segs, creating
// []any{} when the path is missing. A non-array value blocking the path is
// reported rather than silently replaced, mirroring setPath's loud-failure
// policy for scalar-blocking paths.
//
// Callers are responsible for rejecting targets whose file format cannot
// represent arrays (e.g. .env); this helper operates on the in-memory document
// only.
func appendArrayElement(m map[string]any, segs []string, value string) ([]any, error) {
	leaf, key, err := walkToLeaf(m, segs, true, "append to")
	if err != nil {
		return nil, err
	}
	arr, ok := leaf[key].([]any)
	if !ok {
		if leaf[key] != nil {
			return nil, fmt.Errorf("cannot append to %q: existing value is not an array (%T)",
				strings.Join(segs, "."), leaf[key])
		}
		arr = []any{}
	}
	arr = append(arr, value)
	leaf[key] = arr
	return arr, nil
}

// removeArrayElement removes the first []any element equal to value from the
// array at segs. A missing value is a no-op (returns the unchanged slice);
// a missing or non-array path is an error.
func removeArrayElement(m map[string]any, segs []string, value string) ([]any, error) {
	leaf, key, err := walkToLeaf(m, segs, false, "remove from")
	if err != nil {
		return nil, err
	}
	arr, ok := leaf[key].([]any)
	if !ok {
		return nil, fmt.Errorf("cannot remove from %q: existing value is not an array (%T)",
			strings.Join(segs, "."), leaf[key])
	}
	idx := slices.IndexFunc(arr, func(e any) bool { return e == value })
	if idx == -1 {
		return arr, nil
	}
	arr = slices.Delete(arr, idx, idx+1)
	leaf[key] = arr
	return arr, nil
}

// parseValue interprets the right-hand side of key=value. A valid JSON literal
// keeps its type (1234 stays a number, true a boolean, [1,2] an array);
// anything else is stored as a plain string. Quote to force a string:
//
//	a.b.c='"1234"'
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

// snapshot detaches a value from the document so a report can outlive later
// mutations of it.
//
// Only slices need this, and they need it badly: removeArrayElement calls
// slices.Delete, which shifts elements left inside the *existing* backing
// array and zeroes the tail. A Change recorded earlier holds a slice header
// into that same array, so without a copy an --append followed by a
// --remove-from on the same key makes the append's line read ["b", null]
// instead of ["a","b"] — the file is written correctly, but the report lies.
func snapshot(v any) any {
	s, ok := v.([]any)
	if !ok {
		return v
	}
	return slices.Clone(s)
}
