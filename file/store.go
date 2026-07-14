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
	dir string
}

// NewFileStore 建立並初始化一個 FileStore 實例。
// 若指定目錄不存在，會自動嘗試建立。
func NewFileStore[T any](dir string) (*FileStore[T], error) {
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

	return &FileStore[T]{dir: absDir}, nil
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
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
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

