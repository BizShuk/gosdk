package pstate

import (
	"cmp"
	"slices"
)

// 節點圖的幾何常數 (單位: 終端機欄 / 列).
const (
	BOX_HEIGHT = 3 // 上框 + 內容 + 下框
	ROW_PITCH  = 4 // 一個 slot 佔的列數 (box + 1 列間隔)
	COL_GAP    = 6 // 欄與欄之間的連線通道寬度
	BOX_MIN_W  = 16
	BOX_MAX_W  = 30
	GRAPH_TOP  = 1 // 第 0 列是欄位標題
	GRAPH_LEFT = 1
)

// slot 是欄位中的一格. ID 為空代表 dummy: 跨越多欄的連線在中間欄位佔用的通道.
type slot struct {
	ID    string
	Col   int
	Row   int
	Preds []int // 前一欄的 slot index
}

// graphLayout 是整張圖在虛擬畫布上的位置. 畫布可以比畫面寬, 由 viewport 捲動顯示.
type graphLayout struct {
	Slots  []slot
	Titles []string
	ColX   []int
	ColW   []int
	Width  int
	Height int
}

// buildGraph 把節點排成由左到右的分層圖 (Sugiyama 簡化版):
// 分欄 → 長連線插入 dummy → barycenter 排序減少交叉 → 計算座標.
func buildGraph(nodes []node, l Layout) graphLayout {
	colOf, titles := assignColumns(nodes, l)
	g := graphLayout{Titles: titles}
	if len(nodes) == 0 {
		return g
	}

	index := map[string]int{}
	for _, n := range nodes {
		index[n.ID] = len(g.Slots)
		g.Slots = append(g.Slots, slot{ID: n.ID, Col: colOf[n.ID]})
	}
	for _, n := range nodes {
		to := index[n.ID]
		for _, d := range n.Deps {
			from := index[d]
			if g.Slots[from].Col >= g.Slots[to].Col { // priority 版面可能逆向; 只在 divider 列顯示
				continue
			}
			for c := g.Slots[from].Col + 1; c < g.Slots[to].Col; c++ {
				g.Slots = append(g.Slots, slot{Col: c, Preds: []int{from}})
				from = len(g.Slots) - 1
			}
			g.Slots[to].Preds = append(g.Slots[to].Preds, from)
		}
	}

	g.order(len(titles))
	g.place(nodes, len(titles))
	return g
}

// order 以 barycenter 啟發法反覆排序每一欄, 讓連線的兩端盡量在相近的列.
func (g *graphLayout) order(cols int) {
	byCol := make([][]int, cols)
	for i, s := range g.Slots {
		byCol[s.Col] = append(byCol[s.Col], i)
	}
	succs := make([][]int, len(g.Slots))
	for i, s := range g.Slots {
		for _, p := range s.Preds {
			succs[p] = append(succs[p], i)
		}
	}
	setRows := func(c int) {
		for r, i := range byCol[c] {
			g.Slots[i].Row = r
		}
	}
	for c := range cols {
		setRows(c)
	}
	sortBy := func(c int, neighbors func(i int) []int) {
		key := func(i int) float64 {
			ns := neighbors(i)
			if len(ns) == 0 {
				return float64(g.Slots[i].Row)
			}
			sum := 0
			for _, n := range ns {
				sum += g.Slots[n].Row
			}
			return float64(sum) / float64(len(ns))
		}
		slices.SortStableFunc(byCol[c], func(a, b int) int { return cmp.Compare(key(a), key(b)) })
		setRows(c)
	}
	for range 3 {
		for c := 1; c < cols; c++ {
			sortBy(c, func(i int) []int { return g.Slots[i].Preds })
		}
		for c := cols - 2; c >= 0; c-- {
			sortBy(c, func(i int) []int { return succs[i] })
		}
	}
}

