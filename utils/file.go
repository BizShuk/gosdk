package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

func GetFileName(fpath string) string {
	base := filepath.Base(fpath)
	fname := strings.TrimSuffix(base, filepath.Ext(base))
	return fname
}

type FileCallback func(string) error

func NewFilelistCallback(pattern string, f FileCallback) error {
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		zap.L().Error("failed to get file list", zap.Any("pattern", pattern), zap.Error(err))
		return err
	}

	for _, fpath := range fileList {
		if err := f(fpath); err != nil {
			return err
		}
	}
	return nil
}

func NewFileOpenCallback(fpath string, fn func(f *os.File) error) error {
	f, err := os.Open(fpath)
	if err != nil {
		zap.L().Error("failed to open file", zap.Any("file", fpath), zap.Error(err))
		return err
	}
	defer f.Close()

	if err := fn(f); err != nil {
		return err
	}

	return nil
}

func NewCSVFilelistCallback(pattern string, rowProcessor RecordProcessor) error {
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		zap.L().Error("file glob failed", zap.Any("pattern", pattern), zap.Error(err))
		return err
	}

	for _, fpath := range fileList {
		if err := ProcessCSVFile(fpath, rowProcessor); err != nil {
			return err
		}
	}
	return nil
}
