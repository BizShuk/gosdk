package log

import (
	"context"
	"log/slog"
)

// LevelSplitHandler 將日誌分流到 stdout 與 stderr 的自訂處理器。
// >= slog.LevelWarn 的日誌（Warn, Error）會輸出到 stderrHandler，
// 其餘日誌（Debug, Info）則輸出到 stdoutHandler。
type LevelSplitHandler struct {
	stdoutHandler slog.Handler
	stderrHandler slog.Handler
}

// Enabled 根據日誌等級決定是否啟用。我們分別將 Enabled 的判斷交由對應的內部處理器。
func (h *LevelSplitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level >= slog.LevelWarn {
		return h.stderrHandler.Enabled(ctx, level)
	}
	return h.stdoutHandler.Enabled(ctx, level)
}

// Handle 根據日誌等級分流到對應的實體處理器。
func (h *LevelSplitHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		return h.stderrHandler.Handle(ctx, r)
	}
	return h.stdoutHandler.Handle(ctx, r)
}

// WithAttrs 支援帶有 Attrs 的分流處理器。
func (h *LevelSplitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LevelSplitHandler{
		stdoutHandler: h.stdoutHandler.WithAttrs(attrs),
		stderrHandler: h.stderrHandler.WithAttrs(attrs),
	}
}

// WithGroup 支援帶有 Group 的分流處理器。
func (h *LevelSplitHandler) WithGroup(name string) slog.Handler {
	return &LevelSplitHandler{
		stdoutHandler: h.stdoutHandler.WithGroup(name),
		stderrHandler: h.stderrHandler.WithGroup(name),
	}
}
