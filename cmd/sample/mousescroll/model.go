package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// logSize is how many recent mouse events the event log keeps.
const logSize = 8

// model holds the horizontal pan state and the recent mouse event log.
type model struct {
	panes  int // panes in the strip
	step   int // columns per horizontal wheel tick
	width  int // terminal width
	height int // terminal height
	offset int // leftmost visible column of the strip

	events []string // newest last, at most logSize
	counts map[tea.MouseButton]int
}

func newModel(panes, step int) model {
	return model{
		panes:  max(panes, 1),
		step:   max(step, 1),
		width:  100, // until the first WindowSizeMsg
		height: 30,
		counts: map[tea.MouseButton]int{},
	}
}

func (m model) Init() tea.Cmd { return nil }

// paneWidth makes every pane as wide as the window, so neighbours are always
// off-screen until the strip is panned.
func (m model) paneWidth() int { return max(m.width, 20) }

func (m model) maxOffset() int { return m.paneWidth() * (m.panes - 1) }

// currentPane is the pane covering the centre of the window.
func (m model) currentPane() int {
	return min((m.offset+m.paneWidth()/2)/m.paneWidth(), m.panes-1)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		pane := m.currentPane()
		m.width, m.height = msg.Width, msg.Height
		m.offset = pane * m.paneWidth() // keep the same pane in view after resize

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "h", "left":
			m.pan(-m.step)
		case "l", "right":
			m.pan(m.step)
		case "H", "shift+left", "pgup":
			m.snap(m.currentPane() - 1)
		case "L", "shift+right", "pgdown":
			m.snap(m.currentPane() + 1)
		case "0", "home":
			m.snap(0)
		case "$", "end":
			m.snap(m.panes - 1)
		}

	case tea.MouseMsg:
		m.record(msg)
		switch dx := horizontalDelta(msg); {
		case dx != 0:
			m.pan(dx * m.step)
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			m.snap(m.currentPane()) // click snaps a half-visible pane into place
		}
	}
	return m, nil
}

// horizontalDelta maps a mouse event to -1 (left), +1 (right) or 0.
// Shift + vertical wheel counts as horizontal for terminals that never send
// wheel-left / wheel-right.
func horizontalDelta(msg tea.MouseMsg) int {
	switch {
	case msg.Button == tea.MouseButtonWheelLeft:
		return -1
	case msg.Button == tea.MouseButtonWheelRight:
		return 1
	case msg.Shift && msg.Button == tea.MouseButtonWheelUp:
		return -1
	case msg.Shift && msg.Button == tea.MouseButtonWheelDown:
		return 1
	}
	return 0
}

func (m *model) pan(dx int) {
	m.offset = min(max(m.offset+dx, 0), m.maxOffset())
}

func (m *model) snap(pane int) {
	m.offset = min(max(pane, 0), m.panes-1) * m.paneWidth()
}

// record appends a mouse event to the log; pure motion only bumps the counter
// so it does not flood the log.
func (m *model) record(msg tea.MouseMsg) {
	m.counts[msg.Button]++
	if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonNone {
		return
	}
	line := fmt.Sprintf("%-28s x=%-4d y=%-4d", msg.String(), msg.X, msg.Y)
	m.events = append(m.events, line)
	if len(m.events) > logSize {
		m.events = m.events[len(m.events)-logSize:]
	}
}
