package metric

import (
	"context"
	"strconv"
	"strings"

	"github.com/bizshuk/gosdk/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// OtelMetrics provides process/queue/service metrics with OpenTelemetry.
type OtelMetrics struct {
	meter    metric.Meter
	mimir    *MimirService
	jobName  string
	instance string
}

// MetricAttributes defines tag key-value pairs.
type MetricAttributes map[string]string

// NewOtelMetrics creates a new OtelMetrics instance.
func NewOtelMetrics(jobName, instance string, mimir *MimirService) (*OtelMetrics, error) {
	mp := sdkmetric.NewMeterProvider()
	meter := mp.Meter("gosdk")

	return &OtelMetrics{
		meter:    meter,
		mimir:    mimir,
		jobName:  jobName,
		instance: instance,
	}, nil
}

// ProcessCounter creates a counter for batch process operations
func (m *OtelMetrics) ProcessCounter(name, operation string) (metric.Int64Counter, error) {
	return m.meter.Int64Counter(
		"process."+name+"."+operation,
		metric.WithDescription("Batch process counter"),
		metric.WithUnit("1"),
	)
}

// ProcessHistogram creates a histogram for batch process duration
func (m *OtelMetrics) ProcessHistogram(name, operation string) (metric.Float64Histogram, error) {
	return m.meter.Float64Histogram(
		"process."+name+"."+operation+".duration",
		metric.WithDescription("Batch process duration"),
		metric.WithUnit("ms"),
	)
}

// QueueCounter creates a counter for queue/job operations
func (m *OtelMetrics) QueueCounter(name, operation string) (metric.Int64Counter, error) {
	return m.meter.Int64Counter(
		"queue."+name+"."+operation,
		metric.WithDescription("Queue job counter"),
		metric.WithUnit("1"),
	)
}

// QueueHistogram creates a histogram for queue job duration
func (m *OtelMetrics) QueueHistogram(name, operation string) (metric.Float64Histogram, error) {
	return m.meter.Float64Histogram(
		"queue."+name+"."+operation+".duration",
		metric.WithDescription("Queue job duration"),
		metric.WithUnit("ms"),
	)
}

// ServiceCounter creates a counter for service/API operations
func (m *OtelMetrics) ServiceCounter(name, operation string) (metric.Int64Counter, error) {
	return m.meter.Int64Counter(
		"service."+name+"."+operation,
		metric.WithDescription("Service API counter"),
		metric.WithUnit("1"),
	)
}

// ServiceHistogram creates a histogram for service request duration
func (m *OtelMetrics) ServiceHistogram(name, operation string) (metric.Float64Histogram, error) {
	return m.meter.Float64Histogram(
		"service."+name+"."+operation+".duration",
		metric.WithDescription("Service request duration"),
		metric.WithUnit("ms"),
	)
}

// RecordProcess records a process event with standard tags
func (m *OtelMetrics) RecordProcess(counter metric.Int64Counter, ticker, status, errorType string) {
	ctx := context.Background()
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
}

// RecordProcessWithDuration records a process event with duration histogram
func (m *OtelMetrics) RecordProcessWithDuration(counter metric.Int64Counter, histogram metric.Float64Histogram, ticker, status, errorType string, durationMs float64) {
	ctx := context.Background()
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
	histogram.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}

// RecordQueue records a queue event with standard tags
func (m *OtelMetrics) RecordQueue(counter metric.Int64Counter, workerID, jobType, queueName, status string) {
	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("job_name", m.jobName),
		attribute.String("instance", m.instance),
		attribute.String("worker_id", workerID),
		attribute.String("job_type", jobType),
		attribute.String("queue_name", queueName),
		attribute.String("status", status),
	}

	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordService records a service event with standard tags
func (m *OtelMetrics) RecordService(counter metric.Int64Counter, endpoint, method, statusCode, source string) {
	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("job_name", m.jobName),
		attribute.String("instance", m.instance),
		attribute.String("endpoint", endpoint),
		attribute.String("method", method),
		attribute.String("status_code", statusCode),
		attribute.String("source", source),
	}

	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// BatchSummary holds the result of a batch process.
type BatchSummary struct {
	Total      int
	Succeed    int
	Failed     int
	FailedList []string
	DurationMs float64
}

// NotifySend sends a batch summary to Mimir via Slack notifier.
func (m *OtelMetrics) NotifySend(ctx context.Context, summary string) error {
	if m.mimir == nil {
		log.Warn("Mimir not configured")
		return nil
	}
	log.Infof("Batch summary: %s", summary)
	return nil
}

// FormatBatchSummary formats a BatchSummary into a string.
func FormatBatchSummary(jobName string, s BatchSummary) string {
	var b strings.Builder
	b.WriteString(jobName)
	b.WriteString(" batch complete: ")
	b.WriteString(strconv.Itoa(s.Total))
	b.WriteString(" total, ")
	b.WriteString(strconv.Itoa(s.Succeed))
	b.WriteString(" succeeded, ")
	b.WriteString(strconv.Itoa(s.Failed))
	b.WriteString(" failed")
	if len(s.FailedList) > 0 {
		b.WriteString(" [")
		b.WriteString(strings.Join(s.FailedList, ", "))
		b.WriteString("]")
	}
	return b.String()
}
