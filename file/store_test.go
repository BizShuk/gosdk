package file

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// mockUser 是測試用結構,實作 validator.IValidator。
type mockUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (m mockUser) Validate() error {
	if m.ID <= 0 {
		return errors.New("id must be positive")
	}
	if m.Name == "" {
		return errors.New("name cannot be empty")
	}
	return nil
}

func TestNewStoreDefaults(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore[mockUser](filepath.Join(dir, "nested"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if got := s.Path("alice"); got != filepath.Join(dir, "nested", "alice.json") {
		t.Errorf("Path = %q", got)
	}
	info, err := os.Stat(s.Dir())
	if err != nil {
		t.Fatalf("dir not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("dir perm = %o, want 755", perm)
	}
}

func TestNewStoreOptions(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore[mockUser](filepath.Join(dir, "secure"),
		WithDirPerm(0o700), WithFilePerm(0o600), WithExt("jsonl"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if got := s.Path("wal"); got != filepath.Join(dir, "secure", "wal.jsonl") {
		t.Errorf("Path = %q, 前導點應自動補上", got)
	}
	info, _ := os.Stat(s.Dir())
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}
}

func TestNewStoreRejectsEmptyDir(t *testing.T) {
	if _, err := NewStore[mockUser](""); err == nil {
		t.Fatal("空目錄應回錯誤")
	}
}

func TestSafeName(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	bad := []string{"", "a/b", `a\b`, "../etc/passwd", ".."}
	for _, n := range bad {
		if err := s.safeName(n); err == nil {
			t.Errorf("safeName(%q) 應回錯誤", n)
		}
	}
	// "." 與其他點開頭的名稱只是隱藏檔,不是路徑穿越,應放行。
	for _, n := range []string{"alice", "run-01", "2026-07-29", ".", ".hidden"} {
		if err := s.safeName(n); err != nil {
			t.Errorf("safeName(%q) = %v, 應通過", n, err)
		}
	}
}

func TestSafeNameContainmentWithEmptyExt(t *testing.T) {
	// WithExt("") 時 Path(".") 會 Clean 成目錄本身,Path("..") 會變成上層
	// 目錄。containment 檢查必須擋下這兩者,否則 Delete 會刪錯東西。
	s, err := NewStore[mockUser](t.TempDir(), WithExt(""))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{".", ".."} {
		if err := s.safeName(n); err == nil {
			t.Errorf("safeName(%q) 在 WithExt(\"\") 下應被擋掉", n)
		}
	}
	if err := s.safeName("alice"); err != nil {
		t.Errorf("safeName(\"alice\") = %v, 應通過", err)
	}
}

func TestDotNameRoundTripsThroughList(t *testing.T) {
	// safeName 放行隱藏檔,List 就必須列得出來 —— 否則寫得進去卻看不到。
	s, _ := NewStore[mockUser](t.TempDir())
	if err := s.Write(".hidden", mockUser{ID: 1, Name: "Hidden"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	names, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 1 || names[0] != ".hidden" {
		t.Errorf("List = %v, want [.hidden]", names)
	}
}

func TestCustomReceivesFullPath(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	var seen string
	err := s.Custom("raw", func(path string) error {
		seen = path
		return os.WriteFile(path, []byte("hi"), 0o644)
	})
	if err != nil {
		t.Fatalf("Custom: %v", err)
	}
	if seen != s.Path("raw") {
		t.Errorf("Custom 收到 %q, want %q", seen, s.Path("raw"))
	}
	if b, _ := os.ReadFile(s.Path("raw")); string(b) != "hi" {
		t.Errorf("檔案內容 = %q", b)
	}
}

func TestCustomPropagatesError(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	sentinel := errors.New("boom")
	if err := s.Custom("x", func(string) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Errorf("Custom err = %v, want 包住 sentinel", err)
	}
}

func TestResolvePathExpandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	got, err := resolvePath("~/somewhere")
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	if got != filepath.Join(home, "somewhere") {
		t.Errorf("resolvePath = %q, want %q", got, filepath.Join(home, "somewhere"))
	}
}

func TestIsNil(t *testing.T) {
	var p *mockUser
	if !isNil(p) {
		t.Error("nil 指標應為 true")
	}
	if isNil(mockUser{ID: 1}) {
		t.Error("非指標值應為 false")
	}
	if isNil(nil) == false {
		t.Error("裸 nil 應為 true")
	}
}
