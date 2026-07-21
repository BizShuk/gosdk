package cmd

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

// renderShowTable formats the merged entries as a tabwriter table. When
// withSource is true, every key is followed by the values it overrode; the
// renderer is the single place that decides what the --source flag shows.
func renderShowTable(entries []Entry, withSource bool) string {
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

// renderChangeReport formats the mutations a single update or delete call
// applied, the file they were written to, and any non-fatal notices about them.
func renderChangeReport(report ChangeReport) string {
	var out strings.Builder
	for _, ch := range report.Changes {
		switch ch.Kind {
		case ChangeAdded:
			fmt.Fprintf(&out, "add    %s: %s\n", ch.Key, formatValue(ch.New))
		case ChangeUpdated:
			fmt.Fprintf(&out, "update %s: %s -> %s\n", ch.Key, formatValue(ch.Old), formatValue(ch.New))
		case ChangeDeleted:
			fmt.Fprintf(&out, "delete %s: was %s\n", ch.Key, formatValue(ch.Old))
		}
	}
	fmt.Fprintf(&out, "written to %s\n", report.Path)
	for _, warning := range report.Warnings {
		fmt.Fprintln(&out, warning)
	}
	return out.String()
}
