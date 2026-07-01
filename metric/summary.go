package metric

import (
	"fmt"
	"strings"
)

// BatchSummary holds the result of a batch process. It is the Go equivalent
// of the Python `BatchSummary` dataclass declared in `metric/otel.py` and
// follows the conventions described in `SPEC.md`.
type BatchSummary struct {
	Total      int      `json:"total"`
	Succeed    int      `json:"succeed"`
	Failed     int      `json:"failed"`
	FailedList []string `json:"failed_list,omitempty"`
	DurationMs float64  `json:"duration_ms,omitempty"`
}

// FormatBatchSummary formats a BatchSummary into a single-line notification
// string for downstream notifiers (e.g. `notify.SlackNotifier`). Mirrors the
// Python `format_batch_summary` helper.
//
// Example output:
//
//	[ticker-fetch] batch finished: 100 total, 95 succeed, 5 failed (took 1234.5ms)
//	failed items: AAPL-timeout, GOOG-rate_limit
func FormatBatchSummary(jobName string, summary BatchSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] batch finished: %d total, %d succeed, %d failed",
		jobName, summary.Total, summary.Succeed, summary.Failed)

	if summary.DurationMs > 0 {
		fmt.Fprintf(&b, " (took %.1fms)", summary.DurationMs)
	}

	if len(summary.FailedList) > 0 {
		fmt.Fprintf(&b, "\nfailed items: %s", strings.Join(summary.FailedList, ", "))
	}

	return b.String()
}
