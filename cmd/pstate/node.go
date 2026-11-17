package pstate

import "time"

// node 是 pipeline 中的一個步驟: 工作函式加上它目前的狀態.
type node struct {
	ID       string
	Name     string   // 顯示名稱, 空字串時顯示 ID
	Deps     []string // 上游節點 ID, 登記時必須已存在
	Priority int      // 數字越小越早; LayoutPriority 依此分欄

	Status    Status
	Message   string // 最近一次狀態附帶的說明 (Pending 原因, Failed 錯誤)
	StartedAt time.Time
	EndedAt   time.Time
	Attempts  int // 進入 Running 的次數

	fn      Func
	trigger chan struct{} // Pending 時由 Trigger 送出訊號
}

// NodeOption 調整 Pipeline.Add 登記的節點.
type NodeOption func(*node)

// After 宣告上游節點; 全部 Finished 後才會啟動本節點.
func After(ids ...string) NodeOption {
	return func(n *node) { n.Deps = append(n.Deps, ids...) }
}

// Priority 設定 LayoutPriority 版面的欄位, 數字小的在左.
func Priority(v int) NodeOption { return func(n *node) { n.Priority = v } }

// Name 設定顯示名稱, 預設為 ID.
func Name(s string) NodeOption { return func(n *node) { n.Name = s } }

// Label 回傳顯示名稱.
func (n node) Label() string {
	if n.Name != "" {
		return n.Name
	}
	return n.ID
}

// Elapsed 回傳本次執行經過的時間; 尚未開始為 0, 仍在跑或等待中則算到 now.
func (n node) Elapsed(now time.Time) time.Duration {
	if n.StartedAt.IsZero() {
		return 0
	}
	if n.EndedAt.IsZero() {
		return now.Sub(n.StartedAt)
	}
	return n.EndedAt.Sub(n.StartedAt)
}
