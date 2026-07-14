package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bizshuk/gosdk/utils"
	"github.com/bizshuk/gosdk/validator"
)

// FileStore 是一個泛型檔案儲存庫，用於在指定目錄下存取物件
type FileStore[T any] struct {
	dir  string
	perm os.FileMode
}

// StoreOption 是一個用於設定 FileStore 的函式類型
type StoreOption[T any] func(*FileStore[T])

// WithFilePerm 設定 FileStore 儲存檔案時的檔案權限 (file permission)
func WithFilePerm[T any](perm os.FileMode) StoreOption[T] {
	return func(s *FileStore[T]) {
		s.perm = perm
	}
}

// NewFileStore 建立並初始化一個 FileStore 實例。
// 若指定目錄不存在，會自動嘗試建立。
func NewFileStore[T any](dir string, opts ...StoreOption[T]) (*FileStore[T], error) {
	if dir == "" {
		return nil, errors.New("directory path cannot be empty")
	}

	absDir, err := utils.ResolvePath(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve directory path: %w", err)
	}

	// 確保目錄存在
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", absDir, err)
	}

	s := &FileStore[T]{
		dir:  absDir,
		perm: 0o644,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Store 將物件以 JSON 格式儲存至名為 name.json 的檔案中。
// 在儲存前，會先檢查物件是否實作 validator.IValidator 介面並執行驗證。
func (s *FileStore[T]) Store(name string, obj T) error {
	if name == "" {
		return errors.New("file name cannot be empty")
	}

	// 安全性檢查：防範路徑穿越 (path traversal)
	if strings.ContainsAny(name, `/\`) || filepath.Base(name) != name {
		return fmt.Errorf("invalid file name: %s", name)
	}

	// 第一步：驗證物件是否實作 validator.IValidator 介面
	if val, ok := any(obj).(validator.IValidator); ok {
		if !utils.IsNil(val) {
			if err := val.Validate(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
		}
	}

	// 序列化為 JSON 格式
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	filePath := filepath.Join(s.dir, name+".json")
	if err := os.WriteFile(filePath, data, s.perm); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Read 自 name.json 檔案讀取 JSON 物件並反序列化為泛型 T。
func (s *FileStore[T]) Read(name string) (T, error) {
	var zero T

	if name == "" {
		return zero, errors.New("file name cannot be empty")
	}

	// 安全性檢查
	if strings.ContainsAny(name, `/\`) || filepath.Base(name) != name {
		return zero, fmt.Errorf("invalid file name: %s", name)
	}

	filePath := filepath.Join(s.dir, name+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return zero, fmt.Errorf("failed to read file: %w", err)
	}

	var obj T
	if err := json.Unmarshal(data, &obj); err != nil {
		return zero, fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return obj, nil
}

// UpdateDir 更新儲存庫的專屬目錄路徑。
// 若指定的新目錄不存在，會自動嘗試建立。
func (s *FileStore[T]) UpdateDir(dir string) error {
	if dir == "" {
		return errors.New("directory path cannot be empty")
	}

	absDir, err := utils.ResolvePath(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve directory path: %w", err)
	}

	// 確保目錄存在
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", absDir, err)
	}

	s.dir = absDir
	return nil
}

// Dir 回傳憑證目錄。
func (s *FileStore[T]) Dir() string { return s.dir }

// Path 回傳某個憑證的檔案路徑。
func (s *FileStore[T]) Path(name string) string {
	return filepath.Join(s.dir, name+".json")
}

// List 讀取目錄下所有的 JSON 檔案並反序列化為 []*T。
func (s *FileStore[T]) List() ([]*T, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var list []*T
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		obj, err := s.Read(name)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}
		val := obj
		list = append(list, &val)
	}
	return list, nil
}

// Delete 刪除指定的 JSON 檔案。
func (s *FileStore[T]) Delete(name string) error {
	if name == "" {
		return errors.New("file name cannot be empty")
	}

	// 安全性檢查
	if strings.ContainsAny(name, `/\`) || filepath.Base(name) != name {
		return fmt.Errorf("invalid file name: %s", name)
	}

	filePath := s.Path(name)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %w", err)
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}
