---
name: golang-gosdk
description: Use when developing, reviewing, or refactoring Go applications that utilize the github.com/bizshuk/gosdk library for configuration management, HTTP routing, logging, or data processing.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuestion
user-invocable: true
disable-model-invocation: true
context: fork
---

# golang-gosdk

## Overview

A unified reference for using the `github.com/bizshuk/gosdk` library. This SDK provides reusable modules for configuration management, Gin-based HTTP service skeletons, structured logging, and common data processing utilities to establish a consistent foundation across Go projects.

## Prerequisites & Versioning

`GitHub Repository:` `github.com/bizshuk/gosdk`
`Required Go Version:` `1.26.0` (or newer)
`Required Version:` `d54814c` (or newer — introduces `MetricService` / `NewVictoriaMetricsService`)

> [!WARNING]
> If the project's `go.mod` specifies a version older than `d54814c` for `github.com/bizshuk/gosdk`, or if the local `version` file does not match, `WARN THE USER to update the SDK` before proceeding with major refactoring or implementation.

## When to Use

- Initializing a new Go service that requires configuration loading (`.env`, `yaml`, `embed.FS`).
- Setting up a Gin HTTP server with standardized middlewares (correlation IDs, security headers, health checks).
- Implementing structured, level-based logging using `zap`.
- Processing CSV files with automatic archiving and row-based callbacks.
- Dealing with CJK character encoding conversions (GBK, Big5 to UTF-8).
- Pushing time-series metrics to a VictoriaMetrics / Mimir / any Prometheus remote-write endpoint.

## Quick Reference & Common Patterns

### 1. Initialization & Configuration

Configuration is globally managed via `viper` — no global config struct. The SDK `config.Default()` loads and merges files automatically (dual-file pattern: base + `.local` override):

1. `.env` / `.env.local`
2. `config.yaml` / `config.local.yaml`
3. `settings.json` / `settings.local.json`

Environment variables prefixed with `APP_` override config values (`APP_SERVER_PORT` → `server.port`).

#### Standard Pattern (Application Config Package)

Create a `config/config.go` in your application. This is the **single source of truth** for configuration initialization:

```go
// config/config.go
package config

import (
    "github.com/bizshuk/gosdk/config"
    "github.com/spf13/viper"
)

func Init() {
    // 1. Load config files from ~/.config/<app_name>/ and working dir
    config.Default(config.WithAppName("<app_name>"))

    // 2. Set defaults explicitly — acts as fallback when config files omit keys
    viper.SetDefault("server.port", 8080)
    viper.SetDefault("db.driver", "sqlite")
    viper.SetDefault("log.level", "info")
}
```

Then in `main.go` or any entry point:

```go
import (
    "<module>/config"
    "github.com/bizshuk/gosdk/log"
    "github.com/spf13/viper"
)

func main() {
    config.Init()
    log.Init()

    // Access values directly via viper — no global struct needed
    port := viper.GetInt("server.port")
    host := viper.GetString("server.host")
    debug := viper.GetBool("app.debug")
}
```

> [!IMPORTANT]
> **Do NOT create a global config struct.** Use `viper.Get*()` directly where the value is needed. This is configuration dependency injection — each consumer pulls only the keys it requires.

#### Optional: Embed Default JSON

If you want a `settings.json` auto-created in `~/.config/<app_name>/` on first run:

```go
//go:embed default_settings.json
var defaultSettingJSON string

func Init() {
    config.Default(
        config.WithAppName("<app_name>"),
        config.WithDefaultValue(defaultSettingJSON),
    )
    // viper.SetDefault(...) for additional keys
}
```

#### Priority Order (highest → lowest)

| Priority | Source                        |
| -------- | ----------------------------- |
| 1        | `APP_*` environment variables |
| 2        | `.local` override files       |
| 3        | Base config files             |
| 4        | `viper.SetDefault()` values   |

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

The `log` package provides `Init()` to configure `zap` globally (level, format, timestamp). After calling `log.Init()`, use `zap.L()` (structured) or `zap.S()` (sugar) directly — no wrapper functions.

```go
import (
    "github.com/bizshuk/gosdk/log"
    "go.uber.org/zap"
)

func main() {
    log.Init() // configures zap globally based on PROFILE and LOG_LEVEL

    // Structured logging (preferred for production — zero-alloc)
    zap.L().Info("server started", zap.Int("port", 8080))
    zap.L().Error("connection failed", zap.Error(err))

    // Sugar logging (convenient for quick/format strings)
    zap.S().Infof("listening on %s", addr)
    zap.S().Warnf("retry %d/%d", attempt, maxRetries)
}
```

