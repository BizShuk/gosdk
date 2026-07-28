// file/sample 展示 github.com/bizshuk/gosdk/file 的常見用法。
//
// 五個範例對應五種真實場景,依序是:
//
//  1. 憑證庫     — 一個帳號一個檔,鎖 owner-only 權限 (0700/0600)
//  2. 單一設定檔 — 整個檔案是一份文件,缺檔時退回預設值
//  3. 事件日誌   — JSONL 追加,以 Seq 取 checkpoint 之後的事件並壓縮前綴
//  4. 混型別日誌 — 首行 meta、其餘 turn,用 Scan 交出的原始位元組自行分流
//  5. 巢狀目錄   — Sub 取得第二層 Store,對應 <workspace>/<session>.jsonl
//
// 執行:
//
//	go run ./file/sample
//
// 範例全部寫在系統暫存目錄下,執行結束會自行清掉。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bizshuk/gosdk/file"
)

func main() {
	root, err := os.MkdirTemp("", "gosdk-file-sample-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(root)

	fmt.Printf("sample root: %s\n", root)

	credentialStore(filepath.Join(root, "auth"))
	singleDocument(filepath.Join(root, "config"))
	eventLog(filepath.Join(root, "wal"))
	mixedRecordLog(filepath.Join(root, "sessions"))
	nestedDirs(filepath.Join(root, "workspaces"))
}

// ---------------------------------------------------------------------------
// 1. 憑證庫:一個帳號一個檔
// ---------------------------------------------------------------------------

// Credential 實作 validator.IValidator,Write 會在序列化前自動呼叫它。
type Credential struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
}

func (c Credential) Validate() error {
	if c.Provider == "" {
		return errors.New("provider must not be empty")
	}
	if c.APIKey == "" {
		return errors.New("api_key must not be empty")
	}
	return nil
}

