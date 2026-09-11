# TUI (bubbletea)

Interactive `monitor` only. Static CLI tables stay `tui.Table` ([cli.md](cli.md)). Live examples: `~/projects/tools/pm2/tui`, `~/projects/data/vid-note/tui`.

**Stack.** `charmbracelet/bubbletea` v1 + `lipgloss`. `bubbles` only for `viewport` / `textinput`. No tview/tcell. Flat panels + one `│` divider — no per-panel borders. Pin a **private** `runewidth.Condition{EastAsianWidth: false, StrictEmojiNeutral: true}`; never the package-global.

```tree
tui/
  model.go keys.go commands.go
  theme/palette.go          # views never hardcode colors
  views/                    # PURE: ViewContext in, string out
    layout.go               # RenderLayout — the only View() entry
    header.go banner.go tree.go detail.go logs.go footer.go
    width.go format.go      # the only place width is measured
```

`View()` snapshots `ViewContext` and calls `RenderLayout`. Views never touch Model.

**Budget.** Header/banner/footer = 1 row each; body gets the rest. Optional panels collapse entirely when the remainder would drop below a usable minimum — never squeeze. Every panel is exact `w×h` (`padLines`). Assert `lipgloss.Height(frame) == ctx.Height`. Seed `width: 100, height: 30` before the first `WindowSizeMsg`. Replace bars (search takes the banner row); never stack. `lipgloss.Width/Height` include padding, not border.

**Width.** `len` is bytes; measure only via the pinned engine. Crop then pad. Measure before styling (ANSI is 0 cols). Account fixed overhead (`" ▸ ● title  42"` spends 6 cols). Prefer ASCII `>`/`v`. Wrap by display width; CJK needs a hard cut.

**Keys.** Typing gate first (`/` search must not fire `q`). Banner is never focused — `t`/`tab` cycles it. Focused panel owns arrows.

| Key | Action |
| --- | ------ |
| `q`, `ctrl+c` | quit (`ctrl+c` works while typing) |
| `p` | pause selected; wrong kind → notice, never silent no-op |
| `s` | sort cycle: each field twice (useful direction first, then reverse) |
| `r` | retry; block live leases, allow expired |
| `t`, `tab` | cycle category; must work on empty filter |
| `/` | subsequence search; Enter keeps filter, Esc clears |
| `↑↓` `jk` | cursor-centered window: `start := max(min(cursor-visible/2, total-visible), 0)` |
| `→←` `lh` | tree: expand-or-enter / parent-or-collapse |
| wheel | hit-test pointer (`msg.X`), not focus; `WithMouseCellMotion` (kills native selection) |

Subsequence: every query rune in order, gaps allowed (`aeg` matches `abcdefg`). Client-side per keystroke; sort order stays whatever `s` says. Empty states name *which* empty ("no sources" vs "no match for query").

**Async.** Never touch Model from a goroutine. Echo the request key in the reply and drop stale. Restore selection across refreshes. Cheap tick 2s; expensive work slower; skip collapsed panels. Act → notice + immediate reload. Notice TTL > tick. Side-panel errors must not clobber the main error. Jobs that must survive quit use `context.Background()`.

**Logs.** Bounded chan, non-blocking send, drain bursts, **always re-arm** `waitForLogs`. No `fmt`/`slog` on stdout while alt-screen is up — capture `realStdout`, `WithOutput(realStdout)`, point slog at a file. Ring of raw lines (~2000); 1 log line = 1 screen row. `follow = viewport.AtBottom()`.

**Tests.** Views: feed `ViewContext`, assert `screen.StringWidth(line) == w`. Pin crop/pad with CJK, emoji, ANSI. Layout breakpoints: min, each optional panel collapse, large. Controller: `Update` with synthetic msgs.

| Symptom | Fix |
| ------- | --- |
| Rows wrap / panel shifts | crop/pad via pinned engine; count fixed overhead |
| Misalign only in CJK locale / `●` | `EastAsianWidth: false`; ASCII markers |
| Colored cells misalign | never `text/tabwriter` with ANSI |
| Header walks off | close height budget exactly |
| Screen shreds | redirect logs; no stdout |
| Log pane freezes | re-arm `waitForLogs` |
| Cursor jumps every tick | restore selection after snapshot |
| Wrong details under row | drop replies whose key no longer matches |
