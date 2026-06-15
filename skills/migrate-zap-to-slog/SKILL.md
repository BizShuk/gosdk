---
name: migrate-zap-to-slog
description: >-
    Use when migrating Go code from go.uber.org/zap to the standard log/slog
    package, or when a Go project contains `import "go.uber.org/zap"` /
    `zap.L()` / `zap.S()` calls that need replacement. Covers the zap → slog
    API mapping, Fatal handling (slog has no Fatal level), Sugar-style
    formatted calls, NewDevelopment/NewProduction logger construction, and
    viper-driven LOG_LEVEL/LOG_FORMAT configuration. Triggers: user asks to
    "migrate zap to slog", "replace zap with log/slog", or new code is being
    written while zap still exists in the module.
argument-hint: "[path]"
allowed-tools: Bash, Read, Edit, Grep, Glob
effort: medium
context: fork
disable-model-invocation: false
user-invocable: true
---

# migrate-zap-to-slog

Migrate a Go project from `go.uber.org/zap` to the standard `log/slog` package
(introduced in Go 1.21). The skill walks through four phases — Detect, Migrate
the log initializer, Migrate call sites, Verify — and gives the full API
mapping table for translating zap calls to slog idioms.

## When to use

- A Go module imports `go.uber.org/zap` and the user wants to remove the
  dependency.
- New code is being written while old zap calls still exist; the user wants
  the codebase to stop accumulating zap.
- The user explicitly asks to "use log/slog" or "replace zap with slog".

## When NOT to use

- The project does not use zap (no `import "go.uber.org/zap"`). Use the
  `golang-dev` skill for general Go work.
- The migration requires keeping zap temporarily for backward compatibility
  with downstream consumers. In that case, migrate only the internal log
  initializer and leave callers using the existing API surface; the user
  should split the work into two phases.
- The project uses a third-party logging wrapper (e.g. `logr`, `zerolog`)
  that already abstracts over slog. Migration is unnecessary.

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

## Quick reference

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

## Reference

- `log/slog` package docs: <https://pkg.go.dev/log/slog>
- `slog.Handler` interface: <https://pkg.go.dev/log/slog#Handler>
- `slog.Level` constants: `slog.LevelDebug (-4)`, `slog.LevelInfo (0)`,
  `slog.LevelWarn (4)`, `slog.LevelError (8)`
