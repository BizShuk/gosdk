# pstate (pipeline run + graph TUI)

Multi-step jobs with dependencies (release, ETL, batch) use `github.com/bizshuk/gosdk/cmd/pstate`. Do not hand-roll goroutine fan-out, `sync.WaitGroup` stage barriers, or a custom progress TUI.

**Shape.** pstate *runs* your functions; callers never report status. Wrap each step, declare upstreams, run.

```go
type BuildArgs struct{ Target string }

func build(ctx context.Context, t *pstate.T, a BuildArgs) error {
	t.Logf("building %s", a.Target) // log pane, never fmt/slog to stdout
	return compile(ctx, a.Target)    // error -> failed, retryable in the TUI
}

p := pstate.New()
_ = p.Add("fetch", pstate.Do(fetch))
_ = p.Add("build", pstate.Wrap(build, BuildArgs{Target: "linux"}), pstate.After("fetch"))
_ = p.Add("approve", pstate.Do(func(ctx context.Context, t *pstate.T) error {
	return t.Wait(ctx, "waiting for release manager") // pending until enter / p.Trigger
}), pstate.After("build"))

err := pstate.Run(ctx, p, pstate.WithTitle("release")) // TUI
// CI / no terminal: err := p.Execute(ctx)
```

| Rule | Why |
| ---- | --- |
| Params go through `Wrap(fn, params)`, not captured mutable state | Retry re-calls with the value saved at wrap time; divider shows it via `%+v` |
| `Add` upstreams before downstreams | `After` rejects unknown IDs; that is what makes cycles impossible |
| Respect `ctx` in every step | Quitting the TUI cancels running steps |
| Log with `t.Log` / `t.Error` | stdout writes shred the alt screen |
| Long waits on humans/webhooks use `t.Wait` + `p.Trigger(id)` | Shows `pending` instead of a frozen `running` |
| Failure only blocks descendants | Independent branches keep running; `Execute` returns `errors.Join` of failed nodes |

**Options.** `WithLayout(LayoutDependency|LayoutPriority|LayoutTimeline)`, `WithGraphHeight` (default 15), `WithLogHeight` (default 5), `pstate.Priority(n)` / `pstate.Name(s)` per node.

**Keys / mouse.** arrows select (view centers on selection), `<` `>` pan, `r` retry failed, `enter` trigger pending, click to select then click again to retry, `tab` layout, `f` filter log, `[ ]` `- +` resize, drag divider/footer. Horizontal pan by wheel: `option+wheel` — VS Code / Antigravity xterm.js drops `shift+wheel` and `deltaX`; probe a terminal with `go run ./cmd/sample/mousescroll`.

**Testing your pipeline.** Call `p.Execute(ctx)` in tests and assert the returned error; no TUI needed. Example: `go run ./cmd/pstate/sample` (`-headless`, `-wide N`).

Building a different TUI: [tui.md](tui.md).
