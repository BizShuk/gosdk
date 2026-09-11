# Logging

```go
config.Default(config.WithAppName("myapp"))
log.Init() // LOG_LEVEL + LOG_FORMAT → slog.SetDefault(); stdout
slog.Info("server started", "port", 8080)
slog.Error("query failed", "err", err)
```

| Key | Values | Default |
| --- | ------ | ------- |
| `LOG_LEVEL` | `debug` / `info` / `warn` / `error` | `info` |
| `LOG_FORMAT` | `text` / `json` | `text` |

Package-level `slog.*` with `"k", v` pairs. No `log.Info` / `log.Errorf` wrappers, no printf, no zap. Re-call `log.Init()` after config reload. Child logger: `slog.With("request_id", id)`.
