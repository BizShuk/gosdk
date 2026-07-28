# gosdk/file Store 統一化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `gosdk/file` 改造成一個能同時吃下「單檔文件」與「JSONL 追加日誌」兩種存取型態的泛型 `Store[T]`，讓 pm2 / agentSDK / auth / skills / sessiond 五個 repo 的七份重複 file store 之後能收斂到同一個實作。

**Architecture:** `Store[T]` 的本體是`一個目錄`，檔名一律由呼叫端傳入。單檔操作（`Write`/`Read`）把整個檔案當一份 JSON 文件；JSONL 操作（`Append`/`Scan`）把檔案當每行一筆的追加日誌。JSONL 分兩層：`Scan` 是把`原始位元組 (raw bytes)` 交給呼叫端的基礎層，`Find`/`Filter`/`Count` 是建在 `Scan` 之上、會自動解碼成 `T` 的便利層。所有能力都提供但都不強制使用，呼叫端只用它需要的那幾個。

**Tech Stack:** Go 1.26.0、stdlib（`encoding/json`、`bufio`、`os`、`sync`、`reflect`）、`github.com/bizshuk/gosdk/validator`。不新增任何外部依賴。

## Global Constraints

- Module `github.com/bizshuk/gosdk`，Go 1.26.0。
- **不得新增外部依賴。** `gosdk/file` 完成後的 import 集合只允許 stdlib 加上 `github.com/bizshuk/gosdk/validator`。
- **不得 import `github.com/bizshuk/gosdk/utils`。** 原因：`utils/file.go` 內含 `github.com/gocarina/gocsv`，而 Go 是以 package 為單位載入，任何對 `utils` 的引用都會把 CSV 函式庫拖進相依圖。`ai/auth` 目前直接依賴只有 3 個且完全沒有 gosdk，這條限制是它能低成本導入的前提。代價是 Task 1 會在 `file/path.go` 內重寫約 35 行 `resolvePath` / `isNil`。已評估過的替代方案是把 CSV 相關函式移出 `utils`，但 `data/stock` 有 10 處以上呼叫 `utils.LoadCSV` / `utils.WriteCSV`，屬破壞性變更，不納入本計畫。
- 常數一律 `SCREAMING_SNAKE_CASE`（含 unexported 與函式內 block-scoped）；`var` 維持 MixedCaps。
- 註解使用`繁體中文`，與現行 `file/store.go` 一致。
- 錯誤一律 `fmt.Errorf("...: %w", err)` 包裝。
- 現行 `FileStore[T]` / `NewFileStore` / `StoreOption[T]` / `UpdateDir` 全部移除，不留相容層。已用 `grep -rn "gosdk/file" --include=*.go ~/projects` 確認全工作區零使用者。
- 五個 repo 的實際遷移`不在本計畫範圍`。它們的 gosdk 版本各不相同（pm2 v1.2.7、sessiond v1.2.5、agentSDK v1.2.1、skills 為 pseudo-version），需各自 bump，另開計畫。
- 開工前建立分支：`git checkout -b w-file-store`。工作目錄 `/Users/shuk/projects/platform/gosdk`。

## File Structure

| 檔案 | 職責 |
| --- | --- |
| `file/store.go` | `Store[T]` 型別、`Options`、`NewStore`、名稱安全檢查、`Path`/`Dir`、`Custom` |
| `file/path.go` | unexported `resolvePath` / `isNil`，切斷對 `gosdk/utils` 的依賴 |
| `file/document.go` | 單檔文件操作：`Write` / `Read` / `ReadOr` |
| `file/dir.go` | 目錄層操作：`List` / `Delete` / `Exists` / `Sub` |
| `file/jsonl.go` | JSONL 基礎層：`Append` / `Scan` |
| `file/query.go` | JSONL 便利層：`Find` / `Filter` / `Count` / `TruncateWhile` |
| `file/store_test.go` | 建構、選項、名稱守衛、`Custom` |
| `file/document_test.go` | 單檔讀寫、atomic、權限、validator、decode hook |
| `file/dir_test.go` | 列表、刪除、存在、子目錄 |
| `file/jsonl_test.go` | 追加、掃描、提早中止 |
| `file/query_test.go` | 條件查詢、計數、前綴壓縮 |

現行 `file/store.go` 與 `file/store_test.go` 會在 Task 1、Task 2 被改寫。

---

### Task 1: Store 骨架、選項與路徑工具

**Files:**
- Modify: `file/store.go`（整檔改寫）
- Create: `file/path.go`
- Modify: `file/store_test.go`（整檔改寫）

**Interfaces:**
- Consumes: 無（第一個任務）
- Produces:
  - `type Store[T any] struct`（欄位皆 unexported）
  - `type Options struct { DirPerm, FilePerm os.FileMode; Ext string; Atomic bool; DecodeHook func([]byte) ([]byte, error) }`
  - `type Option func(*Options)`
  - `func WithDirPerm(m os.FileMode) Option`
  - `func WithFilePerm(m os.FileMode) Option`
  - `func WithExt(ext string) Option`
  - `func WithAtomicWrite(on bool) Option`
  - `func WithDecodeHook(fn func([]byte) ([]byte, error)) Option`
  - `func NewStore[T any](dir string, opts ...Option) (*Store[T], error)`
  - `func (s *Store[T]) Dir() string`
  - `func (s *Store[T]) Path(name string) string`
  - `func (s *Store[T]) Custom(name string, fn func(path string) error) error`
  - `var ErrNotFound error`
  - `var ErrStopScan error`
  - unexported：`func (s *Store[T]) safeName(name string) error`、`func resolvePath(path string) (string, error)`、`func isNil(v any) bool`

**設計備註（實作者必讀）：** `Option` 刻意`不帶型別參數`。若寫成 `Option[T]`，呼叫端就得寫 `file.WithDirPerm[Cred](0o700)`，每個選項都要重複型別名。非泛型的 `Option` 讓呼叫端只需 `file.NewStore[Cred](dir, file.WithDirPerm(0o700))`。

- [ ] **Step 1: 建立分支**

```bash
cd /Users/shuk/projects/platform/gosdk
git checkout -b w-file-store
```

- [ ] **Step 2: 寫失敗測試 — 改寫 `file/store_test.go`**

整檔取代為：

