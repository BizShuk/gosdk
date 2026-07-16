---
name: golang-gosdk
description: Use when developing, reviewing, or refactoring Go applications that utilize the github.com/bizshuk/gosdk library for configuration management, HTTP routing, logging, or data processing.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuestion
user-invocable: true
disable-model-invocation: false
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
- Implementing structured, level-based logging using stdlib `log/slog`.
- Processing CSV files with automatic archiving and row-based callbacks.
- Dealing with CJK character encoding conversions (GBK, Big5 to UTF-8).
- Pushing time-series metrics to a VictoriaMetrics / Mimir / any Prometheus remote-write endpoint.

## Quick Reference & Common Patterns

### 1. Initialization & Configuration

This is Default way to load the configuration.
Create a `config/config.go` in your application. This is the **single source of truth** for configuration initialization

Configuration is globally managed via `viper` — no global config struct. The SDK `config.Default(opts ...ConfigOption)` loads and merges files automatically (dual-file pattern: base + `.local` override):

1. `.env` / `.env.local`
2. `config.yaml` / `config.local.yaml`
3. `settings.json` / `settings.local.json`

Environment variables prefixed with `APP_` override config values (`APP_SERVER_PORT` → `server.port`).

#### `config.Default()` Options

`Default()` takes functional options. Without any option it loads from the working dir (`.` and `./conf`).

| Option                      | Effect                                                                                                                                              |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `WithAppName("myapp")`      | Sets the app name and enables the user config dir `~/.config/myapp` (`GetAppConfigDir()`). Required for `WithDefaultValue` and the app dir helpers. |
| `WithDefaultValue(jsonStr)` | Auto-creates `settings.json` in `GetAppConfigDir()` on first run if it does not exist. **No-op unless `WithAppName` is also set.**                  |

#### Where `config.Default()` Loads From

Every format loader (`.env`, `config.yaml`, `settings.json`) searches the **same three directories, in this fixed order** — viper uses the _first_ directory in which the named file exists:

```tree
1. .                       # current working dir
2. ./conf                  # conf/ subdir of cwd
3. ~/.config/<appName>     # GetAppConfigDir() — EMPTY unless WithAppName is set
```

So a `settings.json` in the working dir shadows one in `~/.config/<appName>`. The app's installed/home config lives in dir 3; project-local overrides live in dirs 1–2.

**Common config in `GetAppConfigDir()` → `settings.json`.** When you ship an app with `WithAppName`, the canonical user-level config file in `~/.config/<appName>/` is `settings.json`, because `WithDefaultValue` bootstraps exactly that file there on first run. `.env` is also searched in the same dir but, by convention, `.env` is the working-dir/dev mechanism (secrets, local overrides) rather than the installed app config.

Sibling app dirs derived from `GetAppConfigDir()` (all require `WithAppName`):

| Helper              | Path                       | Purpose        |
| ------------------- | -------------------------- | -------------- |
| `GetAppConfigDir()` | `~/.config/<appName>`      | config files   |
| `GetAppLogDir()`    | `~/.config/<appName>/log`  | log output     |
| `GetAppDataDir()`   | `~/.config/<appName>/data` | data (e.g. DB) |

> [!IMPORTANT]
> These dirs are a fixed convention — do NOT add options to redirect config into the data dir or vice versa.

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

Across formats, `Default()` merges env → yaml → json with whole-map override, so JSON wins over YAML wins over dotenv; within each format `.local` wins over its base.

| Priority | Source                                                 |
| -------- | ------------------------------------------------------ |
| 1        | `APP_*` environment variables (`viper.AutomaticEnv()`) |
| 2        | `settings.local.json` → `settings.json`                |
| 3        | `config.local.yaml` → `config.yaml`                    |
| 4        | `.env.local` → `.env`                                  |
| 5        | `viper.SetDefault()` values                            |

#### Setting a Default and Overriding It

There are two distinct "defaults" — don't confuse them:

| Mechanism                     | What it is                                      | Where it sits in priority        |
| ----------------------------- | ----------------------------------------------- | -------------------------------- |
| `viper.SetDefault(key, val)`  | In-code fallback value for a single key         | Lowest (priority 5 above)        |
| `config.WithDefaultValue(js)` | Bootstraps a whole `settings.json` file on disk | A file source (priority 2 above) |

`Set the default`, then let any higher-priority source override it. Always call `viper.SetDefault()` **after** `config.Default()` so file/env values take precedence:

```go
func Init() {
    config.Default(config.WithAppName("myapp"))

    // Default — used only when no file/env supplies the key
    viper.SetDefault("server.port", 8080)
}
```

The override then happens automatically by priority, no extra code:

```text
viper.SetDefault("server.port", 8080)   // 1) fallback = 8080
# settings.json: { "server": { "port": 9000 } }   → viper.GetInt("server.port") == 9000  (file overrides default)
# APP_SERVER_PORT=9100 (env)                       → viper.GetInt("server.port") == 9100  (env overrides file)
```

