package pstate

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel 建立已執行完的 pipeline: build 第一次失敗, deploy 因此沒跑.
func newTestModel(t *testing.T) (model, *Pipeline, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	p := New()
	must(t, p.Add("checkout", ok()))
	must(t, p.Add("lint", ok(), After("checkout")))
	must(t, p.Add("build", Do(func(context.Context, *T) error {
		if calls.Add(1) == 1 {
			return errors.New("exit 2")
		}
		return nil
	}), After("checkout")))
	must(t, p.Add("deploy", ok(), After("lint", "build")))
	_ = p.Execute(context.Background())

	m := newModel(context.Background(), p, defaultOptions())
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m, p, &calls
}

func update(m model, msg tea.Msg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

func key(s string) tea.KeyMsg {
	switch s {
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func mouse(action tea.MouseAction, button tea.MouseButton, x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: action, Button: button}
}

// screenOf 回傳節點框中心在螢幕上的座標 (graph 從第 1 列開始).
func screenOf(t *testing.T, m model, id string) (x, y int) {
	t.Helper()
	g := buildGraph(m.snap.Nodes, m.layout)
	i, ok := g.find(id)
	if !ok {
		t.Fatalf("%s not in graph", id)
	}
	bx, by, bw := g.rect(i)
	f := m.viewContext().frame()
	return bx + bw/2 - clampScroll(m.scrollX, g.Width, m.width), by + 1 + 1 - clampScroll(m.scrollY, g.Height, f.Graph)
}

func TestClickSelectsThenRetriesFailedNode(t *testing.T) {
	m, p, calls := newTestModel(t)
	x, y := screenOf(t, m, "build")
	m, _ = update(m, mouse(tea.MouseActionPress, tea.MouseButtonLeft, x, y))
	if m.selected != "build" {
		t.Fatalf("first click selects: %q", m.selected)
	}
	if status(t, p, "build").Status != Failed {
		t.Fatal("first click must not retry")
	}
	m, _ = update(m, mouse(tea.MouseActionPress, tea.MouseButtonLeft, x, y))
	_ = m
	if status(t, p, "build").Status != NotStarted {
		t.Fatal("second click on the selected failed node should queue a retry")
	}
	must(t, p.Execute(context.Background()))
	if calls.Load() != 2 || status(t, p, "deploy").Status != Finished {
		t.Fatal("retry should rerun build and then unblock deploy")
	}
}

func TestRetryKeyAndTriggerKey(t *testing.T) {
	m, p, _ := newTestModel(t)
	m.selected = "build"
	m, _ = update(m, key("r"))
	if status(t, p, "build").Status != NotStarted {
		t.Fatal("r should retry the selected failed node")
	}

	q := New()
	must(t, q.Add("gate", Do(func(ctx context.Context, t *T) error { return t.Wait(ctx, "approve") })))
	done := make(chan error, 1)
	go func() { done <- q.Execute(context.Background()) }()
	eventually(t, func() bool { return status(t, q, "gate").Status == Pending })
	gm := newModel(context.Background(), q, defaultOptions())
	gm, _ = update(gm, key("enter"))
	must(t, <-done)
}

func TestShiftWheelPansWideGraph(t *testing.T) {
	p := New()
	prev := ""
	for i := range 12 {
		id := fmt.Sprintf("step-%02d", i)
		if prev == "" {
			must(t, p.Add(id, ok()))
		} else {
			must(t, p.Add(id, ok(), After(prev)))
		}
		prev = id
	}
	m := newModel(context.Background(), p, defaultOptions())
	m, _ = update(m, tea.WindowSizeMsg{Width: 80, Height: 30})

	m, _ = update(m, tea.MouseMsg{X: 10, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, Shift: true})
	if m.scrollX != PAN_STEP {
		t.Fatalf("shift+wheel down should pan right: %d", m.scrollX)
	}
	m, _ = update(m, tea.MouseMsg{X: 10, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, Shift: true})
	m, _ = update(m, tea.MouseMsg{X: 10, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, Shift: true})
	if m.scrollX != 0 {
		t.Fatalf("pan must stop at the left edge: %d", m.scrollX)
	}
	m, _ = update(m, mouse(tea.MouseActionPress, tea.MouseButtonWheelRight, 10, 5))
	if m.scrollX != PAN_STEP {
		t.Fatalf("horizontal wheel should pan too: %d", m.scrollX)
	}

	for range 11 {
		m, _ = update(m, key("right"))
	}
	g := buildGraph(m.snap.Nodes, m.layout)
	if m.selected != "step-11" || m.scrollX != g.Width-80 {
		t.Fatalf("keyboard selection should reveal the last node: selected %q scrollX %d want %d", m.selected, m.scrollX, g.Width-80)
	}
}

func TestDragBarsResizePanes(t *testing.T) {
	m, _, _ := newTestModel(t) // 120x30, graph 15 → divider 第 16 列, footer 第 22 列
	m, _ = update(m, mouse(tea.MouseActionPress, tea.MouseButtonLeft, 10, 16))
	m, _ = update(m, mouse(tea.MouseActionMotion, tea.MouseButtonLeft, 10, 10))
	m, _ = update(m, mouse(tea.MouseActionRelease, tea.MouseButtonNone, 10, 10))
	if m.graphHeight != 9 {
		t.Fatalf("graph height %d want 9", m.graphHeight)
	}
	footer := m.viewContext().frame().footerRow()
	m, _ = update(m, mouse(tea.MouseActionPress, tea.MouseButtonLeft, 10, footer))
	m, _ = update(m, mouse(tea.MouseActionMotion, tea.MouseButtonLeft, 10, footer+5))
	if m.logHeight != 10 {
		t.Fatalf("log height %d want 10", m.logHeight)
	}
	assertFrame(t, m.View(), 120, m.viewContext().frame().Height)
}

func TestDefaultsAreHalfScreenGraphAndFiveLogRows(t *testing.T) {
	m, _, _ := newTestModel(t)
	if f := m.viewContext().frame(); f.Graph != 15 || f.Logs != 5 {
		t.Fatalf("frame %+v", f)
	}
}

func TestTabCyclesLayoutAndQuit(t *testing.T) {
	m, _, _ := newTestModel(t)
	for _, want := range []Layout{LayoutPriority, LayoutTimeline, LayoutDependency} {
		m, _ = update(m, key("tab"))
		if m.layout != want {
			t.Fatalf("layout %v want %v", m.layout, want)
		}
	}
	_, cmd := update(m, key("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should quit")
	}
}

func TestContextCancelQuits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := newModel(ctx, New(), defaultOptions())
	cancel()
	if _, ok := m.waitChanged()().(tea.QuitMsg); !ok {
		t.Fatal("cancelled context should quit")
	}
}

func wideModel(t *testing.T, width int) model {
	t.Helper()
	p := New()
	prev := ""
	for i := range 12 {
		id := fmt.Sprintf("step-%02d", i)
		opts := []NodeOption{}
		if prev != "" {
			opts = append(opts, After(prev))
		}
		must(t, p.Add(id, ok(), opts...))
		prev = id
	}
	m := newModel(context.Background(), p, defaultOptions())
	m, _ = update(m, tea.WindowSizeMsg{Width: width, Height: 30})
	return m
}

func TestArrowKeysScrollWithSelection(t *testing.T) {
	m := wideModel(t, 80)
	prev := m.scrollX
	moved := 0
	for range 11 {
		m, _ = update(m, key("right"))
		if m.scrollX > prev {
			moved++
		}
		prev = m.scrollX
	}
	if moved < 8 {
		t.Fatalf("right arrow should scroll on most steps once the graph overflows; scrolled %d of 11", moved)
	}
	for range 3 { // 最右端的幾欄置中時都會被夾在右緣, 多按幾次才離開
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyLeft})
	}
	if m.scrollX >= prev {
		t.Fatal("left arrow should scroll back")
	}
}

func TestPanKeysIgnoreSelection(t *testing.T) {
	m := wideModel(t, 80)
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyShiftRight})
	m, _ = update(m, key(">"))
	if m.scrollX != 2*PAN_STEP || m.selected != "step-00" {
		t.Fatalf("pan keys move the view only: scrollX %d selected %q", m.scrollX, m.selected)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyShiftLeft})
	m, _ = update(m, key("<"))
	m, _ = update(m, key("<"))
	if m.scrollX != 0 {
		t.Fatalf("pan stops at the left edge: %d", m.scrollX)
	}
}

func TestWheelFallbacksPanHorizontally(t *testing.T) {
	m := wideModel(t, 80) // 一列 12 個節點: 高度放得下, 寬度放不下
	m, _ = update(m, tea.MouseMsg{X: 10, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if m.scrollX != PAN_STEP {
		t.Fatalf("plain wheel pans when the graph fits vertically: %d", m.scrollX)
	}
	m, _ = update(m, tea.MouseMsg{X: 10, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, Alt: true})
	if m.scrollX != 2*PAN_STEP {
		t.Fatalf("option+wheel pans: %d", m.scrollX)
	}
}
