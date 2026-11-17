package pstate

import (
	"io"
	"os"
)

const (
	// DEFAULT_GRAPH_HEIGHT 是 graph 區塊預設列數, 約半個終端機高; 版面偏向橫向展開.
	DEFAULT_GRAPH_HEIGHT = 15
	// DEFAULT_LOG_HEIGHT 是 log pane 預設列數.
	DEFAULT_LOG_HEIGHT = 5
)

type options struct {
	title       string
	layout      Layout
	graphHeight int
	logHeight   int
	output      io.Writer
	summary     bool
}

func defaultOptions() options {
	return options{layout: LayoutDependency, graphHeight: DEFAULT_GRAPH_HEIGHT, logHeight: DEFAULT_LOG_HEIGHT, output: os.Stdout, summary: true}
}

// Option 調整 Run 的行為.
type Option func(*options)

// WithTitle 設定畫面左上角的標題.
func WithTitle(title string) Option { return func(o *options) { o.title = title } }

// WithLayout 設定初始版面, 執行中可用 tab 切換.
func WithLayout(l Layout) Option { return func(o *options) { o.layout = l } }

// WithGraphHeight 設定 graph 區塊的列數 (最少 MIN_GRAPH_ROWS); 執行中可用 [ ] 或拖曳 divider 列調整.
func WithGraphHeight(rows int) Option {
	return func(o *options) { o.graphHeight = max(rows, MIN_GRAPH_ROWS) }
}

// WithLogHeight 設定 log pane 的列數, 0 = 隱藏; 執行中可用 - + 或拖曳最下方的說明列調整.
func WithLogHeight(rows int) Option { return func(o *options) { o.logHeight = max(rows, 0) } }

// WithOutput 設定畫面輸出位置, 預設 os.Stdout. TUI 執行期間呼叫端不可再寫同一個 writer.
func WithOutput(w io.Writer) Option { return func(o *options) { o.output = w } }

// WithSummary 決定離開 TUI 後是否把最後一個畫面印到 output, 預設開啟, 讓結果留在終端機的捲動紀錄裡.
func WithSummary(on bool) Option { return func(o *options) { o.summary = on } }
