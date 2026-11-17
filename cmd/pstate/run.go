package pstate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Run 依相依順序平行執行 p 的所有節點, 同時以全螢幕 TUI 顯示節點圖與 log.
// 失敗的節點可在畫面上點擊 (或按 r) 以相同參數重跑, 使用者按 q 或 ctx 取消後離開,
// 離開時仍在執行的節點會收到 ctx 取消. 回傳值同 Execute.
//
// TUI 期間不可寫 stdout (含 slog 的預設 handler), 否則畫面會破版; 工作函式請用 T.Log,
// 或把 slog 導到檔案. 滑鼠操作需要開啟 mouse tracking, 期間終端機原生的文字選取會失效.
func Run(ctx context.Context, p *Pipeline, opts ...Option) error {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		p.loop(runCtx, false)
		close(done)
	}()

	prog := tea.NewProgram(newModel(ctx, p, o), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(o.output))
	final, err := prog.Run()
	if err != nil && !errors.Is(err, tea.ErrInterrupted) { // SIGINT 視同使用者離開
		cancel()
		<-done
		return fmt.Errorf("pstate: run tui: %w", err)
	}
	if m, ok := final.(model); ok && o.summary {
		m.refresh()
		fmt.Fprintln(o.output, summaryFrame(m))
	}
	cancel()
	<-done
	return p.result()
}

// summaryFrame 是離開 TUI 後留在終端機的畫面: 不含操作說明列.
func summaryFrame(m model) string {
	frame := strings.Split(m.View(), "\n")
	return strings.Join(frame[:max(len(frame)-1, 0)], "\n")
}
