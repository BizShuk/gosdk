package pstate

import (
	"fmt"
	"time"
)

type borderSet struct{ tl, tr, bl, br, h, v, in, out rune }

var (
	borderLight = borderSet{'╭', '╮', '╰', '╯', '─', '│', '┤', '├'}
	borderHeavy = borderSet{'┏', '┓', '┗', '┛', '━', '┃', '┫', '┣'}
)

func emptyGraph(w, h int) []string {
	return block([]string{"", styleDim.Render(` no nodes yet: call Pipeline.Add(id, pstate.Do(fn))`)}, w, h)
}

// clampScroll 把捲動位移限制在內容範圍內.
func clampScroll(offset, content, view int) int {
	return min(max(offset, 0), max(content-view, 0))
}

// renderGraph 把節點圖畫在虛擬畫布上, 再輸出 (ScrollX, ScrollY) 起的 w x h 視窗.
func renderGraph(ctx viewContext, w, h int) []string {
	if len(ctx.Snap.Nodes) == 0 {
		return emptyGraph(w, h)
	}
	g := buildGraph(ctx.Snap.Nodes, ctx.Layout)
	c := drawGraph(g, ctx)
	x0, y0 := clampScroll(ctx.ScrollX, g.Width, w), clampScroll(ctx.ScrollY, g.Height, h)
	return block(c.render(x0, y0, w, h, paintText), w, h)
}

func drawGraph(g graphLayout, ctx viewContext) *canvas {
	c := newCanvas(g.Width, g.Height)
	for col, t := range g.Titles {
		c.text(g.ColX[col], 0, t, g.ColW[col], paint{kind: paintDim})
	}

	byID := make(map[string]node, len(ctx.Snap.Nodes))
	for _, n := range ctx.Snap.Nodes {
		byID[n.ID] = n
	}
	hasIn, hasOut := map[int]bool{}, map[int]bool{}

	// 每條相依是一串 hop: 來源 → (dummy ...) → 目標; 與選取節點相連的整串標成 hot.
	for t, s := range g.Slots {
		if s.ID == "" {
			continue
		}
		for _, p := range s.Preds {
			chain := []int{t, p}
			for g.Slots[chain[len(chain)-1]].ID == "" {
				chain = append(chain, g.Slots[chain[len(chain)-1]].Preds[0])
			}
			src := g.Slots[chain[len(chain)-1]].ID
			hot := ctx.Selected != "" && (src == ctx.Selected || s.ID == ctx.Selected)
			for k := len(chain) - 1; k > 0; k-- {
				drawHop(c, g, chain[k], chain[k-1], hot)
			}
			for _, d := range chain[1 : len(chain)-1] {
				x, y, w := g.rect(d)
				c.hline(x, x+w-1, y+1, hot)
			}
			hasIn[t], hasOut[chain[len(chain)-1]] = true, true
		}
	}

	for i, s := range g.Slots {
		if s.ID == "" {
			continue
		}
		x, y, w := g.rect(i)
		drawBox(c, x, y, w, byID[s.ID], s.ID == ctx.Selected, ctx.Frame, ctx.Snap.Now, hasIn[i], hasOut[i])
	}
	return c
}

// drawHop 畫相鄰兩欄 slot 之間的一段連線: 出右框 → 目標的主幹上垂直移動 → 進左框.
func drawHop(c *canvas, g graphLayout, from, to int, hot bool) {
	fx, fy, fw := g.rect(from)
	tx, ty, _ := g.rect(to)
	xs, ys, yt := fx+fw-1, fy+1, ty+1
	cx := g.channel(from, to)
	c.hline(xs, cx, ys, hot)
	c.vline(cx, ys, yt, hot)
	c.hline(cx, tx, yt, hot)
}

// drawBox 畫一個 BOX_HEIGHT 列高的節點框: 狀態標記, 名稱, 執行時間. 選取中的節點用粗框.
func drawBox(c *canvas, x, y, w int, n node, selected bool, frame int, now time.Time, in, out bool) {
	b, border := borderLight, paint{kind: paintBorder, status: n.Status}
	if selected {
		b, border = borderHeavy, paint{kind: paintBorderSel, status: n.Status}
	}
	for dx := 1; dx < w-1; dx++ {
		c.set(x+dx, y, b.h, border)
		c.set(x+dx, y+2, b.h, border)
		c.set(x+dx, y+1, ' ', paint{})
	}
	c.set(x, y, b.tl, border)
	c.set(x+w-1, y, b.tr, border)
	c.set(x, y+2, b.bl, border)
	c.set(x+w-1, y+2, b.br, border)
	left, right := b.v, b.v
	if in {
		left = b.in
	}
	if out {
		right = b.out
	}
	c.set(x, y+1, left, border)
	c.set(x+w-1, y+1, right, border)

	inner := w - 4
	dur := ""
	if !n.StartedAt.IsZero() {
		dur = formatDuration(n.Elapsed(now))
	}
	if n.Attempts > 1 {
		dur += fmt.Sprintf(" #%d", n.Attempts)
	}
	c.set(x+2, y+1, []rune(statusMarker(n.Status, frame))[0], border)
	labelPaint := paint{}
	if selected {
		labelPaint = paint{kind: paintLabelSel}
	}
	labelW := inner - 2 - textWidth(dur) - 1
	c.text(x+4, y+1, clip(n.Label(), labelW), labelW, labelPaint)
	c.text(x+2+inner-textWidth(dur), y+1, dur, textWidth(dur), paint{kind: paintDim})
}
