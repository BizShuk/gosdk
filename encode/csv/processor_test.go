package csv

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessCSVFile(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test_processor.csv")

	// 建立測試用的 CSV 檔案
	f, err := os.Create(fpath)
	if err != nil {
		t.Fatalf("failed to create test csv file: %v", err)
	}
	writer := csv.NewWriter(f)
	rows := [][]string{
		{"header1", "header2"}, // header，第 0 行，會被 ProcessCSVFile 跳過 (i < 1)
		{"val1", "val2"},       // 第 1 行，符合長度 >= 2，會被處理
		{"val3"},               // 第 2 行，長度 < 2，會被略過 (len(row) < 2)
		{"val4", "val5"},       // 第 3 行，符合長度 >= 2，會被處理
	}
	for _, r := range rows {
		_ = writer.Write(r)
	}
	writer.Flush()
	f.Close()

	t.Run("without archiving", func(t *testing.T) {
		var processed [][]string
		var fnames []string
		err := ProcessCSVFile(fpath, false, func(fname string, row []string) error {
			processed = append(processed, row)
			fnames = append(fnames, fname)
			return nil
		})

		if err != nil {
			t.Fatalf("ProcessCSVFile failed: %v", err)
		}

		if len(processed) != 2 {
			t.Errorf("expected 2 processed rows, got %d", len(processed))
		}

		if processed[0][0] != "val1" || processed[1][0] != "val4" {
			t.Errorf("unexpected rows content: %v", processed)
		}

		for _, name := range fnames {
			if name != "test_processor" {
				t.Errorf("expected filename 'test_processor', got %q", name)
			}
		}

		// 檢查是否有產生 .archived 檔（因為 archive = false，所以不應該產生）
		if _, err := os.Stat(fpath + ".archived"); err == nil {
			t.Error(".archived file should not be created when archive is false")
		}
	})

	t.Run("with archiving", func(t *testing.T) {
		var processed [][]string
		err := ProcessCSVFile(fpath, true, func(fname string, row []string) error {
			processed = append(processed, row)
			return nil
		})

		if err != nil {
			t.Fatalf("ProcessCSVFile failed: %v", err)
		}

		if len(processed) != 2 {
			t.Errorf("expected 2 processed rows, got %d", len(processed))
		}

		// 檢查是否有產生 .archived 檔
		if _, err := os.Stat(fpath + ".archived"); os.IsNotExist(err) {
			t.Error(".archived file should be created when archive is true")
		}

		// 再次執行（archive = true），因為有 .archived，應該要直接跳過，不執行 callback
		var processedSecondRun [][]string
		err = ProcessCSVFile(fpath, true, func(fname string, row []string) error {
			processedSecondRun = append(processedSecondRun, row)
			return nil
		})

		if err != nil {
			t.Fatalf("ProcessCSVFile second run failed: %v", err)
		}

		if len(processedSecondRun) != 0 {
			t.Errorf("expected 0 processed rows in second run, got %d", len(processedSecondRun))
		}
	})
}
