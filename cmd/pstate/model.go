package pstate

import (
	"context"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TICK_INTERVAL 是 spinner 與執行中時間的更新頻率.
const TICK_INTERVAL = 150 * time.Millisecond

type tickMsg time.Time

type changedMsg struct{}

// model 是 bubbletea 的狀態. 只在 Update 內被修改, goroutine 不直接碰它.
type model struct {
	ctx  context.Context
	p    *Pipeline
	opts options

	snap        snapshot
	width       int
	height      int
	layout      Layout
	graphHeight int
	logHeight   int
	selected    string
	logFilter   bool
	logOffset   int
	logTotal    int
	frame       int
	drag        dragTarget
	scrollX     int
	scrollY     int
}

func newModel(ctx context.Context, p *Pipeline, o options) model {
	m := model{
		ctx: ctx, p: p, opts: o,
		width: 100, height: 30, // 第一個 WindowSizeMsg 之前的預設尺寸
		layout: o.layout, graphHeight: o.graphHeight, logHeight: o.logHeight,
	}
	m.refresh()
	return m
}

func (m model) Init() tea.Cmd { return tea.Batch(tick(), m.waitChanged()) }

func tick() tea.Cmd {
	return tea.Tick(TICK_INTERVAL, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// waitChanged 等待下一次狀態變更; 收到後必須重新掛上, 否則畫面會停止更新.
func (m model) waitChanged() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-m.p.changed:
			return changedMsg{}
		case <-m.ctx.Done():
			return tea.QuitMsg{}
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.frame++
		m.refresh()
		return m, tick()
	case changedMsg:
		m.refresh()
		return m, m.waitChanged()
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

func (m model) View() string { return renderFrame(m.viewContext()) }

func (m model) viewContext() viewContext {
	return viewContext{
		Snap: m.snap, Title: m.opts.title, Layout: m.layout,
		Width: m.width, Height: m.height,
		GraphHeight: m.graphHeight, LogHeight: m.logHeight, Selected: m.selected,
		LogFilter: m.logFilter, LogOffset: m.logOffset, Frame: m.frame,
		ScrollX: m.scrollX, ScrollY: m.scrollY,
	}
}

// refresh 取得新的快照, 保留選取, 並在使用者往上捲時維持畫面停在原本的紀錄上.
func (m *model) refresh() {
	m.snap = m.p.snapshot()
	if !slices.ContainsFunc(m.snap.Nodes, func(n node) bool { return n.ID == m.selected }) {
		m.selected = ""
		if len(m.snap.Nodes) > 0 {
			m.selected = m.snap.Nodes[0].ID
		}
	}
	total := len(logLines(m.viewContext()))
	if m.logOffset > 0 {
		m.logOffset += total - m.logTotal
	}
	m.logTotal = total
	m.clampLog()
}

func (m *model) clampLog() {
	m.logOffset = clampLogOffset(m.logOffset, m.logTotal, m.viewContext().frame().Logs)
}

// setGraphHeight / setLogHeight 只接受目前終端機放得下的值, 讓拖曳與按鍵不會累積看不見的高度.
func (m *model) setGraphHeight(rows int) {
	m.graphHeight = min(max(rows, MIN_GRAPH_ROWS), max(m.height-3-m.logHeight, MIN_GRAPH_ROWS))
}

func (m *model) setLogHeight(rows int) {
	graph := m.viewContext().frame().Graph
	m.logHeight = min(max(rows, 0), max(m.height-3-graph, 0))
}

// pan 移動節點圖的 viewport 並限制在畫布範圍內.
func (m *model) pan(dx, dy int) {
	g := buildGraph(m.snap.Nodes, m.layout)
	f := m.viewContext().frame()
	m.scrollX = clampScroll(clampScroll(m.scrollX, g.Width, m.width)+dx, g.Width, m.width)
	m.scrollY = clampScroll(clampScroll(m.scrollY, g.Height, f.Graph)+dy, g.Height, f.Graph)
}

// reveal 捲動 viewport 讓選取節點出現在畫面上: 橫向把它置中 (每次換欄畫面都跟著移動),
// 縱向只在它超出畫面時才捲.
func (m *model) reveal() {
	if m.layout == LayoutTimeline {
		return
	}
	g := buildGraph(m.snap.Nodes, m.layout)
	i, ok := g.find(m.selected)
	if !ok {
		return
	}
	x, y, w := g.rect(i)
	viewW, viewH := m.width, m.viewContext().frame().Graph
	sy := clampScroll(m.scrollY, g.Height, viewH)
	if y-GRAPH_TOP < sy {
		sy = y - GRAPH_TOP
	} else if y+BOX_HEIGHT > sy+viewH {
		sy = y + BOX_HEIGHT - viewH
	}
	m.scrollX = clampScroll(x+w/2-viewW/2, g.Width, viewW)
	m.scrollY = clampScroll(sy, g.Height, viewH)
}

// activate 對選取節點做它目前狀態下唯一合理的動作: 失敗就重試, 等待中就觸發.
func (m *model) activate() {
	for _, n := range m.snap.Nodes {
		if n.ID != m.selected {
			continue
		}
		switch n.Status {
		case Failed:
			_ = m.p.Retry(n.ID)
		case Pending:
			_ = m.p.Trigger(n.ID)
		}
	}
}
