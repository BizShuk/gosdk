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

Configuration is globally managed via `viper` and automatically loads from `.env`, `config.<profile>.yaml`, and `settings.json` based on the configuration path and functional options.

```go
import (
    "github.com/bizshuk/gosdk/config"
    "github.com/bizshuk/gosdk/config/db"
    "github.com/bizshuk/gosdk/log"
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

    // 3. (Optional) Initialize DB
    if len(config.GlobalConfig.DB) > 0 {
        gormDB, err := db.NewDBConfig("default").Create()
        if err != nil {
            log.Fatalf("DB connect failed: %v", err)
        }
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

### 5. Metrics (Mimir / Prometheus Remote Write)

Push time-series metrics to Mimir via the `metric` package. The endpoint is read from the `MIMIR_URL` viper key (default `http://localhost:9009/api/v1/push`).

```go
import (
    "time"
    "github.com/bizshuk/gosdk/metric"
)

svc := metric.NewMimirService()

// Single metric
_ = svc.Send(metric.Metric{
    Name:      "stock.analysis.latency", // dots are auto-converted to underscores
    Timestamp: time.Now().Unix(),
    Value:     12.5,
    Tags:      map[string]string{"host": "worker-1", "project": "stock"},
})

// Batch (preferred for throughput — single remote-write request)
_ = svc.SendMulti([]metric.Metric{ /* ... */ })
```

Key behaviors:

- `Metric.Name` is sanitized via `strings.ReplaceAll(name, ".", "_")` — Prometheus disallows `.` in metric names.
- `Tags` map becomes Prometheus labels (`__name__` is reserved and set from `Name`).
- `Timestamp` is **seconds** since epoch (`int64`), converted internally via `time.Unix(ts, 0)`.
- Each `SendMulti` call uses a 30s context timeout; the HTTP client reuses idle connections (`MaxIdleConnsPerHost: 100`).
- `SendTest()` is a debugging helper that emits 7 fake samples spaced 10 minutes apart — handy for verifying the pipeline end-to-end.

## Common Mistakes

| Mistake                               | Correction                                                                                                                                               |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Using `fmt.Println` or standard `log` | Always use `github.com/bizshuk/gosdk/log` to ensure JSON formatting in production and consistent log levels.                                             |
| Hardcoding `viper` keys for DB        | Use `db.NewDBConfig("connectionName").Create()` which encapsulates the dialect selection and connection string logic.                                    |
| Re-implementing security headers      | Use `mw.Helmet()` instead of manually writing headers. It contains up-to-date best practices (e.g., `Permissions-Policy`, `Cross-Origin-Opener-Policy`). |
| Manual CSV opening and iteration      | Use `csv.ProcessCSVFile` which handles skipping headers, filtering empty rows, and `.archived` marker generation.                                        |
| Calling `WithDefaultValue` alone      | `WithDefaultValue` only writes if using `WithAppName` to ensure it is written to the correct folder. |
| Using `.` in Mimir metric names manually escaped | `metric.MimirService` sanitizes `.` → `_` automatically via `sanitizeMetricName`; don't pre-mangle names. |
| Passing milliseconds to `Metric.Timestamp` | Field expects **seconds** (epoch); use `time.Now().Unix()`, not `UnixMilli()`. |
| Sending one metric at a time in tight loops | Prefer `SendMulti` to batch samples into a single remote-write request (lower overhead, fewer HTTP round trips). |
