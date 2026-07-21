---
name: gosdk-migrate
description: >-
    Use when upgrading a Go project to current github.com/bizshuk/gosdk
    conventions, or when it still uses an API the SDK has since replaced.
    Covers the whole migration catalog: zap → log/slog, ConfigSchema structs →
    flat viper keys, PROFILE switching → dual-file (base + .local) loading,
    nested `db:` blocks → flat SQLITE_PATH / MYSQL_DSN / POSTGRES_DSN,
    removed log wrapper funcs → package-level slog, cobra
    `NewXxxCmd()` constructors → package-level exported vars + init(),
    standalone versioning CLI → gosdk/cmd subcommands, and
    MimirService → MetricService. Always loads the golang-gosdk skill first
    for the canonical target-state API. Triggers: "upgrade gosdk", "migrate to
    gosdk", "update to the latest gosdk", "gosdk 升級", "migrate zap to slog",
    or a build breaking after a gosdk bump.
argument-hint: "[path]"
allowed-tools: Bash, Read, Edit, Grep, Glob
effort: medium
context: fork
disable-model-invocation: false
user-invocable: true
---

# gosdk-migrate

Upgrade a Go project to the current `github.com/bizshuk/gosdk` conventions.
The SDK has replaced several APIs over time; this skill detects which of them
a project still uses, applies each migration, and verifies the result.

## Step 0 (required): load `golang-gosdk`

**Before touching any code, invoke the `golang-gosdk` skill.** It is the
canonical reference for the *target state* — config loading and key format,
logging, HTTP, metrics, notifications, and the `gosdk/cmd` subcommand catalog.
This skill only describes how to get *from* the old API *to* that target; it
deliberately does not duplicate the target-state documentation.

```text
Skill(golang-gosdk)   →  what the code should look like
Skill(gosdk-migrate)  →  how to get there from what it looks like now
```

## Migration catalog

Run the Detect step of each migration. Skip the ones that return nothing.

| # | Migration | Detect with |
| - | --------- | ----------- |
| 1 | `go.uber.org/zap` → `log/slog` | `grep -rn "go.uber.org/zap" --include="*.go" .` |
| 2 | `log.Info()` / `log.Errorf()` wrappers → package-level `slog.*` | `grep -rn "gosdk/log\"" -A20 --include="*.go" . \| grep -E "log\.(Info\|Warn\|Error\|Debug)f?\("` |
| 3 | `ConfigSchema` / `ServerConfig` / `DBConfig` structs → flat viper keys | `grep -rn "config/common\|ConfigSchema" --include="*.go" .` |
| 4 | `PROFILE` env switching → dual-file (base + `.local`) | `grep -rn "PROFILE" --include="*.go" --include="*.env*" .` |
| 5 | Nested `db:` yaml block → flat `SQLITE_PATH` / `MYSQL_DSN` / `POSTGRES_DSN` | `grep -rn "^db:" --include="*.yaml" .` |
| 6 | `APP_X_Y` assumed to override `x.y` | `grep -rn "APP_[A-Z_]*" --include="*.md" --include="*.go" .` |
| 7 | `func NewXxxCmd() *cobra.Command` → package-level exported var + `init()` | `grep -rn "func New.*Cmd() \*cobra.Command" --include="*.go" .` |
| 8 | Standalone versioning CLI → `gosdk/cmd` subcommands | `grep -rn "gosdk/cmd/versioning" --include="*.go" .` |
| 9 | `MimirService` / `NewMimirService()` → `MetricService` | `grep -rn "MimirService" --include="*.go" .` |

Migration 1 is the largest and is documented in full below. Migrations 2–9 are
each a short, mechanical change and are documented after it.

## When NOT to use

- The project does not depend on `github.com/bizshuk/gosdk` at all. Use
  `golang-dev` for general Go work.
- Only new code is being written against a current SDK — no migration needed;
  use `golang-gosdk` directly.
- A migration must be deferred for downstream compatibility (e.g. other
  modules import the old log wrappers). Migrate the internals only and keep
  the old surface as a thin shim; split the work into two phases.

