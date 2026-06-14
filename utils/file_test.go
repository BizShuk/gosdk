package utils

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFileUtilities(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("FileExists and SaveFile", func(t *testing.T) {
		fpath := filepath.Join(tmpDir, "test.txt")
		if FileExists(fpath) {
			t.Error("expected file not to exist")
		}

		content := "hello world"
		err := SaveFile(fpath, bytes.NewBufferString(content))
		if err != nil {
			t.Fatalf("failed to save file: %v", err)
		}

		if !FileExists(fpath) {
			t.Error("expected file to exist")
		}

		data, err := os.ReadFile(fpath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != content {
			t.Errorf("expected %q, got %q", content, string(data))
		}
	})

	t.Run("WriteCSV and ParseCSVFile", func(t *testing.T) {
		fpath := filepath.Join(tmpDir, "test.csv")
		rows := [][]string{
			{"col1", "col2"},
			{"val1", "val2"},
		}

		err := WriteCSV(fpath, rows)
		if err != nil {
			t.Fatalf("failed to save csv: %v", err)
		}

		reader, file, err := ParseCSVFile(fpath)
		if err != nil {
			t.Fatalf("failed to parse csv file: %v", err)
		}
		defer file.Close()

		parsedRows, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("failed to read all rows: %v", err)
		}

		if len(parsedRows) != 2 || parsedRows[1][0] != "val1" {
			t.Errorf("unexpected rows content: %v", parsedRows)
		}
	})

	t.Run("WriteCSV with Struct Slice", func(t *testing.T) {
		type TestStruct struct {
			ID   string `csv:"id"`
			Name string `csv:"name"`
		}
		fpath := filepath.Join(tmpDir, "struct_test.csv")
		data := []TestStruct{
			{ID: "1", Name: "Alice"},
			{ID: "2", Name: "Bob"},
		}

		err := WriteCSV(fpath, data)
		if err != nil {
			t.Fatalf("failed to write csv with struct: %v", err)
		}

		content, err := os.ReadFile(fpath)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}

		expected := "id,name\n1,Alice\n2,Bob\n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("GetFileName", func(t *testing.T) {
		expected := "test"
		got := GetFileName("/path/to/test.csv")
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("NewFilelistCallback", func(t *testing.T) {
		fpath1 := filepath.Join(tmpDir, "file1.log")
		fpath2 := filepath.Join(tmpDir, "file2.log")

		_ = SaveFile(fpath1, bytes.NewBufferString("1"))
		_ = SaveFile(fpath2, bytes.NewBufferString("2"))

		var visited []string
		err := NewFilelistCallback(filepath.Join(tmpDir, "*.log"), func(p string) error {
			visited = append(visited, filepath.Base(p))
			return nil
		})
		if err != nil {
			t.Fatalf("callback failed: %v", err)
		}

		if len(visited) != 2 {
			t.Errorf("expected to visit 2 files, visited %v", visited)
		}
	})

	t.Run("NewFileOpenCallback", func(t *testing.T) {
		fpath := filepath.Join(tmpDir, "open_test.txt")
		content := "content"
		_ = SaveFile(fpath, bytes.NewBufferString(content))

		err := NewFileOpenCallback(fpath, func(f *os.File) error {
			data, err := os.ReadFile(f.Name())
			if err != nil {
				return err
			}
			if string(data) != content {
				t.Errorf("expected %q, got %q", content, string(data))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("callback failed: %v", err)
		}
	})

	t.Run("NewCSVFilelistCallback", func(t *testing.T) {
		fpath := filepath.Join(tmpDir, "cb_test.csv")
		rows := [][]string{
			{"header1", "header2"},
			{"v1", "v2"},
		}
		_ = WriteCSV(fpath, rows)

		// 刪除可能存在的歸檔檔
		_ = os.Remove(fpath + ".archived")

		var rowsProcessed int
		err := NewCSVFilelistCallback(fpath, func(fname string, row []string) error {
			rowsProcessed++
			if fname != "cb_test" {
				t.Errorf("expected filename 'cb_test', got %q", fname)
			}
			if len(row) < 2 || row[0] != "v1" {
				t.Errorf("unexpected row content: %v", row)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("callback failed: %v", err)
		}

		if rowsProcessed != 1 {
			t.Errorf("expected 1 row processed, got %d", rowsProcessed)
		}

		// 第二次呼叫，因為有 .archived 檔存在，應該會跳過
		rowsProcessed2 := 0
		err = NewCSVFilelistCallback(fpath, func(fname string, row []string) error {
			rowsProcessed2++
			return nil
		})
		if err != nil {
			t.Fatalf("second callback failed: %v", err)
		}
		if rowsProcessed2 != 0 {
			t.Errorf("expected 0 rows processed due to archive, got %d", rowsProcessed2)
		}
	})

	t.Run("ResolvePath", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to get home directory: %v", err)
		}

		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{"tilde only", "~", home},
			{"tilde with subpath", "~/" + filepath.Base(tmpDir), filepath.Join(home, filepath.Base(tmpDir))},
			{"absolute path", "/absolute/path", "/absolute/path"},
			{"relative path", "./test", ""}, // Context-dependent, skip exact check
			{"../ relative", "../test", ""}, // Context-dependent, skip exact check
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ResolvePath(tt.input)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tt.name == "../ relative" || tt.name == "relative path" {
					// Just verify it doesn't error and returns a clean absolute path
					if !filepath.IsAbs(got) {
						t.Errorf("expected absolute path, got %q", got)
					}
				} else {
					// Normalize for comparison
					expected, _ := filepath.Abs(tt.expected)
					if got != expected {
						t.Errorf("expected %q, got %q", expected, got)
					}
				}
			})
		}
	})

	t.Run("ResolvePath with env vars", func(t *testing.T) {
		home := os.Getenv("HOME")
		if home == "" {
			t.Skip("HOME env not set")
		}

		tests := []struct {
			name  string
			input string
		}{
			{"$HOME env var", "$HOME/test"},
			{"${HOME} env var", "${HOME}/test"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ResolvePath(tt.input)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				expected := filepath.Join(home, "test")
				if got != expected {
					t.Errorf("expected %q, got %q", expected, got)
				}
			})
		}
	})

	t.Run("CreateIfNotExist", func(t *testing.T) {
		fpath := filepath.Join(tmpDir, "config_create_if_not_exist.txt")
		_ = os.Remove(fpath) // 確保檔案不存在

		// Case 1: 檔案不存在，寫入預設值
		defaultValue := "default-value"
		if err := CreateIfNotExist(fpath, defaultValue); err != nil {
			t.Fatalf("unexpected error creating file: %v", err)
		}

		if !FileExists(fpath) {
			t.Fatal("expected file to be created")
		}

		data, err := os.ReadFile(fpath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != defaultValue {
			t.Errorf("expected file content %q, got %q", defaultValue, string(data))
		}

		// Case 2: 檔案已存在，應保持原樣 (忽略新的 defaultValue)
		anotherValue := "another-value"
		if err := CreateIfNotExist(fpath, anotherValue); err != nil {
			t.Fatalf("unexpected error on existing file: %v", err)
		}

		data2, err := os.ReadFile(fpath)
		if err != nil {
			t.Fatalf("failed to read file again: %v", err)
		}
		if string(data2) != defaultValue {
			t.Errorf("expected file content to remain %q, got %q", defaultValue, string(data2))
		}

		// Case 3: 測試家目錄波浪號 (~) 展開支援
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to get home directory: %v", err)
		}
		realPath := filepath.Join(home, "config_create_if_not_exist_test.txt")
		_ = os.Remove(realPath) // 確保家目錄測試檔案原本不存在
		defer os.Remove(realPath)

		if err := CreateIfNotExist("~/config_create_if_not_exist_test.txt", "home-value"); err != nil {
			t.Fatalf("unexpected error creating file in home dir: %v", err)
		}

		if !FileExists(realPath) {
			t.Fatal("expected file to be created in home directory")
		}

		homeData, err := os.ReadFile(realPath)
		if err != nil {
			t.Fatalf("failed to read file from home directory: %v", err)
		}
		if string(homeData) != "home-value" {
			t.Errorf("expected %q, got %q", "home-value", string(homeData))
		}
	})

	t.Run("LoadCSV", func(t *testing.T) {
		type TestStruct struct {
			ID   string `csv:"id"`
			Name string `csv:"name"`
		}
		fpath := filepath.Join(tmpDir, "load_csv_test.csv")
		content := "id,name\n1,Alice\n2,Bob\n"
		if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write mock csv file: %v", err)
		}

		records, err := LoadCSV[TestStruct](fpath)
		if err != nil {
			t.Fatalf("LoadCSV failed: %v", err)
		}

		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}

		if records[0].ID != "1" || records[0].Name != "Alice" {
			t.Errorf("unexpected record 0: %+v", records[0])
		}
		if records[1].ID != "2" || records[1].Name != "Bob" {
			t.Errorf("unexpected record 1: %+v", records[1])
		}
	})

	t.Run("CreateFileWithOptions", func(t *testing.T) {
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
	})

	t.Run("OpenFileAndWriteFileWithWriterOption", func(t *testing.T) {
		fpath := filepath.Join(tmpDir, "writer_option_test.txt")
		_ = os.Remove(fpath)

		var w io.Writer

		// 1. 檔案不存在且未指定 WithCreate()，OpenFile 應失敗且 w 應為 nil
		_, err := OpenFile(fpath, WithReturnWriter(&w))
		if err == nil {
			t.Error("expected error when file does not exist")
		}
		if w != nil {
			t.Errorf("expected w to be nil, got %v", w)
		}

		// 2. 檔案不存在，指定 WithCreate()，OpenFile 應成功，w 應被設為開啟的 *os.File
		file, err := OpenFile(fpath, WithCreate(), WithReturnWriter(&w))
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
		defer file.Close()

		if w == nil {
			t.Fatal("expected w to be set")
		}
		osFile, ok := w.(*os.File)
		if !ok {
			t.Fatalf("expected w to be *os.File, got %T", w)
		}
		if osFile.Name() != fpath {
			t.Errorf("expected file path %q, got %q", fpath, osFile.Name())
		}

		// 3. 測試 WriteFile 搭配 WithReturnWriter 成功寫入
		fpath2 := filepath.Join(tmpDir, "writer_option_write_test.txt")
		var w2 io.Writer
		err = WriteFile(fpath2, bytes.NewBufferString("hello"), WithCreate(), WithReturnWriter(&w2))
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if w2 == nil {
			t.Fatal("expected w2 to be set")
		}
		osFile2, ok := w2.(*os.File)
		if !ok {
			t.Fatalf("expected w2 to be *os.File, got %T", w2)
		}
		if osFile2.Name() != fpath2 {
			t.Errorf("expected file path %q, got %q", fpath2, osFile2.Name())
		}
	})
}


