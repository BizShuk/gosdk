package vault

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	v, err := New([]byte("secret-123"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.SetAll(map[string]string{
		"API_KEY":     "sk-test-1234567890",
		"DB_PASSWORD": `p@ss w0rd#1"quoted"`,
		"EMPTY":       "",
	}); err != nil {
		t.Fatal(err)
	}

	data, err := v.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	v2, err := Open(data, []byte("secret-123"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := v2.Get("DB_PASSWORD")
	if err != nil || got != `p@ss w0rd#1"quoted"` {
		t.Fatalf("Get 結果錯誤: %q, err=%v", got, err)
	}
	if v2.Len() != 3 {
		t.Fatalf("Len = %d, want 3", v2.Len())
	}
}

func TestWrongPassword(t *testing.T) {
	v, _ := New([]byte("correct"))
	_ = v.Set("K", "v")
	data, _ := v.Marshal()

	if _, err := Open(data, []byte("wrong")); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("預期 ErrWrongPassword, 實際: %v", err)
	}
}

func TestSwappedCiphertextRejected(t *testing.T) {
	v, _ := New([]byte("pw"))
	_ = v.Set("A", "value-a")
	_ = v.Set("B", "value-b")

	// 模擬攻擊者將兩個密文互換:AAD 綁定變數名稱,應解密失敗
	v.secrets["A"], v.secrets["B"] = v.secrets["B"], v.secrets["A"]
	if _, err := v.Get("A"); err == nil {
		t.Fatal("密文調換後仍可解密,AAD 綁定失效")
	}
}

func TestParseEnv(t *testing.T) {
	env := `
# comment
API_KEY=sk-123
export REDIS_URL=redis://localhost:6379
QUOTED="hello world"
SINGLE='single quoted'
INVALID LINE
`
	got := ParseEnv([]byte(env))
	want := map[string]string{
		"API_KEY":   "sk-123",
		"REDIS_URL": "redis://localhost:6379",
		"QUOTED":    "hello world",
		"SINGLE":    "single quoted",
	}
	if len(got) != len(want) {
		t.Fatalf("解析出 %d 個變數, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestMarshalEnvQuoting(t *testing.T) {
	out := string(MarshalEnv(map[string]string{
		"PLAIN":  "abc",
		"SPACES": "a b",
		"EMPTY":  "",
	}))
	if !strings.Contains(out, `SPACES="a b"`) || !strings.Contains(out, `EMPTY=""`) {
		t.Fatalf("引號處理錯誤:\n%s", out)
	}
}

func TestBadFormat(t *testing.T) {
	if _, err := Open([]byte(`{"magic":"nope"}`), []byte("pw")); !errors.Is(err, ErrBadFormat) {
		t.Fatalf("預期 ErrBadFormat, 實際: %v", err)
	}
}

func TestKeysOfFileNeedsNoPassword(t *testing.T) {
	v, err := New([]byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.SetAll(map[string]string{"B_KEY": "2", "A_KEY": "1"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), ".env.vault")
	if err := v.SaveFile(path); err != nil {
		t.Fatal(err)
	}

	// 沒有密碼也能列出名稱:只有值被加密。
	got, err := KeysOfFile(path)
	if err != nil {
		t.Fatalf("KeysOfFile: %v", err)
	}
	if !slices.Equal(got, []string{"A_KEY", "B_KEY"}) {
		t.Errorf("KeysOfFile = %v, want [A_KEY B_KEY](排序後)", got)
	}
}

func TestKeysOfFileRejectsNonVault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.env")
	if err := os.WriteFile(path, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := KeysOfFile(path); !errors.Is(err, ErrBadFormat) {
		t.Fatalf("KeysOfFile 對明文檔 err = %v, want ErrBadFormat", err)
	}
}
