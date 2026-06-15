package metric

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// PortStatus 代表應用層監測的埠口狀態
type PortStatus struct {
	Host        string
	Port        int
	Service     string
	Domain      string
	IsOpen      bool
	LatencyMs   float64
	PID         string
	ProcessName string
}

var (
	statusGauge  otelmetric.Float64Gauge
	latencyGauge otelmetric.Float64Gauge
)

func TestOtelSample(t *testing.T) {
	// 初始化 slog 結構化日誌（測試環境使用 debug level + text handler）
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	ctx := context.Background()

	err := InitMeterProvider(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize MeterProvider: %v", err)
	}

	slog.Debug("Initializing OpenTelemetry TracerProvider")
	err = InitTracerProvider(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize TracerProvider: %v", err)
	}

	defer func() {
		// 關閉以確保所有緩衝指標與 traces 皆已導出
		if err := ShutdownOTel(ctx); err != nil {
			slog.Error("Failed to shutdown Meter/Tracer Provider", "err", err)
		} else {
			slog.Debug("Meter/Tracer Provider shutdown successfully")
		}
	}()

	// 2. 在服務/應用層註冊自訂指標
	// 使用 SDK 封裝的 Meter 函數獲取 meter 實例
	meter := Meter("port_listenor")

	statusGauge, err = meter.Float64Gauge(
		"port_check_status",
		otelmetric.WithDescription("1 = port open, 0 = port closed"),
	)
	if err != nil {
		t.Fatalf("Failed to create status gauge: %v", err)
	}

	latencyGauge, err = meter.Float64Gauge(
		"port_check_latency_ms",
		otelmetric.WithDescription("Port check latency in milliseconds"),
	)
	if err != nil {
		t.Fatalf("Failed to create latency gauge: %v", err)
	}

	// 3. 模擬埠口監測並更新狀態
	mockStatuses := []PortStatus{
		{
			Host:        "127.0.0.1",
			Port:        8080,
			Service:     "web-api",
			Domain:      "production",
			IsOpen:      true,
			LatencyMs:   12.5,
			PID:         "1234",
			ProcessName: "go-web-server",
		},
		{
			Host:        "127.0.0.1",
			Port:        3306,
			Service:     "mysql-db",
			Domain:      "production",
			IsOpen:      false,
			LatencyMs:   0.0,
			PID:         "",
			ProcessName: "",
		},
	}

	slog.Debug("Recording port statuses to OpenTelemetry and Tracing...")
	UpdateStatuses(ctx, mockStatuses)

	slog.Debug("Simulate waiting for telemetry export interval...")
	time.Sleep(2 * time.Second)

	slog.Debug("Sample execution finished.")
}

// UpdateStatuses 將埠口檢測狀態記錄到 OpenTelemetry 指標中，並記錄 Trace Spans
func UpdateStatuses(ctx context.Context, statuses []PortStatus) {
	tracer := Tracer("port_checker")
	ctx, parentSpan := tracer.Start(ctx, "UpdateStatuses")
	defer parentSpan.End()

	for _, s := range statuses {
		_, span := tracer.Start(ctx, fmt.Sprintf("CheckPort:%s:%d", s.Host, s.Port))
		span.SetAttributes(
			attribute.String("host", s.Host),
			attribute.Int("port", s.Port),
			attribute.Bool("is_open", s.IsOpen),
		)

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

		slog.Debug("Recording status",
			"service", s.Service,
			"port", s.Port,
			"value", val,
			"latency_ms", s.LatencyMs,
		)

		statusGauge.Record(ctx, val, otelmetric.WithAttributes(
			attribute.String("host", s.Host),
			attribute.String("port", fmt.Sprintf("%d", s.Port)),
			attribute.String("service", s.Service),
			attribute.String("domain", s.Domain),
			attribute.String("pid", pid),
			attribute.String("process_name", procName),
		))

		latencyGauge.Record(ctx, s.LatencyMs, otelmetric.WithAttributes(
			attribute.String("host", s.Host),
			attribute.String("port", fmt.Sprintf("%d", s.Port)),
			attribute.String("service", s.Service),
		))

		span.End()
	}
}
