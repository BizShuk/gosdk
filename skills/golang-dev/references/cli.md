# CLI (cobra)

```tree
cmd/
  root.go      # RootCmd + Execute
  monitor.go   # monitor (alias m) — only interactive command
  logs.go      # logs (alias log)
  list.go      # list (alias l)
  ps.go        # ps — only if the app owns long-lived OS processes
  web.go       # web
main.go        # cmd.Execute() only
```

Package-level exported vars, flags in `init()`, one file per command (`deployLocal.go` for sub-subcommands). Never `NewXxxCmd()`. `RunE` not `Run`. Bind flags with `viper.BindPFlag`.

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
var WebCmd = &cobra.Command{
    Use: "web", Short: "Start the HTTP server",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Listening on :%d\n", viper.GetInt("port"))
        return nil
    },
}

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
