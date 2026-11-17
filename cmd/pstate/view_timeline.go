package pstate

import (
	"strings"
	"time"
)

// timelineWindow 回傳甘特圖在 h 列 (含時間軸一列) 內顯示的節點, 以選取節點為中心.
func timelineWindow(ctx viewContext, h int) []node {
	rows := timelineOrder(ctx.Snap.Nodes)
	visible := max(h-1, 1)
	sel := 0
	for i, n := range rows {
		if n.ID == ctx.Selected {
			sel = i
		}
	}
	first := windowStart(sel, visible, len(rows))
	return rows[first:min(first+visible, len(rows))]
}

// windowStart 回傳以 cursor 為中心, 長度 visible 的視窗起點.
func windowStart(cursor, visible, total int) int {
	return max(min(cursor-visible/2, total-visible), 0)
}

// renderTimeline 繪製甘特圖: 每個節點一列, 橫軸是時間, 平行的節點在同一段時間重疊.
func renderTimeline(ctx viewContext, w, h int) []string {
	if len(ctx.Snap.Nodes) == 0 {
		return emptyGraph(w, h)
	}
	now := ctx.Snap.Now
	nameW := 8
	for _, n := range ctx.Snap.Nodes {
		nameW = max(nameW, textWidth(n.Label())+4)
	}
	nameW = min(nameW, 28, w/3)
	const durW = 8
	barW := max(w-nameW-durW-2, 1)

	t0, t1 := timeSpan(ctx.Snap.Nodes, now)
	total := t1.Sub(t0)
	if total <= 0 {
		total = time.Millisecond
	}

	axis := ""
	if !t0.IsZero() {
		left, right := t0.Format("15:04:05"), "+"+formatDuration(total)
		axis = left + strings.Repeat(" ", max(barW-textWidth(left)-textWidth(right), 1)) + right
	}
	out := []string{styleDim.Render(fit("", nameW) + " " + fit(axis, barW))}

	for _, n := range timelineWindow(ctx, h) {
		gutter := " "
		label := clip(n.Label(), nameW-3)
		if n.ID == ctx.Selected {
			gutter = statusStyle(n.Status).Bold(true).Render("|")
			label = styleTitle.Underline(true).Render(label)
		}
		name := gutter + statusStyle(n.Status).Render(statusMarker(n.Status, ctx.Frame)) + " " + label
		dur := ""
		if !n.StartedAt.IsZero() {
			dur = formatDuration(n.Elapsed(now))
		}
		out = append(out, fit(name, nameW)+" "+timelineBar(n, t0, total, now, barW)+" "+fit(strings.Repeat(" ", max(durW-len(dur), 0))+dur, durW))
	}
	return block(out, w, h)
}

// timelineBar 以背景色畫出節點在時間軸上的區段; 只用空白字元, 寬度在任何 locale 下都是 1.
func timelineBar(n node, t0 time.Time, total time.Duration, now time.Time, w int) string {
	if n.StartedAt.IsZero() {
		return styleDim.Render(fit(" "+n.Status.String(), w))
	}
	end := n.EndedAt
	if end.IsZero() {
		end = now
	}
	x0 := int(float64(n.StartedAt.Sub(t0)) / float64(total) * float64(w))
	x1 := int(float64(end.Sub(t0))/float64(total)*float64(w) + 0.999)
	x0 = min(max(x0, 0), w-1)
	x1 = min(max(x1, x0+1), w)
	return strings.Repeat(" ", x0) + statusStyle(n.Status).Reverse(true).Render(strings.Repeat(" ", x1-x0)) + strings.Repeat(" ", w-x1)
}
