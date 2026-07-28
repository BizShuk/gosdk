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

// 預設值。
const (
	DEFAULT_DIR_PERM  os.FileMode = 0o755
	DEFAULT_FILE_PERM os.FileMode = 0o644
	DEFAULT_EXT       string      = ".json"
)

// Options 是 Store 的可調參數。
type Options struct {
	// DirPerm 是建立目錄時的權限。
	DirPerm os.FileMode
	// FilePerm 是建立檔案時的權限。
	FilePerm os.FileMode
	// Ext 是檔名副檔名,含前導點。缺點會自動補上。
	Ext string
	// Atomic 決定 Write 是否走 temp + rename。預設開啟。
	Atomic bool
	// DecodeHook 在 json.Unmarshal 之前改寫原始位元組,供 schema 遷移使用。
	DecodeHook func([]byte) ([]byte, error)
}

// Option 以函式選項模式調整 Options。刻意不帶型別參數,否則呼叫端得寫
// file.WithDirPerm[Cred](0o700),每個選項都要重複一次型別名。
type Option func(*Options)

// WithDirPerm 設定目錄權限。
func WithDirPerm(m os.FileMode) Option { return func(o *Options) { o.DirPerm = m } }

// WithFilePerm 設定檔案權限。
func WithFilePerm(m os.FileMode) Option { return func(o *Options) { o.FilePerm = m } }

// WithExt 設定副檔名,前導點可省略。
func WithExt(ext string) Option { return func(o *Options) { o.Ext = ext } }

// WithAtomicWrite 決定 Write 是否走 temp + rename。
func WithAtomicWrite(on bool) Option { return func(o *Options) { o.Atomic = on } }

// WithDecodeHook 註冊解碼前的位元組改寫函式。
func WithDecodeHook(fn func([]byte) ([]byte, error)) Option {
	return func(o *Options) { o.DecodeHook = fn }
}

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

	cfg := Options{
		DirPerm:  DEFAULT_DIR_PERM,
		FilePerm: DEFAULT_FILE_PERM,
		Ext:      DEFAULT_EXT,
		Atomic:   true,
	}
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
	// filepath.Base(".") 是 "."、Base("..") 是 "..",兩者都會通過上面的
	// 檢查。搭配 WithExt("") 時 Path("..") 會解析到上層目錄,是真的路徑
	// 穿越。一併擋掉所有點開頭的名稱 —— List 本來就跳過點檔案,允許寫入
	// 卻列不出來只會造成不對稱。
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("file: invalid name: %s", name)
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
