package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	sdkcsv "github.com/bizshuk/gosdk/encode/csv"
	"github.com/gocarina/gocsv"
)

func FileExists(fpath string) bool {
	p, err := filepath.Abs(fpath)
	_, err = os.Stat(p)

	return err == nil
}

// Deprecated: use WriteFile instead.
func SaveFile(absPath string, payload io.Reader) error {
	return WriteFile(absPath, payload, WithCreate())
}

// WriteCSV writes data (either [][]string or a slice of structs) to a CSV file.
// If data is [][]string, it writes the raw rows using standard encoding/csv.
// Otherwise, it marshals the struct slice using gocsv.
func WriteCSV(absPath string, data any) error {
	slog.Debug("Save CSV to", "file_path", absPath)

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
				slog.Debug("Warning: could not write row to CSV", "err", err)
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
		slog.Debug("error opening file", "err", err)
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
		slog.Error("failed to get file list", "pattern", pattern, "err", err)
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
		slog.Error("failed to open file", "file", fpath, "err", err)
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
		slog.Error("file glob failed", "pattern", pattern, "err", err)
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
		slog.Debug("Writing default value to", "path", path)
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

// OpenFile opens or creates a file at absPath according to provided options.
func OpenFile(absPath string, opts ...FileOptionFunc) (*os.File, error) {
	var config FileOptions
	for _, opt := range opts {
		opt(&config)
	}

	if config.writer != nil {
		*config.writer = nil
	}

	exists := FileExists(absPath)
	if !exists {
		if !config.create {
			return nil, fmt.Errorf("file %s does not exist and WithCreate was not specified: %w", absPath, os.ErrNotExist)
		}
	} else {
		if config.backup {
			if err := backupAndRemoveIfExists(absPath); err != nil {
				return nil, fmt.Errorf("failed to backup existing file: %w", err)
			}
			if err := pruneBackups(absPath, config.backupKeep); err != nil {
				return nil, fmt.Errorf("failed to prune backups: %w", err)
			}
		}
	}

	// 確保目錄存在
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// 建立或開啟檔案進行寫入（直接 truncate 覆寫）
	out, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", absPath, err)
	}

	if config.writer != nil {
		*config.writer = out
	}

	return out, nil
}

// WriteFile writes payload to absPath according to provided options.
// Default behavior is to fail if file doesn't exist, and override if it does.
func WriteFile(absPath string, payload io.Reader, opts ...FileOptionFunc) error {
	var config FileOptions
	for _, opt := range opts {
		opt(&config)
	}

	out, err := OpenFile(absPath, opts...)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, payload); err != nil {
		if config.writer != nil {
			*config.writer = nil
		}
		return fmt.Errorf("failed to save file %s: %w", absPath, err)
	}

	return nil
}

// CreateFile is an alias for WriteFile.
func CreateFile(absPath string, payload io.Reader, opts ...FileOptionFunc) error {
	return WriteFile(absPath, payload, opts...)
}

// BACKUP_EXT is appended to a file's path to name its latest backup.
const BACKUP_EXT = ".bak"

// BACKUP_DATE_LAYOUT names a rotated backup: <path>.bak.<date>, the date being
// the rotated backup's modification time, i.e. when that content was last written.
const BACKUP_DATE_LAYOUT = "20060102-150405"

// backupAndRemoveIfExists moves path to path.bak. An existing path.bak is first
// rotated to path.bak.<date> so it is never overwritten; path.bak is always the
// most recent backup.
func backupAndRemoveIfExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	backupPath := path + BACKUP_EXT
	if info, err := os.Stat(backupPath); err == nil {
		if err := moveFile(backupPath, datedBackupPath(backupPath, info.ModTime())); err != nil {
			return fmt.Errorf("rotate backup %s: %w", backupPath, err)
		}
	}

	if err := moveFile(path, backupPath); err != nil {
		return fmt.Errorf("rename/copy backup failed: %w", err)
	}

	return nil
}

// datedBackupPath returns backupPath.<date>, adding -1, -2, ... when two
// backups share the same second.
func datedBackupPath(backupPath string, modTime time.Time) string {
	base := backupPath + "." + modTime.Format(BACKUP_DATE_LAYOUT)
	candidate := base
	for i := 1; FileExists(candidate); i++ {
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return candidate
}

// datedBackup is one <path>.bak.<date>[-<n>] file.
type datedBackup struct {
	path string
	at   time.Time
	seq  int
}

// pruneBackups keeps <path>.bak and the newest keep-1 dated backups, deleting
// the rest. keep <= 0 keeps everything. Files whose suffix is not a
// BACKUP_DATE_LAYOUT date are never touched.
func pruneBackups(path string, keep int) error {
	if keep <= 0 {
		return nil
	}
	prefix := filepath.Base(path) + BACKUP_EXT + "."
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return err
	}

	var dated []datedBackup
	for _, e := range entries {
		suffix, ok := strings.CutPrefix(e.Name(), prefix)
		if !ok || e.IsDir() {
			continue
		}
		stamp, seqText, _ := strings.Cut(suffix, "-") // layout is YYYYMMDD-HHMMSS
		rest, seqText, hasSeq := strings.Cut(seqText, "-")
		at, err := time.ParseInLocation(BACKUP_DATE_LAYOUT, stamp+"-"+rest, time.Local)
		if err != nil {
			continue
		}
		seq := 0
		if hasSeq {
			if seq, err = strconv.Atoi(seqText); err != nil || seq < 1 {
				continue
			}
		}
		dated = append(dated, datedBackup{path: filepath.Join(filepath.Dir(path), e.Name()), at: at, seq: seq})
	}

	sort.Slice(dated, func(i, j int) bool { // newest first
		if !dated[i].at.Equal(dated[j].at) {
			return dated[i].at.After(dated[j].at)
		}
		return dated[i].seq > dated[j].seq
	})
	for _, b := range dated[min(keep-1, len(dated)):] {
		if err := os.Remove(b.path); err != nil {
			return err
		}
	}
	return nil
}

// moveFile renames src to dst, falling back to copy + remove across devices.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return copyAndRemove(src, dst)
	}
	return nil
}

// copyAndRemove copies src to dst and then removes src.
func copyAndRemove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	in.Close()
	out.Close()

	return os.Remove(src)
}