`Force an override in code` (highest priority of all — beats env and files) with `viper.Set()`; use this for computed/test values, not for normal config:

```go
viper.Set("server.port", 0) // explicit Set wins over every other source until unset
```

> [!IMPORTANT]
> Order matters: `viper.SetDefault()` BEFORE `config.Default()` would be clobbered only if a file/env provides the key — but calling it AFTER is the safe convention so the default never accidentally shadows a freshly loaded value. Reserve `viper.Set()` for deliberate hard overrides (tests, derived values); it outranks user config.

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

#### File Write & Open with Options

`utils.WriteFile` (write a payload) and `utils.OpenFile` (get an `*os.File` to write yourself) share the same functional options. `WriteFile` opens via `OpenFile`, `io.Copy`s the payload, and closes for you; `OpenFile` hands back the file (you `Close()` it). `utils.CreateFile` is an alias of `WriteFile`.

| Option                 | Effect                                                                                           | Default if omitted                           |
| ---------------------- | ------------------------------------------------------------------------------------------------ | -------------------------------------------- |
| `WithCreate()`         | Create the file (and parent dirs) if it does not exist.                                          | Returns `os.ErrNotExist` when file is absent |
| `WithBackup()`         | Before overwriting an existing file, rename it to `<file>.bak` (chains to `.bak.bak` if needed). | Existing file is truncated/overwritten       |
| `WithReturnWriter(&w)` | Capture the underlying `*os.File` into your `io.Writer` for further writes.                      | No handle exposed                            |

```go
import (
    "io"
    "strings"

    "github.com/bizshuk/gosdk/utils"
)

// Write, creating the file if missing
err := utils.WriteFile("out/report.txt", strings.NewReader("hello"), utils.WithCreate())

// Overwrite but keep a .bak of the previous version
err = utils.WriteFile("out/report.txt", payload, utils.WithCreate(), utils.WithBackup())

// OpenFile: write incrementally yourself (remember to Close)
f, err := utils.OpenFile("out/data.bin", utils.WithCreate())
if err != nil {
    return err
}
defer f.Close()
// f.Write(...)

// Capture the writer for downstream use
var w io.Writer
err = utils.WriteFile("out/log.txt", payload, utils.WithCreate(), utils.WithReturnWriter(&w))
// w now refers to the opened file; reset to nil on any write error
```

Key behaviors:

- **Fails closed by default**: without `WithCreate()`, a missing target returns `os.ErrNotExist` — pass `WithCreate()` for create-or-overwrite semantics.
- **Always truncates**: an existing file is overwritten (`os.Create` semantics); combine with `WithBackup()` if you must preserve the prior content.
- **`SaveFile` is deprecated**: it just calls `WriteFile(..., WithCreate())` — use `WriteFile` directly.

### 4. Logging

The `log` package provides `Init()` to configure the stdlib `log/slog` global default (level + format). It reads two viper keys and calls `slog.SetDefault()`. After `log.Init()`, use the package-level `slog.*` functions directly — there is no logger object to thread around and no wrapper functions.

| Viper key    | Values (case-insensitive)           | Default |
| ------------ | ----------------------------------- | ------- |
| `LOG_LEVEL`  | `debug` / `info` / `warn` / `error` | `info`  |
| `LOG_FORMAT` | `text` / `json`                     | `text`  |

```go
import (
    "log/slog"

    "github.com/bizshuk/gosdk/config"
    "github.com/bizshuk/gosdk/log"
)

func main() {
    config.Default() // load viper settings first (LOG_LEVEL / LOG_FORMAT)
    log.Init()       // reads LOG_LEVEL + LOG_FORMAT, calls slog.SetDefault()

    // Structured logging — pass attributes as key/value pairs
    slog.Info("server started", "port", 8080)
    slog.Error("connection failed", "err", err)
    slog.Warn("retry", "attempt", attempt, "max", maxRetries)

    // Child logger with shared attributes
    reqLog := slog.With("request_id", id)
    reqLog.Info("handling request")
}
```

> [!IMPORTANT]
> **Do NOT use wrapper functions like `log.Info()`, `log.Errorf()` or printf-style formatting.** Use the package-level `slog.*` functions with key/value attributes. Call `log.Init()` again after config reloads to apply the latest `LOG_LEVEL` / `LOG_FORMAT`. Output is fixed to `os.Stdout`.

> [!NOTE]
> Migrating from zap? Use the `migrate-zap-to-slog` skill. Core mapping: `zap.L()` / `zap.S()` → package-level `slog.*`; typed fields `zap.Int("port", 8080)` → plain pairs `"port", 8080`; `zap.S().Infof("…%s", x)` → build the message or pass attrs (slog has no printf form).