---

# Migration 1: `go.uber.org/zap` → `log/slog`

Four phases — Detect, Migrate the log initializer, Migrate call sites, Verify.

## Phase 1: Detect

Run these checks first to scope the work:

```bash
# Files importing zap
grep -rln "go.uber.org/zap" --include="*.go" .

# Direct call sites (zap.L() / zap.S() / zap.Sugar() / zap.NewXxx)
grep -rn "zap\." --include="*.go" . | grep -v "_test.go"
grep -rn "zap\." --include="*_test.go" .
```

If either command returns nothing, the project does not use zap. Stop.

Record the count of call sites and the list of files. This becomes the
checklist for Phase 3.

## Phase 2: Migrate the log initializer

A zap project typically has one `log.Init()` function (or `init()`) that
calls `zap.NewProductionConfig()` and `zap.ReplaceGlobals(logger)`. Replace
it with a slog-based initializer that reads `LOG_LEVEL` and `LOG_FORMAT`
from viper (or any config source the project uses).

Reference implementation:

```go
package log

import (
    "log/slog"
    "os"
    "strings"

    "github.com/spf13/viper"
)

func init() { Init() }

func Init() {
    level := GetLogLevel()
    format := parseFormat(viper.GetString("LOG_FORMAT"))

    opts := &slog.HandlerOptions{Level: level}

    var handler slog.Handler
    switch format {
    case "json":
        handler = slog.NewJSONHandler(os.Stdout, opts)
    default:
        handler = slog.NewTextHandler(os.Stdout, opts)
    }
    slog.SetDefault(slog.New(handler))
}

func GetLogLevel() slog.Level { return parseLevel(viper.GetString("LOG_LEVEL")) }

func parseLevel(s string) slog.Level {
    switch strings.ToLower(strings.TrimSpace(s)) {
    case "debug":
        return slog.LevelDebug
    case "warn", "warning":
        return slog.LevelWarn
    case "error":
        return slog.LevelError
    default:
        return slog.LevelInfo
    }
}

func parseFormat(s string) string {
    if strings.ToLower(strings.TrimSpace(s)) == "json" {
        return "json"
    }
    return "text"
}
```

Defaults: `LOG_LEVEL=info`, `LOG_FORMAT=text`. Empty / unset values
fall back to defaults. Case-insensitive.

## Phase 3: Migrate call sites

Apply the mapping table to every file from Phase 1. Use `Edit` (not
`sed`) so you can see the diff and avoid mass-replacement mistakes in
comments or strings.

| zap                                      | slog                                          |
| ---------------------------------------- | --------------------------------------------- |
| `zap.L().Info("m", zap.String("k", v))`  | `slog.Info("m", "k", v)`                      |
| `zap.L().Info("m", zap.Int("n", 1))`     | `slog.Info("m", "n", 1)`                      |
| `zap.L().Info("m", zap.Any("o", obj))`   | `slog.Info("m", "o", obj)`                    |
| `zap.L().Error("m", zap.Error(err))`     | `slog.Error("m", "err", err)`                 |
| `zap.S().Info("m")`                      | `slog.Info("m")`                              |
| `zap.S().Infof("Server %s", addr)`       | `slog.Info("Server starting", "addr", addr)`  |
| `zap.S().Errorf("err: %v", err)`         | `slog.Error("operation failed", "err", err)`  |
| `zap.S().Warnf("warn: %s", msg)`         | `slog.Warn("warn", "msg", msg)`               |
| `zap.L().Sugar().Errorf("warn %v", err)` | `slog.Warn("message", "err", err)`            |
| `zap.L().Core().Enabled(level)`          | `slog.Default().Enabled(ctx, level)`          |
| `zap.S().Fatal("msg")`                   | `slog.Error("msg"); os.Exit(1)`               |
| `zap.S().Fatalf("err: %v", err)`         | `slog.Error("fatal", "err", err); os.Exit(1)` |

### Special cases

