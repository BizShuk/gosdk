package utils

import (
	"encoding/csv"
	"io"
	"os"

	"go.uber.org/zap"
)

// RecordProcessor is a callback function for processing a single CSV row.
type RecordProcessor func(fname string, row []string) error

// ProcessCSVFile parses a CSV file and iterates over its rows.
// It skips the specified number of header lines and ignores rows with fewer than minCols columns.
// archive parameter determines whether to create .archived mark file.
func ProcessCSVFile(fpath string, archive bool, processor RecordProcessor) error {
	if archive {
		if _, err := os.Stat(fpath + ".archived"); err == nil {
			return nil
		}
	}

	defer func() {
		if archive {
			if _, err := os.Create(fpath + ".archived"); err != nil {
				zap.L().Error("failed to create archived file", zap.Any("file", fpath))
			}
		}
	}()

	f, err := os.Open(fpath)
	if err != nil {
		zap.L().Error("failed to open file", zap.Any("file", fpath), zap.Error(err))
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	fname := GetFileName(fpath)

	for i := 0; ; i++ {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if i < 1 {
			continue
		}
		if len(row) < 2 {
			continue
		}
		if err := processor(fname, row); err != nil {
			zap.L().Error("process row failed", zap.Error(err))
			continue
		}
	}

	return nil
}
