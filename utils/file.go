package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	sdkcsv "github.com/bizshuk/gosdk/encode/csv"
	"github.com/gocarina/gocsv"
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
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
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

// WriteCSV writes data (either [][]string or a slice of structs) to a CSV file.
// If data is [][]string, it writes the raw rows using standard encoding/csv.
// Otherwise, it marshals the struct slice using gocsv.
func WriteCSV(absPath string, data any) error {
	zap.L().Info("Save CSV to ", zap.String("file path", absPath))

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	out, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", absPath, err)
	}
	defer out.Close()

	switch v := data.(type) {
	case [][]string:
		writer := csv.NewWriter(out)
		defer writer.Flush()

		for _, row := range v {
			if err := writer.Write(row); err != nil {
				zap.L().Sugar().Errorf("Warning: could not write row to CSV: %v", err)
			}
		}
		return nil
	default:
		if err := gocsv.MarshalFile(v, out); err != nil {
			return fmt.Errorf("failed to marshal csv: %w", err)
		}
		return nil
	}
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

// Deprecated: use github.com/mitchellh/go-homedir or similar specialized package instead.
func ResolvePath(path string) (string, error) {
	// Expand environment variables first ($HOME, ${HOME}, etc.)
	expanded := os.ExpandEnv(path)

	// Expand ~ to user's home directory
	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get user home directory: %w", err)
		}
		if len(expanded) == 1 {
			expanded = home
		} else if expanded[1] == '/' || expanded[1] == '\\' {
			expanded = filepath.Join(home, expanded[2:])
		}
	}

	// Convert to absolute path and clean
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("convert to absolute path: %w", err)
	}

	// Clean the path to resolve ./ and ../
	return filepath.Clean(absPath), nil
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

func NewCSVFilelistCallback(pattern string, rowProcessor sdkcsv.RecordProcessor) error {
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		zap.L().Error("file glob failed", zap.Any("pattern", pattern), zap.Error(err))
		return err
	}

	for _, fpath := range fileList {
		// Default to archive=true for backward compatibility in file list callback
		if err := sdkcsv.ProcessCSVFile(fpath, true, rowProcessor); err != nil {
			return err
		}
	}
	return nil
}

// CreateIfNotExist checks if a file exists at the given path.
// If the file does not exist, it creates one with the provided defaultValue.
// It supports "~" as the home directory.
func CreateIfNotExist(path string, defaultValue string) error {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get user home directory: %w", err)
		}
		if len(path) == 1 {
			path = home
		} else if path[1] == '/' || path[1] == '\\' {
			path = filepath.Join(home, path[2:])
		}
	}

	dir := filepath.Dir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		zap.L().Info("Writing default value to", zap.String("path", path))
		if err := os.WriteFile(path, []byte(defaultValue), 0o644); err != nil {
			return fmt.Errorf("write default value to %s: %w", path, err)
		}
	}

	return nil
}

// LoadCSV 是一個泛型函數，能將 CSV 檔案直接解析為任意指定結構體 (Structure) 的切片 (Slice)
func LoadCSV[T any](inputPath string) ([]T, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", inputPath, err)
	}
	defer f.Close()

	var records []T
	if err := gocsv.UnmarshalFile(f, &records); err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	return records, nil
}
