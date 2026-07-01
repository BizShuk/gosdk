package metric

import (
	"context"
	"slices"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// newTestOtelMetrics builds an OtelMetrics bound to a sdkmetric.MeterProvider
// backed by a manual reader. This keeps tests hermetic — no global state
// is registered and no exporter is contacted.
func newTestOtelMetrics(t *testing.T, jobName, instance string) (*OtelMetrics, *sdkmetric.ManualReader) {
	t.Helper()
	mr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(mr))
	return newOtelMetricsWithMeter(mp.Meter("gosdk"), jobName, instance), mr
}

func collect(t *testing.T, mr *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := mr.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("manual reader collect: %v", err)
	}
	return rm
}

func findSum(t *testing.T, rm metricdata.ResourceMetrics, name string) *metricdata.Sum[int64] {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
					return &sum
				}
			}
		}
	}
	t.Fatalf("sum %q not found in %v", name, allMetricNames(rm))
	return nil
}

func findHist(t *testing.T, rm metricdata.ResourceMetrics, name string) *metricdata.Histogram[float64] {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				if h, ok := m.Data.(metricdata.Histogram[float64]); ok {
					return &h
				}
			}
		}
	}
	t.Fatalf("histogram %q not found in %v", name, allMetricNames(rm))
	return nil
}

func allMetricNames(rm metricdata.ResourceMetrics) []string {
	var out []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			out = append(out, m.Name)
		}
	}
	return out
}

func attrValue(pts []attribute.KeyValue, key string) (string, bool) {
	for _, a := range pts {
		if string(a.Key) == key {
			return a.Value.String(), true
		}
	}
	return "", false
}

func TestOtelMetrics_ProcessCounter_Caching(t *testing.T) {
	m, _ := newTestOtelMetrics(t, "job1", "inst1")

	c1, err := m.ProcessCounter("ticker.fetch")
	if err != nil {
		t.Fatalf("first ProcessCounter: %v", err)
	}
	c2, err := m.ProcessCounter("ticker.fetch")
	if err != nil {
		t.Fatalf("second ProcessCounter: %v", err)
	}
	// Caching should reuse the same instrument pointer.
	if c1 != c2 {
		t.Errorf("expected cached counter to be the same pointer")
	}

	// Different domain must produce a different instrument.
	c3, err := m.QueueCounter("ticker.fetch")
	if err != nil {
		t.Fatalf("QueueCounter: %v", err)
	}
	if c1 == c3 {
		t.Errorf("counter for process.ticker.fetch should differ from queue.ticker.fetch")
	}
}

func TestOtelMetrics_ProcessCounter_Naming(t *testing.T) {
	m, mr := newTestOtelMetrics(t, "job1", "inst1")
	c, err := m.ProcessCounter("ticker.fetch")
	if err != nil {
		t.Fatalf("ProcessCounter: %v", err)
	}
	c.Add(context.Background(), 1)

	rm := collect(t, mr)
	if names := allMetricNames(rm); !slices.Contains(names, "process_ticker_fetch") {
		t.Fatalf("expected process_ticker_fetch in %v", names)
	}
}

func TestOtelMetrics_RecordProcessWithDuration_Success(t *testing.T) {
	m, mr := newTestOtelMetrics(t, "job1", "inst1")
	c, _ := m.ProcessCounter("ticker.fetch")
	h, _ := m.ProcessHistogram("ticker.fetch")

	m.RecordProcessWithDuration(context.Background(), c, h, "AAPL", "success", "", 150.5)

	rm := collect(t, mr)
	sum := findSum(t, rm, "process_ticker_fetch")
	if len(sum.DataPoints) != 1 {
		t.Fatalf("want 1 datapoint, got %d", len(sum.DataPoints))
	}
	dp := sum.DataPoints[0]
	if dp.Value != 1 {
		t.Errorf("counter value: want 1, got %v", dp.Value)
	}

	attrs := dp.Attributes.ToSlice()
	if v, ok := attrValue(attrs, "status"); !ok || v != "success" {
		t.Errorf("status attr: want success, got %q (present=%v)", v, ok)
	}
	if v, ok := attrValue(attrs, "ticker"); !ok || v != "AAPL" {
		t.Errorf("ticker attr: want AAPL, got %q (present=%v)", v, ok)
	}
	if v, ok := attrValue(attrs, "job_name"); !ok || v != "job1" {
		t.Errorf("job_name attr: want job1, got %q (present=%v)", v, ok)
	}
	if v, ok := attrValue(attrs, "instance"); !ok || v != "inst1" {
		t.Errorf("instance attr: want inst1, got %q (present=%v)", v, ok)
	}
	if _, ok := attrValue(attrs, "error_type"); ok {
		t.Errorf("error_type attr should be omitted on success")
	}

	hist := findHist(t, rm, "process_ticker_fetch")
	if len(hist.DataPoints) != 1 {
		t.Fatalf("histogram: want 1 datapoint, got %d", len(hist.DataPoints))
	}
	if hist.DataPoints[0].Count != 1 {
		t.Errorf("histogram count: want 1, got %d", hist.DataPoints[0].Count)
	}
}

func TestOtelMetrics_RecordProcessWithDuration_Failure(t *testing.T) {
	m, mr := newTestOtelMetrics(t, "job1", "inst1")
	c, _ := m.ProcessCounter("ticker.fetch")
	h, _ := m.ProcessHistogram("ticker.fetch")

	m.RecordProcessWithDuration(context.Background(), c, h, "AAPL", "failure", "timeout", 3000.0)

	rm := collect(t, mr)
	sum := findSum(t, rm, "process_ticker_fetch")
	dp := sum.DataPoints[0]
	if v, ok := attrValue(dp.Attributes.ToSlice(), "status"); !ok || v != "failure" {
		t.Errorf("status attr: want failure, got %q (present=%v)", v, ok)
	}
	if v, ok := attrValue(dp.Attributes.ToSlice(), "error_type"); !ok || v != "timeout" {
		t.Errorf("error_type attr: want timeout, got %q (present=%v)", v, ok)
	}
}

func TestOtelMetrics_RecordProcessWithDuration_NoDuration(t *testing.T) {
	m, mr := newTestOtelMetrics(t, "job1", "inst1")
	c, _ := m.ProcessCounter("ticker.fetch")
	h, _ := m.ProcessHistogram("ticker.fetch")

	// durationMs=0: counter still +1, histogram must not be touched.
	m.RecordProcessWithDuration(context.Background(), c, h, "AAPL", "success", "", 0)

	rm := collect(t, mr)
	sum := findSum(t, rm, "process_ticker_fetch")
	if len(sum.DataPoints) != 1 || sum.DataPoints[0].Value != 1 {
		t.Errorf("counter should be +1 even when durationMs=0; got %+v", sum.DataPoints)
	}

	// Histogram should not be present (we never call Record on it).
	if names := allMetricNames(rm); slices.Contains(names, "process_ticker_fetch") {
		// metric name same as counter; verify by checking data type.
		// Iterate again to ensure no histogram datapoints.
		for _, sm := range rm.ScopeMetrics {
			for _, mm := range sm.Metrics {
				if mm.Name == "process_ticker_fetch" {
					if _, ok := mm.Data.(metricdata.Histogram[float64]); ok {
						t.Errorf("histogram should not have any data when durationMs=0")
					}
				}
			}
		}
	}
}
