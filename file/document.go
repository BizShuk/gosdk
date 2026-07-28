package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/bizshuk/gosdk/validator"
)

// Write 把 v 序列化為 JSON 並寫入 name 對應的檔案。
//
// 若 v 實作 validator.IValidator,會先驗證再寫入 —— 驗證失敗時不會產生
// 任何檔案。預設走 temp + rename 的 atomic 寫入,中途中斷不會留下半截檔案。
func (s *Store[T]) Write(name string, v T) error {
	if err := s.safeName(name); err != nil {
		return err
	}

	if val, ok := any(v).(validator.IValidator); ok && !isNil(val) {
		if err := val.Validate(); err != nil {
			return fmt.Errorf("file: validation failed for %s: %w", name, err)
		}
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("file: marshal %s: %w", name, err)
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeBytes(name, data)
}

// Read 讀取並反序列化 name 對應的檔案。檔案不存在時回傳包住 ErrNotFound
// 的錯誤,呼叫端可用 errors.Is 判斷。
func (s *Store[T]) Read(name string) (T, error) {
	var zero T
	if err := s.safeName(name); err != nil {
		return zero, err
	}

	s.mu.Lock()
	raw, err := s.readBytes(name)
	s.mu.Unlock()
	if err != nil {
		return zero, err
	}

	if s.opts.DecodeHook != nil {
		raw, err = s.opts.DecodeHook(raw)
		if err != nil {
			return zero, fmt.Errorf("file: decode hook %s: %w", name, err)
		}
	}

	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return zero, fmt.Errorf("file: unmarshal %s: %w", name, err)
	}
	return v, nil
}

// ReadOr 是 Read 的無錯誤版本:任何失敗都退回 def。
//
// 它吞掉的不只是「檔案不存在」—— JSON 損毀與權限錯誤同樣會靜靜回傳
// def。只在「缺檔是唯一預期失敗」時使用;需要區分損毀情況的呼叫端請用 Read。
func (s *Store[T]) ReadOr(name string, def T) T {
	v, err := s.Read(name)
	if err != nil {
		return def
	}
	return v
}

// writeBytes 把 data 寫進 name 對應的檔案。呼叫端須持有 s.mu。
func (s *Store[T]) writeBytes(name string, data []byte) error {
	path := s.Path(name)

	if !s.opts.Atomic {
		if err := os.WriteFile(path, data, s.opts.FilePerm); err != nil {
			return fmt.Errorf("file: write %s: %w", name, err)
		}
		return nil
	}

	tmp, err := os.CreateTemp(s.dir, ".tmp-*"+s.opts.Ext)
	if err != nil {
		return fmt.Errorf("file: create temp for %s: %w", name, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // rename 成功後這裡是 no-op

	if err := tmp.Chmod(s.opts.FilePerm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("file: chmod temp for %s: %w", name, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("file: write temp for %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("file: close temp for %s: %w", name, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("file: commit %s: %w", name, err)
	}
	return nil
}

// readBytes 讀出 name 對應的檔案內容。呼叫端須持有 s.mu。
func (s *Store[T]) readBytes(name string) ([]byte, error) {
	raw, err := os.ReadFile(s.Path(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("file: read %s: %w", name, err)
	}
	return raw, nil
}
