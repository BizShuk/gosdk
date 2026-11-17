package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func send(m model, msgs ...tea.Msg) model {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(model)
	}
	return m
}

func wheel(b tea.MouseButton, shift bool) tea.MouseMsg {
	return tea.MouseMsg{Button: b, Action: tea.MouseActionPress, Shift: shift}
}

func TestHorizontalWheelPans(t *testing.T) {
	m := send(newModel(3, 4), tea.WindowSizeMsg{Width: 40, Height: 30})

	m = send(m, wheel(tea.MouseButtonWheelRight, false), wheel(tea.MouseButtonWheelRight, false))
	eq(t, m.offset, 8)

	m = send(m, wheel(tea.MouseButtonWheelLeft, false))
	eq(t, m.offset, 4)
	eq(t, m.counts[tea.MouseButtonWheelRight], 2)
}

func TestShiftVerticalWheelPans(t *testing.T) {
	m := send(newModel(3, 4), tea.WindowSizeMsg{Width: 40, Height: 30})
	m = send(m, wheel(tea.MouseButtonWheelDown, true))
	eq(t, m.offset, 4)

	m = send(m, wheel(tea.MouseButtonWheelDown, false)) // plain vertical wheel does not pan
	eq(t, m.offset, 4)
}

func TestPanClampsToStrip(t *testing.T) {
	m := send(newModel(3, 100), tea.WindowSizeMsg{Width: 40, Height: 30})
	m = send(m, wheel(tea.MouseButtonWheelLeft, false))
	eq(t, m.offset, 0)

	m = send(m, wheel(tea.MouseButtonWheelRight, false), wheel(tea.MouseButtonWheelRight, false))
	eq(t, m.offset, 80) // 2 panes of 40 columns to the right
	eq(t, m.currentPane(), 2)
}

func TestClickSnapsToNearestPane(t *testing.T) {
	m := send(newModel(3, 4), tea.WindowSizeMsg{Width: 40, Height: 30})
	m.offset = 28 // pane 2 covers the centre
	m = send(m, tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	eq(t, m.offset, 40)
}

func TestViewFitsWindowWidth(t *testing.T) {
	m := send(newModel(4, 4), tea.WindowSizeMsg{Width: 50, Height: 30})
	m = send(m, wheel(tea.MouseButtonWheelRight, false))
	for i, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > 50 {
			t.Errorf("line %d width %d > 50", i, w)
		}
	}
}

func eq(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}
