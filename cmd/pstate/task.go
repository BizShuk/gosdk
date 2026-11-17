package pstate

import (
	"context"
	"fmt"
)

// Func 是被 pstate 包起來的一段工作: 要呼叫的函式, 以及包裝當下保存的參數.
// 重試時以同一份參數再呼叫一次.
type Func struct {
	call   func(ctx context.Context, t *T) error
	params any // 只用於顯示; nil 代表沒有參數
}

// Wrap 把帶參數的工作函式包成 Func. params 在此刻以值保存, 之後的每次重試都拿到同一份.
//
//	p.Add("build", pstate.Wrap(build, BuildArgs{Target: "linux"}), pstate.After("fetch"))
func Wrap[P any](fn func(ctx context.Context, t *T, params P) error, params P) Func {
	return Func{
		call:   func(ctx context.Context, t *T) error { return fn(ctx, t, params) },
		params: params,
	}
}

// Do 把不需要參數的工作函式包成 Func.
func Do(fn func(ctx context.Context, t *T) error) Func { return Func{call: fn} }

func (f Func) describe() string {
	if f.params == nil {
		return ""
	}
	return fmt.Sprintf("%+v", f.params)
}

// T 是傳給工作函式的節點 handle, 用來寫 log 與等待觸發.
type T struct {
	p  *Pipeline
	id string
}

// ID 回傳節點 ID.
func (t *T) ID() string { return t.id }

// Log 在 log pane 寫一筆這個節點的訊息.
func (t *T) Log(text string) { t.p.push(t.id, LevelInfo, text) }

// Logf 是 Log 的格式化版本.
func (t *T) Logf(format string, args ...any) { t.p.push(t.id, LevelInfo, fmt.Sprintf(format, args...)) }

// Error 在 log pane 寫一筆錯誤訊息, 不改變節點狀態.
func (t *T) Error(text string) { t.p.push(t.id, LevelError, text) }

// Wait 讓節點進入 Pending 並阻塞, 直到被觸發 (UI 對選取節點按 enter 或點擊, 或程式呼叫
// Pipeline.Trigger) 後回到 Running 並回傳 nil; ctx 取消時回傳 ctx.Err().
func (t *T) Wait(ctx context.Context, reason string) error {
	trigger := t.p.pend(t.id, reason)
	select {
	case <-trigger:
		t.p.resume(t.id)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
