package pstate

import (
	"fmt"
	"slices"
	"time"
)

// Layout 決定節點由左到右的排列依據.
type Layout int

const (
	// LayoutDependency 依相依深度分欄: 沒有上游的在最左, 每往下游一層右移一欄.
	LayoutDependency Layout = iota
	// LayoutPriority 依 Priority 分欄, 數字小的在左.
	LayoutPriority
	// LayoutTimeline 以甘特圖呈現, 橫軸是時間, 平行執行的節點會在同一時段重疊.
	LayoutTimeline
)

var layoutNames = [...]string{"dependency", "priority", "timeline"}

func (l Layout) String() string {
	if l < 0 || int(l) >= len(layoutNames) {
		return fmt.Sprintf("layout(%d)", int(l))
	}
	return layoutNames[l]
}

func (l Layout) next() Layout { return (l + 1) % Layout(len(layoutNames)) }

// assignColumns 依 layout 決定每個節點的欄位 (由左到右從 0 起算) 與各欄標題.
func assignColumns(nodes []node, l Layout) (colOf map[string]int, titles []string) {
	keys := map[string]int{}
	if l == LayoutPriority {
		for _, n := range nodes {
			keys[n.ID] = n.Priority
		}
	} else {
		keys = depths(nodes)
	}
	uniq := make([]int, 0, len(keys))
	for _, k := range keys {
		uniq = append(uniq, k)
	}
	slices.Sort(uniq)
	uniq = slices.Compact(uniq)

	colOf = make(map[string]int, len(nodes))
	for id, k := range keys {
		colOf[id], _ = slices.BinarySearch(uniq, k)
	}
	titles = make([]string, len(uniq))
	for i, k := range uniq {
		if l == LayoutPriority {
			titles[i] = fmt.Sprintf("priority %d", k)
		} else {
			titles[i] = fmt.Sprintf("stage %d", k+1)
		}
	}
	return colOf, titles
}

// depths 計算每個節點的相依深度. Add 保證上游先登記, 因此依登記順序一次即可算完.
func depths(nodes []node) map[string]int {
	d := make(map[string]int, len(nodes))
	for _, n := range nodes {
		for _, dep := range n.Deps {
			d[n.ID] = max(d[n.ID], d[dep]+1)
		}
		if _, ok := d[n.ID]; !ok {
			d[n.ID] = 0
		}
	}
	return d
}

// timelineOrder 回傳甘特圖的列順序: 已開始的依開始時間, 未開始的接在後面並維持登記順序.
func timelineOrder(nodes []node) []node {
	out := slices.Clone(nodes)
	slices.SortStableFunc(out, func(a, b node) int {
		switch {
		case a.StartedAt.IsZero() && b.StartedAt.IsZero():
			return 0
		case a.StartedAt.IsZero():
			return 1
		case b.StartedAt.IsZero():
			return -1
		}
		return a.StartedAt.Compare(b.StartedAt)
	})
	return out
}

// timeSpan 回傳甘特圖橫軸的起訖. 沒有節點開始過時回傳零值.
func timeSpan(nodes []node, now time.Time) (start, end time.Time) {
	for _, n := range nodes {
		if n.StartedAt.IsZero() {
			continue
		}
		if start.IsZero() || n.StartedAt.Before(start) {
			start = n.StartedAt
		}
		e := n.EndedAt
		if e.IsZero() {
			e = now
		}
		if e.After(end) {
			end = e
		}
	}
	return start, end
}
