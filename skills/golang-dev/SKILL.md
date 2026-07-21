---
name: golang-dev
description: >
    Use when writing Go code in a gosdk-based project — building a CLI with
    cobra, configuring apps with viper, adding database connections (SQLite,
    MySQL, or PostgreSQL via gorm), structured logging with slog, test setup
    with testify, escape analysis for hot paths, or selecting between stdlib
    and third-party libraries.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuestion
user-invocable: true
disable-model-invocation: false
context: fork
---

# golang-dev

Go development best-practices guide. Covers library choices, build/test commands, and escape analysis.

## 1. CLI Development (cobra)

`When to use:` Any Go project exposing a command-line interface.

Always use `github.com/spf13/cobra`. Structure:

```tree
cmd/
  root.go      # Root command + global flags
  serve.go     # Subcommand: serve
  migrate.go   # Subcommand: migrate
main.go        # Only calls cmd.Execute()
```

### Root command pattern

```go
// cmd/root.go
package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "Short description of myapp",
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(InitConfig)
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.myapp.yaml)")
}

func InitConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, _ := os.UserHomeDir()
        viper.AddConfigPath(home)
        viper.AddConfigPath(".")
        viper.SetConfigName(".myapp")
        viper.SetConfigType("yaml")
    }
    viper.AutomaticEnv()
    _ = viper.ReadInConfig()
}
```

### Subcommand pattern

```go
// cmd/serve.go
package cmd

import (
    "fmt"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start the HTTP server",
    RunE: func(cmd *cobra.Command, args []string) error {
        port := viper.GetInt("port")
        fmt.Printf("Listening on :%d\n", port)
        // start server...
        return nil
    },
}

func init() {
    rootCmd.AddCommand(serveCmd)
    serveCmd.Flags().IntP("port", "p", 8080, "server port")
    viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
}
```

### Key rules

- Use `RunE` (not `Run`) so errors propagate instead of silently failing.
- `PersistentFlags()` for flags inherited by all subcommands; `Flags()` for command-specific.
- Bind flags to viper via `viper.BindPFlag()` in `init()` to unify flag and config access.

### CLI usage metrics (gosdk cobra hook)

If `github.com/bizshuk/gosdk` is available, record every CLI invocation by wiring the metric hook into the root command before `Execute()`:

```go
import "github.com/bizshuk/gosdk/metric"

func init() {
    metric.CobraCMDHook(rootCmd)
}
```

Each execution emits one metric via Prometheus remote-write to the backend configured by the `METRIC_URL` viper key (default: VictoriaMetrics `http://localhost:8428/api/v1/write`; override with `viper.Set("METRIC_URL", ...)`, the `APP_METRIC_URL` env var when using `config.Default()`, or `METRIC_URL` env var with `viper.AutomaticEnv()`):

```text
command_line_trigger{cmd="myapp sub leaf", flag="env-verbose"} = 1
```

- `cmd` — full command chain, root → leaf (`cmd.CommandPath()`).
- `flag` — flags the user actually set, collected across the whole chain, alphabetically sorted, joined with `-`; empty string when no flag was set.
- The hook wraps (does not replace) an existing `PersistentPreRunE` on root. If any subcommand defines its own `PersistentPreRunE`, set `cobra.EnableTraverseRunHooks = true` (once, in `init()`), otherwise cobra skips the root hook and no metric is emitted for that subcommand.

### Monitor subcommand

- `monitor` sub command is used to overall monitoring for the command, with `--merge` is one time fetch and merge into pre defined files

---

## 2. Configuration (viper)

`When to use:` Any Go project that needs configuration management.

IMPORTANT: If `github.com/bizshuk/gosdk` is available, always use its `config.Default()` for configuration loading. Fall back to raw `viper` manual setup only if the SDK is not supported or available.

### Gosdk DB connections (SQLite / MySQL / PostgreSQL)

For database access, use the `db` package. Each storage type is a service with its own global singleton and a flat `<TYPE>_<FIELD>` viper key:

