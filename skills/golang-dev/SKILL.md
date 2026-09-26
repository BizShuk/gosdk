---
name: golang-dev
description: >
    Use when writing or reviewing Go in a gosdk-based project — cobra CLI,
    config.Default/viper, database connection (db.Init, DB_DRIVER + DB_DSN
    for mysql or sqlite), slog, MVC layering
    (handler/<domain>, handler/route, handler/middleware, svc/<domain>,
    model, cmd/web.go), naming (stutter, acronyms, gopls
    rename), Gin HTTP, bubbletea/lipgloss panel TUI, metrics, notify,
    testify, escape analysis, HTTP client/server, gRPC, TLS, or
    registry/init() providers. Triggers: gosdk, config.Default, MVC,
    rename, slog, cobra, MetricService, TUI, bubbletea, connection
    pool, timeouts, drain response body, DB_DSN, DB_DRIVER, gorm,
    MYSQL_DSN, SQLITE_PATH, handler/route, middleware, svc/, cmd/web.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuestion
user-invocable: true
disable-model-invocation: false
context: fork
---

# golang-dev

Go + `github.com/bizshuk/gosdk` playbook. Prefer SDK helpers over reimplementation.

Load **only** the chapter that matches the task. Do not read the rest.

| Chapter | Load when |
| ------- | --------- |
| [cli.md](references/cli.md) | cobra, where `main.go` goes, one- and two-layer subcommands, `ConfigCmd`, `CobraCMDHook` |
| [tui.md](references/tui.md) | `monitor`, bubbletea, columns misalign, flicker |
| [config.md](references/config.md) | `config.Default`, viper keys, dual-file; `db.Init` / `db.Open` with `DB_DRIVER` + `DB_DSN`, `DB_LOG`, `DB_TRANSLATE_ERROR`; legacy `MYSQL_DSN` / `SQLITE_PATH` |
| [logging.md](references/logging.md) | slog, `LOG_LEVEL`, zap leftovers |
| [http.md](references/http.md) | client/server timeouts, drain body, TLS, retries |
| [layers.md](references/layers.md) | MVC web service on Gin: `handler/<domain>` / `handler/route` / `handler/middleware` / `svc/<domain>` / `model/`, `cmd/web.go` wiring, identity auth middleware (Bearer + cookie), where interfaces live |
| [naming.md](references/naming.md) | stutter, acronyms, `gopls rename` |
| [metrics.md](references/metrics.md) | `metric.Send`, notify, CSV, `homedir.Expand` |
| [libraries.md](references/libraries.md) | which library to pick |
| [registry.md](references/registry.md) | pluggable providers, `init()` self-registration |
| [build.md](references/build.md) | `go build`/`test`, race, escape analysis |

Review / SOLID / dead code / pprof: `golang-review`.
