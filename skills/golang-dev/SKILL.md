---
name: golang-dev
description: >
    Use when writing Go code in a gosdk-based project — building a CLI with
    cobra, configuring apps with viper, adding database connections (SQLite,
    MySQL, or PostgreSQL via gorm), structured logging with slog, test setup
    with testify, escape analysis for hot paths, wiring a registry with
    init() self-registration for pluggable providers/drivers, or selecting
    between stdlib and third-party libraries.
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
  monitor.go   # Subcommand: monitor (alias m)
  logs.go      # Subcommand: logs    (alias log)
  list.go      # Subcommand: list    (alias l)
  ps.go        # Subcommand: ps      (only if long-lived processes exist)
  web.go       # Subcommand: web
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

// RootCmd is exported so subcommand files (and the SDK's ready-made commands)
// attach to it from their own init().
var RootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "Short description of myapp",
}

func Execute() {
    if err := RootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(InitConfig)
    RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.myapp.yaml)")
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

// WebCmd — one file per command, file name matches the command name.
var WebCmd = &cobra.Command{
    Use:   "web",
    Short: "Start the HTTP server",
    RunE: func(cmd *cobra.Command, args []string) error {
        port := viper.GetInt("port")
        fmt.Printf("Listening on :%d\n", port)
        // start server...
        return nil
    },
}

func init() {
    RootCmd.AddCommand(WebCmd)
    WebCmd.Flags().IntP("port", "p", 8080, "server port")
    viper.BindPFlag("port", WebCmd.Flags().Lookup("port"))
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
    metric.CobraCMDHook(RootCmd)
}
```

Each execution emits one metric via Prometheus remote-write to the backend configured by the `METRIC_URL` viper key (default: VictoriaMetrics `http://localhost:8428/api/v1/write`; override with `viper.Set("METRIC_URL", ...)`, the `APP_METRIC_URL` env var when using `config.Default()`, or `METRIC_URL` env var with `viper.AutomaticEnv()`):

```text
command_line_trigger{cmd="myapp sub leaf", flag="env-verbose"} = 1
```

- `cmd` — full command chain, root → leaf (`cmd.CommandPath()`).
- `flag` — flags the user actually set, collected across the whole chain, alphabetically sorted, joined with `-`; empty string when no flag was set.
- The hook wraps (does not replace) an existing `PersistentPreRunE` on root. If any subcommand defines its own `PersistentPreRunE`, set `cobra.EnableTraverseRunHooks = true` (once, in `init()`), otherwise cobra skips the root hook and no metric is emitted for that subcommand.

### Standard subcommand vocabulary

Every CLI built on gosdk uses the `same name for the same job`, so a user who learns one binary already knows the next. Only implement the ones the app actually has — but when the job exists, it gets this name and this alias, never a synonym (`status`, `tail`, `dashboard`, `serve`, `top` are all wrong).

| Command   | Alias | Kind        | Purpose                                                                        |
| --------- | ----- | ----------- | ------------------------------------------------------------------------------ |
| `monitor` | `m`   | interactive | TUI for viewing and operating on live state                                    |
| `logs`    | `log` | streaming   | Recent process log; `--process` switches to recent processes (default last 20) |
| `list`    | `l`   | snapshot    | One-shot dump of current process status / stats, then exit                     |
| `ps`      | —     | snapshot    | Live OS process table with PIDs — only when the app owns long-lived processes  |
| `config`  | —     | snapshot    | Show and modify configuration — register gosdk's `cmd.ConfigCmd`               |
| `web`     | —     | server      | Start the web server                                                           |

```go
// cmd/monitor.go
package cmd

import "github.com/spf13/cobra"

// MonitorCmd is the interactive view; every other command prints once and exits.
var MonitorCmd = &cobra.Command{
    Use:     "monitor",
    Aliases: []string{"m"},
    Short:   "Interactive TUI for live process state",
    RunE: func(cmd *cobra.Command, args []string) error {
        return monitor.Run(cmd.Context())
    },
}

func init() {
    RootCmd.AddCommand(MonitorCmd)
}
```

`Key rules:`

- `monitor` is the `only` interactive command. It owns the TUI (alt screen, key handling, refresh loop) and is the only place a command may take over the terminal or accept an operation on a running process. Everything else writes to stdout and exits so it stays pipeable.
- `logs` vs `list` is `time-series vs state`: `logs` answers "what happened", `list` answers "what is true right now". Never merge them into one command with a flag.
- `logs --process` changes the `unit` of the listing, not the format: without it you get the merged recent log across processes, with it you get the recent `processes` themselves. Default limit is `20` for both, overridable with `-n/--limit`.
- `list` must terminate. No watch loop, no follow — that is `monitor`'s job.
- `ps` exists `only` if the binary spawns or supervises long-lived OS processes that have a real `PID` — a daemon, a worker pool, a pm2-style supervisor. A CLI that just runs and exits must not ship a `ps`. It prints one row per live process, `PID first`, then the columns that let a user act on it: name, state, uptime, and resource use when cheap to obtain. It reports only; killing or restarting belongs to `monitor`.
- `ps` vs `list` vs `logs --process` — three different objects, so three different commands: `ps` lists `OS processes` (PID-keyed, live now), `list` lists `domain state` (jobs, tasks, entities the app manages), `logs --process` lists `log records grouped by process` (historical, may include processes that already exited). Do not fold `ps` into `list` behind a flag.
- `ps` has no alias — the name is already two characters, and every alias added here collides with muscle memory from the system `ps`.
- Tabular output for `ps` and `list` goes through gosdk's `tui.Table` (`Draw(w, hasTotalRow, highlightLastCol)`), which handles Unicode borders, multi-line cells and ANSI-aware column widths. Do not hand-roll padding with `text/tabwriter` — it counts escape sequences as visible width and misaligns any coloured column.
- `config` is not hand-rolled: `root.AddCommand(cmd.ConfigCmd)` from `github.com/bizshuk/gosdk/cmd` gives the merged view, provenance and `--update/--add/--delete` for free.
- `web` only serves. Migrations, seeding and one-off jobs are their own commands — a server start must be safe to run repeatedly.
- Aliases are `single letters for the interactive/snapshot pair` (`m`, `l`) and the natural singular for `logs` (`log`). Do not invent extra aliases; ambiguity across binaries costs more than the keystrokes saved.

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

