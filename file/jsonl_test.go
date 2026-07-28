package file

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// logLine 是 JSONL 測試用的記錄,刻意不實作 validator.IValidator。
type logLine struct {
	Type string `json:"type"`
	Seq  int    `json:"seq"`
	Msg  string `json:"msg"`
}

func newLogStore(t *testing.T) *Store[logLine] {
	t.Helper()
	s, err := NewStore[logLine](t.TempDir(), WithExt(".jsonl"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestAppendWritesOneLinePerRecord(t *testing.T) {
	s := newLogStore(t)
	n, err := s.Append("run1",
		logLine{Type: "turn", Seq: 1, Msg: "a"},
		logLine{Type: "turn", Seq: 2, Msg: "b"})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if n != 2 {
		t.Errorf("Append = %d, want 2", n)
	}
	raw, err := os.ReadFile(s.Path("run1"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("行數 = %d, want 2 (內容: %q)", len(lines), raw)
	}
	if !bytes.HasSuffix(raw, []byte("\n")) {
		t.Error("最後一行應以換行結尾")
	}
}

func TestAppendAccumulates(t *testing.T) {
	s := newLogStore(t)
	if _, err := s.Append("run1", logLine{Seq: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append("run1", logLine{Seq: 2}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.Path("run1"))
	if got := strings.Count(string(raw), "\n"); got != 2 {
		t.Errorf("累計行數 = %d, want 2", got)
	}
}

func TestAppendZeroRecordsIsNoop(t *testing.T) {
	s := newLogStore(t)
	n, err := s.Append("run1")
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if n != 0 {
		t.Errorf("Append = %d, want 0", n)
	}
	if _, err := os.Stat(s.Path("run1")); !os.IsNotExist(err) {
		t.Error("零筆追加不應建立檔案")
	}
}

func TestAppendAppliesFilePerm(t *testing.T) {
	s, _ := NewStore[logLine](t.TempDir(), WithExt(".jsonl"), WithFilePerm(0o600))
	if _, err := s.Append("run1", logLine{Seq: 1}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.Path("run1"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
}

func TestScanVisitsEveryLineInOrder(t *testing.T) {
	s := newLogStore(t)
	_, _ = s.Append("run1", logLine{Seq: 1}, logLine{Seq: 2}, logLine{Seq: 3})

	var seen []int
	err := s.Scan("run1", func(raw []byte) error {
		var l logLine
		if err := json.Unmarshal(raw, &l); err != nil {
			return err
		}
		seen = append(seen, l.Seq)
		return nil
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(seen) != 3 || seen[0] != 1 || seen[1] != 2 || seen[2] != 3 {
		t.Errorf("seen = %v, want [1 2 3]", seen)
	}
}

func TestScanMissingFileIsEmptyNotError(t *testing.T) {
	s := newLogStore(t)
	calls := 0
	err := s.Scan("ghost", func([]byte) error { calls++; return nil })
	if err != nil {
		t.Fatalf("Scan 對缺檔應回 nil,實得 %v", err)
	}
	if calls != 0 {
		t.Errorf("回呼被叫了 %d 次, want 0", calls)
	}
}

func TestScanStopsOnErrStopScan(t *testing.T) {
	s := newLogStore(t)
	_, _ = s.Append("run1", logLine{Seq: 1}, logLine{Seq: 2}, logLine{Seq: 3})

	calls := 0
	err := s.Scan("run1", func([]byte) error {
		calls++
		if calls == 2 {
			return ErrStopScan
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ErrStopScan 不應向外冒出: %v", err)
	}
	if calls != 2 {
		t.Errorf("回呼被叫了 %d 次, want 2", calls)
	}
}

func TestScanPropagatesCallbackError(t *testing.T) {
	s := newLogStore(t)
	_, _ = s.Append("run1", logLine{Seq: 1})
	sentinel := errors.New("boom")
	err := s.Scan("run1", func([]byte) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("Scan err = %v, want 包住 sentinel", err)
	}
}

func TestScanSkipsBlankLines(t *testing.T) {
	s := newLogStore(t)
	if err := os.WriteFile(s.Path("run1"),
		[]byte("{\"seq\":1}\n\n{\"seq\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := s.Scan("run1", func([]byte) error { calls++; return nil }); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("回呼被叫了 %d 次, want 2 (空行應跳過)", calls)
	}
}

func TestScanHandlesLongLines(t *testing.T) {
	s := newLogStore(t)
	big := logLine{Seq: 1, Msg: strings.Repeat("x", 200*1024)}
	if _, err := s.Append("run1", big); err != nil {
		t.Fatal(err)
	}
	var got logLine
	err := s.Scan("run1", func(raw []byte) error { return json.Unmarshal(raw, &got) })
	if err != nil {
		t.Fatalf("Scan 應能處理超過 bufio 預設 64KB 的行: %v", err)
	}
	if len(got.Msg) != 200*1024 {
		t.Errorf("Msg 長度 = %d, want %d", len(got.Msg), 200*1024)
	}
}
