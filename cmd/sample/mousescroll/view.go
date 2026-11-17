package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	paneColors = []lipgloss.Color{"#5f87ff", "#5fd787", "#ffaf5f", "#d75fd7", "#5fd7d7", "#ff5f5f"}
	headStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd75f"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	hitStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#87ff87"))
)

// chromeLines is the height of everything that is not the pane strip:
// header, minimap, blank, log title, log rows, help.
const chromeLines = 5 + logSize

func (m model) View() string {
	stripHeight := max(m.height-chromeLines, 3)

	var b strings.Builder
	b.WriteString(headStyle.Render(fmt.Sprintf(
		"pane %d/%d  offset %d/%d  wheel-left %d  wheel-right %d",
		m.currentPane()+1, m.panes, m.offset, m.maxOffset(),
		m.counts[tea.MouseButtonWheelLeft], m.counts[tea.MouseButtonWheelRight],
	)))
	b.WriteString("\n" + m.minimap() + "\n")

	for _, line := range m.strip(stripHeight) {
		b.WriteString(ansi.Cut(line, m.offset, m.offset+m.width) + "\n")
	}

	b.WriteString("\n" + dimStyle.Render("recent mouse events (motion without button is counted, not logged)") + "\n")
	for i := range logSize {
		line := ""
		if i < len(m.events) {
			line = m.events[i]
			if strings.Contains(line, "wheel left") || strings.Contains(line, "wheel right") {
				line = hitStyle.Render(line)
			}
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(dimStyle.Render("wheel ←/→ or shift+wheel: pan  h/l: pan  H/L: prev/next pane  click: snap  q: quit"))

	lines := strings.Split(b.String(), "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "")
	}
	return strings.Join(lines, "\n")
}

// strip renders every pane side by side as full-width lines; View cuts the
// visible window out of it.
func (m model) strip(height int) []string {
	w := m.paneWidth()
	cols := make([]string, m.panes)
	for i := range m.panes {
		color := paneColors[i%len(paneColors)]
		box := lipgloss.NewStyle().
			Width(w - 2).Height(height - 2).
			Border(lipgloss.RoundedBorder()).BorderForeground(color).
			Foreground(color)
		cols[i] = box.Render(paneBody(i, w-2))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, cols...), "\n")
}

func paneBody(i, width int) string {
	ruler := strings.Repeat(fmt.Sprintf("%d", (i+1)%10), max(width-2, 1))
	return fmt.Sprintf(" PANE %d\n\n %s\n\n Swipe two fingers left/right on the trackpad,\n or tilt the mouse wheel, to reach the neighbouring pane.", i+1, ruler)
}

// minimap shows which part of the strip is visible: one cell per pane,
// filled where the window currently overlaps it.
func (m model) minimap() string {
	const cell = 6
	total := cell * m.panes
	from := m.offset * total / (m.paneWidth() * m.panes)
	to := (m.offset + m.width) * total / (m.paneWidth() * m.panes)
	var b strings.Builder
	for c := range total {
		color := paneColors[(c/cell)%len(paneColors)]
		ch := "░"
		if c >= from && c < to {
			ch = "█"
		}
		b.WriteString(lipgloss.NewStyle().Foreground(color).Render(ch))
	}
	return b.String()
}
