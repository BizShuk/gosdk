package pstate

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// Execute 不開 TUI, 依相依順序平行執行所有節點, 直到沒有節點在跑也沒有可啟動的節點.
// 失敗節點的下游不會執行; 回傳值同 Run.
func (p *Pipeline) Execute(ctx context.Context) error {
	p.loop(ctx, true)
	return p.result()
}

// loop 反覆啟動可執行的節點. untilIdle 為 false 時持續等待 Retry / Add 直到 ctx 取消 (TUI 使用).
func (p *Pipeline) loop(ctx context.Context, untilIdle bool) {
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		ready, active := p.claimReady()
		for _, n := range ready {
			wg.Go(func() { p.complete(n.ID, call(ctx, n.fn, &T{p: p, id: n.ID})) })
		}
		if untilIdle && active == 0 {
			return
		}
		select {
		case <-p.wake:
		case <-ctx.Done():
			return
		}
	}
}

// call 執行工作函式, 並把 panic 轉成 error, 讓一個節點的 bug 不會拖垮整條 pipeline.
func call(ctx context.Context, f Func, t *T) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
	}()
	return f.call(ctx, t)
}
