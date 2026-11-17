package pstate

import tea "github.com/charmbracelet/bubbletea"

// dragTarget 是正在被拖曳的邊界. 每個邊界都是它上方區塊的下緣.
type dragTarget int

const (
	dragNone  dragTarget = iota
	dragGraph            // divider 列: graph 的下緣
	dragLogs             // footer 列: log pane 的下緣
)

// PAN_STEP 是滾輪一格平移的欄數.
const PAN_STEP = 6

// handleMouse 處理點擊選取 / 重試, 拖曳調整高度與滾輪. 位置一律以 msg.Y 判斷 (hit-test).
func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	f := m.viewContext().frame()
	inGraph := msg.Y >= 1 && msg.Y < f.dividerRow()
	inLogs := msg.Y > f.dividerRow() && msg.Y < f.footerRow()

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		switch {
		case msg.Y == f.dividerRow():
			m.drag = dragGraph
		case msg.Y == f.footerRow():
			m.drag = dragLogs
		case inGraph:
			m.click(msg.X, msg.Y-1)
		}
	case msg.Action == tea.MouseActionMotion && m.drag != dragNone:
		if m.drag == dragGraph {
			m.setGraphHeight(msg.Y - 1)
		} else {
			m.setLogHeight(msg.Y - f.dividerRow() - 1)
		}
	case msg.Action == tea.MouseActionRelease:
		m.drag = dragNone
	case msg.Button == tea.MouseButtonWheelLeft || msg.Button == tea.MouseButtonWheelRight:
		if inGraph {
			m.pan(wheelStep(msg)*PAN_STEP, 0)
		}
	case msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown:
		switch {
		case inLogs:
			m.logOffset -= wheelStep(msg) * 3
		case inGraph && m.layout == LayoutTimeline:
			m.move(0, wheelStep(msg))
		case inGraph && (msg.Shift || msg.Alt || !m.graphOverflowsVertically()):
			// VS Code 系列的 xterm.js 會丟掉帶 shift 的滾輪 (consumeWheelEvent: shiftKey → 0),
			// 只有 option+wheel 送得進來, 因此 Alt 也算; 圖高度放得下時, 上下滾輪本來無事可做, 直接當左右平移.
			m.pan(wheelStep(msg)*PAN_STEP, 0)
		case inGraph:
			m.pan(0, wheelStep(msg)*2)
		}
	}
	m.logTotal = len(logLines(m.viewContext()))
	m.clampLog()
	return m, nil
}

// wheelStep 把滾輪方向轉成 -1 (上 / 左) 或 +1 (下 / 右).
func wheelStep(msg tea.MouseMsg) int {
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelLeft {
		return -1
	}
	return 1
}

// click 處理 graph 區塊內 (x, y) 的點擊: 點到未選取的節點就選取, 點已選取的節點就 activate (重試 / 觸發).
func (m *model) click(x, y int) {
	id := ""
	if m.layout == LayoutTimeline {
		rows := timelineWindow(m.viewContext(), m.viewContext().frame().Graph)
		if y >= 1 && y-1 < len(rows) {
			id = rows[y-1].ID
		}
	} else {
		g := buildGraph(m.snap.Nodes, m.layout)
		f := m.viewContext().frame()
		id = g.hit(x+clampScroll(m.scrollX, g.Width, m.width), y+clampScroll(m.scrollY, g.Height, f.Graph))
	}
	switch {
	case id == "":
	case id == m.selected:
		m.activate()
	default:
		m.selected = id
		if m.logFilter {
			m.logOffset = 0
		}
	}
}

// graphOverflowsVertically 回報節點圖是否比 graph 區塊高, 也就是上下滾輪是否有東西可捲.
func (m model) graphOverflowsVertically() bool {
	return buildGraph(m.snap.Nodes, m.layout).Height > m.viewContext().frame().Graph
}
