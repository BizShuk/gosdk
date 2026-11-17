package pstate

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansiRE.ReplaceAllString(s, "") }

var t0 = time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)

func sampleSnapshot() snapshot {
	type args struct{ Target string }
	return snapshot{
		Now: t0.Add(10 * time.Second),
		Nodes: []node{
			{ID: "checkout", Status: Finished, StartedAt: t0, EndedAt: t0.Add(time.Second)},
			{ID: "build-api", Deps: []string{"checkout"}, Priority: 1, Status: Running, StartedAt: t0.Add(time.Second)},
			{ID: "build-web", Name: "前端建置", Deps: []string{"checkout"}, Priority: 1, Status: Failed, Message: "exit 2", StartedAt: t0.Add(time.Second), EndedAt: t0.Add(4 * time.Second), Attempts: 2,
				fn: Wrap(func(context.Context, *T, args) error { return nil }, args{Target: "web"})},
			{ID: "approval", Deps: []string{"build-api", "build-web"}, Priority: 2, Status: Pending, Message: "waiting for release manager 🚀", StartedAt: t0.Add(5 * time.Second)},
			{ID: "deploy", Deps: []string{"approval", "checkout"}, Priority: 3},
		},
		Logs: []LogEntry{
			{Time: t0, NodeID: "checkout", Text: "finished in 1.0s"},
			{Time: t0.Add(4 * time.Second), NodeID: "build-web", Level: LevelError, Text: "failed\n\tstack line 1\n\tstack line 2"},
			{Time: t0.Add(5 * time.Second), NodeID: "approval", Text: "pending"},
		},
	}
}

// wideSnapshot 是一條比畫面寬很多的 pipeline: 12 個 stage 串成一列.
func wideSnapshot() snapshot {
	s := snapshot{Now: t0}
	prev := ""
	for i := range 12 {
		n := node{ID: fmt.Sprintf("step-%02d", i)}
		if prev != "" {
			n.Deps = []string{prev}
		}
		s.Nodes = append(s.Nodes, n)
		prev = n.ID
	}
	return s
}

// assertFrame 確認畫面剛好 w x h, 每一列的顯示寬度都是 w.
func assertFrame(t *testing.T, frame string, w, h int) {
	t.Helper()
	lines := strings.Split(frame, "\n")
	if len(lines) != h {
		t.Fatalf("height: got %d rows want %d", len(lines), h)
	}
	for i, l := range lines {
		if got := textWidth(l); got != w {
			t.Fatalf("row %d width %d want %d: %q", i, got, w, plain(l))
		}
	}
}

func TestRenderFrameExactSize(t *testing.T) {
	sizes := [][2]int{{40, 10}, {80, 24}, {120, 40}, {200, 60}, {30, 5}}
	for _, snap := range []snapshot{sampleSnapshot(), wideSnapshot()} {
		for _, layout := range []Layout{LayoutDependency, LayoutPriority, LayoutTimeline} {
			for _, sz := range sizes {
				t.Run(fmt.Sprintf("%s/%dx%d", layout, sz[0], sz[1]), func(t *testing.T) {
					ctx := viewContext{Snap: snap, Layout: layout, Width: sz[0], Height: sz[1], GraphHeight: DEFAULT_GRAPH_HEIGHT, LogHeight: DEFAULT_LOG_HEIGHT, Selected: "build-web", ScrollX: 9999, ScrollY: 9999}
					assertFrame(t, renderFrame(ctx), sz[0], ctx.frame().Height)
				})
			}
		}
	}
}

func TestRenderFrameEmptyPipeline(t *testing.T) {
	ctx := viewContext{Width: 80, Height: 40, GraphHeight: 15, LogHeight: 5}
	frame := renderFrame(ctx)
	assertFrame(t, frame, 80, 23)
	if !strings.Contains(plain(frame), "no nodes yet") {
		t.Fatal("empty state should say there are no nodes")
	}
}

func TestPlanFrame(t *testing.T) {
	cases := []struct{ term, graph, logs, wantGraph, wantLogs, wantHeight int }{
		{60, 15, 5, 15, 5, 23},   // 高終端機: 只用需要的列數, 不撐滿
		{20, 15, 5, 12, 5, 20},   // 先縮 graph
		{8, 15, 5, 3, 2, 8},      // graph 到底線後再縮 log
		{40, 1, 0, 3, 0, 6},      // graph 不低於 MIN_GRAPH_ROWS
		{40, 30, 10, 27, 10, 40}, // 拖大後剛好塞滿
	}
	for _, c := range cases {
		f := planFrame(c.term, c.graph, c.logs)
		if f.Graph != c.wantGraph || f.Logs != c.wantLogs || f.Height != c.wantHeight {
			t.Fatalf("planFrame(%d,%d,%d) = %+v", c.term, c.graph, c.logs, f)
		}
	}
}