func credentialStore(dir string) {
	section("1. 憑證庫")

	// 憑證含長期 secret,目錄與檔案權限一律鎖到 owner-only。
	s, err := file.NewStore[Credential](dir,
		file.WithDirPerm(0o700), file.WithFilePerm(0o600))
	if err != nil {
		log.Fatal(err)
	}

	must(s.Write("anthropic", Credential{Provider: "anthropic", APIKey: "sk-ant-xxx"}))
	must(s.Write("openai", Credential{Provider: "openai", APIKey: "sk-oai-yyy"}))

	// 驗證失敗的物件不會產生任何檔案。
	if err := s.Write("broken", Credential{Provider: "none"}); err != nil {
		fmt.Printf("  驗證攔截: %v\n", err)
	}

	// List 只回名稱、不解碼 —— 壞掉的檔案不會讓整批列舉失敗,由呼叫端
	// 自行決定要跳過還是報錯。
	names, err := s.List()
	if err != nil {
		log.Fatal(err)
	}
	for _, n := range names {
		cred, err := s.Read(n)
		if err != nil {
			fmt.Printf("  略過 %s: %v\n", n, err)
			continue
		}
		fmt.Printf("  %-10s provider=%s key=%s\n", n, cred.Provider, mask(cred.APIKey))
	}

	info, err := os.Stat(s.Path("anthropic"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  檔案權限: %04o\n", info.Mode().Perm())
}

// ---------------------------------------------------------------------------
// 2. 單一設定檔:整檔一份文件,缺檔退回預設值
// ---------------------------------------------------------------------------

// InstallsFile 是「整個檔案就是一份文件」的典型:一個固定檔名、
// 一個頂層結構、內含一個 slice。
type InstallsFile struct {
	Version int      `json:"version"`
	Entries []string `json:"entries"`
}

func singleDocument(dir string) {
	section("2. 單一設定檔")

	s, err := file.NewStore[*InstallsFile](dir)
	if err != nil {
		log.Fatal(err)
	}

	// ReadOr 是無錯誤版本,直接退回預設值。適用於「缺檔是唯一預期失敗」
	// 的場合 —— 它同時也會吞掉 JSON 損毀與權限錯誤。
	cfg := s.ReadOr("installs", &InstallsFile{Version: 1})
	fmt.Printf("  首次讀取: version=%d entries=%v\n", cfg.Version, cfg.Entries)

	cfg.Entries = append(cfg.Entries, "skills/markdownlint", "skills/pm2")
	must(s.Write("installs", cfg))

	got := s.ReadOr("installs", &InstallsFile{Version: 1})
	fmt.Printf("  寫入後  : version=%d entries=%v\n", got.Version, got.Entries)

	// 需要區分「缺檔」與「檔案損毀」的呼叫端請用 Read + errors.Is。
	if _, err := s.Read("never-written"); errors.Is(err, file.ErrNotFound) {
		fmt.Printf("  Read 對缺檔回 ErrNotFound: %v\n", err)
	}
}

// ---------------------------------------------------------------------------
// 3. 事件日誌:JSONL 追加 + Seq checkpoint + 前綴壓縮
// ---------------------------------------------------------------------------

// Event 是同質 JSONL 的記錄:整個檔案只有這一種型別,所以可以直接用
// typed 便利層 (Filter / Count / TruncateWhile)。
type Event struct {
	Seq  int    `json:"seq"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

func eventLog(dir string) {
	section("3. 事件日誌")

	s, err := file.NewStore[Event](dir, file.WithExt(".jsonl"))
	if err != nil {
		log.Fatal(err)
	}

	n, err := s.Append("run-01",
		Event{Seq: 1, Kind: "start", Text: "boot"},
		Event{Seq: 2, Kind: "tool", Text: "grep"},
		Event{Seq: 3, Kind: "tool", Text: "edit"},
		Event{Seq: 4, Kind: "done", Text: "ok"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  追加 %d 筆\n", n)

	// 取 checkpoint 之後的事件。predicate 是逐次呼叫的參數而非建構子
	// 選項 —— sinceSeq 每次都不同。
	const SINCE_SEQ = 2
	after, err := s.Filter("run-01", func(e Event) bool { return e.Seq > SINCE_SEQ })
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  seq > %d: %d 筆 %v\n", SINCE_SEQ, len(after), seqs(after))

	// 多個 predicate 以 AND 組合。
	tools, err := s.Filter("run-01",
		func(e Event) bool { return e.Kind == "tool" },
		func(e Event) bool { return e.Seq > 2 })
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  kind=tool AND seq>2: %v\n", seqs(tools))

	// 壓縮完成後截斷 WAL。這是前綴丟棄:從檔頭往下走,遇到第一筆不符合
	// 的記錄就停,不會誤刪中段。
	must(s.TruncateWhile("run-01", func(e Event) bool { return e.Seq <= 2 }))
	rest, err := s.Filter("run-01")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  截斷 seq<=2 後剩: %v\n", seqs(rest))
}

// ---------------------------------------------------------------------------
// 4. 混型別日誌:首行 meta、其餘 turn
// ---------------------------------------------------------------------------

// Meta 與 Turn 是同一個檔案裡的兩種記錄型別。Store[T] 只有一個型別參數,
// 裝不下兩者 —— 這正是 Scan 交出原始位元組的理由:呼叫端先看 type 欄位,
// 再決定要解成哪一個型別。
type Meta struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Workspace string `json:"workspace"`
}

type Turn struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// typePeek 只解出判別欄位,避免為了分流而把整筆解兩次。
type typePeek struct {
	Type string `json:"type"`
}

func mixedRecordLog(dir string) {
	section("4. 混型別日誌")

	// 型別參數用 json.RawMessage:這個 Store 只負責搬位元組,解碼由呼叫端
	// 全權處理。Append 也照樣可用 —— RawMessage 序列化後就是原文。
	s, err := file.NewStore[json.RawMessage](dir, file.WithExt(".jsonl"))
	if err != nil {
		log.Fatal(err)
	}

	meta := Meta{Type: "meta", SessionID: "sess-abc", Workspace: "/Users/me/projects"}
	turns := []Turn{
		{Type: "turn", Index: 1, Text: "第一輪"},
		{Type: "turn", Index: 2, Text: "第二輪"},
	}

	records := []json.RawMessage{mustJSON(meta)}
	for _, t := range turns {
		records = append(records, mustJSON(t))
	}
	if _, err := s.Append("sess-abc", records...); err != nil {
		log.Fatal(err)
	}

	// Scan 逐行交出原始位元組,呼叫端自行分流。命中的結果 append 進閉包
	// 變數,所以每一行只解碼一次。
	var (
		gotMeta  Meta
		gotTurns []Turn
	)
	err = s.Scan("sess-abc", func(raw []byte) error {
		var peek typePeek
		if err := json.Unmarshal(raw, &peek); err != nil {
			return err
		}
		switch peek.Type {
		case "meta":
			return json.Unmarshal(raw, &gotMeta)
		case "turn":
			var t Turn
			if err := json.Unmarshal(raw, &t); err != nil {
				return err
			}
			gotTurns = append(gotTurns, t)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  meta : session=%s workspace=%s\n", gotMeta.SessionID, gotMeta.Workspace)
	fmt.Printf("  turns: %d 筆\n", len(gotTurns))

	// 冪等追加:先數出已持久化幾筆 turn,再跳過重複的部分。這裡只解出
	// 判別欄位而不解整筆 —— 每次 hook fire 都要掃全檔,解碼成本會累積。
	// 若連這都嫌貴,Scan 交出的是原始位元組,可以直接做 bytes.Contains。
	persisted := 0
	must(s.Scan("sess-abc", func(raw []byte) error {
		var peek typePeek
		if err := json.Unmarshal(raw, &peek); err != nil {
			return err
		}
		if peek.Type == "turn" {
			persisted++
		}
		return nil
	}))

	incoming := []Turn{
		{Type: "turn", Index: 1, Text: "第一輪"}, // 重複,應跳過
		{Type: "turn", Index: 2, Text: "第二輪"}, // 重複,應跳過
		{Type: "turn", Index: 3, Text: "第三輪"}, // 新的
	}
	var fresh []json.RawMessage
	for _, t := range incoming {
		if t.Index <= persisted {
			continue
		}
		fresh = append(fresh, mustJSON(t))
	}
	appended, err := s.Append("sess-abc", fresh...)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  已存 %d 筆,送進 %d 筆,實際追加 %d 筆\n",
		persisted, len(incoming), appended)
}

// ---------------------------------------------------------------------------
// 5. 巢狀目錄:Sub
// ---------------------------------------------------------------------------

func nestedDirs(dir string) {
	section("5. 巢狀目錄")

	// safeName 拒絕含 / 的名稱,Sub 是取得第二層的正當途徑。子 Store
	// 繼承全部選項。
	root, err := file.NewStore[Turn](dir, file.WithExt(".jsonl"), file.WithDirPerm(0o700))
	if err != nil {
		log.Fatal(err)
	}

	// 工作區路徑含 /,先編碼成單一路徑片段。
	encoded := encodeWorkspace("/Users/me/projects/gosdk")
	ws, err := root.Sub(encoded)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := ws.Append("sess-001", Turn{Type: "turn", Index: 1, Text: "hello"}); err != nil {
		log.Fatal(err)
	}

	names, err := ws.List()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  子目錄  : %s\n", ws.Dir())
	fmt.Printf("  session : %v\n", names)

	count, err := ws.Count("sess-001")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  turn 數 : %d\n", count)

	info, err := os.Stat(ws.Dir())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  權限繼承: %04o\n", info.Mode().Perm())
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func section(title string) { fmt.Printf("\n=== %s ===\n", title) }

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	return raw
}

func seqs(events []Event) []int {
	out := make([]int, 0, len(events))
	for _, e := range events {
		out = append(out, e.Seq)
	}
	return out
}

func mask(secret string) string {
	if len(secret) <= 6 {
		return "***"
	}
	return secret[:6] + "***"
}

// encodeWorkspace 把絕對路徑壓成單一目錄片段,讓一層目錄就能列出某個
// 工作區的全部 session,不必深層巢狀。
func encodeWorkspace(p string) string {
	if p == "" {
		return "_unknown"
	}
	out := make([]rune, 0, len(p))
	for _, r := range p {
		if r == '/' {
			out = append(out, '%', '2', 'F')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
