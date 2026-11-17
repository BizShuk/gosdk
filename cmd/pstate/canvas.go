package pstate

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// 連線方向位元: 一格同時連向哪些鄰格, 決定要畫成哪個 box-drawing 字元.
const (
	linkL uint8 = 1 << iota
	linkR
	linkU
	linkD
)

var linkRunes = map[uint8]rune{
	linkL: '─', linkR: '─', linkL | linkR: '─',
	linkU: '│', linkD: '│', linkU | linkD: '│',
	linkR | linkD: '┌', linkL | linkD: '┐', linkR | linkU: '└', linkL | linkU: '┘',
	linkL | linkR | linkD: '┬', linkL | linkR | linkU: '┴',
	linkU | linkD | linkR: '├', linkU | linkD | linkL: '┤',
	linkL | linkR | linkU | linkD: '┼',
}

// paint 是一格的樣式; 可比較, 讓 render 把相同樣式的連續格子合併成一段.
type paint struct {
	kind   paintKind
	status Status
}

type paintKind uint8

const (
	paintNone      paintKind = iota
	paintDim                 // 欄位標題, 一般連線
	paintHot                 // 與選取節點相連的連線
	paintBorder              // 節點框線與狀態標記, 顏色依 status
	paintBorderSel           // 選取節點的框線
	paintLabelSel            // 選取節點的名稱
)

const wideTail rune = -1 // 寬字元的第二格

type cell struct {
	r     rune
	links uint8
	paint paint
}

// canvas 是可以比畫面大的虛擬畫布, render 時只輸出 viewport 範圍.
type canvas struct {
	w, h  int
	cells []cell
}

func newCanvas(w, h int) *canvas {
	return &canvas{w: w, h: h, cells: make([]cell, max(w*h, 0))}
}

func (c *canvas) at(x, y int) *cell {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return nil
	}
	return &c.cells[y*c.w+x]
}

// text 從 (x, y) 寫入一段文字, 最多 limit 欄; 寬字元佔兩格.
func (c *canvas) text(x, y int, s string, limit int, p paint) {
	end := x + limit
	for _, r := range s {
		rw := ansi.StringWidth(string(r))
		if rw == 0 {
			continue
		}
		if x+rw > end {
			return
		}
		if cl := c.at(x, y); cl != nil {
			cl.r, cl.paint = r, p
		}
		if rw == 2 {
			if cl := c.at(x+1, y); cl != nil {
				cl.r, cl.paint = wideTail, p
			}
		}
		x += rw
	}
}

func (c *canvas) set(x, y int, r rune, p paint) {
	if cl := c.at(x, y); cl != nil {
		cl.r, cl.paint = r, p
	}
}

// link 在兩個相鄰格子之間加上連線; hot 的連線會蓋過一般連線的顏色.
func (c *canvas) link(x1, y1, x2, y2 int, hot bool) {
	a, b := c.at(x1, y1), c.at(x2, y2)
	if a == nil || b == nil {
		return
	}
	switch {
	case x2 == x1+1:
		a.links |= linkR
		b.links |= linkL
	case x2 == x1-1:
		a.links |= linkL
		b.links |= linkR
	case y2 == y1+1:
		a.links |= linkD
		b.links |= linkU
	case y2 == y1-1:
		a.links |= linkU
		b.links |= linkD
	}
	for _, cl := range []*cell{a, b} {
		if hot {
			cl.paint = paint{kind: paintHot}
		} else if cl.paint.kind == paintNone {
			cl.paint = paint{kind: paintDim}
		}
	}
}

func (c *canvas) hline(x1, x2, y int, hot bool) {
	step := sign(x2 - x1)
	for x := x1; x != x2; x += step {
		c.link(x, y, x+step, y, hot)
	}
}

func (c *canvas) vline(x, y1, y2 int, hot bool) {
	step := sign(y2 - y1)
	for y := y1; y != y2; y += step {
		c.link(x, y, x, y+step, hot)
	}
}

// render 輸出從 (x0, y0) 起 w x h 的範圍, 每列剛好 w 欄.
func (c *canvas) render(x0, y0, w, h int, style func(paint, string) string) []string {
	out := make([]string, h)
	for row := range h {
		var b strings.Builder
		var run strings.Builder
		cur := paint{}
		flush := func() {
			if run.Len() > 0 {
				b.WriteString(style(cur, run.String()))
				run.Reset()
			}
		}
		for col := 0; col < w; col++ {
			r, p := ' ', paint{}
			if cl := c.at(x0+col, y0+row); cl != nil {
				r, p = cl.r, cl.paint
				switch {
				case r == wideTail:
					if col > 0 {
						continue // 已由前一格的寬字元輸出
					}
					r = ' '
				case r == 0 && cl.links != 0:
					r = linkRunes[cl.links]
				case r == 0:
					r = ' '
				case ansi.StringWidth(string(r)) == 2 && col == w-1:
					r = ' ' // 寬字元在右緣只剩一格, 不輸出半個字
				}
			}
			if p != cur {
				flush()
				cur = p
			}
			run.WriteRune(r)
		}
		flush()
		out[row] = b.String()
	}
	return out
}
