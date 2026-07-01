package metric

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// OtelMetrics provides high-level, business-semantic metric instruments for
// process / queue / service operations. It is the Go counterpart of the
// Python `OtelMetrics` class in `metric/otel.py` and follows the
// process-metrics specification in `SPEC.md`.
//
// The struct owns an `sdkmetric.MeterProvider` and a process/queue/service
// set of counters + histograms. Instrument instances are cached per
// `domain.name` key so the same name returns the same instrument across
// calls — matching the OTel SDK's dedup semantics while avoiding the
// per-call `Int64Counter` allocation.
//
// Naming convention: the caller passes dotted names such as
// `"ticker.fetch"`. The OTel instrument name is the dot-to-underscore form
// (`"ticker_fetch"`) which is the de-facto Prometheus / Mimir standard.
// Histogram unit is always `"ms"`; callers are responsible for unit
// conversion before calling `Record*WithDuration`.
type OtelMetrics struct {
	jobName       string
	instance      string
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter

	mu         sync.RWMutex
	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Float64Histogram
}

// NewOtelMetrics constructs an OtelMetrics with a fresh OTLP HTTP exporter
// pointed at the configured endpoint (viper `OTLP_METRIC_URL`, default
// `http://localhost:8428/opentelemetry/v1/metrics`).
//
// Resource attributes: `service.name="gosdk"`, `job_name=<jobName>`,
// `instance=<instance>`.
//
// The constructed MeterProvider is registered globally via
// `otel.SetMeterProvider` (mirroring the Python implementation). Callers
// MUST invoke `Shutdown(ctx)` on the returned instance to flush pending
// metrics before process exit.
//
// The exported `histogram` unit is `"ms"`; pass the duration in
// milliseconds when calling `RecordProcessWithDuration`.
func NewOtelMetrics(jobName, instance string) (*OtelMetrics, error) {
	metricURL := viperGetOTLPMetricURL()

	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpointURL(metricURL),
	}
	if strings.HasPrefix(metricURL, "http://") {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	exp, err := otlpmetrichttp.New(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("gosdk"),
		attribute.String("job_name", jobName),
		attribute.String("instance", instance),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(30*time.Second))),
	)
	otel.SetMeterProvider(mp)

	return &OtelMetrics{
		jobName:       jobName,
		instance:      instance,
		meterProvider: mp,
		meter:         mp.Meter("gosdk"),
		counters:      make(map[string]metric.Int64Counter),
		histograms:    make(map[string]metric.Float64Histogram),
	}, nil
}

// newOtelMetricsWithMeter is a test-only constructor that wires a caller-
// provided Meter (e.g. backed by sdkmetric.NewManualReader) without
// registering a global MeterProvider. Production code MUST use
// `NewOtelMetrics`.
func newOtelMetricsWithMeter(meter metric.Meter, jobName, instance string) *OtelMetrics {
	return &OtelMetrics{
		jobName:    jobName,
		instance:   instance,
		meter:      meter,
		counters:   make(map[string]metric.Int64Counter),
		histograms: make(map[string]metric.Float64Histogram),
	}
}

// viperGetOTLPMetricURL resolves the OTLP endpoint from viper with a
// sensible default.
func viperGetOTLPMetricURL() string {
	if v := viper.GetString("OTLP_METRIC_URL"); v != "" {
		return v
	}
	return "http://localhost:8428/opentelemetry/v1/metrics"
}

// Shutdown flushes any pending metrics and shuts down the underlying
// MeterProvider. Safe to call multiple times.
func (m *OtelMetrics) Shutdown(ctx context.Context) error {
	if m.meterProvider == nil {
		return nil
	}
	return m.meterProvider.Shutdown(ctx)
}

// ProcessCounter returns the cached Int64Counter for the process domain,
// creating it on first call. `name` is a dotted operation name such as
// `"ticker.fetch"`; the underlying OTel instrument is registered as
// `"ticker_fetch"`.
func (m *OtelMetrics) ProcessCounter(name string) (metric.Int64Counter, error) {
	return m.getOrCreateCounter("process", name)
}

