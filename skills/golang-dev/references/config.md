# Config

Always `config.Default(opts ...ConfigOption)`. No global `ConfigSchema`. Read with `viper.Get*()`.

| Option | Effect |
| ------ | ------ |
| `WithAppName("myapp")` | Enables `~/.config/myapp` (`GetAppConfigDir` / `GetAppDataDir` / `GetAppLogsDir`) |
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

Key casing is fixed per file:

| File | Casing | Example | Not |
| ---- | ------ | ------- | --- |
| `.env*` (`.env`, `.env.local`, `.env.vault`, `.env.example`) | `SCREAMING_SNAKE_CASE` — credentials | `DB_DSN`, `SLACK_TOKEN`, `IP_HASH_SALT` | `db_dsn`, `slackToken` |
| `settings*.json` (`settings.json`, `settings.local.json`, `settings.example.json`) | `snake_case` — words split by `_` | `server_addr`, `log_level`, `fetch_interval` | `serveraddr`, `serverAddr`, `SERVER_ADDR` |

viper lowercases every key, so the two forms meet at one key (`ip_hash_salt`); the `_` is what lets `UPPER(key)` produce the matching env name. A run-together lowercase key (`serveraddr`) only maps to `SERVERADDR`, which no one writes. Code reads the lowercase form: `viper.GetString("db_dsn")` and `viper.GetString("DB_DSN")` are the same call — pick the lowercase one.

## Shipped defaults (`config/example/`)

A service that reads **any** setting keeps its examples in `config/example/`, one per config file it uses:

| Example | Seeds | Holds |
| ------- | ----- | ----- |
| `settings.example.json` | `settings.json` | every non-secret key, each with a working value — the single source of defaults (`viper.SetDefault` only for computed values) |
| `.env.example` | `.env` | credential key **names only**, every value empty (`DB_DSN=`) — `.env*` is for credentials |

A key lives in exactly one example. `.env` outranks `settings.json`, so a key listed in both reads as the empty `.env` value and the JSON default is dead. The loader never reads `*.example.*` (files resolve by exact name), so the examples never shadow real config.

`config/example/example.go` (`package example`) embeds them and registers them for `config default`; the code that calls `config.Default` imports it and passes the settings as the runtime fallback layer. Same directory as the files, so `go:embed` needs no `..`, and the name never collides with gosdk's `config`.

```go
// config/example/example.go
package example

//go:embed settings.example.json
var Settings []byte

//go:embed .env.example
var Env []byte

func init() { // cfgcmd = gosdk/cmd/config
    cfgcmd.MustRegisterDefault("settings.json", Settings)
    cfgcmd.MustRegisterDefault(".env", Env)
}
```

```go
// cmd/root.go — the command tree must also mount sdkcmd.ConfigCmd, or "config default" is unreachable
config.Default(config.WithAppName(APP_NAME), config.WithDefaultValue(string(example.Settings)))
```

A repo that already has its own `config` package (`config/config.go` beside `example/`) may embed `example/settings.example.json` there instead — one embed site per repo, never both.

`WithDefaultValue` (gosdk ≥ v1.3.27) only merges the JSON under every file (`app config` lists it with source `default`, gosdk ≥ v1.3.28) — a key missing from an old or empty `settings.json` still reads its shipped value; it never writes a file. Older pins wrote `settings.json` on first run and needed a hand-rolled `viper.SetDefault` loop for the fallback; both go away on upgrade.

Invalid JSON or a second registration panics at `init()`; overriding a dependency's seed on purpose is `cfgcmd.SetDefault`. `cmd.ConfigCmd` brings `config default`, which copies every registered example into `GetAppConfigDir()` (`.env` created `0600`):

| Command | When | Effect |
| ------- | ---- | ------ |
| `app config default` | first install | writes `~/.config/<app>/settings.json` + `.env` byte-for-byte; an existing file is skipped with a warning |
| `app config default --merge` (`--dry-run` first) | after an upgrade, or a host pre-seeded the file | adds only missing keys, keeps every existing value |
| `app config default --force` | reset | replaces the files |
| `--file .env` | one file only | any of the above, limited to that file |

Then fill the empty `.env` values on the machine (`app config --file .env --update DB_DSN=...`, or by hand); they never go into the repo.

Legacy layouts and where they go:

| Found | Becomes |
| ----- | ------- |
| `config/default_settings.json` + `//go:embed default_settings.json` | `config/example/settings.example.json`, embed path `example/settings.example.json` |
| `settings.example.json` / `.env.example` / `config.example.yaml` at repo root or in `cmd/` | `config/example/` + `example.go` |
| `config/example/env` (dotenv with values) | secrets → `.env.example` (empty), everything else → `settings.example.json` |
| inline `const defaultSettings = `{...}`` | `config/example/settings.example.json` |
| `viper.SetDefault` loop over the embedded JSON (`applyEmbeddedDefaults`) | deleted once the repo pins gosdk ≥ v1.3.27 (`WithDefaultValue` is the fallback layer) |
| setup script `cp config/example/env ~/.config/<app>/.env` | `app config default` (until ≥ v1.3.27, `cp` both examples) |

Conflicts to avoid:

- `config default` is the only path that writes into the config directory. No `CreateIfNotExist`, no seed-on-start helper, no Dockerfile `COPY` of the example.
- A credential the host already keeps in `settings.json` (liva seeds `ip_hash_salt` there) stays in `settings.example.json` until the host file moves too: `--merge --file .env` would add it empty to `.env`, which outranks `settings.json` and blanks the real value.
- A pre-existing `settings.json` (liva `setup-app-config.sh` seeds `{}` or a per-host salt, the image writes `{}`) makes plain `config default` a no-op — deploy runs `--merge`. A bare `{}` left as is reads every key empty (`server_addr` empty → listens on `:80`).
- Names are fixed: `config/example/settings.example.json` and `config/example/.env.example` — not repo-root examples, not `config/default_settings.json`, not `config/example/env`. No `*.local.example.*`: `.local` files belong to the operator (`config --update` writes `settings.local.json`).

## DB

One service, one database; configuration picks the driver. `Init` refuses a second call.

```go
config.Default(config.WithAppName("myapp"))
if err := db.Init(); err != nil { return err } // DB_DRIVER + DB_DSN
gormDB := db.Default.DB()

slog.Info("db", "target", db.Default.Target()) // password masked

extra, err := db.Open("trifecta") // DB_DSN_TRIFECTA; other *_TRIFECTA keys fall back to the primary
```

Keys: `DB_DRIVER` (`mysql` | `sqlite`), `DB_DSN` (single DSN string, not host/port/user). Empty `DB_DSN` with sqlite derives `<app config>/data/default.db`. `DB_LOG` (default `false`, gorm output discarded) and `DB_TRANSLATE_ERROR` (default `true`, `gorm.ErrDuplicatedKey` etc.) switch gorm behaviour — don't `gorm.Open` yourself to get them. An extra connection is an explicit decision, never a default.

Legacy (gone since v1.3.18): `SQLITE_PATH` / `MYSQL_DSN` / `POSTGRES_DSN`, `db.InitSQLite` / `InitMySQL` / `InitPostgres`, `DefaultSQLite` / `DefaultMySQL`, and "whichever key is set picks the driver". Migrating a deployed service: code switches to the new keys in one go (no dual-read); on the host, add `DB_DRIVER` + `DB_DSN` next to the old key, deploy, then delete the old key. A SQLite file moves to `data/default.db` only while the service is stopped — a new binary started first creates an empty `default.db` and serves it silently. Workspace rules (one DB per service, approval for extras) live in `inf-spec` `references/database.md`.
