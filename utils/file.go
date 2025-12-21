package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

func FileExists(fpath string) bool {
	p, err := filepath.Abs(fpath)
	_, err = os.Stat(p)

	return err == nil

}

func SaveFile(absPath string, payload io.Reader) error {
	zap.L().Info("Save File to ", zap.String("file path", absPath))
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	// Create the file
	out, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", absPath, err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, payload)
	if err != nil {
		return fmt.Errorf("failed to save file %s: %w", absPath, err)
	}

	return nil
}

func SaveCSV(absPath string, rows [][]string) error {
	zap.L().Info("Save File to ", zap.String("file path", absPath))

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	out, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", absPath, err)
	}
	defer out.Close()

	writer := csv.NewWriter(out)
	defer writer.Flush()

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			zap.L().Sugar().Errorf("Warning: could not write row to CSV: %v", err)
		}
	}

	return nil
}

func ParseCSVFile(fpath string) (*csv.Reader, *os.File, error) {
	f, err := os.Open(fpath)
	if err != nil {
		zap.L().Sugar().Errorf("error opening file: %v", err)
		return nil, f, err
	}

	return csv.NewReader(f), f, nil
}
