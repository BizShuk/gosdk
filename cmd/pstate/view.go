package pstate

import (
	"fmt"
	"strings"
)

// MIN_GRAPH_ROWS 是 graph 區塊最少保留的列數 (欄位標題 + 一張卡片).
const MIN_GRAPH_ROWS = 3

// viewContext 是繪製一個畫面所需的全部輸入. view 函式只讀它, 不碰 model.
type viewContext struct {
	Snap        snapshot
	Title       string
	Layout      Layout
	Width       int
	Height      int    // 終端機高度; 畫面只用 frameLayout 算出的列數
	GraphHeight int    // graph 區塊想要的列數
	LogHeight   int    // log pane 想要的列數; 0 = 隱藏
	Selected    string // 選取中的節點 ID
	LogFilter   bool   // 只顯示選取節點的 log
	LogOffset   int    // log 往上捲的列數, 0 = 跟隨最新
	ScrollX     int    // 節點圖 viewport 在畫布上的左緣
	ScrollY     int    // 節點圖 viewport 在畫布上的上緣
	Frame       int    // spinner 影格
}

// frameLayout 是畫面由上到下的分配:
// header(1) + graph + divider(1, 選取節點資訊與 log 狀態, 可拖曳) + logs + footer(1, 可拖曳).
type frameLayout struct {
	Graph  int
	Logs   int
	Height int
}

func (f frameLayout) dividerRow() int { return 1 + f.Graph }
func (f frameLayout) footerRow() int  { return f.Height - 1 }

// planFrame 依想要的高度與終端機高度分配列數. 放不下時先縮 graph (不低於 MIN_GRAPH_ROWS), 再縮 log.
func planFrame(termHeight, graph, logs int) frameLayout {
	avail := max(termHeight-3, 0)
	graph, logs = max(graph, MIN_GRAPH_ROWS), max(logs, 0)
	if graph+logs > avail {
		graph = max(avail-logs, MIN_GRAPH_ROWS)
	}
	if graph+logs > avail {
		logs = max(avail-graph, 0)
	}
	graph = min(graph, avail)
	return frameLayout{Graph: graph, Logs: logs, Height: min(graph+logs+3, termHeight)}
}

func (ctx viewContext) frame() frameLayout {
	return planFrame(ctx.Height, ctx.GraphHeight, ctx.LogHeight)
}

// renderFrame 是 View() 唯一的入口, 回傳 Width x frameLayout.Height 的畫面.
func renderFrame(ctx viewContext) string {
	w, f := ctx.Width, ctx.frame()

	rows := make([]string, 0, f.Height)
	rows = append(rows, renderHeader(ctx))
	if ctx.Layout == LayoutTimeline {
		rows = append(rows, renderTimeline(ctx, w, f.Graph)...)
	} else {
		rows = append(rows, renderGraph(ctx, w, f.Graph)...)
	}
	rows = append(rows, renderDivider(ctx, w, f.Logs))
	rows = append(rows, renderLogs(ctx, w, f.Logs)...)
	rows = append(rows, styleDim.Render(fit(" q quit  r/click retry  enter trigger  arrows select  < > pan  option+wheel pan  tab layout  f filter log  [ ] - + size", w)))
	return strings.Join(block(rows, w, f.Height), "\n")
}

var headerOrder = []Status{Running, Pending, Failed, Finished, NotStarted}

func renderHeader(ctx viewContext) string {
	counts := map[Status]int{}
	for _, n := range ctx.Snap.Nodes {
		counts[n.Status]++
	}
	parts := make([]string, 0, len(headerOrder))
	for _, s := range headerOrder {
		parts = append(parts, statusStyle(s).Render(fmt.Sprintf("%s%d %s", statusMarker(s, ctx.Frame), counts[s], s)))
	}
	title := ctx.Title
	if title == "" {
		title = "pstate"
	}
	line := " " + styleTitle.Render(title) + "  " + strings.Join(parts, "  ") + styleDim.Render("  layout:"+ctx.Layout.String())
	return fit(line, ctx.Width)
}

// renderDivider 是 graph 與 log 之間的分隔列: 左側是選取節點的完整資訊, 右側是 log 與捲動狀態.
func renderDivider(ctx viewContext, w, logRows int) string {
	byID := map[string]node{}
	for _, n := range ctx.Snap.Nodes {
		byID[n.ID] = n
	}
	left := ""
	if n, ok := byID[ctx.Selected]; ok {
		left = fmt.Sprintf(" %s %s  %s", statusMarker(n.Status, ctx.Frame), n.Label(), n.Status)
		if !n.StartedAt.IsZero() {
			left += " " + formatDuration(n.Elapsed(ctx.Snap.Now))
		}
		if n.Attempts > 1 {
			left += fmt.Sprintf("  attempt %d", n.Attempts)
		}
		if params := n.fn.describe(); params != "" {
			left += "  params " + params
		}
		var waiting []string
		for _, d := range n.Deps {
			if byID[d].Status != Finished {
				waiting = append(waiting, d)
			}
		}
		if n.Status == NotStarted && len(waiting) > 0 {
			left += "  waiting for " + strings.Join(waiting, ", ")
		}
		if n.Message != "" {
			left += "  | " + n.Message
		}
		switch n.Status {
		case Failed:
			left += "  [r retry]"
		case Pending:
			left += "  [enter trigger]"
		}
	}

	scope := "all"
	if ctx.LogFilter {
		scope = ctx.Selected
	}
	lines := len(logLines(ctx))
	mode := "follow"
	if off := clampLogOffset(ctx.LogOffset, lines, logRows); off > 0 {
		mode = fmt.Sprintf("-%d", off)
	}
	right := fmt.Sprintf(" logs:%s %d %s ", scope, lines, mode)
	if logRows == 0 {
		right = " logs hidden "
	}
	if ctx.Layout != LayoutTimeline && len(ctx.Snap.Nodes) > 0 {
		g := buildGraph(ctx.Snap.Nodes, ctx.Layout)
		if g.Width > w {
			right = fmt.Sprintf(" pan %d-%d/%d", clampScroll(ctx.ScrollX, g.Width, w), min(clampScroll(ctx.ScrollX, g.Width, w)+w, g.Width), g.Width) + right
		}
	}
	left = clip(left, max(w-textWidth(right)-1, 0))
	return styleBar.Render(fit(left, w-textWidth(right)) + right)
}
