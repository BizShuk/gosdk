package utils

import (
	"bytes"
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

	t.Run("SaveCSV and ParseCSVFile", func(t *testing.T) {
		fpath := filepath.Join(tmpDir, "test.csv")
		rows := [][]string{
			{"col1", "col2"},
			{"val1", "val2"},
		}

		err := SaveCSV(fpath, rows)
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
		_ = SaveCSV(fpath, rows)

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
			{"../ relative", "../test", ""},                          // Context-dependent, skip exact check
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
}
