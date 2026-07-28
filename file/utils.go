package file

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// 本檔收攏套件內的通用輔助函式:不掛在 Store 上、不碰套件狀態的純函式。

// resolvePath 展開環境變數與 ~ 後回傳絕對路徑。
//
// 這裡刻意不呼叫 gosdk/utils.ResolvePath:utils 套件內含
// github.com/gocarina/gocsv,而 Go 以 package 為單位載入,任何引用都會把
// CSV 函式庫拖進相依圖。file 是葉節點儲存套件,要能被極輕量的專案
// (例如只有三個直接依賴的 ai/auth) 導入而不付出額外代價。
func resolvePath(path string) (string, error) {
	expanded := os.ExpandEnv(path)

	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get user home directory: %w", err)
		}
		switch {
		case len(expanded) == 1:
			expanded = home
		case expanded[1] == '/' || expanded[1] == '\\':
			expanded = filepath.Join(home, expanded[2:])
		}
	}

	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return filepath.Clean(absPath), nil
}

// isNil 判斷 interface 值是否為 nil,含「非 nil interface 包住 nil 指標」
// 這個 Go 經典陷阱。用於 validator 檢查前的防呆。
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

// matchAll 以 AND 組合所有 predicate。空的 preds 一律通過,nil 的個別
// predicate 視為不設限。
func matchAll[T any](v T, preds []func(T) bool) bool {
	for _, p := range preds {
		if p != nil && !p(v) {
			return false
		}
	}
	return true
}
