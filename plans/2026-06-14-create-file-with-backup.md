# 建立具備備份與選項功能的檔案建立 (Create File with Options) 實作計畫

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

`目標` (Goal): 實作 `WriteFile(absPath string, payload io.Reader, opts ...FileOption) error` 核心寫檔函式，並提供 `CreateFile` 別名 (Alias) 以及重構 `SaveFile` 來呼叫它。
- 支援 `WithBackup()` 選項：若啟用且目標檔案已存在，則遞迴地將已存在的檔案備份為 `*.bak`、`*.bak.bak` 等，並移走原檔；若未啟用此選項，則直接覆寫。
- 支援 `WithCreate()` 選項：若啟用且目標檔案不存在，則建立新檔；若未啟用且檔案不存在，則回傳錯誤（例如 `os.ErrNotExist` 相關包裝錯誤）。

`架構` (Architecture):
1. 定義 `FileOption` 函式指標型別與內部 `createOpts` 結構體在 `utils/file_option.go`。
2. 實作 `WriteFile` 於 `utils/file.go` 中：
   - 解析 options，取得最終設定。
   - 檢查檔案是否存在：
     - 若檔案不存在：若 `opts.create` 為 `false`，回傳錯誤；若為 `true`，建立新檔。
     - 若檔案已存在：若 `opts.backup` 為 `true`，呼叫遞迴備份函式 `backupAndRemoveIfExists` 處理已存在檔案；若為 `false`，則直接進行覆寫。
3. 實作 `CreateFile` 為 `WriteFile` 的別名函式。
4. 重構 `SaveFile` 使其呼叫 `WriteFile(absPath, payload, WithCreate())`。

`技術棧` (Tech Stack): Go Standard Library (`os`, `io`, `path/filepath`, `fmt`)

---

### Task 1: 撰寫測試案例與實作功能

`檔案` (Files):
- 修改: `utils/file.go`
- 新增: `utils/file_option.go`
- 測試: `utils/file_test.go`

- [ ] `步驟 1` (Step 1): 在 `utils/file_test.go` 撰寫測試 `TestCreateFileWithOptions`。

```go
func TestCreateFileWithOptions(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "option_test.txt")

	// 1. 檔案不存在，且未指定 WithCreate() -> 應報錯且不建立檔案
	err := CreateFile(fpath, bytes.NewBufferString("no-create")) // 傳入空 option
	if err == nil {
		t.Error("Expected error when file does not exist and WithCreate() is not specified")
	}

	// 2. 檔案不存在，指定 WithCreate() -> 建立檔案
	content1 := "version 1"
	err = CreateFile(fpath, bytes.NewBufferString(content1), WithCreate())
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	data, _ := os.ReadFile(fpath)
	if string(data) != content1 {
		t.Errorf("Expected content %q, got %q", content1, string(data))
	}

	// 3. 檔案已存在，未指定 WithBackup() -> 應直接覆寫 (Override)，不備份
	content2 := "version 2"
	err = CreateFile(fpath, bytes.NewBufferString(content2), WithCreate()) // 未指定 WithBackup
	if err != nil {
		t.Fatalf("Failed to override file: %v", err)
	}
	data, _ = os.ReadFile(fpath)
	if string(data) != content2 {
		t.Errorf("Expected content %q, got %q", content2, string(data))
	}
	if FileExists(fpath + ".bak") {
		t.Error("Expected backup file NOT to exist")
	}

	// 4. 檔案已存在，指定 WithBackup() -> 備份舊檔並寫入新檔
	content3 := "version 3"
	err = CreateFile(fpath, bytes.NewBufferString(content3), WithCreate(), WithBackup())
	if err != nil {
		t.Fatalf("Failed to create with backup: %v", err)
	}
	data, _ = os.ReadFile(fpath)
	if string(data) != content3 {
		t.Errorf("Expected content %q, got %q", content3, string(data))
	}
	bakPath := fpath + ".bak"
	if !FileExists(bakPath) {
		t.Error("Expected backup file to exist")
	}
	bakData, _ := os.ReadFile(bakPath)
	if string(bakData) != content2 {
		t.Errorf("Expected backup content %q, got %q", content2, string(bakData))
	}

	// 5. 再次寫入，且指定 WithBackup() -> 遞迴備份
	content4 := "version 4"
	err = CreateFile(fpath, bytes.NewBufferString(content4), WithCreate(), WithBackup())
	if err != nil {
		t.Fatalf("Failed to create with recursive backup: %v", err)
	}
	data, _ = os.ReadFile(fpath)
	if string(data) != content4 {
		t.Errorf("Expected content %q, got %q", content4, string(data))
	}
	if !FileExists(bakPath) {
		t.Error("Expected backup file to exist")
	}
	bakData, _ = os.ReadFile(bakPath)
	if string(bakData) != content3 {
		t.Errorf("Expected backup content %q, got %q", content3, string(bakData))
	}
	bakBakPath := bakPath + ".bak"
	if !FileExists(bakBakPath) {
		t.Error("Expected recursive backup file to exist")
	}
	bakBakData, _ := os.ReadFile(bakBakPath)
	if string(bakBakData) != content2 {
		t.Errorf("Expected recursive backup content %q, got %q", content2, string(bakBakData))
	}
}
```

- [ ] `步驟 2` (Step 2): 執行測試，驗證測試失敗。

指令: `go test -v ./utils -run TestFileUtilities/CreateFileWithOptions`
預期結果: 編譯失敗，找不到相應實作。

- [ ] `步驟 3` (Step 3): 實作選項機制與核心邏輯。

`utils/file_option.go` 定義：
```go
package utils

type createOpts struct {
	backup bool
	create bool
}

type FileOption func(*createOpts)

func WithBackup() FileOption {
	return func(o *createOpts) {
		o.backup = true
	}
}

func WithCreate() FileOption {
	return func(o *createOpts) {
		o.create = true
	}
}
```

`utils/file.go` 實作：
```go
func SaveFile(absPath string, payload io.Reader) error {
	zap.L().Info("Save File to ", zap.String("file path", absPath))
	return WriteFile(absPath, payload, WithCreate())
}

func WriteFile(absPath string, payload io.Reader, opts ...FileOption) error {
	var config createOpts
	for _, opt := range opts {
		opt(&config)
	}

	exists := FileExists(absPath)
	if !exists {
		if !config.create {
			return fmt.Errorf("file %s does not exist and WithCreate was not specified: %w", absPath, os.ErrNotExist)
		}
	} else {
		if config.backup {
			if err := backupAndRemoveIfExists(absPath); err != nil {
				return fmt.Errorf("failed to backup existing file: %w", err)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	out, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", absPath, err)
	}
	defer out.Close()

	if _, err = io.Copy(out, payload); err != nil {
		return fmt.Errorf("failed to save file %s: %w", absPath, err)
	}

	return nil
}

func CreateFile(absPath string, payload io.Reader, opts ...FileOption) error {
	return WriteFile(absPath, payload, opts...)
}
```

- [ ] `步驟 4` (Step 4): 重新執行測試，驗證測試通過。

- [ ] `步驟 5` (Step 5): 執行完整專案測試，確認無 regression。
