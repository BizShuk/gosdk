package pstate

import "time"

// Level 區分一般訊息與錯誤, log pane 以顏色區隔.
type Level int

const (
	LevelInfo Level = iota
	LevelError
)

// LogEntry 是 log pane 的一筆紀錄. NodeID 為空代表 pipeline 層級的訊息.
type LogEntry struct {
	Time   time.Time
	NodeID string
	Level  Level
	Text   string
}

// logRing 保留最近 cap 筆紀錄, 超過時丟掉最舊的.
type logRing struct {
	buf   []LogEntry
	start int
	size  int
}

func newLogRing(capacity int) *logRing {
	return &logRing{buf: make([]LogEntry, capacity)}
}

func (r *logRing) push(e LogEntry) {
	if len(r.buf) == 0 {
		return
	}
	idx := (r.start + r.size) % len(r.buf)
	r.buf[idx] = e
	if r.size < len(r.buf) {
		r.size++
		return
	}
	r.start = (r.start + 1) % len(r.buf)
}

func (r *logRing) list() []LogEntry {
	out := make([]LogEntry, r.size)
	for i := range r.size {
		out[i] = r.buf[(r.start+i)%len(r.buf)]
	}
	return out
}