```go
import "github.com/bizshuk/gosdk/db"

config.Default()

if viper.IsSet("SQLITE_PATH") {
    if err := db.InitSQLite(); err != nil { /* handle */ }
}
if viper.IsSet("MYSQL_DSN") {
    if err := db.InitMySQL(); err != nil { /* handle */ }
}
if viper.IsSet("POSTGRES_DSN") {
    if err := db.InitPostgres(); err != nil { /* handle */ }
}

// Anywhere later in the process:
gormDB := db.DefaultSQLite.DB()
defer db.DefaultSQLite.Close()
```

YAML (flat keys, uppercase underscore):

```yaml
SQLITE_PATH: ./app.db
```

Env var (with `APP_` prefix from `config.Default()`): `APP_SQLITE_PATH`, `APP_MYSQL_DSN`, `APP_POSTGRES_DSN`.

**Why a singleton per storage type:** micro-service concept forbids two of the same storage type in one process. `Init<Storage>()` refuses double-init and returns an error if called twice. The `*gorm.DB` is cached in the singleton, so any code path that needs it just reads `db.DefaultSQLite.DB()` instead of opening its own connection.

Always use `github.com/spf13/viper`. Loading precedence (highest wins):

1. `Environment variables` (`viper.AutomaticEnv()`)
2. `CLI flags` (bound via `viper.BindPFlag()`)
3. `Config file` (YAML preferred)
4. `Defaults` (`viper.SetDefault()`)

### Config struct pattern

For non-DB settings, unmarshal nested config (e.g., `server.*` blocks) into a typed struct. Database access uses the `db` package above; do not include DB fields here.

```go
type Config struct {
    Server ServerConfig `mapstructure:"server"`
}

type ServerConfig struct {
    Port         int    `mapstructure:"port"`
    ReadTimeout  int    `mapstructure:"read_timeout"`
}

func LoadConfig() (*Config, error) {
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unmarshal config: %w", err)
    }
    return &cfg, nil
}
```

### Corresponding YAML

```yaml
# .myapp.yaml
server:
    port: 8080
    read_timeout: 30
```

### Common pitfalls

- `Env prefix:` Call `viper.SetEnvPrefix("MYAPP")` so env vars like `MYAPP_SERVER_PORT` map correctly.
- `Nested keys in env:` Use `viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))` so `server.port` maps to `MYAPP_SERVER_PORT`.
- `Type mismatch:` `viper.Unmarshal` uses `mapstructure` tags, not `json` or `yaml` tags.

---

## 3. Common Libraries

| Category   | Library                       | When to use                                                                                      |
| ---------- | ----------------------------- | ------------------------------------------------------------------------------------------------ |
| Logging    | `log/slog` (stdlib)           | Structured logging. Use gosdk `log.Init()` (see Section 3.1); call package-level `slog.*` after. |
| Testing    | `github.com/stretchr/testify` | `assert` (continue on fail), `require` (stop on fail).                                           |
| Linting    | `golangci-lint`               | Meta-linter. Install: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`.   |
| HTTP       | `net/http` (stdlib)           | Prefer stdlib. Use `github.com/gin-gonic/gin` only when routing complexity justifies it.         |
| Hot Reload | `github.com/air-verse/air`    | Dev-time file watcher. Run `air` instead of `go run`.                                            |
| Mocking    | `github.com/bytedance/mockey` | Runtime monkey-patching for tests.                                                               |
| CLI        | `github.com/spf13/cobra`      | CLI scaffolding (see Section 1).                                                                 |
| Config     | `github.com/spf13/viper`      | Config management (see Section 2).                                                               |

### 3.1 Logging (slog)

`When to use:` Any Go project that needs structured logging.

IMPORTANT: If `github.com/bizshuk/gosdk` is available, always use its `log.Init()`. It wraps stdlib `log/slog` and registers the global default via `slog.SetDefault()`. Fall back to constructing a `slog.Handler` manually only if the SDK is not available.

```go
import (
    "log/slog"

    "github.com/bizshuk/gosdk/config"
    "github.com/bizshuk/gosdk/log"
)

