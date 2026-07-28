# Registry + init() self-registration — 完整範例 (Full Sample)

`skills/golang-dev/SKILL.md` 第 `7` 節的延伸材料：完整 registry package、環境查詢注入、
守護測試、陷阱表與檢查清單。SKILL.md 只留最小骨架，細節在這裡。

---

## 1. 五個構件 (The Five Pieces)

```text
registry/            ← 只依賴介面所在的 package，不 import 任何實作
  ├─ Entry           ← 建構方式 + CLI/wizard 需要的 metadata
  ├─ Register(Entry) ← 重複註冊直接 panic
  ├─ Lookup/Names/Entries/New
  └─ Options         ← 建構參數；環境查詢以 func 注入，不 import viper

impl/<name>/
  ├─ <name>.go       ← 真正的實作與 New(...Option)
  └─ register.go     ← 只有一個 init()，把自己塞進 registry

impl/all/all.go      ← blank-import 全部實作的 meta-package（選用）
```

依賴方向永遠是 `impl → registry`，never 反向。這就是為什麼 registry 不會
因為多一個實作而改動，也是為什麼不會有 import cycle。

---

## 2. Registry package 完整骨架

```go
// Package registry is the single place that maps a name to its constructor.
//
// Implementations register themselves from their package init() — the
// registry imports no implementation. Each binary decides which ones to
// link by blank-importing them; Go's linker drops the rest. The set of
// "registered names" is therefore a property of the linking binary, not
// of this package.
package registry

import (
    "fmt"
    "os"
    "sort"
    "strings"
    "sync"
)

// DEFAULT is used when the requested name is empty.
const DEFAULT = "minimax"

// Factory builds one implementation from resolved options.
type Factory func(Options) (Thing, error)

// Entry is everything a caller needs: how to build it, plus enough
// metadata for a `--list` listing or a wizard menu.
type Entry struct {
    Name       string   // canonical, lower-case
    Label      string   // human-facing; a menu renders this
    Note       string   // one-line caveat ("default", "OAuth only")
    APIKeyEnv  []string // credential env vars, highest precedence first
    BaseURLEnv string   // endpoint override; empty = adapter default only
    New        Factory
    Catalog    func() []ModelSpec // optional per-entry extras
}

var (
    mu      sync.RWMutex
    entries = map[string]Entry{}
)

// Register adds an entry. It panics on a duplicate name — idiomatic Go
// for init()-time contract violations (see database/sql.Register).
func Register(e Entry) {
    if e.Name == "" || e.New == nil {
        panic(fmt.Sprintf("registry: Register requires Name and New (got %+v)", e))
    }
    key := strings.ToLower(strings.TrimSpace(e.Name))
    mu.Lock()
    defer mu.Unlock()
    if _, exists := entries[key]; exists {
        panic(fmt.Sprintf("registry: %q already registered", e.Name))
    }
    entries[key] = e
}

// Names lists registered names, sorted — stable menu / listing output.
func Names() []string {
    mu.RLock()
    defer mu.RUnlock()
    out := make([]string, 0, len(entries))
    for k := range entries {
        out = append(out, k)
    }
    sort.Strings(out)
    return out
}

// Lookup normalizes case and space; an empty name resolves to DEFAULT.
func Lookup(name string) (Entry, bool) {
    key := strings.ToLower(strings.TrimSpace(name))
    if key == "" {
        key = DEFAULT
    }
    mu.RLock()
    defer mu.RUnlock()
    e, ok := entries[key]
    return e, ok
}

// New builds the named thing. The error lists what IS registered, which
// is the single most useful debugging line when a blank import is missing.
func New(name string, o Options) (Thing, error) {
    e, ok := Lookup(name)
    if !ok {
        return nil, fmt.Errorf("registry: unknown name %q (registered: %s)",
            name, strings.Join(Names(), ", "))
    }
    t, err := e.New(o.Resolve(e))
    if err != nil {
        return nil, fmt.Errorf("registry: %s: %w", e.Name, err)
    }
    return t, nil
}
```

---

## 3. 實作端：一個檔案、一個 init()

把註冊碼獨立成 `register.go`，`不要`混進實作檔。理由：讀者一眼看得出這個
package 有 side effect；要拆成獨立 module 時也只需搬一個檔案。

```go
// impl/minimax/register.go
package minimax

import "example.com/proj/registry"

func init() {
    registry.Register(registry.Entry{
        Name:       "minimax",
        Label:      "MiniMax",
        Note:       "default; OpenAI-compatible",
        APIKeyEnv:  []string{"MINIMAX_API_KEY"},
        BaseURLEnv: "MINIMAX_BASE_URL",
        New: func(o registry.Options) (registry.Thing, error) {
            var opts []Option
            if o.Model != "" {
                opts = append(opts, WithModel(o.Model))
            }
            if o.APIKey != "" {
                opts = append(opts, WithAPIKey(o.APIKey))
            }
            if o.BaseURL != "" {
                opts = append(opts, WithBaseURL(o.BaseURL))
            }
            return New(opts...)
        },
        Catalog: DefaultCatalog,
    })
}
```

