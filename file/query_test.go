package file

import (
	"bytes"
	"os"
	"testing"
)

func seededLog(t *testing.T) *Store[logLine] {
	t.Helper()
	s := newLogStore(t)
	_, err := s.Append("run1",
		logLine{Type: "meta", Seq: 0, Msg: "header"},
		logLine{Type: "turn", Seq: 1, Msg: "a"},
		logLine{Type: "turn", Seq: 2, Msg: "b"},
		logLine{Type: "turn", Seq: 3, Msg: "c"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func writeFileHelper(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestFindReturnsFirstMatch(t *testing.T) {
	s := seededLog(t)
	got, ok, err := s.Find("run1", func(l logLine) bool { return l.Type == "meta" })
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !ok {
		t.Fatal("應找到 meta 行")
	}
	if got.Msg != "header" {
		t.Errorf("Msg = %q, want header", got.Msg)
	}
}

func TestFindNoMatch(t *testing.T) {
	s := seededLog(t)
	_, ok, err := s.Find("run1", func(l logLine) bool { return l.Type == "nope" })
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ok {
		t.Error("不該找到")
	}
}

func TestFindMissingFile(t *testing.T) {
	s := newLogStore(t)
	_, ok, err := s.Find("ghost", func(logLine) bool { return true })
	if err != nil {
		t.Fatalf("Find 對缺檔應回 nil error: %v", err)
	}
	if ok {
		t.Error("缺檔不該有命中")
	}
}

func TestFilterAndsMultiplePredicates(t *testing.T) {
	s := seededLog(t)
	got, err := s.Filter("run1",
		func(l logLine) bool { return l.Type == "turn" },
		func(l logLine) bool { return l.Seq > 1 })
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].Seq != 2 || got[1].Seq != 3 {
		t.Errorf("got = %+v, want seq 2,3", got)
	}
}

func TestFilterNoPredicatesReturnsAll(t *testing.T) {
	s := seededLog(t)
	got, err := s.Filter("run1")
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("len = %d, want 4", len(got))
	}
}

func TestFilterMissingFileReturnsEmpty(t *testing.T) {
	s := newLogStore(t)
	got, err := s.Filter("ghost")
	if err != nil {
		t.Fatalf("Filter 對缺檔應回 nil error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestCountWithPredicate(t *testing.T) {
	s := seededLog(t)
	n, err := s.Count("run1", func(l logLine) bool { return l.Type == "turn" })
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Errorf("Count = %d, want 3", n)
	}
}

func TestCountMissingFileIsZero(t *testing.T) {
	s := newLogStore(t)
	n, err := s.Count("ghost")
	if err != nil {
		t.Fatalf("Count 對缺檔應回 nil error: %v", err)
	}
	if n != 0 {
		t.Errorf("Count = %d, want 0", n)
	}
}

func TestQueryAppliesDecodeHook(t *testing.T) {
	s, err := NewStore[logLine](t.TempDir(), WithExt(".jsonl"),
		WithDecodeHook(func(raw []byte) ([]byte, error) {
			return bytes.ReplaceAll(raw, []byte(`"kind":`), []byte(`"type":`)), nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Custom("run1", func(path string) error {
		return writeFileHelper(path, "{\"kind\":\"turn\",\"seq\":1}\n")
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Filter("run1", func(l logLine) bool { return l.Type == "turn" })
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1 (decode hook 應已把 kind 改寫成 type)", len(got))
	}
}

func TestFilterReportsBadLine(t *testing.T) {
	s := newLogStore(t)
	if err := s.Custom("run1", func(path string) error {
		return writeFileHelper(path, "{\"seq\":1}\n{oops\n")
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Filter("run1"); err == nil {
		t.Fatal("壞行應回錯誤")
	}
}

func TestTruncateWhileDropsLeadingPrefix(t *testing.T) {
	s := seededLog(t)
	err := s.TruncateWhile("run1", func(l logLine) bool { return l.Seq <= 1 })
	if err != nil {
		t.Fatalf("TruncateWhile: %v", err)
	}
	got, err := s.Filter("run1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].Seq != 2 || got[1].Seq != 3 {
		t.Errorf("got = %+v, want seq 2,3", got)
	}
}

func TestTruncateWhileStopsAtFirstKeeper(t *testing.T) {
	s := newLogStore(t)
	// seq 1 要丟、seq 5 要留、seq 2 雖然符合 drop 條件但在 keeper 之後,
	// 前綴語意下必須保留。
	_, _ = s.Append("run1", logLine{Seq: 1}, logLine{Seq: 5}, logLine{Seq: 2})
	if err := s.TruncateWhile("run1", func(l logLine) bool { return l.Seq < 3 }); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Filter("run1")
	if len(got) != 2 || got[0].Seq != 5 || got[1].Seq != 2 {
		t.Errorf("got = %+v, want seq 5,2", got)
	}
}

func TestTruncateWhileDropsAllRemovesFile(t *testing.T) {
	s := seededLog(t)
	if err := s.TruncateWhile("run1", func(logLine) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if s.Exists("run1") {
		t.Error("全部丟棄後應刪除檔案")
	}
	n, err := s.Count("run1")
	if err != nil || n != 0 {
		t.Errorf("Count = %d, err = %v, want 0, nil", n, err)
	}
}

func TestTruncateWhileKeepsAll(t *testing.T) {
	s := seededLog(t)
	if err := s.TruncateWhile("run1", func(logLine) bool { return false }); err != nil {
		t.Fatal(err)
	}
	n, _ := s.Count("run1")
	if n != 4 {
		t.Errorf("Count = %d, want 4", n)
	}
}

func TestTruncateWhileMissingFileIsNoop(t *testing.T) {
	s := newLogStore(t)
	if err := s.TruncateWhile("ghost", func(logLine) bool { return true }); err != nil {
		t.Errorf("缺檔應是 no-op,實得 %v", err)
	}
}