```go
package file

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// mockUser 是測試用結構,實作 validator.IValidator。
type mockUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (m mockUser) Validate() error {
	if m.ID <= 0 {
		return errors.New("id must be positive")
	}
	if m.Name == "" {
		return errors.New("name cannot be empty")
	}
	return nil
}

func TestNewStoreDefaults(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore[mockUser](filepath.Join(dir, "nested"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if got := s.Path("alice"); got != filepath.Join(dir, "nested", "alice.json") {
		t.Errorf("Path = %q", got)
	}
	info, err := os.Stat(s.Dir())
	if err != nil {
		t.Fatalf("dir not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("dir perm = %o, want 755", perm)
	}
}

func TestNewStoreOptions(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore[mockUser](filepath.Join(dir, "secure"),
		WithDirPerm(0o700), WithFilePerm(0o600), WithExt("jsonl"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if got := s.Path("wal"); got != filepath.Join(dir, "secure", "wal.jsonl") {
		t.Errorf("Path = %q, 前導點應自動補上", got)
	}
	info, _ := os.Stat(s.Dir())
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}
}

func TestNewStoreRejectsEmptyDir(t *testing.T) {
	if _, err := NewStore[mockUser](""); err == nil {
		t.Fatal("空目錄應回錯誤")
	}
}

func TestSafeName(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	bad := []string{"", "a/b", `a\b`, "../etc/passwd", "."}
	for _, n := range bad {
		if err := s.safeName(n); err == nil {
			t.Errorf("safeName(%q) 應回錯誤", n)
		}
	}
	for _, n := range []string{"alice", "run-01", "2026-07-29"} {
		if err := s.safeName(n); err != nil {
			t.Errorf("safeName(%q) = %v, 應通過", n, err)
		}
	}
}

func TestCustomReceivesFullPath(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	var seen string
	err := s.Custom("raw", func(path string) error {
		seen = path
		return os.WriteFile(path, []byte("hi"), 0o644)
	})
	if err != nil {
		t.Fatalf("Custom: %v", err)
	}
	if seen != s.Path("raw") {
		t.Errorf("Custom 收到 %q, want %q", seen, s.Path("raw"))
	}
	if b, _ := os.ReadFile(s.Path("raw")); string(b) != "hi" {
		t.Errorf("檔案內容 = %q", b)
	}
}

func TestCustomPropagatesError(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	sentinel := errors.New("boom")
	if err := s.Custom("x", func(string) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Errorf("Custom err = %v, want 包住 sentinel", err)
	}
}

func TestResolvePathExpandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	got, err := resolvePath("~/somewhere")
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	if got != filepath.Join(home, "somewhere") {
		t.Errorf("resolvePath = %q, want %q", got, filepath.Join(home, "somewhere"))
	}
}

func TestIsNil(t *testing.T) {
	var p *mockUser
	if !isNil(p) {
		t.Error("nil 指標應為 true")
	}
	if isNil(mockUser{ID: 1}) {
		t.Error("非指標值應為 false")
	}
	if isNil(nil) == false {
		t.Error("裸 nil 應為 true")
	}
}
```

- [ ] **Step 3: 執行測試確認失敗**

Run: `go test ./file/ -v`
Expected: FAIL，編譯錯誤 `undefined: NewStore`、`undefined: WithDirPerm`、`undefined: resolvePath`、`undefined: isNil`（舊的 `FileStore` 仍在但測試已不引用它）

- [ ] **Step 4: 建立 `file/path.go`**

```go
package file

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// resolvePath 展開環境變數與 ~ 後回傳絕對路徑。
//
// 這裡刻意不呼叫 gosdk/utils.ResolvePath:utils 套件內含
// github.com/gocarina/gocsv,而 Go 以 package 為單位載入,任何引用都會把
// CSV 函式庫拖進相依圖。file 是葉節點儲存套件,要能被極輕量的專案
// (例如只有三個直接依賴的 ai/auth) 導入而不付出額外代價。
func resolvePath(path string) (string, error) {
	expanded := os.ExpandEnv(path)

	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get user home directory: %w", err)
		}
		switch {
		case len(expanded) == 1:
			expanded = home
		case expanded[1] == '/' || expanded[1] == '\\':
			expanded = filepath.Join(home, expanded[2:])
		}
	}

	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return filepath.Clean(absPath), nil
}

// isNil 判斷 interface 值是否為 nil,含「非 nil interface 包住 nil 指標」
// 這個 Go 經典陷阱。用於 validator 檢查前的防呆。
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}
```

- [ ] **Step 5: 改寫 `file/store.go`**

整檔取代為：

```go
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
```

- [ ] **Step 6: 執行測試確認通過**

Run: `go test ./file/ -v`
Expected: PASS，8 個測試全綠

- [ ] **Step 7: 確認相依圖已切斷 utils**

Run: `go list -deps ./file | grep -E "gocsv|gosdk/utils|gosdk/encode"`
Expected: 無輸出（`exit 1`）。若印出任何一行，表示還有檔案 import 了 `gosdk/utils`，必須改掉才能繼續。

- [ ] **Step 8: Commit**

```bash
git add file/store.go file/path.go file/store_test.go
git commit -m "refactor(file): rebuild Store[T] skeleton with non-generic options

Store 本體改為目錄概念,檔名由呼叫端傳入。新增 ErrNotFound /
ErrStopScan 哨兵與 Custom 逃生口。自帶 resolvePath/isNil 以切斷對
gosdk/utils 的依賴 (utils 會拖入 gocsv)。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 2: 單檔文件操作 Write / Read / ReadOr

**Files:**
- Create: `file/document.go`
- Create: `file/document_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Store[T]`、`Options`、`ErrNotFound`、`safeName`、`Path`、`isNil`
- Produces:
  - `func (s *Store[T]) Write(name string, v T) error`
  - `func (s *Store[T]) Read(name string) (T, error)`
  - `func (s *Store[T]) ReadOr(name string, def T) T`
  - unexported：`func (s *Store[T]) writeBytes(name string, data []byte) error`、`func (s *Store[T]) readBytes(name string) ([]byte, error)`

**設計備註：** `ReadOr` 吞掉的不只是「檔案不存在」，JSON 損毀與權限錯誤也會靜靜回傳 `def`。只有在「缺檔是唯一預期失敗」時才該用它；需要區分損毀的呼叫端（例如 pm2 對 `dump.json` 有專屬的格式不相容訊息）必須用 `Read`。

- [ ] **Step 1: 寫失敗測試 — 建立 `file/document_test.go`**

```go
package file

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	want := mockUser{ID: 1, Name: "Alice", Email: "alice@example.com"}
	if err := s.Write("alice", want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := s.Read("alice")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != want {
		t.Errorf("Read = %+v, want %+v", got, want)
	}
}

func TestReadMissingReturnsErrNotFound(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	_, err := s.Read("ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read err = %v, want ErrNotFound", err)
	}
}

func TestReadOrFallsBackOnMissing(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	def := mockUser{ID: 99, Name: "default"}
	if got := s.ReadOr("ghost", def); got != def {
		t.Errorf("ReadOr = %+v, want %+v", got, def)
	}
}

