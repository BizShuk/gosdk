package file

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Find 回傳第一筆通過所有 predicate 的記錄。找不到時 ok 為 false。
//
// 命中後立刻停止掃描,適合用來取出檔案內某一筆特定記錄 (例如 JSONL
// 首行的 meta 標頭)。
func (s *Store[T]) Find(name string, preds ...func(T) bool) (T, bool, error) {
	var hit T
	found := false

	err := s.Scan(name, func(raw []byte) error {
		v, err := s.decodeLine(raw)
		if err != nil {
			return err
		}
		if !matchAll(v, preds) {
			return nil
		}
		hit, found = v, true
		return ErrStopScan
	})
	if err != nil {
		var zero T
		return zero, false, err
	}
	return hit, found, nil
}

// Filter 回傳所有通過全部 predicate 的記錄,順序與寫入順序一致。
// 零個 predicate 表示全取。
func (s *Store[T]) Filter(name string, preds ...func(T) bool) ([]T, error) {
	var out []T
	err := s.Scan(name, func(raw []byte) error {
		v, err := s.decodeLine(raw)
		if err != nil {
			return err
		}
		if matchAll(v, preds) {
			out = append(out, v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Count 回傳通過全部 predicate 的記錄筆數。零個 predicate 表示全數。
//
// 典型用途是取得「目前已持久化幾筆」這個標記,讓呼叫端據以跳過重複的
// 追加請求。
func (s *Store[T]) Count(name string, preds ...func(T) bool) (int, error) {
	n := 0
	err := s.Scan(name, func(raw []byte) error {
		v, err := s.decodeLine(raw)
		if err != nil {
			return err
		}
		if matchAll(v, preds) {
			n++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// decodeLine 套用 DecodeHook 後把單行解成 T。
func (s *Store[T]) decodeLine(raw []byte) (T, error) {
	var zero T
	if s.opts.DecodeHook != nil {
		rewritten, err := s.opts.DecodeHook(raw)
		if err != nil {
			return zero, fmt.Errorf("decode hook: %w", err)
		}
		raw = rewritten
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return zero, fmt.Errorf("unmarshal line: %w", err)
	}
	return v, nil
}

// TruncateWhile 從檔頭開始丟棄連續符合 drop 的記錄,遇到第一筆不符合
// (或無法解碼) 的記錄就停止,把剩餘內容整批寫回。
//
// 這是前綴丟棄而非全域過濾。用途是壓縮完成後截斷 WAL —— 已壓縮的部分
// 必然是連續的檔頭區段;若做成全域過濾,一旦中段資料損毀就會悄悄刪掉
// 不該刪的記錄。
//
// 全部行都被丟棄時直接刪除檔案,而不是留一個空檔,如此後續的 Scan 會走
// 「缺檔即空日誌」那條路徑。檔案不存在時是 no-op。
func (s *Store[T]) TruncateWhile(name string, drop func(T) bool) error {
	if err := s.safeName(name); err != nil {
		return err
	}
	if drop == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.Path(name)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file: read %s for truncate: %w", name, err)
	}

	pos := 0
	for pos < len(raw) {
		nl := bytes.IndexByte(raw[pos:], '\n')
		end := len(raw)
		next := len(raw)
		if nl >= 0 {
			end = pos + nl
			next = end + 1
		}
		line := bytes.TrimSpace(raw[pos:end])
		if len(line) > 0 {
			v, err := s.decodeLine(line)
			if err != nil || !drop(v) {
				break
			}
		}
		pos = next
	}

	if pos >= len(raw) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("file: remove %s after truncate: %w", name, err)
		}
		return nil
	}
	if pos == 0 {
		return nil
	}
	return s.writeBytes(name, raw[pos:])
}

// matchAll 以 AND 組合所有 predicate。空的 preds 一律通過。
func matchAll[T any](v T, preds []func(T) bool) bool {
	for _, p := range preds {
		if p != nil && !p(v) {
			return false
		}
	}
	return true
}