### 5. Metrics & Tracing (Remote Write vs OpenTelemetry)

The SDK provides two ways to publish metrics. Depending on the complexity and needs of the project:

1. **Option A: Prometheus Remote Write** (Lightweight, developer-pushed write request — VictoriaMetrics, Mimir, or any remote-write compatible backend).
2. **Option B: OpenTelemetry OTLP** (Standardized OTel SDK for metrics and distributed tracing).

#### Option A: Remote Write (`metric.Send`)

Push time-series metrics to any Prometheus remote-write compatible backend using a lightweight HTTP writer. No MeterProvider lifecycle to manage.

> [!IMPORTANT]
> **Default to the package-level generic `metric.Send[T IMetric](metrics []T)`.** It lazily creates and reuses a global `MetricService` bound to `METRIC_URL` (default VictoriaMetrics `http://localhost:8428/api/v1/write`) and auto-batches in groups of 50. You do NOT need to construct or hold a service. Reach for an explicit `MetricService` (below) **only when you need customization** — a non-default backend URL chosen at runtime, or an injected/testable instance.

Anything that implements the `IMetric` interface can be sent. `metric.Metric` already implements it, so the simplest case is sending `[]metric.Metric` directly:

```go
import (
    "time"
    "github.com/bizshuk/gosdk/metric"
)

// Simplest: send plain Metric values (Metric implements IMetric)
_ = metric.Send([]metric.Metric{
    {
        Name:      "app.operation.duration", // "." auto-sanitized to "_"
        Timestamp: time.Now().Unix(),        // epoch SECONDS (int64), not millis
        Value:     15.4,
        Tags:      map[string]string{"env": "prod", "service": "api"},
    },
    {Name: "app.cpu.usage", Timestamp: time.Now().Unix(), Value: 42.5, Tags: map[string]string{"host": "srv1"}},
})
```

`Send your own domain type` by implementing `IMetric` (`ConvertToMetric() []metric.Metric`) — one struct can expand into several samples:

```go
type ServerStat struct {
    Host string
    CPU  float64
    Mem  float64
}

// ConvertToMetric makes ServerStat satisfy metric.IMetric.
func (s ServerStat) ConvertToMetric() []metric.Metric {
    ts := time.Now().Unix()
    return []metric.Metric{
        {Name: "app.cpu.usage", Timestamp: ts, Value: s.CPU, Tags: map[string]string{"host": s.Host}},
        {Name: "app.memory.usage", Timestamp: ts, Value: s.Mem, Tags: map[string]string{"host": s.Host}},
    }
}

// Now Send a slice of the domain type directly — no manual conversion needed
_ = metric.Send([]ServerStat{
    {Host: "srv1", CPU: 42.5, Mem: 80.0},
    {Host: "srv2", CPU: 31.0, Mem: 65.5},
})
```

`Customization (explicit service)` — only when the default global/`METRIC_URL` is not enough (e.g. a specific backend, or a service you inject for tests):

| Backend                     | Constructor                          | Config key                      | Default endpoint                     |
| --------------------------- | ------------------------------------ | ------------------------------- | ------------------------------------ |
| VictoriaMetrics (`default`) | `metric.NewVictoriaMetricsService()` | `VICTORIAMETRICS_URL`           | `http://localhost:8428/api/v1/write` |
| Mimir (compat alias)        | `metric.NewMimirService()`           | `MIMIR_URL`                     | `http://localhost:9009/api/v1/push`  |
| Any remote-write backend    | `metric.NewMetricService(url)`       | `METRIC_URL` (when `url == ""`) | `http://localhost:8428/api/v1/write` |

```go
svc := metric.NewMetricService("http://metrics.internal:9009/api/v1/push")
_ = svc.Send(metric.Metric{Name: "app.job.done", Timestamp: time.Now().Unix(), Value: 1})
_ = svc.SendMulti(metrics) // batch via an explicit instance
```

Key behaviors:

- **Default path is `metric.Send`** (package-level, `IMetric`, global service); explicit `MetricService` only for customization.
- Sanitization: `Metric.Name` replaces all `.` with `_` (Prometheus disallows dots).
- Timestamp: epoch **seconds** (`time.Now().Unix()`), NOT milliseconds.
- Auto-batching: `metric.Send` chunks into batches of 50; `SendMulti` sends one request for the whole slice.
- High-Performance: HTTP connection pooling (`MaxIdleConnsPerHost: 100`).
- Compatibility: `MimirService` is a type alias of `MetricService`; prefer `metric.Send` / `NewVictoriaMetricsService()` / `NewMetricService(url)` over the deprecated `NewMimirService()`.

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
    "log/slog"

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
    slog.Error("notify failed", "err", err)
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

