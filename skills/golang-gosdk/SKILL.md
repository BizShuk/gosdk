---
name: golang-gosdk
description: Use when developing, reviewing, or refactoring Go applications that utilize the github.com/bizshuk/gosdk library for configuration management, HTTP routing, logging, or data processing.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuest
user-invocable: true
disable-model-invocation: true
context: fork
---

# golang-gosdk

## Overview

A unified reference for using the `github.com/bizshuk/gosdk` library. This SDK provides reusable modules for configuration management, Gin-based HTTP service skeletons, structured logging, and common data processing utilities to establish a consistent foundation across Go projects.

## Prerequisites & Versioning

**GitHub Repository:** `github.com/bizshuk/gosdk`
**Required Version:** `981c48d` (or newer)

> [!WARNING]
> If the project's `go.mod` specifies a version older than `981c48d` for `github.com/bizshuk/gosdk`, or if the local `version` file does not match, **WARN THE USER to update the SDK** before proceeding with major refactoring or implementation.

## When to Use

- Initializing a new Go service that requires configuration loading (`.env`, `yaml`, `embed.FS`).
- Setting up a Gin HTTP server with standardized middlewares (correlation IDs, security headers, health checks).
- Implementing structured, level-based logging using `zap`.
- Processing CSV files with automatic archiving and row-based callbacks.
- Dealing with CJK character encoding conversions (GBK, Big5 to UTF-8).
- Pushing time-series metrics to a Mimir / Prometheus remote-write endpoint.

## Quick Reference & Common Patterns

### 1. Initialization & Configuration

Configuration is globally managed via `viper`. The SDK uses a **dual-file loading pattern** (base file + `.local` override file) and automatically merges settings from search paths (e.g., `.`, `conf`, or the app config dir):

1. `.env` and `.env.local`
2. `config.yaml` and `config.local.yaml`
3. `settings.json` and `settings.local.json`

Environment variables prefixed with `APP_` automatically override configuration values (e.g., `APP_SERVER_PORT` overrides `server.port`, since underscores `_` map to dots `.`).

> [!IMPORTANT]
> **`GlobalConfig` variable has been completely removed.** You must access configuration values directly using `viper.Get*` functions, or deserialize them using `viper.Unmarshal` / `viper.UnmarshalKey`.

```go
import (
    "github.com/bizshuk/gosdk/config"
    "github.com/bizshuk/gosdk/config/common"
    "github.com/bizshuk/gosdk/log"
    "github.com/spf13/viper"
)

func main() {
    // Standard configuration loading:
    // Automatically merges configuration files from the active path.
    config.Default()

    // Preferred way: Use WithAppName to set user config directory (e.g. os.UserConfigDir()/my-app)
    // and optionally write default JSON configurations if missing.
    config.Default(
        config.WithAppName("my-app"),
        config.WithDefaultValue(`{"server": {"port": 8080}}`),
    )

    // Discouraged: WithConfigDir should only be used when a fixed custom directory path is strictly required.
    // config.Default(config.WithConfigDir("/path/to/configs"))

    // 2. Initialize logger based on config
    log.Init()
    log.Info("Configurations loaded")

    // 3. Access configurations via Viper API (GlobalConfig is removed)
    port := viper.GetInt("server.port")
    log.Infof("Server port: %d", port)

    // 4. (Optional) Initialize DB using common package
    if viper.IsSet("db.default") {
        gormDB, err := common.NewDBConfig("default").Create()
        if err != nil {
            log.Fatalf("DB connect failed: %v", err)
        }
        _ = gormDB
    }
}
```

### 2. HTTP Service (Gin)

Standardize HTTP servers using the provided middlewares and default routes.

```go
import (
 "github.com/bizshuk/gosdk/mw"
 "github.com/bizshuk/gosdk/router"
 "github.com/gin-gonic/gin"
)

func HTTPServer() {
 s := gin.Default()

 // Add standardized middlewares
 s.Use(mw.CorrelationID()) // Injects X-Correlation-Id
 s.Use(mw.Helmet())        // Injects security headers (Permissions-Policy, COOP, CSP, etc.)

 // Register default utility routes
 router.Default(s)           // /stats
 router.HealthRouterGroup(s) // /healthz
 router.PingRouterGroup(s)   // /ping

 s.Run(":8080")
}
```

### 3. CSV Processing & Callbacks

Use the `csv` and `utils` packages for robust file handling.

```go
import (
 "github.com/bizshuk/gosdk/encode/csv"
 "github.com/bizshuk/gosdk/utils"
)

// Process multiple CSVs in a directory
err := utils.NewCSVFilelistCallback("data/*.csv", func(fname string, row []string) error {
 // Logic to handle each row
 return nil
})

// Process a single CSV file with auto-archiving (.archived marker)
err := csv.ProcessCSVFile("data/import.csv", true, myRecordProcessor)
```

### 4. Logging

Use the unified `log` wrapper instead of the standard library or direct `zap` calls to ensure format consistency.

```go
import "github.com/bizshuk/gosdk/log"

// Use log package directly
log.Info("Standard info log")
log.Infof("Formatted log: %s", value)
log.Error("Error occurred")
log.Fatalf("Fatal error: %v", err) // Exits application
```

### 5. Metrics & Tracing (Mimir vs OpenTelemetry)

The SDK provides two ways to publish metrics. Depending on the complexity and needs of the project:

1. **Option A: Mimir Remote Write** (Lightweight, developer-pushed Prometheus write request).
2. **Option B: OpenTelemetry OTLP** (Standardized OTel SDK for metrics and distributed tracing).

#### Option A: Mimir Remote Write (Prometheus Remote-Write)

Push time-series metrics to Mimir using a lightweight HTTP-based writer. This requires no MeterProvider lifecycle management. The endpoint is configured via `MIMIR_URL` (default: `http://localhost:9009/api/v1/push`).