**Fatal**: slog has no `Fatal` level. Replace each `zap.Fatal*` call with
`slog.Error(...)` followed by `os.Exit(1)`. Add `"os"` to the file's
imports if it is not already present.

**Sugar**: `zap.S()` (and `zap.L().Sugar()`) return a sugar logger with
printf-style methods (`Infof`, `Errorf`). slog has no direct equivalent.
Rewrite each call to use structured key-value pairs:

- `zap.S().Infof("Server starting on %s", addr)` →
  `slog.Info("Server starting", "addr", addr)`
- `zap.S().Errorf("connection failed: %v", err)` →
  `slog.Error("connection failed", "err", err)`

**Logger construction in tests**:

- `logger, _ := zap.NewDevelopment(); zap.ReplaceGlobals(logger)` →
  `slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))`
- `logger, _ := zap.NewProduction(); zap.ReplaceGlobals(logger)` →
  `slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))`

**Imports**: remove `go.uber.org/zap` and (if unused) `go.uber.org/zap/zapcore`.
Add `"log/slog"` and (if `Fatal` was migrated) `"os"`.

## Phase 4: Verify

Run all four checks. A failure in any one means the migration is incomplete.

```bash
# 1. No zap references remain
grep -rn "go.uber.org/zap" --include="*.go" .    # must be empty
grep -rn "zap\." --include="*.go" .              # must be empty (excluding go.mod)

# 2. Static analysis clean
go vet ./...

# 3. Build succeeds
go build ./...

# 4. Tests pass
go test ./...
```

Then do a runtime check that LOG_LEVEL and LOG_FORMAT actually work:

```go
package main

import (
    "context"
    "log/slog"
    "github.com/bizshuk/gosdk/log"   // or your project's log package
    "github.com/spf13/viper"
)

func main() {
    viper.Set("LOG_LEVEL", "debug")
    viper.Set("LOG_FORMAT", "json")
    log.Init()
    slog.Info("hello", "k", "v")
    _ = context.Background() // for slog.Default().Enabled(ctx, ...)
}
```

Run with `LOG_LEVEL=debug LOG_FORMAT=json` and confirm:

- `slog.Default().Enabled(ctx, slog.LevelDebug)` returns `true`
- Output is a single line of valid JSON per log call
- Switching to `LOG_LEVEL=error` filters out Info/Debug/Warn calls

## Quick reference (Migration 1)

For projects where the call sites use only `Info`/`Warn`/`Error` and no
custom fields, the migration can be done with a single sed pass:

```bash
files=$(grep -rln "zap\.\(Info\|Warn\|Error\|Debug\)" --include="*.go" .)
for f in $files; do
  sed -i '' 's/zap\.L()\.Sugar()/slog/g; s/zap\.L()/slog.Default()/g; s/zap\.S()/slog/g' "$f"
  sed -i '' 's/\.Infof(/.Info(/g; s/\.Errorf(/.Error(/g; s/\.Warnf(/.Warn(/g; s/\.Debugf(/.Debug(/g; s/\.Fatalf(/.Error(/g; s/\.Fatal(/.Error(/g' "$f"
  sed -i '' 's/zap\.String(/slog.String(/g; s/zap\.Int(/slog.Int(/g; s/zap\.Error(/slog.Any(/g; s/zap\.Any(/slog.Any(/g' "$f"
done
```

After the sed pass, `go vet ./...` will report any places where a `slog.String`
or `slog.Any` was used as a positional key (because slog takes variadic
key-value pairs, not typed fields). Fix those by hand:

```go
// Before sed
zap.L().Info("save", zap.String("file", path))

// After sed (broken)
slog.Default().Info("save", slog.String("file", path))  // slog.String returns Attr, not key

// Fix
slog.Default().Info("save", "file", path)
```

For projects with many custom field types, prefer file-by-file `Edit` over
bulk sed.

## Common mistakes

1. **Leaving `zap.L().Core().Enabled(level)`** — this compiles only if zap
   is still imported. Replace with
   `slog.Default().Enabled(ctx, slog.LevelDebug)`.

