package pstate

import tea "github.com/charmbracelet/bubbletea"

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	page := max(m.viewContext().frame().Logs-1, 1)

	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "tab":
		m.layout = m.layout.next()
	case "shift+tab":
		m.layout = (m.layout + Layout(len(layoutNames)) - 1) % Layout(len(layoutNames))
	case "up", "k":
		m.move(0, -1)
	case "down", "j":
		m.move(0, 1)
	case "left", "h":
		m.move(-1, 0)
	case "right", "l":
		m.move(1, 0)
	case "shift+left", "<", ",":
		m.pan(-PAN_STEP, 0)
	case "shift+right", ">", ".":
		m.pan(PAN_STEP, 0)
	case "r", "enter":
		m.activate()
	case "f":
		m.logFilter = !m.logFilter
		m.logOffset = 0
	case "]":
		m.setGraphHeight(m.graphHeight + 1)
	case "[":
		m.setGraphHeight(m.graphHeight - 1)
	case "+", "=":
		m.setLogHeight(m.logHeight + 1)
	case "-", "_":
		m.setLogHeight(m.logHeight - 1)
	case "pgup", "ctrl+u":
		m.logOffset += page
	case "pgdown", "ctrl+d":
		m.logOffset -= page
	case "home", "g":
		m.logOffset = m.logTotal
	case "end", "G":
		m.logOffset = 0
	default:
		return m, nil
	}
	m.logTotal = len(logLines(m.viewContext()))
	m.clampLog()
	return m, nil
}

// move 移動選取. 節點圖: 往 (dx, dy) 方向最近的節點; 甘特圖: 只有 dy.
func (m *model) move(dx, dy int) {
	if len(m.snap.Nodes) == 0 {
		return
	}
	prev := m.selected
	if m.layout == LayoutTimeline {
		rows := timelineOrder(m.snap.Nodes)
		idx := 0
		for i, n := range rows {
			if n.ID == m.selected {
				idx = i
			}
		}
		m.selected = rows[min(max(idx+dy, 0), len(rows)-1)].ID
	} else {
		m.selected = buildGraph(m.snap.Nodes, m.layout).neighbor(m.selected, dx, dy)
		m.reveal()
	}
	if m.logFilter && m.selected != prev {
		m.logOffset = 0
	}
}
