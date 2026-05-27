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

## Quick Reference & Common Patterns

### 1. Initialization & Configuration

Configuration is globally managed via `viper` and automatically loads from `.env` and `config.<profile>.yaml` based on the `PROFILE` environment variable (defaults to `local`).

```go
import (
 "github.com/bizshuk/gosdk/config"
 "github.com/bizshuk/gosdk/config/db"
 "github.com/bizshuk/gosdk/log"
)

func main() {
 // 1. Load config (merges .env and yaml)
 config.Default()

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

## Common Mistakes

| Mistake                               | Correction                                                                                                                                               |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Using `fmt.Println` or standard `log` | Always use `github.com/bizshuk/gosdk/log` to ensure JSON formatting in production and consistent log levels.                                             |
| Hardcoding `viper` keys for DB        | Use `db.NewDBConfig("connectionName").Create()` which encapsulates the dialect selection and connection string logic.                                    |
| Re-implementing security headers      | Use `mw.Helmet()` instead of manually writing headers. It contains up-to-date best practices (e.g., `Permissions-Policy`, `Cross-Origin-Opener-Policy`). |
| Manual CSV opening and iteration      | Use `csv.ProcessCSVFile` which handles skipping headers, filtering empty rows, and `.archived` marker generation.                                        |
