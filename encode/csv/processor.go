package csv

import (
	"encoding/csv"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
				slog.Error("failed to create archived file", "file", fpath)
			}
		}
	}()

	f, err := os.Open(fpath)
	if err != nil {
		slog.Error("failed to open file", "file", fpath, "err", err)
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	fname := getFileName(fpath)

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
			slog.Error("process row failed", "err", err)
			continue
		}
	}

	return nil
}

func getFileName(fpath string) string {
	base := filepath.Base(fpath)
	fname := strings.TrimSuffix(base, filepath.Ext(base))
	return fname
}