func TestReadOrAlsoSwallowsCorruption(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if err := os.WriteFile(s.Path("broken"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	def := mockUser{ID: 7, Name: "fallback"}
	if got := s.ReadOr("broken", def); got != def {
		t.Errorf("ReadOr = %+v, want %+v (損毀也退回預設是刻意行為)", got, def)
	}
}

func TestWriteRunsValidator(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if err := s.Write("bad", mockUser{ID: 0, Name: "x"}); err == nil {
		t.Fatal("Validate 失敗時 Write 應回錯誤")
	}
	if _, err := os.Stat(s.Path("bad")); !os.IsNotExist(err) {
		t.Error("驗證失敗不應留下檔案")
	}
}

func TestWriteAppliesFilePerm(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir(), WithFilePerm(0o600))
	if err := s.Write("alice", mockUser{ID: 1, Name: "Alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(s.Path("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
}

func TestAtomicWriteLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore[mockUser](dir)
	if err := s.Write("alice", mockUser{ID: 1, Name: "Alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("目錄應只剩一個檔案,實得 %d 個", len(entries))
	}
	if entries[0].Name() != "alice.json" {
		t.Errorf("殘留檔案 %q", entries[0].Name())
	}
}

func TestWriteOverwrites(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	_ = s.Write("alice", mockUser{ID: 1, Name: "Alice"})
	if err := s.Write("alice", mockUser{ID: 2, Name: "Bob"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, _ := s.Read("alice")
	if got.Name != "Bob" {
		t.Errorf("Name = %q, want Bob", got.Name)
	}
}

func TestDecodeHookRewritesBeforeUnmarshal(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir(), WithDecodeHook(func(raw []byte) ([]byte, error) {
		return bytes.ReplaceAll(raw, []byte(`"nickname":`), []byte(`"name":`)), nil
	}))
	if err := os.WriteFile(s.Path("legacy"),
		[]byte(`{"id":3,"nickname":"Carol"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read("legacy")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Name != "Carol" {
		t.Errorf("Name = %q, want Carol (decode hook 應已改寫欄位名)", got.Name)
	}
}

func TestWriteRejectsUnsafeName(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if err := s.Write("../escape", mockUser{ID: 1, Name: "x"}); err == nil {
		t.Fatal("路徑穿越名稱應被拒絕")
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./file/ -run 'TestWrite|TestRead|TestAtomic|TestDecodeHook' -v`
Expected: FAIL，編譯錯誤 `s.Write undefined`、`s.Read undefined`、`s.ReadOr undefined`

- [ ] **Step 3: 建立 `file/document.go`**

```go
package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/bizshuk/gosdk/validator"
)

// Write 把 v 序列化為 JSON 並寫入 name 對應的檔案。
//
// 若 v 實作 validator.IValidator,會先驗證再寫入 —— 驗證失敗時不會產生
// 任何檔案。預設走 temp + rename 的 atomic 寫入,中途中斷不會留下半截檔案。
func (s *Store[T]) Write(name string, v T) error {
	if err := s.safeName(name); err != nil {
		return err
	}

	if val, ok := any(v).(validator.IValidator); ok && !isNil(val) {
		if err := val.Validate(); err != nil {
			return fmt.Errorf("file: validation failed for %s: %w", name, err)
		}
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("file: marshal %s: %w", name, err)
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeBytes(name, data)
}

// Read 讀取並反序列化 name 對應的檔案。檔案不存在時回傳包住 ErrNotFound
// 的錯誤,呼叫端可用 errors.Is 判斷。
func (s *Store[T]) Read(name string) (T, error) {
	var zero T
	if err := s.safeName(name); err != nil {
		return zero, err
	}

	s.mu.Lock()
	raw, err := s.readBytes(name)
	s.mu.Unlock()
	if err != nil {
		return zero, err
	}

	if s.opts.DecodeHook != nil {
		raw, err = s.opts.DecodeHook(raw)
		if err != nil {
			return zero, fmt.Errorf("file: decode hook %s: %w", name, err)
		}
	}

	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return zero, fmt.Errorf("file: unmarshal %s: %w", name, err)
	}
	return v, nil
}

// ReadOr 是 Read 的無錯誤版本:任何失敗都退回 def。
//
// 它吞掉的不只是「檔案不存在」—— JSON 損毀與權限錯誤同樣會靜靜回傳
// def。只在「缺檔是唯一預期失敗」時使用;需要區分損毀情況的呼叫端請用 Read。
func (s *Store[T]) ReadOr(name string, def T) T {
	v, err := s.Read(name)
	if err != nil {
		return def
	}
	return v
}

// writeBytes 把 data 寫進 name 對應的檔案。呼叫端須持有 s.mu。
func (s *Store[T]) writeBytes(name string, data []byte) error {
	path := s.Path(name)

	if !s.opts.Atomic {
		if err := os.WriteFile(path, data, s.opts.FilePerm); err != nil {
			return fmt.Errorf("file: write %s: %w", name, err)
		}
		return nil
	}

	tmp, err := os.CreateTemp(s.dir, ".tmp-*"+s.opts.Ext)
	if err != nil {
		return fmt.Errorf("file: create temp for %s: %w", name, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // rename 成功後這裡是 no-op

	if err := tmp.Chmod(s.opts.FilePerm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("file: chmod temp for %s: %w", name, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("file: write temp for %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("file: close temp for %s: %w", name, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("file: commit %s: %w", name, err)
	}
	return nil
}

// readBytes 讀出 name 對應的檔案內容。呼叫端須持有 s.mu。
func (s *Store[T]) readBytes(name string) ([]byte, error) {
	raw, err := os.ReadFile(s.Path(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("file: read %s: %w", name, err)
	}
	return raw, nil
}
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./file/ -v`
Expected: PASS，全部測試綠燈（Task 1 的 8 個 + 本任務的 10 個）

- [ ] **Step 5: Commit**

```bash
git add file/document.go file/document_test.go
git commit -m "feat(file): add Write/Read/ReadOr with atomic write and decode hook

Write 預設走 temp+rename,並在序列化前執行 validator.IValidator。
Read 對缺檔回傳 ErrNotFound;ReadOr 是無錯誤版本,文件明確標示它同時
吞掉損毀與權限錯誤。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 3: 目錄層操作 List / Delete / Exists / Sub

**Files:**
- Create: `file/dir.go`
- Create: `file/dir_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Store[T]`、`safeName`、`Path`、`Dir`、`opts`；Task 2 的 `Write`
- Produces:
  - `func (s *Store[T]) List() ([]string, error)`
  - `func (s *Store[T]) Delete(name string) error`
  - `func (s *Store[T]) Exists(name string) bool`
  - `func (s *Store[T]) Sub(seg string) (*Store[T], error)`

**設計備註：** `List` 回傳`名稱字串`而非 `[]*T`。舊實作解碼每一個檔案，任何一個損毀就讓整批呼叫失敗，而 `ai/auth` 是刻意跳過壞檔並排序的。回名稱列表讓呼叫端自行 `Read` + `continue`，兩種需求都滿足，也讓只想列名的呼叫端不必付解碼成本。

- [ ] **Step 1: 寫失敗測試 — 建立 `file/dir_test.go`**

```go
package file

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListReturnsSortedNames(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	for _, n := range []string{"carol", "alice", "bob"} {
		if err := s.Write(n, mockUser{ID: 1, Name: n}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"alice", "bob", "carol"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestListFiltersByExtAndSkipsDirsAndDotfiles(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore[mockUser](dir)
	_ = s.Write("alice", mockUser{ID: 1, Name: "Alice"})
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".tmp-123.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"alice"}) {
		t.Errorf("List = %v, want [alice]", got)
	}
}

func TestListDoesNotDecode(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore[mockUser](dir)
	_ = s.Write("good", mockUser{ID: 1, Name: "Good"})
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatalf("List 不應因為壞檔失敗: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"broken", "good"}) {
		t.Errorf("List = %v, want [broken good]", got)
	}
}

func TestListEmptyDir(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %v, want 空", got)
	}
}

func TestExists(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if s.Exists("alice") {
		t.Error("尚未寫入前不應存在")
	}
	_ = s.Write("alice", mockUser{ID: 1, Name: "Alice"})
	if !s.Exists("alice") {
		t.Error("寫入後應存在")
	}
	if s.Exists("../escape") {
		t.Error("不安全名稱應直接回 false")
	}
}

func TestDelete(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	_ = s.Write("alice", mockUser{ID: 1, Name: "Alice"})
	if err := s.Delete("alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s.Exists("alice") {
		t.Error("刪除後不應存在")
	}
}

func TestDeleteMissingReturnsErrNotFound(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if err := s.Delete("ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestSubInheritsOptions(t *testing.T) {
	root := t.TempDir()
	s, _ := NewStore[mockUser](root, WithDirPerm(0o700), WithFilePerm(0o600), WithExt(".jsonl"))
	sub, err := s.Sub("workspace-a")
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if sub.Dir() != filepath.Join(root, "workspace-a") {
		t.Errorf("Sub dir = %q", sub.Dir())
	}
	if got := sub.Path("run"); got != filepath.Join(root, "workspace-a", "run.jsonl") {
		t.Errorf("Sub Path = %q, 副檔名應被繼承", got)
	}
	info, err := os.Stat(sub.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("Sub dir perm = %o, want 700", perm)
	}
}

func TestSubRejectsTraversal(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	for _, bad := range []string{"", "..", "a/b", "../escape"} {
		if _, err := s.Sub(bad); err == nil {
			t.Errorf("Sub(%q) 應被拒絕", bad)
		}
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./file/ -run 'TestList|TestExists|TestDelete|TestSub' -v`
Expected: FAIL，編譯錯誤 `s.List undefined`、`s.Exists undefined`、`s.Delete undefined`、`s.Sub undefined`

- [ ] **Step 3: 建立 `file/dir.go`**

```go
package file

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// List 回傳目錄下所有符合副檔名的檔案名稱 (不含副檔名),依字典序排序。
//
// 它刻意不解碼任何檔案。舊版回傳 []*T 並在解碼失敗時中止整批呼叫,但
// 實務上呼叫端多半希望跳過壞檔而不是整個列表失敗。回名稱列表讓呼叫端
// 自行 Read + continue,也讓只需要列名的呼叫端不必付解碼成本。
//
// 以點開頭的檔案一律跳過 —— atomic 寫入產生的 .tmp-* 暫存檔就長這樣。
func (s *Store[T]) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("file: list %s: %w", s.dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || strings.HasPrefix(n, ".") {
			continue
		}
		if s.opts.Ext != "" && !strings.HasSuffix(n, s.opts.Ext) {
			continue
		}
		names = append(names, strings.TrimSuffix(n, s.opts.Ext))
	}
	sort.Strings(names)
	return names, nil
}

// Exists 回報 name 對應的檔案是否存在。名稱不安全時一律回 false。
func (s *Store[T]) Exists(name string) bool {
	if err := s.safeName(name); err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := os.Stat(s.Path(name))
	return err == nil
}

// Delete 移除 name 對應的檔案。檔案不存在時回傳包住 ErrNotFound 的錯誤。
func (s *Store[T]) Delete(name string) error {
	if err := s.safeName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.Path(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return fmt.Errorf("file: delete %s: %w", name, err)
	}
	return nil
}

// Sub 回傳一個以子目錄為根、繼承全部選項的新 Store。
//
// 用於需要巢狀路徑的場景 (例如 sessions/<workspace>/<id>.jsonl):
// safeName 拒絕含 / 的名稱,Sub 是取得第二層的正當途徑。
//
// 注意:子 Store 有自己的互斥鎖,鎖不會跨越父子 —— 需要跨層原子性的
// 呼叫端必須自行協調。
func (s *Store[T]) Sub(seg string) (*Store[T], error) {
	if err := s.safeName(seg); err != nil {
		return nil, err
	}
	return NewStore[T](filepath.Join(s.dir, seg),
		WithDirPerm(s.opts.DirPerm),
		WithFilePerm(s.opts.FilePerm),
		WithExt(s.opts.Ext),
		WithAtomicWrite(s.opts.Atomic),
		WithDecodeHook(s.opts.DecodeHook),
	)
}
```

`file/dir.go` 的 import 區塊為：

```go
import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./file/ -v && go vet ./file/`
Expected: PASS，`go vet` 無輸出

- [ ] **Step 5: Commit**

```bash
git add file/dir.go file/dir_test.go
git commit -m "feat(file): add List/Delete/Exists/Sub

List 回名稱字串而非 []*T:舊版一個壞檔就讓整批失敗,而呼叫端多半要
跳過壞檔。Sub 提供繼承選項的子目錄 Store,讓巢狀路徑不必放寬
safeName 的路徑穿越守衛。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 4: JSONL 基礎層 Append / Scan

**Files:**
- Create: `file/jsonl.go`
- Create: `file/jsonl_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Store[T]`、`safeName`、`Path`、`opts`、`ErrStopScan`
- Produces:
  - `func (s *Store[T]) Append(name string, vs ...T) (int, error)`
  - `func (s *Store[T]) Scan(name string, fn func(raw []byte) error) error`

**設計備註：** `Scan` 的回呼只收`原始位元組`、只回 `error`，不回值。這有兩個理由。第一，原始位元組讓呼叫端能處理同一個檔案內混有多種記錄型別的情況（sessiond 的首行是 `Meta`、其餘是 `Turn`），也讓不想解碼的呼叫端直接做位元組比對。第二，若 `Scan` 回傳 `[][]byte`，呼叫端會被迫解碼兩次（一次在判斷、一次在使用）；讓回呼把命中結果 append 進自己的閉包變數就只解一次。

`Scan` 對不存在的檔案回傳 `nil` 而非 `ErrNotFound`。對一個追加日誌而言，「尚未建立」與「空的」是同一個狀態；這與單檔文件不同，`Read` 那邊兩者確有差別，所以維持 `ErrNotFound`。

- [ ] **Step 1: 寫失敗測試 — 建立 `file/jsonl_test.go`**

```go
package file

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// logLine 是 JSONL 測試用的記錄,刻意不實作 validator.IValidator。
type logLine struct {
	Type string `json:"type"`
	Seq  int    `json:"seq"`
	Msg  string `json:"msg"`
}

func newLogStore(t *testing.T) *Store[logLine] {
	t.Helper()
	s, err := NewStore[logLine](t.TempDir(), WithExt(".jsonl"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestAppendWritesOneLinePerRecord(t *testing.T) {
	s := newLogStore(t)
	n, err := s.Append("run1",
		logLine{Type: "turn", Seq: 1, Msg: "a"},
		logLine{Type: "turn", Seq: 2, Msg: "b"})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if n != 2 {
		t.Errorf("Append = %d, want 2", n)
	}
	raw, err := os.ReadFile(s.Path("run1"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("行數 = %d, want 2 (內容: %q)", len(lines), raw)
	}
	if !bytes.HasSuffix(raw, []byte("\n")) {
		t.Error("最後一行應以換行結尾")
	}
}

func TestAppendAccumulates(t *testing.T) {
	s := newLogStore(t)
	if _, err := s.Append("run1", logLine{Seq: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append("run1", logLine{Seq: 2}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.Path("run1"))
	if got := strings.Count(string(raw), "\n"); got != 2 {
		t.Errorf("累計行數 = %d, want 2", got)
	}
}

func TestAppendZeroRecordsIsNoop(t *testing.T) {
	s := newLogStore(t)
	n, err := s.Append("run1")
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if n != 0 {
		t.Errorf("Append = %d, want 0", n)
	}
	if _, err := os.Stat(s.Path("run1")); !os.IsNotExist(err) {
		t.Error("零筆追加不應建立檔案")
	}
}

func TestAppendAppliesFilePerm(t *testing.T) {
	s, _ := NewStore[logLine](t.TempDir(), WithExt(".jsonl"), WithFilePerm(0o600))
	if _, err := s.Append("run1", logLine{Seq: 1}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.Path("run1"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
}

func TestScanVisitsEveryLineInOrder(t *testing.T) {
	s := newLogStore(t)
	_, _ = s.Append("run1", logLine{Seq: 1}, logLine{Seq: 2}, logLine{Seq: 3})

	var seen []int
	err := s.Scan("run1", func(raw []byte) error {
		var l logLine
		if err := json.Unmarshal(raw, &l); err != nil {
			return err
		}
		seen = append(seen, l.Seq)
		return nil
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(seen) != 3 || seen[0] != 1 || seen[1] != 2 || seen[2] != 3 {
		t.Errorf("seen = %v, want [1 2 3]", seen)
	}
}

func TestScanMissingFileIsEmptyNotError(t *testing.T) {
	s := newLogStore(t)
	calls := 0
	err := s.Scan("ghost", func([]byte) error { calls++; return nil })
	if err != nil {
		t.Fatalf("Scan 對缺檔應回 nil,實得 %v", err)
	}
	if calls != 0 {
		t.Errorf("回呼被叫了 %d 次, want 0", calls)
	}
}

func TestScanStopsOnErrStopScan(t *testing.T) {
	s := newLogStore(t)
	_, _ = s.Append("run1", logLine{Seq: 1}, logLine{Seq: 2}, logLine{Seq: 3})

	calls := 0
	err := s.Scan("run1", func([]byte) error {
		calls++
		if calls == 2 {
			return ErrStopScan
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ErrStopScan 不應向外冒出: %v", err)
	}
	if calls != 2 {
		t.Errorf("回呼被叫了 %d 次, want 2", calls)
	}
}

func TestScanPropagatesCallbackError(t *testing.T) {
	s := newLogStore(t)
	_, _ = s.Append("run1", logLine{Seq: 1})
	sentinel := errors.New("boom")
	err := s.Scan("run1", func([]byte) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("Scan err = %v, want 包住 sentinel", err)
	}
}

func TestScanSkipsBlankLines(t *testing.T) {
	s := newLogStore(t)
	if err := os.WriteFile(s.Path("run1"),
		[]byte("{\"seq\":1}\n\n{\"seq\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := s.Scan("run1", func([]byte) error { calls++; return nil }); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("回呼被叫了 %d 次, want 2 (空行應跳過)", calls)
	}
}

func TestScanHandlesLongLines(t *testing.T) {
	s := newLogStore(t)
	big := logLine{Seq: 1, Msg: strings.Repeat("x", 200*1024)}
	if _, err := s.Append("run1", big); err != nil {
		t.Fatal(err)
	}
	var got logLine
	err := s.Scan("run1", func(raw []byte) error { return json.Unmarshal(raw, &got) })
	if err != nil {
		t.Fatalf("Scan 應能處理超過 bufio 預設 64KB 的行: %v", err)
	}
	if len(got.Msg) != 200*1024 {
		t.Errorf("Msg 長度 = %d, want %d", len(got.Msg), 200*1024)
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./file/ -run 'TestAppend|TestScan' -v`
Expected: FAIL，編譯錯誤 `s.Append undefined`、`s.Scan undefined`

- [ ] **Step 3: 建立 `file/jsonl.go`**

```go
package file

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// SCAN_MAX_LINE_BYTES 是單行的上限。bufio.Scanner 預設只吃 64KB,而一筆
// 帶完整回應內容的事件很容易超過。
const SCAN_MAX_LINE_BYTES = 8 * 1024 * 1024

// Append 把每筆記錄序列化成一行 JSON 附加到 name 對應的檔案末端,
// 回傳實際寫入的筆數。
//
// 追加本身就是它的重點,因此不走 atomic 的 temp + rename。零筆時是
// no-op,不會建立檔案。
func (s *Store[T]) Append(name string, vs ...T) (int, error) {
	if err := s.safeName(name); err != nil {
		return 0, err
	}
	if len(vs) == 0 {
		return 0, nil
	}

	// 先全部序列化再開檔:任何一筆的編碼失敗都不該留下半批寫入。
	lines := make([][]byte, 0, len(vs))
	for i, v := range vs {
		raw, err := json.Marshal(v)
		if err != nil {
			return 0, fmt.Errorf("file: marshal %s record %d: %w", name, i, err)
		}
		lines = append(lines, append(raw, '\n'))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.Path(name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, s.opts.FilePerm)
	if err != nil {
		return 0, fmt.Errorf("file: open %s for append: %w", name, err)
	}
	defer f.Close()

	written := 0
	for _, line := range lines {
		if _, err := f.Write(line); err != nil {
			return written, fmt.Errorf("file: append %s: %w", name, err)
		}
		written++
	}
	return written, nil
}

// Scan 逐行讀取 name 對應的檔案,把每行的原始位元組交給 fn。
//
// 這是 JSONL 的基礎層。交出原始位元組而非解好的 T,是為了讓呼叫端能
// 處理同一個檔案內混有多種記錄型別的情況 (例如首行是 meta、其餘是
// turn),也讓不需要解碼的呼叫端可以直接做位元組比對。fn 只回 error、
// 不回值 —— 命中的結果請 append 進 fn 自己的閉包變數,否則呼叫端會被迫
// 解碼兩次。
//
// fn 回傳 ErrStopScan 會提早中止而不視為錯誤 (比照 stdlib 的 fs.SkipAll)。
// 空白行自動跳過。
//
// 檔案不存在時回傳 nil:對一個追加日誌而言,「尚未建立」與「空的」是
// 同一個狀態。這與單檔的 Read 不同,那裡兩者確有差別,所以回 ErrNotFound。
//
// 傳給 fn 的位元組切片只在該次呼叫期間有效,底層緩衝區會被下一行覆寫。
// 需要保留內容請自行複製。
func (s *Store[T]) Scan(name string, fn func(raw []byte) error) error {
	if err := s.safeName(name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.Path(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file: open %s for scan: %w", name, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), SCAN_MAX_LINE_BYTES)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := fn(line); err != nil {
			if errors.Is(err, ErrStopScan) {
				return nil
			}
			return fmt.Errorf("file: scan %s line %d: %w", name, lineNo, err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("file: read %s: %w", name, err)
	}
	return nil
}
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./file/ -v`
Expected: PASS，全綠

- [ ] **Step 5: Commit**

```bash
git add file/jsonl.go file/jsonl_test.go
git commit -m "feat(file): add JSONL primitives Append and Scan

Scan 交出原始位元組而非解好的 T,讓呼叫端能處理混型別檔案 (meta+turn)
並避免雙重解碼。缺檔視為空日誌;ErrStopScan 支援提早中止。行緩衝區
拉到 8MB,因為單筆事件很容易超過 bufio 預設的 64KB。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 5: JSONL 便利層 Find / Filter / Count

**Files:**
- Create: `file/query.go`
- Create: `file/query_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Store[T]`、`ErrStopScan`、`opts.DecodeHook`；Task 4 的 `Scan`、`Append`
- Produces:
  - `func (s *Store[T]) Find(name string, preds ...func(T) bool) (T, bool, error)`
  - `func (s *Store[T]) Filter(name string, preds ...func(T) bool) ([]T, error)`
  - `func (s *Store[T]) Count(name string, preds ...func(T) bool) (int, error)`
  - unexported：`func (s *Store[T]) decodeLine(raw []byte) (T, error)`、`func matchAll[T any](v T, preds []func(T) bool) bool`

**設計備註：** variadic `preds` 以 AND 組合，零個 predicate 表示全部命中。這三個方法蓋掉先前討論的三種需求：`Find` 找特定記錄（sessiond 的 meta 行）、`Filter` 取條件子集（agentSDK 的 `Seq > sinceSeq`）、`Count` 取目前筆數標記（sessiond 判斷 `Index` 是否已持久化）。

它們建在 `Scan` 之上，因此`每一行都會解碼`。同質檔案用它們最省事；若檔案很大又只需要位元組比對（sessiond 現行的 `strings.Contains(line, "\"type\":\"turn\"")` 就是），直接用 `Scan` 會快得多。這是刻意留給呼叫端的取捨，不在此加第二套 raw 版本 API。

- [ ] **Step 1: 寫失敗測試 — 建立 `file/query_test.go`**

```go
package file

import (
	"bytes"
	"testing"
)

func seededLog(t *testing.T) *Store[logLine] {
	t.Helper()
	s := newLogStore(t)
	_, err := s.Append("run1",
		logLine{Type: "meta", Seq: 0, Msg: "header"},
		logLine{Type: "turn", Seq: 1, Msg: "a"},
		logLine{Type: "turn", Seq: 2, Msg: "b"},
		logLine{Type: "turn", Seq: 3, Msg: "c"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFindReturnsFirstMatch(t *testing.T) {
	s := seededLog(t)
	got, ok, err := s.Find("run1", func(l logLine) bool { return l.Type == "meta" })
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !ok {
		t.Fatal("應找到 meta 行")
	}
	if got.Msg != "header" {
		t.Errorf("Msg = %q, want header", got.Msg)
	}
}

func TestFindNoMatch(t *testing.T) {
	s := seededLog(t)
	_, ok, err := s.Find("run1", func(l logLine) bool { return l.Type == "nope" })
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ok {
		t.Error("不該找到")
	}
}

func TestFindMissingFile(t *testing.T) {
	s := newLogStore(t)
	_, ok, err := s.Find("ghost", func(logLine) bool { return true })
	if err != nil {
		t.Fatalf("Find 對缺檔應回 nil error: %v", err)
	}
	if ok {
		t.Error("缺檔不該有命中")
	}
}

func TestFilterAndsMultiplePredicates(t *testing.T) {
	s := seededLog(t)
	got, err := s.Filter("run1",
		func(l logLine) bool { return l.Type == "turn" },
		func(l logLine) bool { return l.Seq > 1 })
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].Seq != 2 || got[1].Seq != 3 {
		t.Errorf("got = %+v, want seq 2,3", got)
	}
}

func TestFilterNoPredicatesReturnsAll(t *testing.T) {
	s := seededLog(t)
	got, err := s.Filter("run1")
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("len = %d, want 4", len(got))
	}
}

func TestFilterMissingFileReturnsEmpty(t *testing.T) {
	s := newLogStore(t)
	got, err := s.Filter("ghost")
	if err != nil {
		t.Fatalf("Filter 對缺檔應回 nil error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestCountWithPredicate(t *testing.T) {
	s := seededLog(t)
	n, err := s.Count("run1", func(l logLine) bool { return l.Type == "turn" })
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Errorf("Count = %d, want 3", n)
	}
}

func TestCountMissingFileIsZero(t *testing.T) {
	s := newLogStore(t)
	n, err := s.Count("ghost")
	if err != nil {
		t.Fatalf("Count 對缺檔應回 nil error: %v", err)
	}
	if n != 0 {
		t.Errorf("Count = %d, want 0", n)
	}
}

func TestQueryAppliesDecodeHook(t *testing.T) {
	s, err := NewStore[logLine](t.TempDir(), WithExt(".jsonl"),
		WithDecodeHook(func(raw []byte) ([]byte, error) {
			return bytes.ReplaceAll(raw, []byte(`"kind":`), []byte(`"type":`)), nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Custom("run1", func(path string) error {
		return writeFileHelper(path, "{\"kind\":\"turn\",\"seq\":1}\n")
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Filter("run1", func(l logLine) bool { return l.Type == "turn" })
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1 (decode hook 應已把 kind 改寫成 type)", len(got))
	}
}

func TestFilterReportsBadLine(t *testing.T) {
	s := newLogStore(t)
	if err := s.Custom("run1", func(path string) error {
		return writeFileHelper(path, "{\"seq\":1}\n{oops\n")
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Filter("run1"); err == nil {
		t.Fatal("壞行應回錯誤")
	}
}
```

`file/query_test.go` 的 import 區塊為 `"bytes"`、`"os"`、`"testing"`，並在檔案底部加入測試輔助函式：

```go
func writeFileHelper(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./file/ -run 'TestFind|TestFilter|TestCount|TestQuery' -v`
Expected: FAIL，編譯錯誤 `s.Find undefined`、`s.Filter undefined`、`s.Count undefined`

- [ ] **Step 3: 建立 `file/query.go`**

```go
package file

import (
	"encoding/json"
	"fmt"
)

// Find 回傳第一筆通過所有 predicate 的記錄。找不到時 ok 為 false。
//
// 命中後立刻停止掃描,適合用來取出檔案內某一筆特定記錄 (例如 JSONL
// 首行的 meta 標頭)。
func (s *Store[T]) Find(name string, preds ...func(T) bool) (T, bool, error) {
	var hit T
	found := false

	err := s.Scan(name, func(raw []byte) error {
		v, err := s.decodeLine(raw)
		if err != nil {
			return err
		}
		if !matchAll(v, preds) {
			return nil
		}
		hit, found = v, true
		return ErrStopScan
	})
	if err != nil {
		var zero T
		return zero, false, err
	}
	return hit, found, nil
}

// Filter 回傳所有通過全部 predicate 的記錄,順序與寫入順序一致。
// 零個 predicate 表示全取。
func (s *Store[T]) Filter(name string, preds ...func(T) bool) ([]T, error) {
	var out []T
	err := s.Scan(name, func(raw []byte) error {
		v, err := s.decodeLine(raw)
		if err != nil {
			return err
		}
		if matchAll(v, preds) {
			out = append(out, v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Count 回傳通過全部 predicate 的記錄筆數。零個 predicate 表示全數。
//
// 典型用途是取得「目前已持久化幾筆」這個標記,讓呼叫端據以跳過重複的
// 追加請求。
func (s *Store[T]) Count(name string, preds ...func(T) bool) (int, error) {
	n := 0
	err := s.Scan(name, func(raw []byte) error {
		v, err := s.decodeLine(raw)
		if err != nil {
			return err
		}
		if matchAll(v, preds) {
			n++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// decodeLine 套用 DecodeHook 後把單行解成 T。
func (s *Store[T]) decodeLine(raw []byte) (T, error) {
	var zero T
	if s.opts.DecodeHook != nil {
		rewritten, err := s.opts.DecodeHook(raw)
		if err != nil {
			return zero, fmt.Errorf("decode hook: %w", err)
		}
		raw = rewritten
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return zero, fmt.Errorf("unmarshal line: %w", err)
	}
	return v, nil
}

// matchAll 以 AND 組合所有 predicate。空的 preds 一律通過。
func matchAll[T any](v T, preds []func(T) bool) bool {
	for _, p := range preds {
		if p != nil && !p(v) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./file/ -v`
Expected: PASS，全綠

- [ ] **Step 5: Commit**

```bash
git add file/query.go file/query_test.go
git commit -m "feat(file): add typed JSONL query layer Find/Filter/Count

建在 Scan 之上,variadic predicate 以 AND 組合。三者對應先前盤點出的
三種需求:Find 取特定記錄、Filter 取條件子集、Count 取已持久化筆數標記。
每行都會解碼,大檔案只需位元組比對的呼叫端請直接用 Scan。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 6: 前綴壓縮 TruncateWhile

**Files:**
- Modify: `file/query.go`（附加）
- Modify: `file/query_test.go`（附加）

**Interfaces:**
- Consumes: Task 1 的 `Store[T]`、`safeName`、`Path`、`opts`；Task 5 的 `decodeLine`
- Produces: `func (s *Store[T]) TruncateWhile(name string, drop func(T) bool) error`

**設計備註：** 這是`前綴丟棄 (prefix drop)`，不是全域過濾。它從檔頭往下掃，只要 `drop` 回 true 就前進；遇到第一筆 `drop` 回 false（或解碼失敗）就停下，把剩餘的部分整批寫回。設計成前綴而非全域，是因為它的用途是壓縮後截斷 WAL —— 已壓縮的部分必然是連續的檔頭區段，用全域過濾反而會在資料損毀時悄悄刪掉中段記錄。

全部行都被丟棄時直接刪檔，而不是留一個空檔案，這樣後續的 `Scan` 走的是「缺檔即空日誌」那條路徑。

- [ ] **Step 1: 寫失敗測試 — 附加到 `file/query_test.go`**

```go
func TestTruncateWhileDropsLeadingPrefix(t *testing.T) {
	s := seededLog(t)
	err := s.TruncateWhile("run1", func(l logLine) bool { return l.Seq <= 1 })
	if err != nil {
		t.Fatalf("TruncateWhile: %v", err)
	}
	got, err := s.Filter("run1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].Seq != 2 || got[1].Seq != 3 {
		t.Errorf("got = %+v, want seq 2,3", got)
	}
}

func TestTruncateWhileStopsAtFirstKeeper(t *testing.T) {
	s := newLogStore(t)
	// seq 1 要丟、seq 5 要留、seq 2 雖然符合 drop 條件但在 keeper 之後,
	// 前綴語意下必須保留。
	_, _ = s.Append("run1", logLine{Seq: 1}, logLine{Seq: 5}, logLine{Seq: 2})
	if err := s.TruncateWhile("run1", func(l logLine) bool { return l.Seq < 3 }); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Filter("run1")
	if len(got) != 2 || got[0].Seq != 5 || got[1].Seq != 2 {
		t.Errorf("got = %+v, want seq 5,2", got)
	}
}

func TestTruncateWhileDropsAllRemovesFile(t *testing.T) {
	s := seededLog(t)
	if err := s.TruncateWhile("run1", func(logLine) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if s.Exists("run1") {
		t.Error("全部丟棄後應刪除檔案")
	}
	n, err := s.Count("run1")
	if err != nil || n != 0 {
		t.Errorf("Count = %d, err = %v, want 0, nil", n, err)
	}
}

func TestTruncateWhileKeepsAll(t *testing.T) {
	s := seededLog(t)
	if err := s.TruncateWhile("run1", func(logLine) bool { return false }); err != nil {
		t.Fatal(err)
	}
	n, _ := s.Count("run1")
	if n != 4 {
		t.Errorf("Count = %d, want 4", n)
	}
}

func TestTruncateWhileMissingFileIsNoop(t *testing.T) {
	s := newLogStore(t)
	if err := s.TruncateWhile("ghost", func(logLine) bool { return true }); err != nil {
		t.Errorf("缺檔應是 no-op,實得 %v", err)
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./file/ -run TestTruncateWhile -v`
Expected: FAIL，編譯錯誤 `s.TruncateWhile undefined`

- [ ] **Step 3: 附加實作到 `file/query.go`**

在檔案末端加入，並把 import 區塊改為包含 `"bytes"` 與 `"os"`：

```go
// TruncateWhile 從檔頭開始丟棄連續符合 drop 的記錄,遇到第一筆不符合
// (或無法解碼) 的記錄就停止,把剩餘內容整批寫回。
//
// 這是前綴丟棄而非全域過濾。用途是壓縮完成後截斷 WAL —— 已壓縮的部分
// 必然是連續的檔頭區段;若做成全域過濾,一旦中段資料損毀就會悄悄刪掉
// 不該刪的記錄。
//
// 全部行都被丟棄時直接刪除檔案,而不是留一個空檔,如此後續的 Scan 會走
// 「缺檔即空日誌」那條路徑。檔案不存在時是 no-op。
func (s *Store[T]) TruncateWhile(name string, drop func(T) bool) error {
	if err := s.safeName(name); err != nil {
		return err
	}
	if drop == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.Path(name)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file: read %s for truncate: %w", name, err)
	}

	pos := 0
	for pos < len(raw) {
		nl := bytes.IndexByte(raw[pos:], '\n')
		end := len(raw)
		next := len(raw)
		if nl >= 0 {
			end = pos + nl
			next = end + 1
		}
		line := bytes.TrimSpace(raw[pos:end])
		if len(line) > 0 {
			v, err := s.decodeLine(line)
			if err != nil || !drop(v) {
				break
			}
		}
		pos = next
	}

	if pos >= len(raw) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("file: remove %s after truncate: %w", name, err)
		}
		return nil
	}
	if pos == 0 {
		return nil
	}
	return s.writeBytes(name, raw[pos:])
}
```

同時把 `file/query.go` 的 import 區塊改為：

```go
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./file/ -v && go vet ./file/`
Expected: PASS，`go vet` 無輸出

- [ ] **Step 5: 跑競態偵測**

Run: `go test ./file/ -race -count=2`
Expected: PASS，無 `DATA RACE` 報告

- [ ] **Step 6: Commit**

```bash
git add file/query.go file/query_test.go
git commit -m "feat(file): add TruncateWhile for WAL prefix compaction

前綴丟棄而非全域過濾:已壓縮區段必然連續,全域過濾在資料損毀時會誤刪
中段記錄。全數丟棄時直接刪檔,讓後續 Scan 走缺檔即空日誌的路徑。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 7: 文件與交付驗證

**Files:**
- Modify: `CLAUDE.md:135-148`（`utils/` 區塊之後的 `file/` 條目，以及「模組對應」表）
- Create: `plans/2026-07-29-file-store-unification.md` 已存在，不需再建

**Interfaces:**
- Consumes: Task 1–6 的完整 API
- Produces: 無程式碼產出

- [ ] **Step 1: 在 `CLAUDE.md` 的專案結構樹中，於 `db/` 條目之後插入 `file/` 區塊**

找到 `├── encode/                  # 編碼轉換模組` 這一行，在它`之前`插入：

```markdown
├── file/                    # 泛型檔案儲存庫(目錄為單位,單檔文件 + JSONL 兩用)
│   ├── store.go             # Store[T] 型別、Options/Option、NewStore、safeName、Custom
│   ├── path.go              # unexported resolvePath/isNil(刻意不依賴 utils,避開 gocsv)
│   ├── document.go          # 單檔文件:Write / Read / ReadOr(atomic temp+rename)
│   ├── dir.go               # 目錄層:List(回名稱字串) / Delete / Exists / Sub
│   ├── jsonl.go             # JSONL 基礎層:Append / Scan(交出原始位元組)
│   ├── query.go             # JSONL 便利層:Find / Filter / Count / TruncateWhile
│   ├── store_test.go        # 建構、選項、名稱守衛、Custom
│   ├── document_test.go     # 單檔讀寫、atomic、權限、validator、decode hook
│   ├── dir_test.go          # 列表、刪除、存在、子目錄
│   ├── jsonl_test.go        # 追加、掃描、提早中止、長行
│   └── query_test.go        # 條件查詢、計數、前綴壓縮
```

- [ ] **Step 2: 在 `CLAUDE.md` 的「關鍵決策」段落末端追加一條**

```markdown
- `gosdk/file` 的 `Store[T]` 本體是`一個目錄`,檔名一律由呼叫端傳入,同時支援「整檔一份 JSON 文件」與「每行一筆的 JSONL 追加日誌」兩種存取型態 —— 兩種都提供但都不強制使用。JSONL 分兩層:`Scan` 是把原始位元組交給呼叫端的基礎層,因此能處理同一檔案內混有多種記錄型別的情況 (例如首行 meta、其餘 turn),也讓不需解碼的呼叫端直接做位元組比對;`Find`/`Filter`/`Count` 是建在其上、會自動解成 `T` 的便利層。`Read` 對缺檔回 `ErrNotFound`,但 JSONL 讀取路徑把「尚未建立」與「空的」視為同一狀態、回空結果與 `nil` —— 對追加日誌而言這兩者確無語意差別。`file` 刻意`不 import gosdk/utils`:`utils/file.go` 內含 `gocarina/gocsv`,而 Go 以 package 為單位載入,任何引用都會把 CSV 函式庫拖進相依圖,代價是 `file/path.go` 自帶約 35 行的 `resolvePath`/`isNil`
```

- [ ] **Step 3: 在 `CLAUDE.md` 的「模組對應」表中，於「通用驗證」那一列之後插入**

```markdown
| 檔案儲存            | `file/`                                                 | `file.NewStore[T]()`                                                                                                      |
```

- [ ] **Step 4: 全套件驗證**

Run:
```bash
cd /Users/shuk/projects/platform/gosdk
go build ./... && go vet ./... && go test ./file/ -race -count=1 -v && go list -deps ./file | grep -E "gocsv|gosdk/utils|gosdk/encode"
```
Expected: build 與 vet 無輸出、`go test` 全 PASS、最後的 `grep` 無輸出並以 exit code 1 結束（代表相依圖乾淨）

- [ ] **Step 5: 確認公開 API 面貌符合計畫**

Run: `go doc ./file`
Expected: 輸出應含 `ErrNotFound`、`ErrStopScan`、`Option`、`Options`、`Store`、`NewStore`、`WithAtomicWrite`、`WithDecodeHook`、`WithDirPerm`、`WithExt`、`WithFilePerm`，且`不含` `FileStore`、`NewFileStore`、`StoreOption`、`UpdateDir`

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(file): document Store[T] structure and design decisions

記錄目錄為本體、單檔+JSONL 兩用、Scan 基礎層 vs typed 便利層的分層,
以及刻意不依賴 gosdk/utils 的理由與代價。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## 範圍外：後續遷移

以下`不在本計畫內`，每個都需要獨立計畫，且各自要先把該 repo 的 gosdk 版本 bump 到含本次變更的 tag：

| Repo | 現行實作 | 對應到新 API |
| --- | --- | --- |
| `ai/auth` | `utils/store.go`、`utils/active.go` | `NewStore[*model.Credential](dir, WithDirPerm(0o700), WithFilePerm(0o600))`；`List()` 回名稱後自行 `Read` + `continue` 以維持跳過壞檔的行為 |
| `ai/agentSDK` | `memory/filestore` 的 `JSONFileStateStore` + `JSONLFileLog` | state 用 `Write`/`Read`；WAL 用 `WithExt(".jsonl")` + `Append`/`Filter(Seq > since)`/`TruncateWhile`；v1→v2 遷移改用 `WithDecodeHook` |
| `ai/agentSDK` | `agent/session/session.go` | `Write`/`Read`/`List` |
| `ai/skills` | `svc/update/store.go`、`svc/stat/cache.go` | `ReadOr("installs", &InstallsFile{Version: 1})`、`Write` |
| `tools/pm2` | `daemon/process_manager.go:256-282` 的 `dump.json` | `Write("dump", entries)` — 順帶取得目前缺少的 atomic 寫入；`Read` 而非 `ReadOr`，因為它對格式不相容有專屬訊息要保留 |
| `ai/sessiond` | `pkg/store/store.go` | `Sub(encodedWorkspace)` + `WithExt(".jsonl")`；idempotent 追加用 `Count(pred)` 或直接用 `Scan` 做位元組比對 |

`ai/sessiond/pkg/install/install.go` 建議`永久排除`：它對 Claude 自己的 `settings.json` 做 read-modify-write，必須保留未知欄位，本質是 `map[string]any` 合併，不是 `T` 的 round-trip。
