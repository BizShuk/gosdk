# Config

Always `config.Default(opts ...ConfigOption)`. No global `ConfigSchema`. Read with `viper.Get*()`.

| Option | Effect |
| ------ | ------ |
| `WithAppName("myapp")` | Enables `~/.config/myapp` (`GetAppConfigDir` / `GetAppDataDir` / `GetAppLogsDir`) |
| `WithDefaultValue(json)` | Seeds `settings.json` there on first run — no-op without `WithAppName` |
| `WithConfigDir(dir)` | Forces that dir (`~` expanded); data/logs follow |

Search (first hit wins per filename): `.` → `./conf` → `GetAppConfigDir()`. Dual-file per format: base then `.local`.

| Format | Base | Override |
| ------ | ---- | -------- |
| dotenv | `.env` | `.env.local` |
| vault | `.env.vault` | `.env.local.vault` |
| json | `settings.json` | `settings.local.json` |
| yaml | `config.yaml` | `config.local.yaml` |

Merge (later wins): yaml → json → vault → env → OS env. Call `viper.SetDefault` **after** `Default()`. `viper.Set` is for tests/computed values only.

In files, `.` nests and `_` is literal — `a.b.c` ≠ `a_b_c`. OS env is `UPPER(key)` with `.` → `_` (`SERVER_PORT` overrides `server.port`, `LOG_LEVEL` overrides `log_level`). No `SetEnvPrefix`, no `SetEnvKeyReplacer`.

Embed a seed with `cmd/config.MustRegisterDefault("settings.json", bytes)` from `init()`. Duplicate file → error; deliberate override → `SetDefault`.

## DB

One service, one database; configuration picks the driver. `Init` refuses a second call.

```go
config.Default(config.WithAppName("myapp"))
if err := db.Init(); err != nil { return err } // DB_DRIVER + DB_DSN
gormDB := db.Default.DB()

extra, err := db.Open("trifecta") // DB_DSN_TRIFECTA (+ DB_DRIVER_TRIFECTA, else DB_DRIVER)
```

Keys: `DB_DRIVER` (`mysql` | `sqlite`), `DB_DSN` (single DSN string, not host/port/user). Empty `DB_DSN` with sqlite derives `<app config>/data/<app_name>.db`. An extra connection is an explicit decision, never a default.
