package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
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

// RenderShowTable formats the merged entries as a tabwriter table. When
// withSource is true, every key is followed by the values it overrode; the
// renderer is the single place that decides what the --source flag shows.
func RenderShowTable(entries []Entry, withSource bool) string {
	var out strings.Builder
	if len(entries) == 0 {
		fmt.Fprintln(&out, "no configuration found")
		return out.String()
	}

	tw := tabwriter.NewWriter(&out, 0, 0, 2, ' ', 0)
	if withSource {
		fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")
	} else {
		fmt.Fprintln(tw, "KEY\tVALUE")
	}

	for _, e := range entries {
		if !withSource {
			fmt.Fprintf(tw, "%s\t%s\n", e.Key, formatValue(e.Value))
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Key, formatValue(e.Value), e.Source)
		// Listed most recent first, so the layer that came closest to winning
		// is printed directly under the winner.
		for i := len(e.Shadowed) - 1; i >= 0; i-- {
			s := e.Shadowed[i]
			fmt.Fprintf(tw, "\t%s\t%s (overridden)\n", formatValue(s.Value), s.Source)
		}
	}
	_ = tw.Flush()
	return out.String()
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

	switch {
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
