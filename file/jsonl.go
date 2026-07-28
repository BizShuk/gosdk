package file

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// SCAN_MAX_LINE_BYTES 是單行的上限。bufio.Scanner 預設只吃 64KB,而一筆
// 帶完整回應內容的事件很容易超過。
const SCAN_MAX_LINE_BYTES = 8 * 1024 * 1024

// Append 把每筆記錄序列化成一行 JSON 附加到 name 對應的檔案末端,
// 回傳實際寫入的筆數。
//
// 追加本身就是它的重點,因此不走 atomic 的 temp + rename。零筆時是
// no-op,不會建立檔案。
func (s *Store[T]) Append(name string, vs ...T) (int, error) {
	if err := s.safeName(name); err != nil {
		return 0, err
	}
	if len(vs) == 0 {
		return 0, nil
	}

	// 先全部序列化再開檔:任何一筆的編碼失敗都不該留下半批寫入。
	lines := make([][]byte, 0, len(vs))
	for i, v := range vs {
		raw, err := json.Marshal(v)
		if err != nil {
			return 0, fmt.Errorf("file: marshal %s record %d: %w", name, i, err)
		}
		lines = append(lines, append(raw, '\n'))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.Path(name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, s.opts.FilePerm)
	if err != nil {
		return 0, fmt.Errorf("file: open %s for append: %w", name, err)
	}
	defer f.Close()

	written := 0
	for _, line := range lines {
		if _, err := f.Write(line); err != nil {
			return written, fmt.Errorf("file: append %s: %w", name, err)
		}
		written++
	}
	return written, nil
}

// Scan 逐行讀取 name 對應的檔案,把每行的原始位元組交給 fn。
//
// 這是 JSONL 的基礎層。交出原始位元組而非解好的 T,是為了讓呼叫端能
// 處理同一個檔案內混有多種記錄型別的情況 (例如首行是 meta、其餘是
// turn),也讓不需要解碼的呼叫端可以直接做位元組比對。fn 只回 error、
// 不回值 —— 命中的結果請 append 進 fn 自己的閉包變數,否則呼叫端會被迫
// 解碼兩次。
//
// fn 回傳 ErrStopScan 會提早中止而不視為錯誤 (比照 stdlib 的 fs.SkipAll)。
// 空白行自動跳過。
//
// 檔案不存在時回傳 nil:對一個追加日誌而言,「尚未建立」與「空的」是
// 同一個狀態。這與單檔的 Read 不同,那裡兩者確有差別,所以回 ErrNotFound。
//
// 傳給 fn 的位元組切片只在該次呼叫期間有效,底層緩衝區會被下一行覆寫。
// 需要保留內容請自行複製。
func (s *Store[T]) Scan(name string, fn func(raw []byte) error) error {
	if err := s.safeName(name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.Path(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file: open %s for scan: %w", name, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), SCAN_MAX_LINE_BYTES)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := fn(line); err != nil {
			if errors.Is(err, ErrStopScan) {
				return nil
			}
			return fmt.Errorf("file: scan %s line %d: %w", name, lineNo, err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("file: read %s: %w", name, err)
	}
	return nil
}
