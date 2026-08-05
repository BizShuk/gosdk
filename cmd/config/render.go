package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	sdkconfig "github.com/bizshuk/gosdk/config"
)

// formatValue renders a value for display. Non-strings go through JSON so
// numbers, booleans and the empty maps --delete leaves behind stay readable.
//
// The renderer is the only layer that turns config values into printable
// strings, so it owns the policy: how to format numbers, when to fall back to
// fmt.Sprint, how to mark a nil.
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

// formatSource renders where a value was read from: the file path for a config
// file, "$NAME" for an environment variable. The path is printed in full,
// beyond shortening the home directory to "~" — a merged view whose whole job
// is to say *which* settings.json won cannot abbreviate the part that
// distinguishes them.
func formatSource(layer, source string) string {
	if source == "" {
		return layer
	}
	if layer == LAYER_OS_ENV {
		return "$" + source
	}
	return shortenHome(source)
}

// shortenHome replaces a leading home directory with "~". Paths under the app
// config dir are the common case and this keeps them to one readable segment.
func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if !strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return path
	}
	return "~" + path[len(home):]
}

// RenderShowTable formats the merged entries as a tabwriter table.
//
// The SOURCE column is always printed: a merged value whose origin is hidden is
// the thing that makes a config surprising, so the answer travels with every
// row. withOverrides adds the values each key overrode underneath it — that is
// what --source now buys, on top of the always-visible winner.
func RenderShowTable(entries []Entry, withOverrides bool) string {
	var out strings.Builder
	if len(entries) == 0 {
		fmt.Fprintln(&out, "no configuration found")
		return out.String()
	}

	tw := tabwriter.NewWriter(&out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")

	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Key, formatValue(e.Value), formatSource(e.Layer, e.Source))
		if !withOverrides {
			continue
		}
		// Listed most recent first, so the file that came closest to winning
		// is printed directly under the winner.
		for i := len(e.Shadowed) - 1; i >= 0; i-- {
			s := e.Shadowed[i]
			fmt.Fprintf(tw, "\t%s\t%s (overridden)\n", formatValue(s.Value), formatSource(s.Layer, s.Source))
		}
	}
	_ = tw.Flush()
	return out.String()
}

// RenderSourceTable lists every config file the loader chain looks for, in
// merge order, with the path each one resolved to. It answers the question the
// merged view cannot: not "where did this value come from" but "which files
// were consulted at all, and which of them are missing".
func RenderSourceTable(sources []sdkconfig.Source) string {
	var out strings.Builder
	tw := tabwriter.NewWriter(&out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FILE\tLAYER\tPATH")
	for _, s := range sources {
		path := "(not found)"
		if s.Path != "" {
			path = shortenHome(s.Path)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, s.Layer, path)
	}
	_ = tw.Flush()

	fmt.Fprintf(&out, "\nsearched, in order: %s\n",
		strings.Join(displaySearchPaths(), "  ->  "))
	fmt.Fprintln(&out, "listed lowest precedence first; OS environment variables override every file")
	return out.String()
}

// displaySearchPaths renders the loader search path for the footer of the
// source table. The relative entries ("." and "conf") are spelled out: they are
// the ones a reader cannot resolve without knowing which directory the command
// was run from, which is exactly what they are trying to find out.
func displaySearchPaths() []string {
	paths := sdkconfig.SearchPaths()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		out = append(out, shortenHome(p))
	}
	return out
}

// RenderChangeReport formats the mutations a single update or delete call
// applied, the file they were written to, and any non-fatal notices about them.
func RenderChangeReport(report ChangeReport) string {
	var out strings.Builder
	for _, ch := range report.Changes {
		switch ch.Kind {
		case ChangeAdded:
			fmt.Fprintf(&out, "add    %s: %s\n", ch.Key, formatValue(ch.New))
		case ChangeUpdated:
			fmt.Fprintf(&out, "update %s: %s -> %s\n", ch.Key, formatValue(ch.Old), formatValue(ch.New))
		case ChangeDeleted:
			fmt.Fprintf(&out, "delete %s: was %s\n", ch.Key, formatValue(ch.Old))
		case ChangeAppended:
			fmt.Fprintf(&out, "append %s: %s -> %s\n", ch.Key, formatValue(ch.Old), formatValue(ch.New))
		case ChangeRemoved:
			fmt.Fprintf(&out, "remove %s: %s -> %s\n", ch.Key, formatValue(ch.Old), formatValue(ch.New))
		}
	}
	fmt.Fprintf(&out, "written to %s\n", report.Path)
	for _, warning := range report.Warnings {
		fmt.Fprintln(&out, warning)
	}
	return out.String()
}

// RenderDefaultReport formats the outcome of a Default call: what was written
// or would be written, which keys a merge added, and any notices.
func RenderDefaultReport(r DefaultReport) string {
	var b strings.Builder

	if !r.Registered {
		fmt.Fprintf(&b, "no default configuration registered for %s; nothing to do\n", r.File)
		// Naming the files that *are* registered turns the most likely cause
		// — a --file typo — into a one-line diagnosis.
		if others := RegisteredDefaults(); len(others) > 0 {
			fmt.Fprintf(&b, "  registered: %s\n", strings.Join(others, ", "))
		}
		return b.String()
	}

	verb := func(done, planned string) string {
		if r.DryRun {
			return planned
		}
		return done
	}

	// Bytes is only set when the seed was written wholesale — a file that did
	// not exist yet, or one replaced by --force. That case is checked first
	// because it can happen *in merge mode*: a first run with --merge creates
	// the file, and reporting that as "already up to date" would tell an
	// installer nothing happened on the one run where everything did.
	switch {
	case r.Bytes > 0:
		fmt.Fprintf(&b, "%s %s (%d bytes)\n", verb("wrote", "would write"), r.Path, r.Bytes)
	case r.Mode == DefaultModeMerge:
		if len(r.Changes) == 0 {
			fmt.Fprintf(&b, "%s is already up to date\n", r.Path)
			break
		}
		noun := "keys"
		if len(r.Changes) == 1 {
			noun = "key"
		}
		fmt.Fprintf(&b, "%s %d new %s into %s\n",
			verb("merged", "would merge"), len(r.Changes), noun, r.Path)
		for _, ch := range r.Changes {
			fmt.Fprintf(&b, "  + %s\n", ch.Key)
		}
	case r.Written || r.DryRun:
		fmt.Fprintf(&b, "%s %s (%d bytes)\n", verb("wrote", "would write"), r.Path, r.Bytes)
	}

	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", w)
	}
	return b.String()
}