func TestGraphDrawsBoxesAndEdges(t *testing.T) {
	ctx := viewContext{Snap: sampleSnapshot(), Width: 160, Height: 40, GraphHeight: 15, LogHeight: 5, Selected: "build-web"}
	rows := renderGraph(ctx, 160, 15)
	body := plain(strings.Join(rows, "\n"))
	for _, want := range []string{"stage 1", "stage 4", "╭", "╯", "├", "┤", "前端建置", "┏", "┫"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in\n%s", want, body)
		}
	}
	// checkout 的右框接出連線, 而 build-api 的左框接入
	header := plain(rows[0])
	if strings.Index(header, "stage 1") > strings.Index(header, "stage 2") {
		t.Fatal("stages must go left to right")
	}
}

func TestGraphViewportPansHorizontally(t *testing.T) {
	ctx := viewContext{Snap: wideSnapshot(), Width: 80, Height: 40, GraphHeight: 15, LogHeight: 5}
	g := buildGraph(ctx.Snap.Nodes, LayoutDependency)
	if g.Width <= 80 {
		t.Fatalf("canvas should be wider than the screen: %d", g.Width)
	}
	first := plain(renderGraph(ctx, 80, 15)[0])
	if !strings.Contains(first, "stage 1") || strings.Contains(first, "stage 12") {
		t.Fatalf("unpanned view: %q", first)
	}
	ctx.ScrollX = g.Width
	last := plain(renderGraph(ctx, 80, 15)[0])
	if !strings.Contains(last, "stage 12") || strings.Contains(last, "stage 1 ") {
		t.Fatalf("panned to the right edge: %q", last)
	}
	if got := plain(renderDivider(ctx, 80, 5)); !strings.Contains(got, fmt.Sprintf("pan %d-%d/%d", g.Width-80, g.Width, g.Width)) {
		t.Fatalf("divider should show pan position: %q", got)
	}
}

func TestDividerShowsSelectedDetail(t *testing.T) {
	ctx := viewContext{Snap: sampleSnapshot(), Width: 200, Height: 40, GraphHeight: 15, LogHeight: 2, Selected: "build-web", LogOffset: 1}
	got := plain(renderDivider(ctx, 200, 2))
	for _, want := range []string{"前端建置", "failed 3.0s", "attempt 2", "params {Target:web}", "exit 2", "[r retry]", "logs:all 5 -1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("divider missing %q: %q", want, got)
		}
	}
	ctx.Selected = "deploy"
	if got := plain(renderDivider(ctx, 200, 2)); !strings.Contains(got, "waiting for approval") || strings.Contains(got, "checkout") {
		t.Fatalf("divider should list only unfinished deps: %q", got)
	}
}

func TestTimelineBarsOverlapForParallelNodes(t *testing.T) {
	snap := sampleSnapshot()
	api := timelineBar(snap.Nodes[1], t0, 10*time.Second, snap.Now, 50)
	web := timelineBar(snap.Nodes[2], t0, 10*time.Second, snap.Now, 50)
	if textWidth(api) != 50 || textWidth(web) != 50 {
		t.Fatal("bar must be exactly w columns")
	}
	if strings.Index(api, "\x1b") != strings.Index(web, "\x1b") {
		t.Fatal("parallel nodes should start at the same column")
	}
}

func TestLogsFilterAndScroll(t *testing.T) {
	ctx := viewContext{Snap: sampleSnapshot(), Width: 60, Height: 20, LogHeight: 2, Selected: "build-web"}
	all := logLines(ctx)
	if len(all) != 5 || strings.Contains(all[2].Text, "\t") {
		t.Fatalf("multi-line entries expand with tabs replaced: %+v", all)
	}
	rows := renderLogs(ctx, 60, 2)
	if !strings.Contains(plain(rows[1]), "pending") {
		t.Fatalf("follow mode shows newest last: %q", plain(rows[1]))
	}
	ctx.LogOffset = 1
	if got := plain(renderLogs(ctx, 60, 2)[1]); !strings.Contains(got, "stack line 2") {
		t.Fatalf("scrolled up by one: %q", got)
	}
	ctx.LogFilter = true
	for _, l := range logLines(ctx) {
		if strings.Contains(l.Text, "approval") {
			t.Fatalf("filter leaked other node: %q", l.Text)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{0: "0s", 850 * time.Millisecond: "850ms", 12300 * time.Millisecond: "12.3s", 245 * time.Second: "4m05s", 62 * time.Minute: "1h02m"}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Fatalf("formatDuration(%v) = %q want %q", d, got, want)
		}
	}
}
