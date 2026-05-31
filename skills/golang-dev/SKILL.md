---
name: golang-dev
description: >
    Use when writing Go code, choosing Go libraries, setting up build/test
    commands, or investigating memory escape issues. Covers CLI scaffolding
    with cobra, configuration with viper, common libraries (zap, testify,
    golangci-lint, air), build flags, test flags including disabling inlining,
    and escape analysis verification workflow.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuest
user-invocable: true
disable-model-invocation: true
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

---

## 2. Configuration (viper)

`When to use:` Any Go project that needs configuration management.

IMPORTANT: If `github.com/bizshuk/gosdk` is available, always use its `config.Default()` for configuration loading. Fall back to raw `viper` manual setup only if the SDK is not supported or available.

Always use `github.com/spf13/viper`. Loading precedence (highest wins):

1. `Environment variables` (`viper.AutomaticEnv()`)
2. `CLI flags` (bound via `viper.BindPFlag()`)
3. `Config file` (YAML preferred)
4. `Defaults` (`viper.SetDefault()`)

### Config struct pattern

```go
type Config struct {
    Server ServerConfig `mapstructure:"server"`
    DB     DBConfig     `mapstructure:"db"`
}

type ServerConfig struct {
    Port         int    `mapstructure:"port"`
    ReadTimeout  int    `mapstructure:"read_timeout"`
}

type DBConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    Name     string `mapstructure:"name"`
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
db:
    host: localhost
    port: 5432
    name: myapp
```

### Common pitfalls

- `Env prefix:` Call `viper.SetEnvPrefix("MYAPP")` so env vars like `MYAPP_SERVER_PORT` map correctly.
- `Nested keys in env:` Use `viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))` so `server.port` maps to `MYAPP_SERVER_PORT`.
- `Type mismatch:` `viper.Unmarshal` uses `mapstructure` tags, not `json` or `yaml` tags.

---

## 3. Common Libraries

| Category   | Library                       | When to use                                                                                    |
| ---------- | ----------------------------- | ---------------------------------------------------------------------------------------------- |
| Logging    | `go.uber.org/zap`             | Structured logging. `zap.NewProduction()` for JSON, `zap.NewDevelopment()` for console.        |
| Testing    | `github.com/stretchr/testify` | `assert` (continue on fail), `require` (stop on fail).                                         |
| Linting    | `golangci-lint`               | Meta-linter. Install: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`. |
| HTTP       | `net/http` (stdlib)           | Prefer stdlib. Use `github.com/gin-gonic/gin` only when routing complexity justifies it.       |
| Hot Reload | `github.com/air-verse/air`    | Dev-time file watcher. Run `air` instead of `go run`.                                          |
| Mocking    | `github.com/bytedance/mockey` | Runtime monkey-patching for tests.                                                             |
| CLI        | `github.com/spf13/cobra`      | CLI scaffolding (see Section 1).                                                               |
| Config     | `github.com/spf13/viper`      | Config management (see Section 2).                                                             |

### zap quick setup

```go
func NewLogger(isDev bool) (*zap.Logger, error) {
    if isDev {
        return zap.NewDevelopment()
    }
    return zap.NewProduction()
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
