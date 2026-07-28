package file

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListReturnsSortedNames(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	for _, n := range []string{"carol", "alice", "bob"} {
		if err := s.Write(n, mockUser{ID: 1, Name: n}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"alice", "bob", "carol"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestListFiltersByExtAndSkipsDirsAndTempFiles(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore[mockUser](dir)
	_ = s.Write("alice", mockUser{ID: 1, Name: "Alice"})
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".tmp-123.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"alice"}) {
		t.Errorf("List = %v, want [alice]", got)
	}
}

func TestListDoesNotDecode(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore[mockUser](dir)
	_ = s.Write("good", mockUser{ID: 1, Name: "Good"})
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatalf("List 不應因為壞檔失敗: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"broken", "good"}) {
		t.Errorf("List = %v, want [broken good]", got)
	}
}

func TestListEmptyDir(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %v, want 空", got)
	}
}

func TestExists(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if s.Exists("alice") {
		t.Error("尚未寫入前不應存在")
	}
	_ = s.Write("alice", mockUser{ID: 1, Name: "Alice"})
	if !s.Exists("alice") {
		t.Error("寫入後應存在")
	}
	if s.Exists("../escape") {
		t.Error("不安全名稱應直接回 false")
	}
}

func TestDelete(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	_ = s.Write("alice", mockUser{ID: 1, Name: "Alice"})
	if err := s.Delete("alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s.Exists("alice") {
		t.Error("刪除後不應存在")
	}
}

func TestDeleteMissingReturnsErrNotFound(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if err := s.Delete("ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestSubInheritsOptions(t *testing.T) {
	root := t.TempDir()
	s, _ := NewStore[mockUser](root, WithDirPerm(0o700), WithFilePerm(0o600), WithExt(".jsonl"))
	sub, err := s.Sub("workspace-a")
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if sub.Dir() != filepath.Join(root, "workspace-a") {
		t.Errorf("Sub dir = %q", sub.Dir())
	}
	if got := sub.Path("run"); got != filepath.Join(root, "workspace-a", "run.jsonl") {
		t.Errorf("Sub Path = %q, 副檔名應被繼承", got)
	}
	info, err := os.Stat(sub.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("Sub dir perm = %o, want 700", perm)
	}
}

func TestSubRejectsTraversal(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	for _, bad := range []string{"", "..", "a/b", "../escape"} {
		if _, err := s.Sub(bad); err == nil {
			t.Errorf("Sub(%q) 應被拒絕", bad)
		}
	}
}
