// Mousescroll probes mouse and trackpad events in the terminal, with a focus
// on horizontal scroll: wheel-left / wheel-right (two-finger swipe on a
// trackpad) pans a strip of panes that is wider than the window, so content
// left and right of the current view can be reached without the keyboard.
//
// Terminals that never emit horizontal wheel events (e.g. macOS Terminal.app)
// can still pan with Shift + vertical wheel, or with h / l and the arrow keys.
//
// Try:
//
//	go run ./cmd/sample/mousescroll
//	go run ./cmd/sample/mousescroll -panes 8 -step 6
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	panes := flag.Int("panes", 5, "number of panes laid out side by side")
	step := flag.Int("step", 4, "columns moved per horizontal wheel event")
	flag.Parse()

	p := tea.NewProgram(
		newModel(*panes, *step),
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mousescroll:", err)
		os.Exit(1)
	}
}
