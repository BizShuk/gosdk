package file

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	want := mockUser{ID: 1, Name: "Alice", Email: "alice@example.com"}
	if err := s.Write("alice", want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := s.Read("alice")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != want {
		t.Errorf("Read = %+v, want %+v", got, want)
	}
}

func TestReadMissingReturnsErrNotFound(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	_, err := s.Read("ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read err = %v, want ErrNotFound", err)
	}
}

func TestReadOrFallsBackOnMissing(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	def := mockUser{ID: 99, Name: "default"}
	if got := s.ReadOr("ghost", def); got != def {
		t.Errorf("ReadOr = %+v, want %+v", got, def)
	}
}

func TestReadOrAlsoSwallowsCorruption(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if err := os.WriteFile(s.Path("broken"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	def := mockUser{ID: 7, Name: "fallback"}
	if got := s.ReadOr("broken", def); got != def {
		t.Errorf("ReadOr = %+v, want %+v (損毀也退回預設是刻意行為)", got, def)
	}
}

func TestWriteRunsValidator(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if err := s.Write("bad", mockUser{ID: 0, Name: "x"}); err == nil {
		t.Fatal("Validate 失敗時 Write 應回錯誤")
	}
	if _, err := os.Stat(s.Path("bad")); !os.IsNotExist(err) {
		t.Error("驗證失敗不應留下檔案")
	}
}

func TestWriteAppliesFilePerm(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir(), WithFilePerm(0o600))
	if err := s.Write("alice", mockUser{ID: 1, Name: "Alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(s.Path("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
}

func TestAtomicWriteLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore[mockUser](dir)
	if err := s.Write("alice", mockUser{ID: 1, Name: "Alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("目錄應只剩一個檔案,實得 %d 個", len(entries))
	}
	if entries[0].Name() != "alice.json" {
		t.Errorf("殘留檔案 %q", entries[0].Name())
	}
}

func TestWriteOverwrites(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	_ = s.Write("alice", mockUser{ID: 1, Name: "Alice"})
	if err := s.Write("alice", mockUser{ID: 2, Name: "Bob"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, _ := s.Read("alice")
	if got.Name != "Bob" {
		t.Errorf("Name = %q, want Bob", got.Name)
	}
}

func TestDecodeHookRewritesBeforeUnmarshal(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir(), WithDecodeHook(func(raw []byte) ([]byte, error) {
		return bytes.ReplaceAll(raw, []byte(`"nickname":`), []byte(`"name":`)), nil
	}))
	if err := os.WriteFile(s.Path("legacy"),
		[]byte(`{"id":3,"nickname":"Carol"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read("legacy")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Name != "Carol" {
		t.Errorf("Name = %q, want Carol (decode hook 應已改寫欄位名)", got.Name)
	}
}

func TestWriteRejectsUnsafeName(t *testing.T) {
	s, _ := NewStore[mockUser](t.TempDir())
	if err := s.Write("../escape", mockUser{ID: 1, Name: "x"}); err == nil {
		t.Fatal("路徑穿越名稱應被拒絕")
	}
}