// ProcessHistogram returns the cached Float64Histogram for the process
// domain with unit `"ms"`. See `ProcessCounter` for naming rules.
func (m *OtelMetrics) ProcessHistogram(name string) (metric.Float64Histogram, error) {
	return m.getOrCreateHistogram("process", name)
}

// QueueCounter returns the cached Int64Counter for the queue domain.
func (m *OtelMetrics) QueueCounter(name string) (metric.Int64Counter, error) {
	return m.getOrCreateCounter("queue", name)
}

// QueueHistogram returns the cached Float64Histogram for the queue domain.
func (m *OtelMetrics) QueueHistogram(name string) (metric.Float64Histogram, error) {
	return m.getOrCreateHistogram("queue", name)
}

// ServiceCounter returns the cached Int64Counter for the service domain.
func (m *OtelMetrics) ServiceCounter(name string) (metric.Int64Counter, error) {
	return m.getOrCreateCounter("service", name)
}

// ServiceHistogram returns the cached Float64Histogram for the service
// domain.
func (m *OtelMetrics) ServiceHistogram(name string) (metric.Float64Histogram, error) {
	return m.getOrCreateHistogram("service", name)
}

// RecordProcessWithDuration records a single process event. It increments
// the supplied counter by 1 and, when `durationMs > 0`, records the
// duration on the supplied histogram. Standard attributes
// (`job_name`, `instance`, `ticker`, `status`) are always attached; the
// `error_type` attribute is only attached when `errorType` is non-empty.
//
// This is the Go equivalent of the Python `OtelMetrics.record_process`
// helper and the usage example in `SPEC.md` (lines 109-112).
func (m *OtelMetrics) RecordProcessWithDuration(
	ctx context.Context,
	counter metric.Int64Counter,
	histogram metric.Float64Histogram,
	ticker, status, errorType string,
	durationMs float64,
) {
	attrs := []attribute.KeyValue{
		attribute.String("job_name", m.jobName),
		attribute.String("instance", m.instance),
		attribute.String("ticker", ticker),
		attribute.String("status", status),
	}
	if errorType != "" {
		attrs = append(attrs, attribute.String("error_type", errorType))
	}

	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
	if durationMs > 0 {
		histogram.Record(ctx, durationMs, metric.WithAttributes(attrs...))
	}
}

func (m *OtelMetrics) getOrCreateCounter(domain, name string) (metric.Int64Counter, error) {
	key := domain + "." + name
	instrumentName := sanitizeMetricName(key)

	m.mu.RLock()
	if c, ok := m.counters[instrumentName]; ok {
		m.mu.RUnlock()
		return c, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	// Double-check after acquiring write lock.
	if c, ok := m.counters[instrumentName]; ok {
		return c, nil
	}

	c, err := m.meter.Int64Counter(
		instrumentName,
		metric.WithDescription(fmt.Sprintf("%s.%s counter", domain, name)),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("create counter %s: %w", instrumentName, err)
	}
	m.counters[instrumentName] = c
	return c, nil
}

func (m *OtelMetrics) getOrCreateHistogram(domain, name string) (metric.Float64Histogram, error) {
	key := domain + "." + name
	instrumentName := sanitizeMetricName(key)

	m.mu.RLock()
	if h, ok := m.histograms[instrumentName]; ok {
		m.mu.RUnlock()
		return h, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.histograms[instrumentName]; ok {
		return h, nil
	}

	h, err := m.meter.Float64Histogram(
		instrumentName,
		metric.WithDescription(fmt.Sprintf("%s.%s duration", domain, name)),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("create histogram %s: %w", instrumentName, err)
	}
	m.histograms[instrumentName] = h
	return h, nil
}

// sanitizeMetricName converts a dotted name into the underscore form
// expected by Prometheus / Mimir (`process.ticker.fetch` ->
// `process_ticker_fetch`). The implementation lives in `metric.go` and is
// shared between the remote-write and OTel paths.