三個要點：

- Factory 是 `adapter`，把 registry 的扁平 `Options` 翻譯成該實作自己的
  `functional options`。實作的公開 API `不因為` registry 存在而改變 ——
  沒有 registry 時 `minimax.New(WithModel(...))` 一樣能用。
- 空字串一律`不`傳，讓實作套用自己的預設。這樣 zero-value `Options`
  在憑證已 export 的機器上仍能建出可用物件。
- `init()` 不做 I/O、不讀檔、不打網路。它只塞一筆 map entry，成本必須
  是常數時間，因為 `任何` blank-import 這個 package 的 binary 都會付。

---

## 4. all/ meta-package 與 linker 控制

```go
// Package all links every built-in implementation into the binary.
//
// Slim binaries should import the specific packages instead and let
// Go's linker drop the rest.
package all

import (
    _ "example.com/proj/impl/anthropic"
    _ "example.com/proj/impl/google"
    _ "example.com/proj/impl/minimax"
    _ "example.com/proj/impl/ollama"
)
```

- 全功能 CLI：`import _ ".../impl/all"`。
- 精簡 binary（embedded、只出一個 backend 的 sample）：只 blank-import 需要的那個。
- 這是此 pattern 最大的實質好處：`registered set` 是 `linking binary` 的屬性，
  不是 registry package 的屬性。單一 codebase 可以同時出胖 CLI 與瘦 binary。

`main.go` 或 composition root 是唯一該放 blank import 的地方；library
package `不要` blank-import `all`，那會強迫所有下游吞下全部依賴。

---

## 5. 環境查詢用注入，不要 import 設定框架

registry 需要「從環境拿 API key」，但 `不該` 知道 viper、godotenv、
或任何設定框架的存在。做法是把查詢函式變成 `Options` 的欄位：

```go
type Options struct {
    Model   string
    APIKey  string
    BaseURL string

    // APIKeyEnv overrides which env var supplies the credential.
    APIKeyEnv string

    // LookupEnv resolves an env var. nil means os.Getenv.
    LookupEnv func(string) string
}

func (o Options) lookup(key string) string {
    if key == "" {
        return ""
    }
    if o.LookupEnv != nil {
        return o.LookupEnv(key)
    }
    return os.Getenv(key)
}

// Resolve fills empty credential fields from the environment, trying the
// entry's key names in order. Ordering matters: an OAuth token can be
// made to outrank a long-lived API key by listing it first.
func (o Options) Resolve(e Entry) Options {
    if o.APIKey == "" {
        keys := e.APIKeyEnv
        if o.APIKeyEnv != "" {
            keys = []string{o.APIKeyEnv}
        }
        for _, k := range keys {
            if v := o.lookup(k); v != "" {
                o.APIKey = v
                break
            }
        }
    }
    if o.BaseURL == "" {
        o.BaseURL = o.lookup(e.BaseURLEnv)
    }
    return o
}
```

- CLI 傳 viper-backed lookup，讓 `.env` / `config.yaml` 參與：
  `LookupEnv: func(k string) string { return viper.GetString(strings.ToLower(k)) }`
- library caller 留 `nil`，走 `os.Getenv`。
- `Resolve` 要 `exported`：「這次會用到哪個 env var」是 preflight 檢查與
  wizard 顯示都會問的問題，不該只能靠建構一次來間接得知。

---

## 6. 守護測試 (Guard Tests)

registry 的錯誤幾乎都在 init 期或組裝期，單元測試很容易漏。以下四個
table-driven 測試把 contract 釘住：