> [!IMPORTANT]
> **Do NOT use wrapper functions like `log.Info()`, `log.Errorf()`.** These have been removed. Use `zap.L()` or `zap.S()` directly.

### 5. Metrics & Tracing (Remote Write vs OpenTelemetry)

The SDK provides two ways to publish metrics. Depending on the complexity and needs of the project:

1. **Option A: Prometheus Remote Write** (Lightweight, developer-pushed write request — VictoriaMetrics, Mimir, or any remote-write compatible backend).
2. **Option B: OpenTelemetry OTLP** (Standardized OTel SDK for metrics and distributed tracing).

#### Option A: Remote Write (`MetricService`)

Push time-series metrics to any Prometheus remote-write compatible backend using a lightweight HTTP-based writer. This requires no MeterProvider lifecycle management. Backends differ only in the endpoint URL:

| Backend                     | Constructor                          | Config key                            | Default endpoint                     |
| --------------------------- | ------------------------------------ | ------------------------------------- | ------------------------------------ |
| VictoriaMetrics (`default`) | `metric.NewVictoriaMetricsService()` | `VICTORIAMETRICS_URL`                 | `http://localhost:8428/api/v1/write` |
| Mimir (compat alias)        | `metric.NewMimirService()`           | `MIMIR_URL`                           | `http://localhost:9009/api/v1/push`  |
| Any remote-write backend    | `metric.NewMetricService(url)`       | `METRIC_URL` (when `url == ""`)       | `http://localhost:8428/api/v1/write` |

