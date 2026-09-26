# CLI (cobra)

```tree
main.go        # repo root — cmd.Execute() only; binary name = module's last element
cmd/
  root.go      # RootCmd + Execute
  monitor.go   # monitor (alias m) — only interactive command
  logs.go      # logs (alias log)
  list.go      # list (alias l)
  ps.go        # ps — only if the app owns long-lived OS processes
  web.go       # web — Gin server trigger (layers.md)
  web/         # web subcommands (routes, migrate), two-layer rule below
```

`main.go` lives at the **repo root** — the repo name is the command name, so `go install .` yields the right binary with no extra path. Only a repo shipping several binaries uses `cmd/<binary>/main.go`, and then `cmd/` holds mains, not commands.

Package-level exported vars, flags in `init()`, one file per command. Never `NewXxxCmd()`. `RunE` not `Run`. Bind flags with `viper.BindPFlag`.

## Two layers

A subcommand that groups others (`app grafana push`) splits across a file and a directory of the same name:

```tree
main.go            # blank-imports each group: _ ".../cmd/grafana"
cmd/
  root.go          # RootCmd + Execute — knows no domain
  grafana.go       # package cmd — GrafanaCmd, its flags, its viper keys
  grafana/         # package grafana — the second layer, one file per command
    service.go     #   shared derivation/config for this group
    push.go        #   PushCmd; init() { cmd.GrafanaCmd.AddCommand(PushCmd) }
    list.go
svc/grafana/       # what the commands actually do
```

| Layer | File | Answers |
| ----- | ---- | ------- |
| root | `cmd/root.go` | which groups exist |
| first | `cmd/<group>.go` | what the group is, which flags it takes |
| second | `cmd/<group>/<verb>.go` | what pressing it does |

`cmd` must **not** import `cmd/<group>` — the children import `cmd` to reach the group var, so the arrow points one way and `main.go`'s blank import is what attaches them. Adding a group is one file, one directory, one blank import.

Name collision is expected: `cmd/grafana` and `svc/grafana` are both `package grafana`, so alias the service side `<domain>svc` (`grafanasvc "…/svc/grafana"`). Inside a `RunE`, name the cobra parameter `c`, not `cmd` — it would shadow the imported package.

```go
var RootCmd = &cobra.Command{Use: "myapp"}

func Execute() {
    if err := RootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(func() { config.Default(config.WithAppName("myapp")) })
    metric.CobraCMDHook(RootCmd)
    RootCmd.AddCommand(cmd.ConfigCmd, cmd.MajorCmd, cmd.MinorCmd, cmd.PatchCmd)
}
```

```go
// cmd/web.go — RunE body (wiring + http.Server) lives in layers.md
func init() {
    RootCmd.AddCommand(WebCmd)
    WebCmd.Flags().IntP("port", "p", 8080, "server port")
    viper.BindPFlag("port", WebCmd.Flags().Lookup("port"))
}
```

A package-level command keeps parsed flag state across `Execute()` in one process. Tests that run a command twice must reset bound vars to `nil` and clear `f.Changed` via `Flags().VisitAll`.

| Command   | Alias | Kind        | Job |
| --------- | ----- | ----------- | --- |
| `monitor` | `m`   | interactive | TUI ([tui.md](tui.md)); the only command that may take over the terminal |
| `logs`    | `log` | streaming   | What happened; `--process` lists recent processes (default 20) |
| `list`    | `l`   | snapshot    | Domain state now, then exit |
| `ps`      | —     | snapshot    | Live OS processes (PID first); omit if the app has no daemons |
| `config`  | —     | snapshot    | `cmd.ConfigCmd` — do not hand-roll |
| `web`     | —     | server      | Serve only; migrations/seed are other commands |

`logs` vs `list` vs `ps` are three objects — never fold with a flag. Tabular `ps`/`list` output uses `tui.Table` (`Align` one value per header: `0` left, `1` right). `config` writes `settings.local.json` only (`--update`/`--add`/`--delete`; `--source` shows provenance).

`CobraCMDHook` emits `command_line_trigger{cmd, flag}` via `METRIC_URL` (default VictoriaMetrics `:8428/api/v1/write`). If a subcommand has its own `PersistentPreRunE`, set `cobra.EnableTraverseRunHooks = true`.
