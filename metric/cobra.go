package metric

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// CobraHookMetricName is the metric name emitted by CobraCMDHook.
const CobraHookMetricName = "command_line_trigger"

// CobraCMDHook wires a metric hook into root. Every execution of root
// (or any of its subcommands) emits one metric:
//
//	command_line_trigger{cmd="myapp sub action", flag="env-verbose"}
//
// cmd is the full command chain (root -> leaf); flag lists the flags the
// user explicitly set, alphabetically sorted and joined with "-". The
// metric is sent through NewMetricService(""), so the backend is the
// remote-write endpoint configured by METRIC_URL.
//
// The hook wraps (does not replace) any existing PersistentPreRunE on
// the root command. Note on cobra's persistent hooks: by default cobra
// calls only the first persistent hook it finds while walking from the
// leaf toward the root. If any subcommand defines its own
// PersistentPreRunE, set cobra.EnableTraverseRunHooks = true so the
// root's hook (and this metric) is invoked for every subcommand.
func CobraCMDHook(root *cobra.Command) {
	existingPre := root.PersistentPreRunE

	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		emitCommandMetric(cmd) // cobra passes the leaf command here, not root

		if existingPre != nil {
			return existingPre(cmd, args)
		}
		return nil
	}
}

func emitCommandMetric(leaf *cobra.Command) {
	m := Metric{
		Name:      CobraHookMetricName,
		Timestamp: time.Now().Unix(),
		Value:     int64(1),
		Tags: map[string]string{
			"cmd":  leaf.CommandPath(),
			"flag": changedFlags(leaf),
		},
	}

	if err := NewMetricService("").Send(m); err != nil {
		zap.L().Warn("cobra metric hook send failed",
			zap.String("cmd", m.Tags["cmd"]),
			zap.Error(err))
	}
}

// changedFlags returns the names of flags the user explicitly set,
// collected across the whole command chain (persistent flags on parents
// included), alphabetically sorted and joined with "-".
func changedFlags(leaf *cobra.Command) string {
	seen := map[string]bool{}
	for c := leaf; c != nil; c = c.Parent() {
		c.Flags().Visit(func(f *pflag.Flag) { seen[f.Name] = true })
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "-")
}
