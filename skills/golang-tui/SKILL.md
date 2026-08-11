---
name: golang-tui
description: >
    Use when building or debugging a full-screen Go terminal dashboard
    (bubbletea/lipgloss) with a panel layout — top stats bar, category banner,
    tree menu, main view with logs/artifacts, hotkey footer — or when a TUI
    renders broken. Symptoms - rows wrap and shift a panel, CJK/emoji/ANSI
    columns misalign, the screen freezes or flickers, stray prints corrupt
    the frame, stale replies overwrite the selection, the cursor jumps on
    refresh, or the layout falls apart on resize. Also for wiring the standard
    hotkey set (pause/resume, sort cycling, retry, category switch),
    subsequence search, or mouse wheel/click support into a panel TUI.
allowed-tools: Bash, Read, Edit, Grep, Glob
user-invocable: true
disable-model-invocation: false
context: fork
---

# golang-tui

Panel-based TUI dashboards in Go. Canonical live examples of this exact pattern: `~/projects/tools/pm2/tui` and `~/projects/data/vid-note/tui` — read them before inventing a new structure.

## 1. Stack

- `charmbracelet/bubbletea` v1 + `charmbracelet/lipgloss` — the workspace standard. Do not introduce tview/tcell/termui alongside it.
- `charmbracelet/bubbles` only for `viewport` (scrolling log pane) and `textinput` when genuinely needed; tree, tables, and bars are cheaper hand-rolled.
- `mattn/go-runewidth` with a **pinned private Condition** (section 3). Never mutate the package-global condition.
- Static (non-interactive) tables printed by a CLI command are not a dashboard — use gosdk `tui.Table` (`github.com/bizshuk/gosdk/tui`) instead of hand-rolling box-drawing output.
- **Flat panels + a 1-col `│` divider + section-header lines, not borders.** A border spends 2 cols and 2 rows per panel and doubles the size arithmetic that can go wrong. One dashboard has one frame: the terminal itself.

## 2. Architecture: controller vs views

```tree
tui/
├── model.go        # Model struct, Init, Update — ALL state lives here
├── keys.go         # handleKey: the entire key map in one file
├── commands.go     # every tea.Cmd + its typed result msg, side by side
├── theme/
│   └── palette.go  # color tokens; views never hardcode colors
└── views/          # PURE functions: ViewContext in, string out, no state
    ├── layout.go   # RenderLayout — the ONLY entry point View() calls
    ├── header.go   # top stats bar
    ├── banner.go   # category chips
    ├── tree.go     # left menu
    ├── detail.go   # main-view metadata block
    ├── logs.go     # main-view log tail
    ├── artifacts.go
    ├── footer.go   # hotkey hints
    ├── width.go    # crop/pad/wrap — the only place width is measured
    └── format.go   # durations, sizes, status glyphs
```

`View()` builds one `ViewContext` snapshot (width, height, rows, cursor, focus, notice, err, …) and calls `views.RenderLayout(ctx)`. Views never touch the Model. This split is what makes every panel testable headlessly: feed a ViewContext, assert on the string.

## 3. The five panels and the height budget

```text
┌ header   1 row   stats / summary
├ banner   1 row   ‹ Downloads │ Transcodes │ Uploads ›   (category chips)
├ body     rest    ┌ tree 30–36 cols ┬ │ ┬ main view ────────────┐
│                  │ ▸ channel     n │ │ │ metadata / summary    │
│                  │   ▾ videos      │ │ ├────────────────────── │
│                  │                 │ │ │ logs + artifacts      │
├ footer   1 row   hotkeys · sort:date↓ · updated 12:03:04
```

All arithmetic lives in `RenderLayout`, top-down, and must close exactly:

```go
func RenderLayout(ctx ViewContext, now time.Time) string {
    if ctx.Width < minWidth { return "terminal too narrow (min 60 cols)" }

    bodyH := ctx.Height - 3            // header + banner + footer
    if bodyH < 3 { return "terminal too short" }

    // Optional full-width summary borrows from body — and collapses ENTIRELY
    // when body would drop below a usable minimum. A half-rendered summary
    // plus half a tree is worse than no summary.
    summaryH := SummaryHeight(ctx)
    if bodyH-summaryH < minBodyRows { summaryH = 0 }
    bodyH -= summaryH

    leftW := treeWidth(ctx)            // preferred 36, clamped so right ≥ minRightPane
    rightW := ctx.Width - leftW - 1    // 1 = divider column

    body := lipgloss.JoinHorizontal(lipgloss.Top,
        RenderTree(ctx, leftW, bodyH), renderDivider(bodyH),
        renderRightPane(ctx, now, rightW, bodyH))
    return lipgloss.JoinVertical(lipgloss.Left,
        RenderHeader(ctx), RenderBanner(ctx), body, RenderFooter(ctx))
}
```