// place 計算各欄寬度與 x 座標: 欄寬依最長的名稱, 介於 BOX_MIN_W 與 BOX_MAX_W.
func (g *graphLayout) place(nodes []node, cols int) {
	label := map[string]string{}
	for _, n := range nodes {
		label[n.ID] = n.Label()
	}
	g.ColW = make([]int, cols)
	g.ColX = make([]int, cols)
	rows := 0
	for c := range g.ColW {
		g.ColW[c] = BOX_MIN_W
	}
	for _, s := range g.Slots {
		rows = max(rows, s.Row+1)
		if s.ID != "" {
			// 框線 2 + 內距 2 + 標記與空白 2 + 時間 7 + 間隔 1
			g.ColW[s.Col] = min(max(g.ColW[s.Col], textWidth(label[s.ID])+14), BOX_MAX_W)
		}
	}
	x := GRAPH_LEFT
	for c := range cols {
		g.ColX[c] = x
		x += g.ColW[c] + COL_GAP
	}
	g.Width = x - COL_GAP + GRAPH_LEFT
	g.Height = GRAPH_TOP + rows*ROW_PITCH
}

// rect 回傳 slot 在畫布上的左上角與寬度.
func (g graphLayout) rect(i int) (x, y, w int) {
	s := g.Slots[i]
	return g.ColX[s.Col], GRAPH_TOP + s.Row*ROW_PITCH, g.ColW[s.Col]
}

// find 回傳節點的 slot index.
func (g graphLayout) find(id string) (int, bool) {
	for i, s := range g.Slots {
		if s.ID != "" && s.ID == id {
			return i, true
		}
	}
	return 0, false
}

// hit 回傳畫布座標 (x, y) 上的節點 ID; 沒有點到節點時回傳空字串.
func (g graphLayout) hit(x, y int) string {
	for i, s := range g.Slots {
		if s.ID == "" {
			continue
		}
		bx, by, bw := g.rect(i)
		if x >= bx && x < bx+bw && y >= by && y < by+BOX_HEIGHT {
			return s.ID
		}
	}
	return ""
}

// trunkKey 決定一段 hop 使用哪條垂直主幹: 多來源的目標用自己的主幹 (fan-in 匯成一條),
// 單一來源的目標沿用來源的主幹 (fan-out 從一條分出去).
func (g graphLayout) trunkKey(from, to int) int {
	if len(g.Slots[to].Preds) > 1 {
		return to
	}
	return from
}

// channel 回傳 from → to 這段 hop 在兩欄之間通道中使用的 x 座標. 同一通道內的不同主幹依列錯開.
func (g graphLayout) channel(from, to int) int {
	col := g.Slots[to].Col
	key := g.trunkKey(from, to)
	keys := map[int]bool{}
	for t, s := range g.Slots {
		if s.Col != col {
			continue
		}
		for _, p := range s.Preds {
			keys[g.trunkKey(p, t)] = true
		}
	}
	k := 0
	for other := range keys {
		if g.Slots[other].Row < g.Slots[key].Row || (g.Slots[other].Row == g.Slots[key].Row && g.Slots[other].Col < g.Slots[key].Col) {
			k++
		}
	}
	return g.ColX[col] - COL_GAP + 1 + k%(COL_GAP-2)
}

// neighbor 回傳在 (dx, dy) 方向上最接近 id 的節點, 用於方向鍵導覽; dummy 會被跳過.
func (g graphLayout) neighbor(id string, dx, dy int) string {
	i, ok := g.find(id)
	if !ok {
		return id
	}
	cur := g.Slots[i]
	best, bestDist := id, -1
	for _, s := range g.Slots {
		if s.ID == "" || s.ID == id {
			continue
		}
		var dist int
		switch {
		case dx != 0 && s.Col == cur.Col+dx:
			dist = abs(s.Row - cur.Row)
		case dy != 0 && s.Col == cur.Col && sign(s.Row-cur.Row) == dy:
			dist = abs(s.Row - cur.Row)
		default:
			continue
		}
		if bestDist < 0 || dist < bestDist {
			best, bestDist = s.ID, dist
		}
	}
	return best
}

func abs(v int) int { return max(v, -v) }

func sign(v int) int { return cmp.Compare(v, 0) }
