package pstate

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// 這是 pstate 唯一量測顯示寬度的地方. 一律先裁切再補空白, ANSI 序列寬度為 0.

// textWidth 回傳字串的顯示欄寬.
func textWidth(s string) int { return ansi.StringWidth(s) }

// fit 把單行字串裁切或補空白到剛好 w 欄.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if textWidth(s) > w {
		s = ansi.Truncate(s, w, "")
	}
	if pad := w - textWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// clip 裁切到最多 w 欄, 超出時以 `~` 結尾提示被截斷; 不補空白.
func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if textWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "~")
}

// block 把多行內容整理成剛好 w x h: 每行 fit, 不足補空行, 超出截掉.
func block(lines []string, w, h int) []string {
	out := make([]string, h)
	blank := strings.Repeat(" ", max(w, 0))
	for i := range h {
		if i < len(lines) {
			out[i] = fit(lines[i], w)
		} else {
			out[i] = blank
		}
	}
	return out
}