```go
import (
    "time"
    "github.com/bizshuk/gosdk/metric"
)

func main() {
    svc := metric.NewVictoriaMetricsService() // or NewMetricService("") to honor METRIC_URL

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

Key behaviors of `MetricService`:

- Sanitization: `Metric.Name` replaces all `.` with `_` because Prometheus name spec disallows dots.
- Timestamp: Expects **epoch seconds** (`time.Now().Unix()`), NOT milliseconds.
- High-Performance: Uses HTTP connection pooling (`MaxIdleConnsPerHost: 100`).
- Compatibility: `MimirService` is a type alias of `MetricService`; `NewMimirService()` is kept for backward compatibility — prefer `NewVictoriaMetricsService()` or `NewMetricService(url)` in new code.

---

#### Option B: OpenTelemetry (OTLP Metrics & Tracing)

Use the standard OpenTelemetry SDK to collect metrics and export traces. This requires initializing the Meter and Tracer Providers and ensuring they are shut down when the application terminates.
The metric endpoint is read from `OTLP_METRIC_URL` (default: `http://localhost:8428/opentelemetry/v1/metrics` — VictoriaMetrics OTLP receiver). The trace endpoint is read from `OTLP_TRACE_URL` config key — if empty, the OTLP default endpoint (`localhost:4318`) is used.

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
    if err := metric.InitTracerProvider(ctx); err != nil { // endpoint from OTLP_TRACE_URL config; empty = OTLP default (localhost:4318)
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

### 6. Notifications (notify)

Use the `notify` package to send event summaries to one or more destinations. The package is backend-agnostic: all implementations satisfy the `Notifier` interface.

```go
import (
    "context"
    "github.com/bizshuk/gosdk/notify"
)

// Single destination — stdout
n := &notify.StdoutNotifier{}
_ = n.Notify(context.Background(), "job finished: 42 rows processed")

// Slack — token from SLACK_BOT_TOKEN, channel is the Slack channel ID
slackN := notify.NewSlackNotifier(os.Getenv("SLACK_BOT_TOKEN"), "C0123ABCDEF")
_ = slackN.Notify(context.Background(), "deployment succeeded")

// Fan-out to multiple destinations simultaneously
multi := notify.NewMulti(
    &notify.StdoutNotifier{},
    notify.NewSlackNotifier(os.Getenv("SLACK_BOT_TOKEN"), "C0123ABCDEF"),
)
if err := multi.Notify(ctx, "daily report ready"); err != nil {
    // errors.Join — contains errors from ALL failed notifiers
    log.Errorf("notify failed: %v", err)
}
```

Custom notifiers: implement the `Notifier` interface and plug into `NewMulti`:

```go
type EmailNotifier struct{ addr string }

func (e *EmailNotifier) Notify(_ context.Context, summary string) error {
    // send email …
    return nil
}

multi := notify.NewMulti(&notify.StdoutNotifier{}, &EmailNotifier{addr: "ops@example.com"})
```

Key behaviors of the `notify` package:

- **Graceful no-op**: `NewSlackNotifier` with an empty token creates a nil client; `Notify` logs a warning and returns nil without panicking. Safe to initialize unconditionally; skip-at-runtime if env vars are absent.
- **Fan-out error handling**: `Multi.Notify` always calls every registered notifier — it never short-circuits on failure. All errors are joined via `errors.Join`; check the combined error after the call.
- **Format is caller's responsibility**: The `summary string` is an opaque, pre-formatted message. Serialize your struct/report to a string before calling `Notify`.

### 7. Home Path Expansion

Use `github.com/mitchellh/go-homedir` to expand `~` in paths. Call `homedir.Expand()` directly at point of use — do NOT create a custom expand function.

```go
import "github.com/mitchellh/go-homedir"

// Standard pattern: expand and fall back silently on error
dbPath := viper.GetString("state.db_path")  // e.g. "~/.config/myapp/state.db"
path, err := homedir.Expand(dbPath)
if err != nil {
    path = dbPath
}
```

Key rules:

- **No wrappers**: call `homedir.Expand()` inline, DO NOT wrap it in `expandPath()` / `expandHome()`
- **Silent fallback**: on error, use the original path as-is — unless the caller explicitly needs to handle the error
- **No-op when safe**: if the path has no `~` prefix, `Expand()` returns it unchanged

## Common Mistakes

| Mistake                                          | Correction                                                                                                                                                        |
| ------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Using `fmt.Println` or standard `log`            | Import `gosdk/log` for `Init()`, then use `zap.L()` (structured) or `zap.S()` (sugar) for all logging.                                                            |
| Using removed `log.Info()` / `log.Errorf()` etc  | Sugar wrappers have been removed. Use `zap.S().Info()` / `zap.S().Errorf()` or `zap.L().Info()` with `zap.String()` fields.                                       |
| Creating a global config struct                  | Use `viper.Get*()` directly at point of use. No global struct needed — viper IS the global config store.                                                          |
| Setting defaults before `config.Default()`       | Call `viper.SetDefault()` AFTER `config.Default()` so file-loaded values take precedence over defaults.                                                           |
| Hardcoding `viper` keys for DB                   | Use `db.InitSQLite()` / `db.InitMySQL()` which read flat `SQLITE_PATH` / `MYSQL_DSN` from viper, open a connection, and set the corresponding `Default<Storage>` singleton. |
| Re-implementing security headers                 | Use `mw.Helmet()` instead of manually writing headers. It contains up-to-date best practices (e.g., `Permissions-Policy`, `Cross-Origin-Opener-Policy`).          |
| Manual CSV opening and iteration                 | Use `csv.ProcessCSVFile` which handles skipping headers, filtering empty rows, and `.archived` marker generation.                                                 |
| Calling `WithDefaultValue` alone                 | `WithDefaultValue` only writes if using `WithAppName` to ensure it is written to the correct folder.                                                              |
| Using `.` in metric names manually escaped       | `metric.MetricService` sanitizes `.` → `_` automatically via `sanitizeMetricName`; don't pre-mangle names.                                                        |
| Using `NewMimirService()` in new code            | Deprecated compat alias. Use `NewVictoriaMetricsService()` (default backend) or `NewMetricService(url)`.                                                          |
| Passing milliseconds to `Metric.Timestamp`       | Field expects **seconds** (epoch); use `time.Now().Unix()`, not `UnixMilli()`.                                                                                    |
| Sending one metric at a time in tight loops      | Prefer `SendMulti` to batch samples into a single remote-write request (lower overhead, fewer HTTP round trips).                                                  |
| Forgetting to call `ShutdownOTel`                | Always `defer metric.ShutdownOTel(ctx)` at application startup to flush all buffered metrics and trace spans before application exit.                             |
| Passing a struct directly to `Notify`            | `Notifier.Notify` only accepts a `string`. Serialize your payload (e.g., `fmt.Sprintf` or `json.Marshal`) before calling `Notify`.                                |
| Expecting `Multi` to stop on first error         | `Multi.Notify` calls every notifier regardless of errors. Check the combined `errors.Join` error after the call — it may contain errors from multiple notifiers.  |
| Panicking when Slack token is missing            | `NewSlackNotifier("", channelID)` is intentionally a no-op; it logs a warning and returns `nil`. No need to guard the constructor with an `if token != ""` check. |
| Creating custom `expandPath()` / `expandHome()`  | Use `homedir.Expand()` directly at call site. No wrapper function needed — it handles no-`~` paths as no-op.                                                      |
