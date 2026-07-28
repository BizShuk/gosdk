// Package file 提供泛型的檔案儲存庫 Store[T]。
//
// Store 的本體是「一個目錄」,檔名一律由呼叫端傳入。它同時支援兩種
// 存取型態,兩種都提供但都不強制使用:
//
//	單檔文件 — Write / Read / ReadOr:整個檔案是一份 JSON 文件。
//	JSONL   — Append / Scan:檔案是每行一筆的追加日誌。
//
// JSONL 分兩層。Scan 是基礎層,把原始位元組交給呼叫端自己解碼,因此
// 能處理同一個檔案內混有多種記錄型別的情況 (例如首行是 meta、其餘是
// turn),也能讓不想解碼的呼叫端直接做位元組比對。Find / Filter /
// Count 是建在 Scan 之上、會自動解成 T 的便利層,適用於同質檔案。
//
// 可執行的範例見 file/sample:
//
//	go run ./file/sample
package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrNotFound 表示指定的檔案不存在。呼叫端以 errors.Is 判斷,決定要
// 回報錯誤還是退回預設值。
//
// 注意:此哨兵只用於單檔文件操作 (Read)。JSONL 的讀取路徑把「檔案尚未
// 建立」與「檔案為空」視為同一個狀態,回傳空結果與 nil error —— 對一個
// 追加日誌而言這兩者確實沒有語意差別。
var ErrNotFound = errors.New("file: not found")

// ErrStopScan 由 Scan 的回呼回傳,用於提早中止掃描而不視為錯誤,
// 比照 stdlib 的 fs.SkipAll。
var ErrStopScan = errors.New("file: stop scan")

// TEMP_FILE_PREFIX 是 atomic 寫入產生的暫存檔前綴。List 只跳過帶此前綴
// 的檔案,而不是所有點開頭的檔案 —— 呼叫端刻意寫入的隱藏檔應該列得出來。
const TEMP_FILE_PREFIX = ".tmp-"

// Store 是以目錄為單位的泛型檔案儲存庫。
type Store[T any] struct {
	dir  string
	opts Options
	mu   sync.Mutex
}

// NewStore 建立 Store 並確保目錄存在。dir 支援 ~ 與環境變數展開。
func NewStore[T any](dir string, opts ...Option) (*Store[T], error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("file: directory path cannot be empty")
	}

	cfg := defaultOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Ext != "" && !strings.HasPrefix(cfg.Ext, ".") {
		cfg.Ext = "." + cfg.Ext
	}

	absDir, err := resolvePath(dir)
	if err != nil {
		return nil, fmt.Errorf("file: resolve directory: %w", err)
	}
	if err := os.MkdirAll(absDir, cfg.DirPerm); err != nil {
		return nil, fmt.Errorf("file: create directory %s: %w", absDir, err)
	}
	// MkdirAll 對已存在的目錄不會調整權限,明確補一次以確保 0o700 生效。
	if err := os.Chmod(absDir, cfg.DirPerm); err != nil {
		return nil, fmt.Errorf("file: chmod directory %s: %w", absDir, err)
	}

	return &Store[T]{dir: absDir, opts: cfg}, nil
}

// Dir 回傳這個 Store 的絕對目錄路徑。
func (s *Store[T]) Dir() string { return s.dir }

// Path 回傳某個名稱對應的完整檔案路徑。
func (s *Store[T]) Path(name string) string {
	return filepath.Join(s.dir, name+s.opts.Ext)
}

// safeName 阻擋路徑穿越 (path traversal) 與空名稱。
func (s *Store[T]) safeName(name string) error {
	if name == "" {
		return errors.New("file: name cannot be empty")
	}
	if strings.ContainsAny(name, `/\`) || filepath.Base(name) != name {
		return fmt.Errorf("file: invalid name: %s", name)
	}
	// filepath.Base("..") 是 "..",會通過上面的檢查,但它指的是上層目錄 ——
	// 明確擋掉。"." 與其他點開頭的名稱不受影響:它們只是隱藏檔,不是穿越。
	if name == ".." {
		return fmt.Errorf("file: invalid name: %s", name)
	}
	// 最終防線:算出實際路徑,確認它的父目錄就是本 Store 的目錄。這道
	// 檢查不依賴對名稱形狀的枚舉,能擋掉靠副檔名組合出來的邊界情況 ——
	// 例如 WithExt("") 時 Path(".") 會 Clean 成目錄本身,若放行則
	// Delete(".") 會刪掉整個 Store 目錄。
	if filepath.Dir(s.Path(name)) != s.dir {
		return fmt.Errorf("file: name escapes store directory: %s", name)
	}
	return nil
}

// Custom 是逃生口:把完整檔案路徑交給呼叫端,讓它做本套件未提供的操作。
//
// 契約:呼叫期間持有 Store 的互斥鎖,目錄保證存在。但不提供 atomic
// 寫入,也不做任何編解碼 —— 那些都由 fn 自理。優先使用具名方法,只有
// 真的沒有對應能力時才用它。
//
// 互斥鎖不可重入:fn 內不得再呼叫同一個 Store 的任何方法,否則死鎖。
func (s *Store[T]) Custom(name string, fn func(path string) error) error {
	if err := s.safeName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(s.Path(name)); err != nil {
		return fmt.Errorf("file: custom %s: %w", name, err)
	}
	return nil
}
