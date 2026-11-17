package pstate

import "strings"

const LOG_ID_WIDTH = 12

// logLine 是 log pane 的一列; 一筆多行訊息會展開成多列.
type logLine struct {
	Text  string
	Level Level
}

// logLines 依篩選條件把紀錄展開成畫面列, model 也用它計算可捲動範圍.
func logLines(ctx viewContext) []logLine {
	var out []logLine
	indent := strings.Repeat(" ", 8+1+LOG_ID_WIDTH+1)
	for _, e := range ctx.Snap.Logs {
		if ctx.LogFilter && e.NodeID != ctx.Selected {
			continue
		}
		prefix := e.Time.Format("15:04:05") + " " + fit(clip(e.NodeID, LOG_ID_WIDTH), LOG_ID_WIDTH) + " "
		text := strings.ReplaceAll(strings.TrimRight(e.Text, "\n"), "\t", "    ")
		for i, part := range strings.Split(text, "\n") {
			p := indent
			if i == 0 {
				p = prefix
			}
			out = append(out, logLine{Text: p + part, Level: e.Level})
		}
	}
	return out
}

// clampLogOffset 把往上捲的列數限制在可捲動範圍內.
func clampLogOffset(offset, total, visible int) int {
	return min(max(offset, 0), max(total-visible, 0))
}

// renderLogs 繪製 log pane 的 h 列內容; 狀態顯示在 renderDivider.
func renderLogs(ctx viewContext, w, h int) []string {
	if h <= 0 {
		return nil
	}
	lines := logLines(ctx)
	offset := clampLogOffset(ctx.LogOffset, len(lines), h)
	end := len(lines) - offset
	start := max(end-h, 0)

	out := make([]string, 0, h)
	for _, l := range lines[start:end] {
		text := clip(l.Text, w)
		if l.Level == LevelError {
			text = styleError.Render(text)
		}
		out = append(out, text)
	}
	return block(out, w, h)
}