Budget rules that keep the frame exact:

- **Every panel renders to an exact w×h**: pad short content with blank lines (`padLines`), trim long content. A panel one row tall too many makes the whole frame scroll and the header disappears for good. `lipgloss.Height(frame) == ctx.Height` is the cheapest regression assertion you can write.
- **Sub-panels take what they need, the main content gets the rest.** Artifacts: `artH := max(min(len(artifacts)+1, 8), 2)`; if `bodyH-artH` leaves the detail block below its minimum, set `artH = 0` (collapse, don't squeeze).
- **Replace bars, never stack them.** A search bar takes the banner row's place while active; total height stays constant, so the tree does not jump one row the instant `/` is pressed.
- **Before the first `tea.WindowSizeMsg`, width/height are 0.** Seed `New()` with defaults (`width: 100, height: 30`) or gate `View()` behind a ready flag — `strings.Repeat(" ", negative)` panics.
- `lipgloss.Width(n)/Height(n)` include padding but **not** border; `Height(n)` only pads, never truncates. Overflow control is your job.

## 4. Width discipline — why rows wrap and panels shift

One row exceeding its panel width by **one column** makes lipgloss wrap it; every row below shifts, and the panel reads as garbage. All causes reduce to measurement disagreement:

```go
// views/width.go — the ONLY measurement engine, matched to lipgloss.
//
// lipgloss counts Unicode "Ambiguous width" chars (● ○ │ ─ ‹ ›) as 1 col.
// go-runewidth reads LC_CTYPE and, under zh_TW/ja_JP locales, reports the
// SAME chars as 2. One status dot then silently overflows the row.
// CJK and emoji are Wide, not Ambiguous — still 2 cols under both engines.
var screen = &runewidth.Condition{EastAsianWidth: false, StrictEmojiNeutral: true}

func crop(s string, w int) string {          // cut to w cols, … suffix
    if screen.StringWidth(s) <= w { return s }
    return screen.Truncate(s, w, "…")
}

func pad(s string, w int) string {           // crop FIRST, then pad:
    s = crop(s, w)                           // a cut wide char leaves the row
    return s + spaces(w-screen.StringWidth(s)) // 1 col short otherwise
}
```

- `len()` is bytes, `RuneCountInString` is runes; **neither is columns**. Measure only via the pinned engine.
- **Measure before styling.** ANSI escapes add zero columns but `len` counts them. Never `text/tabwriter` for colored cells — it counts escapes into cell width and misaligns every colored column (see `~/projects/tools/port/svc/monitor.go` for the post-mortem).
- **Account fixed overhead explicitly.** A tree row `" ▸ ● title      42"` spends 6 cols on marker/dot/spaces: `nameW := w - 6 - screen.StringWidth(count)`. Undercount by one and the tree "looks broken".
- **Prefer ASCII markers (`>`/`v`) or verify your glyphs are non-Ambiguous.** Emoji width is genuinely terminal-dependent; keep emoji inside a fixed-width padded cell so a disagreement costs one ragged space, not the layout.
- **Wrap by display width** with a hard-cut fallback for CJK (no spaces to break on) — see `wrap()` in vid-note `views/width.go`.

## 5. Focus and key routing

Both the banner and the tree want ←/→. Resolve by layering, not by focus on the banner:

```go
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    if m.typing { return m.handleTyping(msg) } // search input eats keys FIRST:
                                               // otherwise typing "q" quits
    switch msg.String() {
    case "q", "ctrl+c":       return m, tea.Quit
    case "/":                 m.typing = true; return m, nil
    case "t", "tab":          return m.cycleKind()   // banner: dedicated key,
                                                     // banner itself is never focused
    case "s":                 return m.cycleSort()
    case "p":                 return m.togglePause()
    case "r":                 return m.retrySelected()
    case "up", "k":           return m.moveCursor(-1)
    case "down", "j":         return m.moveCursor(+1)
    case "right", "l", "enter": return m.expand()    // focused panel owns arrows
    case "left", "h":         return m.collapse()
    }
    return m, nil
}
```

### House hotkey map

| Key | Action | Semantics the handler must honor |
| --- | --- | --- |
| `q`, `ctrl+c` | quit | `ctrl+c` must also work inside typing mode |
| `p` | pause / resume selected | Acts only on the row kind that can pause; wrong kind → notice, never a silent no-op (a mis-aimed `p` that disables a whole channel is invisible on screen). Paused state must show on the row (`●`→`○`) |
| `s` | sort cycle | Each field appears **twice in a row — its useful direction first, then the reverse** — before the next field. "Useful first" per field: newest-first for dates, pipeline order for stage/status. Footer shows `field↓`; unknown state resets to entry 0 |
| `r` | retry selected | Guard on live leases: a running worker's row blocks with a notice; an **expired** lease passes — that stuck row is why the monitor is open |
| `t`, `tab` | cycle category (banner) | Reload immediately on switch; must still work when the current filter shows zero rows, or the user can never leave an empty category |
| `/` | search | Subsequence match (below); Enter keeps the filter and returns keys to navigation; Esc clears |
| `↑↓`, `jk` | cursor | Cursor-centered window (§5) |
| `→←`, `lh`, `enter` | expand / collapse | Tree semantics (§5) |
| wheel | scroll hovered panel | Routed by pointer position, not focus (below) |

```go
var sortCycle = []struct{ Field SortField; Order SortOrder }{
    {SortByDate, SortDesc}, {SortByDate, SortAsc},       // useful direction first
    {SortByUpdated, SortDesc}, {SortByUpdated, SortAsc},
    {SortByStage, SortAsc}, {SortByStage, SortDesc},
}
// Entry 0 is also the initial sort — share the source or they drift.
```

### Subsequence search

`/` filters by **ordered subsequence**, not substring: every query rune must appear in the target in the same order, gaps allowed — `aeg` matches `abcdefg`, `gfe` does not.

```go
func matchSubseq(query, target string) bool {
    q := []rune(strings.ToLower(query))
    if len(q) == 0 { return true }
    i := 0
    for _, r := range strings.ToLower(target) {
        if r == q[i] {
            if i++; i == len(q) { return true }
        }
    }
    return false
}
```

- Filter client-side per keystroke over the loaded candidate set; it composes with the stale-reply rule (§6) when candidates come from async queries.
- Server-side SQL can express the same thing: `aeg` → `LIKE '%a%e%g%'` (escape user `%_` first).
- Keep matching and ranking separate: the filter never reorders rows on its own — sort order stays whatever `s` says.

### Mouse

```go
p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

case tea.MouseMsg:
    switch msg.Button {
    case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
        delta := 3                              // match bubbles viewport.MouseWheelDelta
        if msg.Button == tea.MouseButtonWheelUp { delta = -3 }
        if msg.X < m.leftW {                    // hit-test: panel under the POINTER,
            return m.moveCursor(delta)          // not the focused one
        }
        var cmd tea.Cmd
        m.vp, cmd = m.vp.Update(msg)            // bubbles viewport handles wheel itself
        return m, cmd
    case tea.MouseButtonLeft:
        if msg.Action != tea.MouseActionPress { break }
        row := m.scrollTop + msg.Y - firstContentRow // subtract chrome, add scroll offset
        if row >= m.scrollTop && row < len(m.visible) { m.cursor = row }
    }
```

- Wheel on the menu moves the **cursor** by ±3; the cursor-centered window follows, so there is one scroll source of truth. Wheel on the log pane forwards the msg to the viewport.
- Enabling mouse capture **takes the terminal's native text selection away** (users must Shift/Option-drag to copy). That is the price of wheel support — decide it deliberately; there is no wheel-only mode.

- **Typing gate first.** `typing` (cursor in the input) and `query` (filter in effect) are separate fields: Enter leaves typing but keeps the filter so j/k can walk the hits; Esc clears. Collapsing them into one bool makes leaving the input erase the search.
- **Tree semantics**: `→` on a collapsed node expands; on an expanded node steps into the first child (same gesture in the user's head). `←` on a child jumps to its parent; on an expanded parent collapses it — and keeps the loaded children cached for free re-expand.
- **Cursor-centered scrolling**: `start := max(min(cursor-visible/2, total-visible), 0)` — a fixed window loses the cursor after ~30 rows.
- **A dead key is worse than a wrong result.** Unknown sort state → reset to the first cycle entry, don't no-op. Show direction in the footer (`date↓`); an invisible state change reads as a broken key.
- **Empty states must say which empty**: "no sources — run `app source add`" vs "no match for “query”". The wrong one tells the user their data is gone.
- Two focusable panels (tree / log viewport) are plenty; store `focus` as one enum and give arrows to the focused one. More than two focus targets on one screen usually means you need a second screen (state machine: `screenTree → screenViewer → screenConfirm`, see pm2 `logbrowser`).

## 6. Async data: commands, ticks, staleness

Never touch the Model from a goroutine. Every read/write goes out as a `tea.Cmd` and comes back as a typed msg with `err` inside:

```go
type artifactsMsg struct { videoID uint; files []File; err error }

case artifactsMsg:
    if m.currentSelection().VideoID == msg.videoID { // stale-reply rejection:
        m.artifacts, m.err = msg.files, msg.err      // a slow reply must not pin
    }                                                // the PREVIOUS video's files
                                                     // under the current one
```

- **Echo the request key in the reply** (videoID, query string) and drop replies that no longer match. Per-keystroke searches run concurrently; the slowest one otherwise rewinds the screen two keystrokes back.
- **Restore selection across refreshes**: capture `sel := m.currentSelection()` before replacing rows, re-find it after — a 2s tick that resets the cursor to row 0 makes the tree unusable.
- **Cadence per cost**: cheap snapshot every 2s tick; expensive stats (per-file `os.Stat` walks) on their own 30s rhythm; **skip queries whose panel is collapsed or whose node is unexpanded**; cap search results (`limit 200`) — the first keystroke matches the whole table.
- **Act, then refresh immediately.** After any action reply, fire a reload — waiting for the next tick makes the key feel dead. Failed actions refresh too (they already mutated retries/leases).
- **Feedback before dispatch** for minutes-long jobs: set `m.running[id]=true` + a notice **before** returning the Cmd, and gate a second press on `running`. DB state can't tell "my job" from another worker's.
- **Notice TTL > tick interval** (5s vs 2s), or the next refresh erases the feedback before it can be read.
- **Error scoping**: a side-panel query failure must not clobber the main error field; a stats hiccup is not a tree outage.
- **Jobs that must survive the TUI** get `context.Background()`, not the program's context — quitting the monitor must not kill a half-finished download.

## 7. Live log pane

Workers must never block on the UI, and the UI must not wake per line:

```go
// Bounded channel; worker side is non-blocking and counts drops.
select { case logCh <- line: default: dropped.Add(1) }

// UI side: block for one line, then drain the burst — 5000 lines/s
// become ~60 Updates/s. ALWAYS re-arm: forgetting to return waitForLogs
// from the logBatchMsg case is the #1 frozen-log-pane bug.
func waitForLogs(ch <-chan LogLine) tea.Cmd {
    return func() tea.Msg {
        first, ok := <-ch
        if !ok { return logsClosedMsg{} }
        batch := []LogLine{first}
        for len(batch) < 256 {
            select { case l, ok := <-ch: if !ok { return logBatchMsg(batch) }; batch = append(batch, l)
            default: return logBatchMsg(batch) }
        }
        return logBatchMsg(batch)
    }
}
```

- **Nothing may print to the terminal while the TUI runs.** A stray `fmt.Println`/`slog` on stdout shreds the frame. Capture `realStdout := os.Stdout` first, `tea.NewProgram(m, tea.WithOutput(realStdout))`, then point `slog` and `os.Stdout/os.Stderr` at a log file (`tea.LogToFile`); subprocesses need fd-level `Dup2`. Mirror into the pane via a second slog handler that does a non-blocking channel send.
- **Ring buffer of raw lines** (~2000), re-rendered on resize from the raw text — never re-wrap already-wrapped strings. Hard-truncate each line to pane width: the invariant *1 log line = 1 screen row* is what keeps scroll math exact.
- **Tail semantics**: render the last h lines; `nil` slice = "loading…", empty = "(no entries)" — different states, different words.
- **Follow mode derives from position**: `follow = viewport.AtBottom()` after every scroll — scrolling up stops the tail chasing the reader; scrolling back down resumes it. Preserve the anchor (`AtBottom()` captured before resize) across `WindowSizeMsg`.

## 8. Why the TUI breaks — failure catalog

| Symptom | Root cause | Fix |
| --- | --- | --- |
| One panel's rows all shifted / "tree looks broken" | A row 1 col over panel width → lipgloss soft-wraps it | Fixed-overhead accounting + `pad(crop())` via one engine (§4) |
| Misaligns only in CJK locale / only rows with `●`/`▸` | Ambiguous-width glyphs: runewidth says 2 under EastAsian LC_CTYPE, lipgloss says 1 | Pinned `runewidth.Condition{EastAsianWidth: false}`; ASCII markers |
| Misaligns only colored cells | `text/tabwriter` counts ANSI escapes as width | Manual columns, ANSI-aware measuring; never tabwriter with color |
| Header scrolls off; frame "walks" upward | Total render height > terminal rows by ≥1 | Close the budget exactly; assert `lipgloss.Height(frame) == Height` |
| Previous frame bleeds through a panel | Short render not padded to panel h | `padLines(lines, w, h)` every panel, every frame |
| Random text shreds the screen | Any write to stdout/stderr while alt-screen active | Redirect all logging to file; `WithOutput(realStdout)` captured before the swap (§7) |
| Panic/blank at startup | `View()` runs before first `WindowSizeMsg`; w/h = 0 | Seed 100×30 defaults or a `ready` gate |
| Log pane freezes after a while | Listener Cmd not re-armed after its msg | Every `logBatchMsg` case returns `waitForLogs` again |
| Whole UI frozen | Blocking I/O inside `Update`/`View` | All I/O in `tea.Cmd`s; Update only folds msgs |
| Pipeline slows when UI busy | Workers blocking-send into the UI | Bounded chan + non-blocking send + visible drop counter |
| Typing a filter triggers hotkeys | No typing gate before the key switch | `if m.typing` first line of `handleKey` |
| Cursor jumps to top every 2s | Refresh rebuilds rows without restoring selection | Capture/restore selection around every snapshot apply |
| Wrong details under selected row | Slow reply applied without match check | Echo videoID/query in msg; drop stale replies |
| Search shows results for an older keystroke | Concurrent per-key queries race | Compare echoed query against current before applying |
| Action feedback never visible | Notice TTL ≤ tick; next refresh wipes it | TTL (5s) > tick (2s) |
| Key "feels dead" after action | UI waits for next tick to show the change | Immediate feedback + immediate reload after actionMsg |
| Double-press launches job twice | Only DB state consulted; lease ≠ "mine" | Local `running[id]` map set before dispatch |
| Quit kills in-flight download | Job Cmd bound to program lifetime | `context.Background()` for must-survive jobs |
| Resize flickers or half-draws | Re-wrapping wrapped text; manual ClearScreen calls | Re-render from raw ring; let the renderer diff; no SIGWINCH handling of your own |
| Fine on 200-video channel, dies on 2000 | Unbounded queries per tick/keystroke; hidden panels still polled | Limits, expanded-only loading, per-cost cadence (§6) |
| Wheel scrolls the focused panel, not the hovered one | `MouseMsg` routed by focus state | Hit-test `msg.X`/`msg.Y` against the layout; pointer wins |
| Wheel does nothing at all | Mouse never enabled | `tea.WithMouseCellMotion()` at program construction |
| Text can't be selected/copied from the terminal anymore | Mouse capture is all-or-nothing | Deliberate tradeoff: accept Shift/Option-drag, or drop mouse support |
| Click selects the wrong row | Y→row mapping missed chrome rows or scroll offset | `row := scrollTop + msg.Y - firstContentRow`; reject rows outside the window |
| Search finds `aeg` but not `gfe`; users report it broken | Subsequence matching is order-preserving by design | Working as intended — say so in the empty-state text ("no subsequence match") |

## 9. Testing

- Views are pure: table-driven tests feed a `ViewContext` and assert on strings — no PTY needed. Assert **widths** (`screen.StringWidth(line) == w` for every line), not just content.
- Pin `crop`/`pad`/`wrap` with CJK ("頻道名稱測試"), emoji, and pre-styled ANSI inputs.
- One layout test per breakpoint: minimum size, size where each optional panel collapses, a big size — assert total `lipgloss.Height/Width` equals the terminal exactly.
- Controller tests drive `Update` with synthetic msgs (`tea.WindowSizeMsg`, `tea.KeyMsg`, your typed msgs) and assert on state — bubbletea models are plain values; no terminal required.
