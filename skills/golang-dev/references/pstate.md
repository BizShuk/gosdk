# pstate (pipeline run + graph TUI)

`github.com/bizshuk/gosdk/cmd/pstate` runs a set of dependent steps in parallel and draws them live as a node graph. Use it for any multi-step job with dependencies (release, ETL, batch, migration). Do not hand-roll goroutine fan-out, `sync.WaitGroup` stage barriers, retry loops for a human to re-run, or a custom progress TUI.

## How to use

pstate *runs* your functions; callers never report status. Three steps: write the step functions, register them with their upstreams, run.

```go
import "github.com/bizshuk/gosdk/cmd/pstate"

// 1. Step function: (ctx, *pstate.T, params) error. Return nil = finished, error = failed.
type BuildArgs struct{ Target, Arch string }

func build(ctx context.Context, t *pstate.T, a BuildArgs) error {
	t.Logf("building %s/%s", a.Target, a.Arch) // goes to the log pane
	return compile(ctx, a.Target, a.Arch)
}

func approve(ctx context.Context, t *pstate.T) error {
	return t.Wait(ctx, "waiting for release manager") // pending until triggered
}

// 2. Register. Upstreams must already be added.
p := pstate.New()
if err := p.Add("fetch", pstate.Do(fetch)); err != nil { return err }
for _, target := range []string{"api", "web", "worker"} {
	id := "build-" + target
	if err := p.Add(id, pstate.Wrap(build, BuildArgs{Target: target, Arch: "arm64"}), pstate.After("fetch")); err != nil {
		return err
	}
}
if err := p.Add("approve", pstate.Do(approve), pstate.After("build-api", "build-web", "build-worker")); err != nil {
	return err
}

// 3. Run: TUI when a human watches, Execute in CI / tests / cron.
err := pstate.Run(ctx, p, pstate.WithTitle("release"))
// err := p.Execute(ctx)
```

| API | Use for |
| --- | ------- |
| `pstate.Wrap(fn, params)` | step with parameters; `params` is copied now and reused on every retry |
| `pstate.Do(fn)` | step without parameters |
| `pstate.After(ids...)` | upstreams; node starts only when all are `finished` |
| `pstate.Priority(n)` / `pstate.Name(s)` | column in the priority layout / display name |
| `t.Log` / `t.Logf` / `t.Error` | lines in the log pane (`Error` is red, status unchanged) |
| `t.Wait(ctx, reason)` | pause as `pending` until `enter` / click / `p.Trigger(id)` |
| `p.Retry(id)` / `p.Trigger(id)` | same actions from code (e.g. a webhook handler) |
| `WithLayout`, `WithGraphHeight(15)`, `WithLogHeight(5)` | initial layout and pane sizes (defaults shown) |

## Effect

**Execution.** What happens to each node, with or without the TUI:

| Situation | Effect |
| --------- | ------ |
| All upstreams `finished` | starts immediately in its own goroutine; independent nodes run concurrently |
| Step returns `nil` | `finished`; downstream nodes become eligible |
| Step returns error or panics | `failed`; first line shown on the graph, full text + stack in the log pane; only its descendants wait as `not_started`, other branches keep going |
| Step calls `t.Wait` | `pending` with the reason; goroutine blocks until triggered, then back to `running` |
| Retry (`r`, second click, `p.Retry`) | only for `failed`: same function, same `Wrap` params, attempt count +1 (`#2`); on success its blocked descendants run |
| User quits / ctx cancelled | running steps get ctx cancelled; last frame is printed to the terminal |
| `Execute` returns | when nothing is running and nothing can start; `nil` if all finished, `errors.Join` of `<id>: <msg>` per failed node, else `ErrIncomplete` |
| `Run` returns | when the user quits; same error contract as `Execute` |

**Screen.** `Run` takes only `graph + log + 3` rows (23 by default), not the full terminal:

```text
 release  |1 running  ~1 pending  x1 failed  +6 finished  .2 not_started  layout:dependency
 stage 1                stage 2                    stage 3                stage 4
 ╭──────────────────╮   ╭─────────────────────╮    ╭──────────────────╮   ╭──────────────╮
 │ + fetch     1.7s ├─┬─┤ + build-api    1.9s ├──┬─┤ ~ approve        ├───┤ . deploy     │
 ╰──────────────────╯ │ ╰─────────────────────╯  │ ╰──────────────────╯   ╰──────────────╯
                      │ ┏━━━━━━━━━━━━━━━━━━━━━┓  │
                      ├─┫ x build-web    2.1s ┣──┤
                      │ ┗━━━━━━━━━━━━━━━━━━━━━┛  │
                      │ ╭─────────────────────╮  │
                      └─┤ / build-worker 2.5s ├──┘
 x build-web  failed 2.1s  params {Target:web Arch:arm64}  | exit 2  [r retry]    logs:all 31 follow
09:12:04 build-web    failed after 2.1s: exit 2
09:12:04 build-web        ld: symbol not found
```

- Header: counts per status. Markers: `+` finished, spinner running, `~` pending, `x` failed, `.` not started; box border uses the status color.
- Graph: columns left to right by dependency depth (or priority); lines join upstream right edge to downstream left edge, fan-in merges into one line. Selected node has a heavy border and bright edges.
- Divider (the bar under the graph): selected node's status, duration, attempt, `params`, unfinished upstreams, error, and the action key; right side shows pan position and log state.
- `timeline` layout (`tab`): one row per node on a shared time axis, so parallel steps visibly overlap.

**Controls.** arrows select along the graph (view re-centers on the selection), `<` `>` pan, `r` retry, `enter` trigger, click selects / click again retries or triggers, `tab` layout, `f` log for selected node only, `[` `]` graph height, `-` `+` log height, drag the divider or footer bar to resize, `pgup` `pgdn` log scroll, `q` quit. Wheel: `option+wheel` pans horizontally; plain wheel scrolls vertically (or pans when the graph fits vertically). `shift+wheel` and trackpad swipes never arrive in VS Code / Antigravity terminals (xterm.js drops them); check a terminal with `go run ./cmd/sample/mousescroll`.

## Rules

| Rule | Why |
| ---- | --- |
| Pass inputs through `Wrap(fn, params)`, not mutable captured variables | retry must see the original inputs; divider shows them via `%+v` |
| `Add` upstreams first; check its error | unknown upstream is rejected, which is what rules out cycles |
| Honor `ctx` in every step | quitting the TUI cancels it; a step that ignores ctx blocks exit |
| No `fmt.Print` / default `slog` to stdout during `Run` | shreds the alt screen; use `t.Log` or point slog at a file |
| Human or webhook gates use `t.Wait` + `p.Trigger` | shows `pending` instead of a frozen `running` |
| Make retried steps idempotent | retry re-runs the whole function, not from where it failed |

## Verify

- Tests: build the pipeline, call `p.Execute(ctx)`, assert the error and side effects; no terminal needed.
- Demo: `go run ./cmd/pstate/sample` (one node fails once, one waits for `enter`), `-headless` for `Execute`, `-wide 12` for a graph wider than the screen.

Building a different TUI: [tui.md](tui.md).
