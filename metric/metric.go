package svc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	metricMu         sync.Mutex
	providerInstance *sdkmetric.MeterProvider

	// Cached meter instruments (created in init)
	statusGauge  metric.Float64Gauge
	latencyGauge metric.Float64Gauge
	initErr      error
)

func init() {
	// Pre-create meter instruments using global meter (binds to real provider via otel.SetMeterProvider later)
	meter := otel.Meter("port_listenor")

	statusGauge, initErr = meter.Float64Gauge(
		"port_check_status",
		metric.WithDescription("1 = port open, 0 = port closed"),
	)
	if initErr != nil {
		panic(fmt.Errorf("failed to create status gauge in init: %w", initErr))
	}

	latencyGauge, initErr = meter.Float64Gauge(
		"port_check_latency_ms",
		metric.WithDescription("Port check latency in milliseconds"),
	)
	if initErr != nil {
		panic(fmt.Errorf("failed to create latency gauge in init: %w", initErr))
	}
}

// InitMeterProvider initializes the SDK MeterProvider, registers it globally, and caches it.
func InitMeterProvider(ctx context.Context, mimirURL string) error {
	metricMu.Lock()
	defer metricMu.Unlock()

	if providerInstance != nil {
		return nil
	}

	var opts []otlpmetrichttp.Option
	if mimirURL != "" {
		opts = append(opts, otlpmetrichttp.WithEndpointURL(mimirURL))
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(provider)
	providerInstance = provider

	return nil
}

// ShutdownOTel shuts down the global meter provider.
func ShutdownOTel(ctx context.Context) error {
	metricMu.Lock()
	defer metricMu.Unlock()

	if providerInstance != nil {
		return providerInstance.Shutdown(ctx)
	}
	return nil
}

// UpdateStatuses records the latest port statuses to OpenTelemetry metrics.
func UpdateStatuses(ctx context.Context, statuses []PortStatus) {
	for _, s := range statuses {
		val := 0.0
		if s.IsOpen {
			val = 1.0
		}
		pid := s.PID
		if pid == "" {
			pid = "unknown"
		}
		procName := s.ProcessName
		if procName == "" {
			procName = "unknown"
		}

		statusGauge.Record(ctx, val, metric.WithAttributes(
			attribute.String("host", s.Host),
			attribute.String("port", fmt.Sprintf("%d", s.Port)),
			attribute.String("service", s.Service),
			attribute.String("domain", s.Domain),
			attribute.String("pid", pid),
			attribute.String("process_name", procName),
		))

		latencyGauge.Record(ctx, s.LatencyMs, metric.WithAttributes(
			attribute.String("host", s.Host),
			attribute.String("port", fmt.Sprintf("%d", s.Port)),
			attribute.String("service", s.Service),
		))
	}
}