```go
// 測試 binary 需要 blank-import all，否則 entries 是空的
import _ "example.com/proj/impl/all"

// 1) 註冊完整 + Names() 已排序
func TestNamesAreSortedAndComplete(t *testing.T) {
    got := registry.Names()
    require.NotEmpty(t, got)
    for i := 1; i < len(got); i++ {
        assert.LessOrEqualf(t, got[i-1], got[i], "Names() must be sorted; got %v", got)
    }
    for _, name := range []string{"anthropic", "google", "minimax", "ollama"} {
        assert.Containsf(t, got, name, "expected built-in %q to be registered", name)
    }
}

// 2) 別處的 DEFAULT 常數必須指到真的有註冊的名字
//    （宣告層 package 常常不能 import registry，只能靠測試對齊）
func TestSpecDefaultIsRegistered(t *testing.T) {
    _, ok := registry.Lookup(spec.DEFAULT_PROVIDER)
    assert.True(t, ok)
    assert.Equal(t, registry.DEFAULT, spec.DEFAULT_PROVIDER)
}

// 3) 名稱正規化：大小寫、前後空白、空字串 → DEFAULT
func TestLookupNormalizesName(t *testing.T) {
    for _, tc := range []struct{ in, want string }{
        {"minimax", "minimax"},
        {"MiniMax", "minimax"},
        {"  ANTHROPIC  ", "anthropic"},
        {"", registry.DEFAULT},
    } {
        t.Run(tc.in, func(t *testing.T) {
            e, ok := registry.Lookup(tc.in)
            require.True(t, ok)
            assert.Equal(t, tc.want, e.Name)
        })
    }
}

// 4) 每筆 Entry 自我描述完整（menu 會渲染 Label；憑證路徑要講清楚）
func TestEveryEntryIsSelfDescribing(t *testing.T) {
    for _, e := range registry.Entries() {
        t.Run(e.Name, func(t *testing.T) {
            assert.NotEmpty(t, e.Name)
            assert.NotEmpty(t, e.Label, "a wizard menu renders Label")
            if !strings.Contains(strings.ToLower(e.Note), "oauth") {
                assert.NotEmptyf(t, e.APIKeyEnv,
                    "every non-OAuth entry must document how its credential resolves; got %+v", e)
            }
        })
    }
}
```

另外測 `unknown name` 的錯誤訊息要`列出已註冊的名字` —— 這正是「忘了
blank import」時唯一能自救的線索。

---

## 7. 常見陷阱 (Pitfalls)

| 症狀                                    | 根因                                                                        | 處置                                                                    |
| --------------------------------------- | --------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `unknown provider "x"`，但檔案明明存在  | 沒有人 blank-import 該 package，linker 整包丟掉                             | 加進 `all/`，或在 composition root 明確 blank-import                    |
| test 通過、production binary 找不到實作 | 只有 `_test.go` 那側 import 了 `all`                                        | 把 blank import 放進 `main.go`，測試檔另外重複一次                      |
| import cycle                            | registry 反過來 import 了某個實作（常見於「registry 想提供 default 實例」） | default `只存名字字串`，不存實例                                        |
| 兩個 module 註冊同名 → panic            | 名稱撞號                                                                    | 這就是 panic 的用意：init 期炸掉，好過 runtime 拿到錯的實作             |
| `Register` 用 `error` 回傳              | `init()` 沒有地方接 error，只能忽略                                         | 保持 panic；contract 違反本來就該炸                                     |
| 資料競爭                                | 有人在 runtime 動態 `Register`                                              | 用 `sync.RWMutex` 保護（骨架已含）；但仍建議只在 init 期註冊            |
| package 名與目錄名不一致                | 例如目錄 `provider/` 內卻 `package registry`                                | 可行但每個 import 都得靠 IDE 才知道要打什麼；`除非有強理由，讓兩者一致` |
| `init()` 變慢/會失敗                    | 在 init 裡讀檔、打網路、解 credential                                       | init 只塞 map；一切延後到 Factory 被呼叫時                              |

---

## 8. 檢查清單 (Checklist)

新增一個實作時：

- [ ] `impl/<name>/register.go` 只有一個 `init()`，只呼叫一次 `Register`
- [ ] `Entry.Name` 全小寫、與目錄名一致
- [ ] `Entry.Label` 有填（menu 會渲染）；`Note` 一行講清楚特殊性
- [ ] 憑證 env var 全部列進 `APIKeyEnv`，順序即優先序
- [ ] Factory 只在欄位非空時才傳 option，空值讓實作套自己的預設
- [ ] 加進 `impl/all/all.go` 的 blank import 清單
- [ ] `go test ./registry/...` 綠燈（守護測試會自動涵蓋新 entry）
- [ ] `go build ./...` 後確認精簡 binary 沒被意外拖進新依賴

改動 registry 本身時：

- [ ] `go list -deps ./registry | grep <module>` 確認沒有 import 到任何實作
- [ ] 新增 `Entry` 欄位要同步更新 `TestEveryEntryIsSelfDescribing`

---

## 參考實作 (Reference Implementation)

`~/projects/ai/agentSDK/provider/`：

- `registry.go` — 本骨架的來源，含 `Options.Resolve` 憑證優先序
- `all/all.go` — meta-package
- `minimax/register.go` 等 `7` 個 adapter 的 `init()`
- `registry_test.go` — 上述四個守護測試
- `cmd/provider.go` — viper-backed `LookupEnv` 注入點
