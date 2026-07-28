package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// List 回傳目錄下所有符合副檔名的檔案名稱 (不含副檔名),依字典序排序。
//
// 它刻意不解碼任何檔案。舊版回傳 []*T 並在解碼失敗時中止整批呼叫,但
// 實務上呼叫端多半希望跳過壞檔而不是整個列表失敗。回名稱列表讓呼叫端
// 自行 Read + continue,也讓只需要列名的呼叫端不必付解碼成本。
//
// 以點開頭的檔案一律跳過 —— atomic 寫入產生的 .tmp-* 暫存檔就長這樣。
func (s *Store[T]) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("file: list %s: %w", s.dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || strings.HasPrefix(n, ".") {
			continue
		}
		if s.opts.Ext != "" && !strings.HasSuffix(n, s.opts.Ext) {
			continue
		}
		names = append(names, strings.TrimSuffix(n, s.opts.Ext))
	}
	sort.Strings(names)
	return names, nil
}

// Exists 回報 name 對應的檔案是否存在。名稱不安全時一律回 false。
func (s *Store[T]) Exists(name string) bool {
	if err := s.safeName(name); err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := os.Stat(s.Path(name))
	return err == nil
}

// Delete 移除 name 對應的檔案。檔案不存在時回傳包住 ErrNotFound 的錯誤。
func (s *Store[T]) Delete(name string) error {
	if err := s.safeName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.Path(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return fmt.Errorf("file: delete %s: %w", name, err)
	}
	return nil
}

// Sub 回傳一個以子目錄為根、繼承全部選項的新 Store。
//
// 用於需要巢狀路徑的場景 (例如 sessions/<workspace>/<id>.jsonl):
// safeName 拒絕含 / 的名稱,Sub 是取得第二層的正當途徑。
//
// 注意:子 Store 有自己的互斥鎖,鎖不會跨越父子 —— 需要跨層原子性的
// 呼叫端必須自行協調。
func (s *Store[T]) Sub(seg string) (*Store[T], error) {
	if err := s.safeName(seg); err != nil {
		return nil, err
	}
	return NewStore[T](filepath.Join(s.dir, seg),
		WithDirPerm(s.opts.DirPerm),
		WithFilePerm(s.opts.FilePerm),
		WithExt(s.opts.Ext),
		WithAtomicWrite(s.opts.Atomic),
		WithDecodeHook(s.opts.DecodeHook),
	)
}