- **Graceful no-op**: `NewSlackNotifier` with an empty token creates a nil client; `Notify` logs a debug message (`slog.Debug`) and returns nil without panicking. Safe to initialize unconditionally; skip-at-runtime if env vars are absent.
- **Fan-out error handling**: `Multi.Notify` always calls every registered notifier — it never short-circuits on failure. All errors are joined via `errors.Join`; check the combined error after the call.
- **Format is caller's responsibility**: The `summary string` is an opaque, pre-formatted message. Serialize your struct/report to a string before calling `Notify`.

### 7. Home Path Expansion

Use `github.com/mitchellh/go-homedir` to expand `~` in paths. Call `homedir.Expand()` directly at point of use — do NOT create a custom expand function.

```go
import "github.com/mitchellh/go-homedir"

// Standard pattern: expand and fall back silently on error
path, err := homedir.Expand("~/.config/myapp/state.db")
if err != nil {
    ...
}
```

Key rules:

- **No wrappers**: call `homedir.Expand()` inline, DO NOT wrap it in `expandPath()` / `expandHome()`
- **Silent fallback**: on error, use the original path as-is — unless the caller explicitly needs to handle the error
- **No-op when safe**: if the path has no `~` prefix, `Expand()` returns it unchanged

## Common Mistakes

| Mistake                                         | Correction                                                                                                                                                                   |
| ----------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Using `fmt.Println` or standard `log`           | Import `gosdk/log` for `Init()`, then use package-level `slog.*` (e.g. `slog.Info(msg, "key", val)`) for all logging.                                                        |
| Using removed `log.Info()` / `log.Errorf()` etc | Wrapper funcs are gone. Use `slog.Info` / `slog.Error` with key/value attrs; no printf form (`slog.Error("notify failed", "err", err)`).                                     |
| Creating a global config struct                 | Use `viper.Get*()` directly at point of use. No global struct needed — viper IS the global config store.                                                                     |
| Setting defaults before `config.Default()`      | Call `viper.SetDefault()` AFTER `config.Default()` so file-loaded values take precedence over defaults.                                                                      |
| Hardcoding `viper` keys for DB                  | Use `db.InitSQLite()` / `db.InitMySQL()` which read flat `SQLITE_PATH` / `MYSQL_DSN` from viper, open a connection, and set the corresponding `Default<Storage>` singleton.  |
| Re-implementing security headers                | Use `mw.Helmet()` instead of manually writing headers. It contains up-to-date best practices (e.g., `Permissions-Policy`, `Cross-Origin-Opener-Policy`).                     |
| Manual CSV opening and iteration                | Use `csv.ProcessCSVFile` which handles skipping headers, filtering empty rows, and `.archived` marker generation.                                                            |
| Calling `WithDefaultValue` alone                | `WithDefaultValue` only writes if using `WithAppName` to ensure it is written to the correct folder.                                                                         |
| Using `.` in metric names manually escaped      | `metric.MetricService` sanitizes `.` → `_` automatically via `sanitizeMetricName`; don't pre-mangle names.                                                                   |
| Using `NewMimirService()` in new code           | Deprecated compat alias. Use `NewVictoriaMetricsService()` (default backend) or `NewMetricService(url)`.                                                                     |
| Constructing a `MetricService` for normal sends | Default to package-level `metric.Send([]T)` (uses the global service + `METRIC_URL`, auto-batches). Build an explicit service only for a custom URL or an injected instance. |
| Manually flattening structs into `[]Metric`     | Implement `IMetric` (`ConvertToMetric() []metric.Metric`) on your domain type and pass it straight to `metric.Send`.                                                         |
| Passing milliseconds to `Metric.Timestamp`      | Field expects **seconds** (epoch); use `time.Now().Unix()`, not `UnixMilli()`.                                                                                               |
| Sending one metric at a time in tight loops     | Prefer `SendMulti` to batch samples into a single remote-write request (lower overhead, fewer HTTP round trips).                                                             |
| Forgetting to call `ShutdownOTel`               | Always `defer metric.ShutdownOTel(ctx)` at application startup to flush all buffered metrics and trace spans before application exit.                                        |
| Passing a struct directly to `Notify`           | `Notifier.Notify` only accepts a `string`. Serialize your payload (e.g., `fmt.Sprintf` or `json.Marshal`) before calling `Notify`.                                           |
| Expecting `Multi` to stop on first error        | `Multi.Notify` calls every notifier regardless of errors. Check the combined `errors.Join` error after the call — it may contain errors from multiple notifiers.             |
| Panicking when Slack token is missing           | `NewSlackNotifier("", channelID)` is intentionally a no-op; it logs a debug message and returns `nil`. No need to guard the constructor with an `if token != ""` check.      |
| Creating custom `expandPath()` / `expandHome()` | Use `homedir.Expand()` directly at call site. No wrapper function needed — it handles no-`~` paths as no-op.                                                                 |
