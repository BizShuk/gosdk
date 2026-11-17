package pstate

import "github.com/charmbracelet/lipgloss"

// 所有顏色集中在這裡, view 不直接寫色碼.
var (
	colorFinished   = lipgloss.Color("2") // green
	colorRunning    = lipgloss.Color("6") // cyan
	colorPending    = lipgloss.Color("3") // yellow
	colorFailed     = lipgloss.Color("1") // red
	colorNotStarted = lipgloss.Color("8") // gray

	styleTitle    = lipgloss.NewStyle().Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(colorNotStarted)
	styleSelected = lipgloss.NewStyle().Reverse(true)
	styleError    = lipgloss.NewStyle().Foreground(colorFailed)
	styleBar      = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("252"))
	styleNodeID   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
)

func statusColor(s Status) lipgloss.Color {
	switch s {
	case Finished:
		return colorFinished
	case Running:
		return colorRunning
	case Pending:
		return colorPending
	case Failed:
		return colorFailed
	default:
		return colorNotStarted
	}
}

func statusStyle(s Status) lipgloss.Style { return lipgloss.NewStyle().Foreground(statusColor(s)) }

// SPINNER 是 Running 節點的動態標記; 刻意只用 ASCII, 避免 CJK locale 下寬度判斷不一致.
var SPINNER = []string{"|", "/", "-", `\`}

// statusMarker 回傳單一字元寬的狀態標記.
func statusMarker(s Status, frame int) string {
	switch s {
	case Finished:
		return "+"
	case Running:
		return SPINNER[frame%len(SPINNER)]
	case Pending:
		return "~"
	case Failed:
		return "x"
	default:
		return "."
	}
}

var (
	styleEdge    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleEdgeHot = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
)

// paintText 以畫布格子的 paint 對應的樣式輸出文字.
func paintText(p paint, s string) string {
	switch p.kind {
	case paintDim:
		return styleEdge.Render(s)
	case paintHot:
		return styleEdgeHot.Render(s)
	case paintBorder:
		return statusStyle(p.status).Render(s)
	case paintBorderSel:
		return statusStyle(p.status).Bold(true).Render(s)
	case paintLabelSel:
		return styleTitle.Underline(true).Render(s)
	default:
		return s
	}
}