2. **Forgetting `os.Exit(1)` after `zap.Fatal*` removal** — `zap.Fatal`
   calls `os.Exit(1)` automatically; slog has no such level. If you only
   write `slog.Error("...")`, the program will continue running after
   what should be a fatal condition.

3. **Type mismatches in `slog` field helpers** — `slog.String("k", v)`
   returns a `slog.Attr`, not a key-value pair. For variadic logging,
   use bare `"k", v` pairs. Reserve `slog.Attr` for `slog.LogAttrs(ctx,
level, msg, attrs...)` calls.

4. **`slog.Default()` instead of `slog.Info(...)` package-level** —
   `slog.Info(...)` is the package-level function that calls
   `slog.Default().Info(...)`. Use the package-level form for less
   noise unless you need a custom logger.

5. **Output target** — slog handlers default to `os.Stderr` in some
   tutorials, but `zap.NewProductionConfig()` writes to `os.Stdout`. Match
   the project's existing target by passing the right `io.Writer` to
   `slog.NewTextHandler` / `slog.NewJSONHandler`.

6. **Pre-existing test failures** — if `go test` fails on tests that did
   not use zap (e.g. global-state test ordering bugs), those are out of
   scope. Document them in the verification report and let the user
   decide whether to fix them in the same PR.

---

# Migrations 2–9

Each is mechanical. Apply only the ones whose Detect step found matches, then
run the shared verification at the end.

## Migration 2: log wrapper funcs → package-level `slog.*`

The SDK's `log` package no longer exports `Info()` / `Warnf()` / `Errorf()`
wrappers. `log.Init()` remains — it reads `LOG_LEVEL` / `LOG_FORMAT` and calls
`slog.SetDefault()`. Everything else moves to stdlib.

```go
log.Info("started")                  →  slog.Info("started")
log.Errorf("failed: %v", err)        →  slog.Error("failed", "err", err)
log.Infof("port %d", port)           →  slog.Info("listening", "port", port)
```

Keep the `gosdk/log` import only if the file calls `log.Init()`; otherwise drop
it and import `log/slog`.

## Migration 3: `ConfigSchema` structs → flat viper keys

`config/common` and its aggregate structs (`ConfigSchema`, `ServerConfig`,
`DBConfig`) were removed. Read values straight from viper.

```go
// Before
cfg := common.GetConfig()
port := cfg.Server.Port

// After
port := viper.GetInt("server.port")
```

If the project genuinely needs a typed struct, keep it **local to the consuming
package** and fill it with `viper.Unmarshal` / `viper.UnmarshalKey` — do not
reintroduce a shared global schema.

## Migration 4: `PROFILE` switching → dual-file loading

`config.Default()` no longer reads a `PROFILE` env var to pick a config file.
Every format now loads a fixed pair — base first, then `.local` on top:

| Format | Base | Override |
| ------ | ---- | -------- |
| dotenv | `.env` | `.env.local` |
| yaml | `config.yaml` | `config.local.yaml` |
| json | `settings.json` | `settings.local.json` |

Rename `config.prod.yaml` / `config.dev.yaml` accordingly: the shared values go
in the base file, the machine-specific ones in `.local` (and `.local` belongs in
`.gitignore`). Delete every `PROFILE=` line and any code reading it.

## Migration 5: nested `db:` block → flat storage keys

Each storage type is now an independent service with its own flat key and its
own guarded initializer.

```yaml
# Before                        # After
db:                             SQLITE_PATH: ./app.db
  driver: sqlite                MYSQL_DSN: user:pass@tcp(host:3306)/db
  path: ./app.db                POSTGRES_DSN: postgres://user:pass@host:5432/db
```

```go
// Before                            // After
db.Connect(cfg.DB)                   db.InitSQLite()      // then db.DefaultSQLite.DB()
```

`InitSQLite()` / `InitMySQL()` / `InitPostgres()` refuse a second call, enforcing
one instance per storage type. MySQL and PostgreSQL take a single DSN string —
do not split it back into host/port/user/password keys.

