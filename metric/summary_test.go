package metric

import (
	"strings"
	"testing"
)

func TestFormatBatchSummary_AllSucceed(t *testing.T) {
	got := FormatBatchSummary("ticker-fetch", BatchSummary{
		Total:   10,
		Succeed: 10,
		Failed:  0,
	})
	for _, want := range []string{"10 total", "10 succeed", "0 failed", "ticker-fetch"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output: %s", want, got)
		}
	}
	for _, notWant := range []string{"took", "failed items"} {
		if strings.Contains(got, notWant) {
			t.Errorf("output should not contain %q\nfull output: %s", notWant, got)
		}
	}
}

func TestFormatBatchSummary_WithFailures(t *testing.T) {
	got := FormatBatchSummary("import-csv", BatchSummary{
		Total:      10,
		Succeed:    8,
		Failed:     2,
		FailedList: []string{"A-timeout", "B-rate_limit"},
	})
	for _, want := range []string{"10 total", "8 succeed", "2 failed", "A-timeout", "B-rate_limit", "failed items:"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output: %s", want, got)
		}
	}
}

func TestFormatBatchSummary_WithDuration(t *testing.T) {
	got := FormatBatchSummary("ticker-fetch", BatchSummary{
		Total:      5,
		Succeed:    5,
		DurationMs: 123.4,
	})
	if !strings.Contains(got, "(took 123.4ms)") {
		t.Errorf("output missing duration marker\nfull output: %s", got)
	}
}

func TestFormatBatchSummary_Empty(t *testing.T) {
	// Should not panic and should still produce a valid header.
	got := FormatBatchSummary("noop", BatchSummary{})
	if !strings.Contains(got, "0 total") || !strings.Contains(got, "0 succeed") || !strings.Contains(got, "0 failed") {
		t.Errorf("empty summary should still render header\nfull output: %s", got)
	}
}

func TestFormatBatchSummary_ZeroDurationNotRendered(t *testing.T) {
	got := FormatBatchSummary("noop", BatchSummary{Total: 1, Succeed: 1, DurationMs: 0})
	if strings.Contains(got, "took") {
		t.Errorf("DurationMs=0 should not be rendered\nfull output: %s", got)
	}
}