## 7. Registry + init() Self-Registration

`When to use:` one interface has `N` interchangeable implementations (providers,
drivers, formats, plugins) and adding one should mean `adding one file` — no
edits to an existing `switch` or map literal.

`When NOT to use:` only `1`–`2` implementations, or the caller already knows the
concrete type. A plain constructor is clearer; a registry is just indirection.

This is the `database/sql/driver` / `image` / `crypto` convention: a `registry`
package owns the `name → Factory` map, implementations register themselves from
their own `init()`, and the registry imports `no` implementation. Dependency
direction is always `impl → registry`, never reversed — which is why there is no
import cycle and why the registry never changes when an implementation is added.

### Layout

```tree
registry/            # depends only on the interface; imports no implementation
  registry.go        # Entry, Register, Lookup, Names, New
impl/<name>/
  <name>.go          # the implementation + New(...Option)
  register.go        # exactly one init(), nothing else
impl/all/all.go      # blank-imports every implementation (optional)
```

### Registry core

```go
package registry

// Factory builds one implementation from resolved options.
type Factory func(Options) (Thing, error)

// Entry is how to build it, plus metadata a `--list` or wizard menu needs.
type Entry struct {
    Name  string // canonical, lower-case, matches the directory name
    Label string // human-facing
    New   Factory
}

var (
    mu      sync.RWMutex
    entries = map[string]Entry{}
)

// Register panics on a duplicate name — idiomatic Go for an init()-time
// contract violation (see database/sql.Register). init() has nowhere to
// receive an error, so returning one would only get ignored.
func Register(e Entry) {
    if e.Name == "" || e.New == nil {
        panic(fmt.Sprintf("registry: Register requires Name and New (got %+v)", e))
    }
    key := strings.ToLower(strings.TrimSpace(e.Name))
    mu.Lock()
    defer mu.Unlock()
    if _, exists := entries[key]; exists {
        panic(fmt.Sprintf("registry: %q already registered", e.Name))
    }
    entries[key] = e
}

// New builds the named thing. The error lists what IS registered — the single
// most useful line when a blank import is missing.
func New(name string, o Options) (Thing, error) {
    e, ok := Lookup(name)
    if !ok {
        return nil, fmt.Errorf("registry: unknown name %q (registered: %s)",
            name, strings.Join(Names(), ", "))
    }
    return e.New(o)
}
```

### Self-registration: one file, one init()

Keep registration in its own `register.go`, never mixed into the implementation
file — the side effect stays visible, and splitting the package into its own
module later moves exactly one file.

```go
// impl/minimax/register.go
package minimax

import "example.com/proj/registry"

func init() {
    registry.Register(registry.Entry{
        Name:  "minimax",
        Label: "MiniMax",
        New: func(o registry.Options) (registry.Thing, error) {
            var opts []Option
            if o.Model != "" {
                opts = append(opts, WithModel(o.Model))
            }
            return New(opts...) // the package's own functional options
        },
    })
}
```

### Key rules

- The Factory is an `adapter`: it translates the registry's flat `Options` into
  the implementation's own functional options. The implementation's public API
  does not change because a registry exists — `minimax.New(WithModel(...))`
  still works without one.
- Never pass an empty string through; let the implementation apply its own
  default. A zero-value `Options` should still build a usable object.
- `init()` does no I/O — no file reads, no network, no credential resolution.
  It inserts one map entry in constant time, because `every` binary that
  blank-imports the package pays that cost.
- The registered set is a property of the `linking binary`, not the registry
  package: a full CLI blank-imports `impl/all`, a slim binary imports only the
  one it needs and the linker drops the rest. Put blank imports in `main.go` or
  the composition root only — a library that blank-imports `all` forces every
  downstream consumer to swallow all dependencies.
- The registry must not import a config framework. If it needs an env lookup,
  inject it as an `Options` field (`LookupEnv func(string) string`, `nil` means
  `os.Getenv`); the CLI passes a viper-backed function.

`Full sample:` [references/registry-pattern.md](references/registry-pattern.md) —
complete registry package, `Options.Resolve` credential precedence, the `all/`
meta-package, four guard tests, pitfall table, and a per-implementation checklist.

---

## 8. Quick Reference

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
