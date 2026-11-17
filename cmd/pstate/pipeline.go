package pstate

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// DEFAULT_MAX_LOGS 是 log pane 保留的紀錄筆數上限.
const DEFAULT_MAX_LOGS = 2000

var (
	ErrEmptyID       = errors.New("pstate: empty node id")
	ErrNilFunc       = errors.New("pstate: nil func")
	ErrDuplicateNode = errors.New("pstate: duplicate node")
	ErrUnknownNode   = errors.New("pstate: unknown node")
	ErrNotFailed     = errors.New("pstate: only failed nodes can be retried")
	ErrNotPending    = errors.New("pstate: only pending nodes can be triggered")
	ErrIncomplete    = errors.New("pstate: pipeline incomplete")
)

// Pipeline 是一組有相依關係的工作. Add 登記, Execute 或 Run 依相依順序平行執行,
// 狀態與 log 由 pstate 自動記錄.
type Pipeline struct {
	mu      sync.Mutex
	nodes   map[string]*node
	order   []string
	logs    *logRing
	changed chan struct{} // 通知 UI 重繪
	wake    chan struct{} // 通知 scheduler 重新檢查可啟動的節點
	now     func() time.Time
}

// snapshot 是某一時刻的完整狀態副本, 繪製只讀它, 不持有鎖.
type snapshot struct {
	Nodes []node // 依登記順序
	Logs  []LogEntry
	Now   time.Time
}

// New 建立空的 Pipeline.
func New() *Pipeline {
	return &Pipeline{
		nodes:   map[string]*node{},
		logs:    newLogRing(DEFAULT_MAX_LOGS),
		changed: make(chan struct{}, 1),
		wake:    make(chan struct{}, 1),
		now:     time.Now,
	}
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// Add 登記一個節點. 上游 (After) 必須已登記, 因此相依關係不可能成環.
func (p *Pipeline) Add(id string, f Func, opts ...NodeOption) error {
	if id == "" {
		return ErrEmptyID
	}
	if f.call == nil {
		return fmt.Errorf("%w: %s", ErrNilFunc, id)
	}
	n := &node{ID: id, fn: f, trigger: make(chan struct{}, 1)}
	for _, opt := range opts {
		opt(n)
	}
	n.Deps = slices.Compact(slices.Clone(n.Deps))

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.nodes[id]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicateNode, id)
	}
	for _, d := range n.Deps {
		if _, ok := p.nodes[d]; !ok {
			return fmt.Errorf("%w: %s depends on %s", ErrUnknownNode, id, d)
		}
	}
	p.nodes[id] = n
	p.order = append(p.order, id)
	signal(p.changed)
	signal(p.wake)
	return nil
}

// Retry 讓失敗的節點以原本的參數重跑; 被它擋住的下游會在它成功後接著執行.
func (p *Pipeline) Retry(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	n, ok := p.nodes[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	if n.Status != Failed {
		return fmt.Errorf("%w: %s is %s", ErrNotFailed, id, n.Status)
	}
	n.Status, n.Message = NotStarted, ""
	n.StartedAt, n.EndedAt = time.Time{}, time.Time{}
	p.logLocked(id, LevelInfo, "retry requested")
	signal(p.wake)
	return nil
}

// Trigger 喚醒在 T.Wait 中等待的節點.
func (p *Pipeline) Trigger(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	n, ok := p.nodes[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	if n.Status != Pending {
		return fmt.Errorf("%w: %s is %s", ErrNotPending, id, n.Status)
	}
	signal(n.trigger)
	return nil
}

// claimReady 把所有「NotStarted 且上游全部 Finished」的節點標成 Running 並回傳,
// 同時回報還有幾個節點在執行.
func (p *Pipeline) claimReady() (ready []node, active int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	for _, id := range p.order {
		n := p.nodes[id]
		if n.Status.active() {
			active++
			continue
		}
		if n.Status != NotStarted || !p.depsFinished(n) {
			continue
		}
		n.Status, n.Message = Running, ""
		n.StartedAt, n.EndedAt = now, time.Time{}
		n.Attempts++
		text := "running"
		if n.Attempts > 1 {
			text = fmt.Sprintf("running (attempt %d)", n.Attempts)
		}
		p.logLocked(id, LevelInfo, text)
		ready = append(ready, *n)
	}
	if len(ready) > 0 {
		signal(p.changed)
	}
	return ready, active + len(ready)
}

func (p *Pipeline) depsFinished(n *node) bool {
	for _, d := range n.Deps {
		if p.nodes[d].Status != Finished {
			return false
		}
	}
	return true
}

// complete 記錄工作函式的結果.
func (p *Pipeline) complete(id string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := p.nodes[id]
	n.EndedAt = p.now()
	took := formatDuration(n.Elapsed(n.EndedAt))
	if err == nil {
		n.Status, n.Message = Finished, ""
		p.logLocked(id, LevelInfo, "finished in "+took)
	} else {
		msg := err.Error()
		n.Status, n.Message = Failed, firstLine(msg)
		p.logLocked(id, LevelError, fmt.Sprintf("failed after %s: %s", took, msg))
	}
	signal(p.wake)
}

// pend 讓節點進入 Pending 並回傳它的觸發 channel.
func (p *Pipeline) pend(id, reason string) <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := p.nodes[id]
	n.Status, n.Message = Pending, reason
	p.logLocked(id, LevelInfo, "pending: "+reason)
	return n.trigger
}

func (p *Pipeline) resume(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := p.nodes[id]
	n.Status, n.Message = Running, ""
	p.logLocked(id, LevelInfo, "triggered, running")
}

func (p *Pipeline) push(id string, lv Level, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.logLocked(id, lv, text)
}

func (p *Pipeline) logLocked(id string, lv Level, text string) {
	p.logs.push(LogEntry{Time: p.now(), NodeID: id, Level: lv, Text: text})
	signal(p.changed)
}

// result 彙整執行結果: 有失敗回傳各節點錯誤, 有未完成回傳 ErrIncomplete, 全部完成回傳 nil.
func (p *Pipeline) result() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var errs []error
	unfinished := 0
	for _, id := range p.order {
		switch n := p.nodes[id]; n.Status {
		case Failed:
			errs = append(errs, fmt.Errorf("%s: %s", id, n.Message))
		case Finished:
		default:
			unfinished++
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if unfinished > 0 {
		return fmt.Errorf("%w: %d nodes not finished", ErrIncomplete, unfinished)
	}
	return nil
}

func (p *Pipeline) snapshot() snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	nodes := make([]node, 0, len(p.order))
	for _, id := range p.order {
		n := *p.nodes[id]
		n.Deps = slices.Clone(n.Deps)
		nodes = append(nodes, n)
	}
	return snapshot{Nodes: nodes, Logs: p.logs.list(), Now: p.now()}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
