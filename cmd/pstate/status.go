package pstate

import "fmt"

// Status 是節點的執行狀態.
type Status int

const (
	// NotStarted 尚未開始: 還在等上游完成, 或上游失敗而被擋住.
	NotStarted Status = iota
	// Running 工作函式執行中.
	Running
	// Pending 工作函式呼叫了 T.Wait, 停下來等待觸發 (UI 按 enter / 點擊, 或 Pipeline.Trigger).
	Pending
	// Finished 工作函式回傳 nil.
	Finished
	// Failed 工作函式回傳 error 或 panic; 可用 Pipeline.Retry 以相同參數重跑.
	Failed
)

var statusNames = [...]string{"not_started", "running", "pending", "finished", "failed"}

func (s Status) String() string {
	if s < 0 || int(s) >= len(statusNames) {
		return fmt.Sprintf("status(%d)", int(s))
	}
	return statusNames[s]
}

// active 表示節點的工作函式仍在執行 (含等待觸發).
func (s Status) active() bool { return s == Running || s == Pending }