## Migration 6: `APP_` env var key assumptions

This one is a **correctness fix, not a rename**. `config.Default()` installs no
`SetEnvKeyReplacer`, so `APP_SERVER_PORT` maps to the flat key `server_port` —
it has never overridden the nested key `server.port`.

Audit every knob documented or intended as env-overridable:

- If it must be settable from the environment, declare it as a **flat
  SCREAMING_SNAKE key** (`SERVER_PORT`, `LOG_LEVEL`, `SQLITE_PATH`) and read it
  as such.
- If it is nested (`server.port`), accept that only config files can set it, and
  correct any README/comment claiming otherwise.

See the Key Format table in `golang-gosdk` for the full matrix.

## Migration 7: cobra constructors → package-level vars

```go
// Before
func NewDeployCmd() *cobra.Command {
    cmd := &cobra.Command{Use: "deploy", ...}
    cmd.Flags().String("region", "us-west", "deployment region")
    return cmd
}

// After
var DeployCmd = &cobra.Command{Use: "deploy", ...}

func init() {
    DeployCmd.Flags().String("region", "us-west", "deployment region")
}
```

Rules: commands are **exported** vars; flags bind in `init()` into package-level
vars; one file per command named after it (`deploy.go`); sub-subcommands take a
prefix (`deployLocal.go`); root commands stay in the executable's `main.go`.

> [!WARNING]
> A package-level command keeps parsed flag state across `Execute()` calls in one
> process — pflag's slice values append on a second `Set`. Any test that runs the
> command more than once must reset the bound vars to `nil` and clear `f.Changed`
> via `Flags().VisitAll`.

## Migration 8: standalone versioning CLI → `gosdk/cmd`

The `cmd/versioning` executable was removed. Its subcommands and the `VERSION`
file helpers now live in `github.com/bizshuk/gosdk/cmd`.

```go
// Before
import "github.com/bizshuk/gosdk/cmd/versioning/cmd"
cmd.Execute()

// After
import "github.com/bizshuk/gosdk/cmd"

func init() {
    RootCmd.AddCommand(cmd.MajorCmd, cmd.MinorCmd, cmd.PatchCmd)
}
```

`cmd.Version`, `ParseVersion()`, `ReadVersion()` and `WriteVersion()` are exported
too, for scripts that need the file without the CLI. Replace any CI step that ran
`go run ./cmd/versioning patch` with the host application's own binary.

## Migration 9: `MimirService` → `MetricService`

`MimirService` / `NewMimirService()` still compile but are deprecated shims. The
backend is chosen by URL, not by type:

```go
// Before                                   // After
metric.NewMimirService("")                  metric.NewMetricService("")   // METRIC_URL
                                            metric.NewVictoriaMetricsService("")
```

Set `METRIC_URL` to the remote-write endpoint (default: VictoriaMetrics
`:8428/api/v1/write`; Mimir is `:9009/api/v1/push`).

## Shared verification

After applying every selected migration:

```bash
grep -rn "go.uber.org/zap" --include="*.go" .        # Migration 1: must be empty
grep -rn "config/common\|ConfigSchema" --include="*.go" .   # Migration 3: must be empty
grep -rn "PROFILE" --include="*.go" .                # Migration 4: must be empty
grep -rn "func New.*Cmd() \*cobra.Command" --include="*.go" .  # Migration 7: must be empty
go mod tidy && go vet ./... && go build ./... && go test ./...
```

Then start the app once and confirm the config actually resolves — a silent
empty string from `viper.GetString()` almost always means `config.Default()` was
never called, not that the key is missing.

## Reference

- `log/slog` package docs: <https://pkg.go.dev/log/slog>
- `slog.Handler` interface: <https://pkg.go.dev/log/slog#Handler>
- `slog.Level` constants: `slog.LevelDebug (-4)`, `slog.LevelInfo (0)`,
  `slog.LevelWarn (4)`, `slog.LevelError (8)`