config.Default() // load viper settings first
log.Init()       // reads LOG_LEVEL + LOG_FORMAT, calls slog.SetDefault()

// Anywhere later — no logger object to thread around:
slog.Info("server started", "port", 8080)
slog.Error("query failed", "err", err)
```

Two viper keys drive it (override with `LOG_LEVEL` / `LOG_FORMAT` env vars under `config.Default()`):

| Key          | Values (case-insensitive)           | Default |
| ------------ | ----------------------------------- | ------- |
| `LOG_LEVEL`  | `debug` / `info` / `warn` / `error` | `info`  |
| `LOG_FORMAT` | `text` / `json`                     | `text`  |

- Call `log.Init()` again after config is (re)loaded to apply the latest level/format. Output target is fixed to `os.Stdout`.
- Prefer the structured key–value form (`slog.Info(msg, "key", val)`) over interpolated strings so logs stay machine-parseable.
- Group repeated attributes with `slog.With(...)` to get a child logger: `l := slog.With("request_id", id); l.Info("...")`.

`Upgrading an older gosdk project?` There is a dedicated `golang-gosdk-migrate` skill covering zap → slog and every other superseded SDK API. Core mapping: `zap.L()`/`zap.S()` → package-level `slog.*`; `zap.String("k", v)` typed fields → plain `"k", v` pairs; `logger.Sugar().Infof(...)` → `slog.Info(msg, attrs...)` (no printf-style formatting — build the message or pass attrs).

### Manual slog fallback (no gosdk)

```go
func NewLogger(isDev bool) *slog.Logger {
    opts := &slog.HandlerOptions{Level: slog.LevelInfo}
    if isDev {
        return slog.New(slog.NewTextHandler(os.Stdout, opts))
    }
    return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
```

### testify quick pattern

```go
func TestGetUser(t *testing.T) {
    assert := assert.New(t)
    require := require.New(t)

    user, err := GetUser(ctx, "123")
    require.NoError(err)            // stops test if err != nil
    assert.Equal("Alice", user.Name) // continues even if fails
}
```

### mockey quick pattern

Use `mockey` to mock functions and methods. Always execute mocking code within `mockey.PatchConcurrently` to ensure thread safety:

```go
func TestGetUserDiscount(t *testing.T) {
    mockey.PatchConcurrently(t, func() {
        // Mock a package-level function
        mockey.Mock(FetchUserFromDB).Return(&User{ID: "123", Age: 65}, nil).Build()

        discount, err := GetUserDiscount("123")
        assert.NoError(t, err)
        assert.Equal(t, 0.2, discount)
    })
}
```

---

## 4. Build Commands

| Task                   | Command                                                                                      |
| ---------------------- | -------------------------------------------------------------------------------------------- |
| Standard build         | `go build ./...`                                                                             |
| Production binary      | `go build -ldflags="-s -w" -o bin/app ./cmd/app`                                             |
| Static binary (no CGO) | `CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/app ./cmd/app`                               |
| Version injection      | `go build -ldflags="-X main.version=1.2.3 -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"` |
| Cross compile (Linux)  | `GOOS=linux GOARCH=amd64 go build -o bin/app-linux ./cmd/app`                                |
| Race detector          | `go build -race ./...`                                                                       |

### Flag explanation

- `-s` strips symbol table, `-w` strips DWARF debug info — ~30-40% smaller binary.
- `CGO_ENABLED=0` produces a fully static binary (no libc dependency) — ideal for `scratch`/`distroless` Docker images.
- `-race` enables the race detector — use in CI but NOT in production (10x slowdown).

---

## 5. Test Commands

DO NOT create interface for pure test or coverage

| Task               | Command                                           |
| ------------------ | ------------------------------------------------- |
| Run all tests      | `go test ./...`                                   |
| Verbose            | `go test -v ./...`                                |
| Race detector      | `go test -race ./...`                             |
| Coverage           | `go test -cover -coverprofile=coverage.out ./...` |
| View coverage HTML | `go tool cover -html=coverage.out`                |
| Specific test      | `go test -run TestFunctionName ./pkg/...`         |
| Benchmarks         | `go test -bench=. -benchmem -run=^$ ./...`        |
| Short mode         | `go test -short ./...`                            |
| Disable caching    | `go test -count=1 ./...`                          |
| Timeout            | `go test -timeout 30s ./...`                      |
| Disable inlining   | `go test -gcflags="all=-N -l" ./...`              |

### Disabling inlining and optimizations

```bash
go test -gcflags="all=-N -l" ./...
```

- `-N` disables all compiler optimizations.
- `-l` disables inlining (function calls remain as actual calls, not inlined).
- `all=` applies the flags to `all packages` being compiled, not just the test package. Without `all=`, only the direct test target gets the flags — dependencies may still be inlined.

`Use cases:`

- Debugging with `dlv` (delve) — requires non-inlined frames for accurate breakpoints and variable inspection.
- Mocking and patching — libraries like `mockey` (or other monkey-patching libraries) rewrite function machine code at runtime. If a target function is inlined or optimized, the patch will fail because there is no independent function entry point to rewrite.
- Accurate escape analysis — inlining can change escape decisions, so disable it to see "true" escape behavior.
- Diagnosing bugs that only manifest with/without compiler optimizations.

---

## 6. Escape Analysis

`When to use:` Investigating heap allocations, optimizing memory-sensitive hot paths.

### Commands

```bash
# Basic: shows escape decisions
go build -gcflags='-m' ./...

# Detailed: shows reasoning for each decision
go build -gcflags='-m=2' ./...

# Specific package
go build -gcflags='-m=2' ./pkg/handler/...

# Filter for escapes only
go build -gcflags='-m=2' ./... 2>&1 | grep "escapes to heap"
```

### Common escape reasons

| Output                                | Cause                                    | Fix                                                |
| ------------------------------------- | ---------------------------------------- | -------------------------------------------------- |
| `leaking param: x`                    | Param stored beyond function scope       | Avoid storing pointer params in long-lived structs |
| `moved to heap: too large`            | Stack frame exceeds ~10MB                | Break into smaller allocations or use `sync.Pool`  |
| `moved to heap: captured by closure`  | Variable captured in closure             | Pass value explicitly as parameter                 |
| `moved to heap: interface conversion` | Concrete assigned to `any`/`interface{}` | Use concrete types in hot paths                    |
| `&x escapes to heap`                  | Returning address of local               | Return value instead of pointer when possible      |

### Verification workflow

1. Run `go build -gcflags='-m=2' ./... 2>&1 | grep "escapes to heap"` to find escaping allocations.
2. Focus on `hot paths` — handlers, loops, frequently called functions. Don't optimize cold code.
3. Run `go test -bench=. -benchmem` to measure allocs/op before and after changes.
4. Apply fix, re-run escape analysis to confirm the allocation moved to stack.
5. Re-run benchmarks to verify measurable improvement.

`Reference:` Stack allocation ~0.26ns vs heap ~10.55ns — ~40x penalty per escaped allocation.

---

## 7. Quick Reference

| Task             | Command                                                        |
| ---------------- | -------------------------------------------------------------- |
| Build (dev)      | `go build ./...`                                               |
| Build (prod)     | `go build -ldflags="-s -w" -o bin/app ./cmd/app`               |
| Build (static)   | `CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/app ./cmd/app` |
| Test             | `go test ./...`                                                |
| Test (race)      | `go test -race ./...`                                          |
| Test (cover)     | `go test -cover -coverprofile=coverage.out ./...`              |
| Test (no inline) | `go test -gcflags="all=-N -l" ./...`                           |
| Test (bench)     | `go test -bench=. -benchmem -run=^$ ./...`                     |
| Escape analysis  | `go build -gcflags='-m=2' ./...`                               |
| Lint             | `golangci-lint run ./...`                                      |
| Vet              | `go vet ./...`                                                 |
| Format           | `gofmt -w .` or `goimports -w .`                               |
| Hot reload       | `air`                                                          |
| Cross compile    | `GOOS=linux GOARCH=amd64 go build -o bin/app ./cmd/app`        |