```go
import (
    "time"
    "github.com/bizshuk/gosdk/metric"
)

func main() {
    svc := metric.NewMimirService()

    // 1. Send a single metric
    _ = svc.Send(metric.Metric{
        Name:      "app.operation.duration", // "." in name will be sanitized to "_" automatically
        Timestamp: time.Now().Unix(),        // expects epoch SECONDS (int64)
        Value:     15.4,
        Tags:      map[string]string{"env": "prod", "service": "api"},
    })

    // 2. Batch send (highly recommended for performance)
    metrics := []metric.Metric{
        {Name: "app.cpu.usage", Timestamp: time.Now().Unix(), Value: 42.5, Tags: map[string]string{"host": "srv1"}},
        {Name: "app.memory.usage", Timestamp: time.Now().Unix(), Value: 80.0, Tags: map[string]string{"host": "srv1"}},
    }
    _ = svc.SendMulti(metrics)
}
```

Key behaviors of `MimirService`:

- Sanitization: `Metric.Name` replaces all `.` with `_` because Prometheus name spec disallows dots.
- Timestamp: Expects **epoch seconds** (`time.Now().Unix()`), NOT milliseconds.
- High-Performance: Uses HTTP connection pooling (`MaxIdleConnsPerHost: 100`).

---

#### Option B: OpenTelemetry (OTLP Metrics & Tracing)

Use the standard OpenTelemetry SDK to collect metrics and export traces. This requires initializing the Meter and Tracer Providers and ensuring they are shut down when the application terminates.
By default, the metric endpoint is read from `MIMIR_URL` (default: `http://localhost:9009/otlp/v1/metrics`), and the trace endpoint is read from `TEMPO_URL`.

```go
import (
    "context"
    "fmt"
    "time"

    "github.com/bizshuk/gosdk/metric"
    "go.opentelemetry.io/otel/attribute"
    otelmetric "go.opentelemetry.io/otel/metric"
)

// config/metric.go
var meter metric.Meter
var latencyGauge Float64Gauge

func InitMetric() {
    ctx := context.Background()

    // 1. Initialize global providers
    if err := metric.InitMeterProvider(ctx); err != nil {
        panic(err)
    }
    if err := metric.InitTracerProvider(ctx, ""); err != nil { // empty tempoURL falls back to TEMPO_URL env
        panic(err)
    }

    // Always defer ShutdownOTel before application exit to flush buffered telemetry
    defer func() {
        if err := metric.ShutdownOTel(ctx); err != nil {
            fmt.Printf("failed to shutdown providers: %v\n", err)
        }
    }()

    // 2. Register metrics using Meter
    meter := metric.Meter("my_app_sensor")

    latencyGauge, err := meter.Float64Gauge(
        "http_request_latency_ms",
        otelmetric.WithDescription("HTTP latency gauge"),
    )
    if err != nil {
        panic(err)
    }
}

func GetLatencyGauge() otelmetric.Float64Gauge {
    return latencyGauge
}

// main.go or other service methods
func main() {
    InitMetric()
    ctx := context.Background()

    latencyGauge := GetLatencyGauge()
    // 3. Record metric values
    latencyGauge.Record(ctx, 23.5, otelmetric.WithAttributes(
        attribute.String("method", "GET"),
        attribute.String("path", "/users"),
    ))

    // 4. Trace spans using Tracer
    tracer := metric.Tracer("my_app_tracer")
    tracedCtx, span := tracer.Start(ctx, "database_query")
    defer span.End()

    span.SetAttributes(attribute.String("db.system", "mysql"))
    // perform work using tracedCtx ...
}
```

Key behaviors of OTel Integration:

- **Shutdown is Critical**: Always use `defer metric.ShutdownOTel(ctx)` at the application entry point to prevent metrics/traces loss.
- **Synchronous Gauges**: The default `Float64Gauge` requires you to record values synchronously using `Record(ctx, val, attrs)`.

## Common Mistakes

| Mistake                                          | Correction                                                                                                                                               |
| ------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Using `fmt.Println` or standard `log`            | Always use `github.com/bizshuk/gosdk/log` to ensure JSON formatting in production and consistent log levels.                                             |
| Accessing `config.GlobalConfig`                  | `GlobalConfig` has been removed. Query values directly via `viper.Get*` or deserialize via `viper.UnmarshalKey`.                                         |
| Hardcoding `viper` keys for DB                   | Use `common.NewDBConfig("connectionName").Create()` which encapsulates the dialect selection and connection string logic.                                |
| Re-implementing security headers                 | Use `mw.Helmet()` instead of manually writing headers. It contains up-to-date best practices (e.g., `Permissions-Policy`, `Cross-Origin-Opener-Policy`). |
| Manual CSV opening and iteration                 | Use `csv.ProcessCSVFile` which handles skipping headers, filtering empty rows, and `.archived` marker generation.                                        |
| Calling `WithDefaultValue` alone                 | `WithDefaultValue` only writes if using `WithAppName` to ensure it is written to the correct folder.                                                     |
| Using `.` in Mimir metric names manually escaped | `metric.MimirService` sanitizes `.` → `_` automatically via `sanitizeMetricName`; don't pre-mangle names.                                                |
| Passing milliseconds to `Metric.Timestamp`       | Field expects **seconds** (epoch); use `time.Now().Unix()`, not `UnixMilli()`.                                                                           |
| Sending one metric at a time in tight loops      | Prefer `SendMulti` to batch samples into a single remote-write request (lower overhead, fewer HTTP round trips).                                         |
| Forgetting to call `ShutdownOTel`                | Always `defer metric.ShutdownOTel(ctx)` at application startup to flush all buffered metrics and trace spans before application exit.                    |
