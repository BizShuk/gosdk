package metric

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var (
	metricMu         sync.Mutex
	providerInstance *sdkmetric.MeterProvider

	traceMu        sync.Mutex
	tracerInstance *sdktrace.TracerProvider
)

// Meter returns a Meter from the global MeterProvider.
func Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return otel.GetMeterProvider().Meter(name, opts...)
}

// Tracer returns a Tracer from the global TracerProvider.
func Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return otel.GetTracerProvider().Tracer(name, opts...)
}

func init() {
	viper.SetDefault("METRIC_URL", "http://localhost:9009/otlp/v1/metrics")
}

// InitMeterProvider initializes the SDK MeterProvider, registers it globally, and caches it.
func InitMeterProvider(ctx context.Context) error {
	metricMu.Lock()
	defer metricMu.Unlock()

	if providerInstance != nil {
		return nil
	}

	mimirURL := viper.GetString("METRIC_URL")

	var opts []otlpmetrichttp.Option
	opts = append(opts, otlpmetrichttp.WithEndpointURL(mimirURL))
	// If URL starts with http://, we need to specify WithInsecure option.
	// We use a simple prefix check.
	if len(mimirURL) >= 7 && mimirURL[:7] == "http://" {
		opts = append(opts, otlpmetrichttp.WithInsecure())
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

// InitTracerProvider initializes the SDK TracerProvider, registers it globally, and caches it.
func InitTracerProvider(ctx context.Context, tempoURL string) error {
	traceMu.Lock()
	defer traceMu.Unlock()

	if tracerInstance != nil {
		return nil
	}

	if tempoURL == "" {
		tempoURL = viper.GetString("TEMPO_URL")
	}

	var opts []otlptracehttp.Option
	if tempoURL != "" {
		opts = append(opts, otlptracehttp.WithEndpointURL(tempoURL))
		if len(tempoURL) >= 7 && tempoURL[:7] == "http://" {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
	} else {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	res, err := resource.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(provider)
	tracerInstance = provider

	return nil
}

// ShutdownOTel shuts down the global meter and tracer providers.
func ShutdownOTel(ctx context.Context) error {
	metricMu.Lock()
	defer metricMu.Unlock()
	traceMu.Lock()
	defer traceMu.Unlock()

	var errs []error
	if providerInstance != nil {
		if err := providerInstance.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown metric provider: %w", err))
		}
		providerInstance = nil
	}

	if tracerInstance != nil {
		if err := tracerInstance.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
		tracerInstance = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
