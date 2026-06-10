# Cobra 指令列指標掛鉤 (Cobra Command-Line Metric Hook)

## Context (背景)

`gosdk` 已經有完整的 `metric` 模組,使用 Prometheus remote-write 協定把指標送進 VictoriaMetrics (預設 `:8428/api/v1/write`) 或 Mimir (`:9009/api/v1/push`),透過 `metric.SendMulti` / `Send` 與 `IMetric` 介面運作。

現在的 CLI 工具 (例如 `cmd/versioning/`, `cmd/gotmpl/`) 都用 `spf13/cobra` 來建立子命令樹,但**沒有任何可觀測性** — 無法知道使用者下了什麼指令、傳了哪些 flag、指令執行的頻率。

目標:**在 root cobra command 單一注入點**上掛一個 hook,讓每次執行任何子命令時自動送出一個 metric,tags 包含:
- `cmd` — 完整指令鏈 (例如 `myapp subcommand action`)
- `flags` — 所有被設定過的 flag 名稱 (例如 `name,verbose`)

預設後端是 VictoriaMetrics;若要切到 Mimir 只需傳入 `WithCobraService(NewMimirService())`。

## 設計重點 (Design Highlights)

### cobra 的 PersistentPreRunE 行為 (已透過 source code 驗證)

- 從 cobra v1.9.1 的 `command.go:972-998`,persistent hooks 是從 **leaf → root** 方向走訪
- 進入 hook 時 `cmd` 參數其實是 **leaf** (不是 root),所以可以直接用 `cmd.Parent()` 一路走到 root
- 預設情況下,只會呼叫「第一個找到」的 persistent hook (不呼叫其他層的) — 若子命令已有自己的 hook,本 hook 就不會被執行
  - 解法:在 docstring 提醒使用者若需要,自行設定 `cobra.EnableTraverseRunHooks = true`

### 採用 PreRun 而非 PostRun

決定在 **PersistentPreRunE** 階段就送指標,理由:
- 簡單:不需要在 Pre/Post 之間用 sync.Map 存狀態
- 永遠會送 (即使 RunE panic 或被中斷,只要 PreRun 跑完就有資料)
- 「嘗試呼叫次數」對監控 CLI 使用率比「完成次數」更有意義

### 旗標收集

走訪 leaf → root 的整條鏈,在每一層呼叫 `c.Flags().Visit()` (changed) 或 `c.Flags().VisitAll()` (all),用 `seen` map 去重複:
- persistent flag 同時存在於 parent 與 child 的 `Flags()` 中,需去重
- 預設採「changed only」,因為全列出所有已定義 flag 對監控沒幫助

### 異步送出 (Async emit)

預設開 goroutine 送,理由:
- remote-write endpoint 可能慢或暫時不可用,不能 block CLI
- 失敗時只 `zap.Warn`,不影響主流程
- 提供 `WithCobraSync()` 給測試或需要同步語意的情境

## 變更檔案 (Files to Change)

| 檔案 | 用途 |
|---|---|
| `metric/cobra.go` | 新增 — 公開 `InstrumentCobra` + options |
| `metric/cobra_test.go` | 新增 — 測試 (httptest + promwrite.Unmarshal 解碼) |
| `CLAUDE.md` | 更新 — Module Mapping 表加入新進入點 + 描述新行為 |

## Public API (在 `metric` 套件內)

```go
const CobraHookMetricName = "cli_command_executed"

type CollectFlagsMode string
const (
    CollectFlagsChanged CollectFlagsMode = "changed"  // 預設
    CollectFlagsAll     CollectFlagsMode = "all"
)

type CobraHookOption func(*cobraHookConfig)
func WithCobraService(s *MetricService) CobraHookOption
func WithCobraMetricName(name string) CobraHookOption
func WithCobraCollectFlags(mode CollectFlagsMode) CobraHookOption
func WithCobraSync() CobraHookOption                              // 預設 async
func WithCobraEmitTimeout(d time.Duration) CobraHookOption
func WithCobraExtraTags(tags map[string]string) CobraHookOption

func InstrumentCobra(root *cobra.Command, opts ...CobraHookOption)
```

## 核心實作 (Implementation Skeleton)

```go
// metric/cobra.go
package metric

import (
    "strings"
    "time"

    "github.com/spf13/cobra"
    "github.com/spf13/pflag"
    "go.uber.org/zap"
)

const CobraHookMetricName = "cli_command_executed"

func InstrumentCobra(root *cobra.Command, opts ...CobraHookOption) {
    cfg := defaultCobraHookConfig()
    for _, o := range opts { o(&cfg) }

    existingPre := root.PersistentPreRunE
    root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
        hctx := &cobraHookContext{leaf: cmd, cfg: &cfg}
        if cfg.async {
            go hctx.emit()
        } else {
            hctx.emit()
        }
        if existingPre != nil {
            return existingPre(cmd, args)
        }
        return nil
    }
}

func (h *cobraHookContext) emit() {
    defer func() {
        if r := recover(); r != nil {
            zap.L().Warn("cobra metric hook panic", zap.Any("recover", r))
        }
    }()
    tags := map[string]string{
        "cmd":   commandPath(h.leaf),
        "flags": h.collectFlagNames(),
    }
    for k, v := range h.cfg.extraTags {
        if _, ok := tags[k]; !ok { tags[k] = v }
    }
    m := Metric{Name: h.cfg.metricName, Timestamp: time.Now().Unix(), Value: int64(1), Tags: tags}
    if err := h.cfg.service.Send(m); err != nil {
        zap.L().Warn("cobra metric hook send failed", zap.String("cmd", tags["cmd"]), zap.Error(err))
    }
}

func (h *cobraHookContext) collectFlagNames() string {
    seen := map[string]bool{}
    var names []string
    for c := h.leaf; c != nil; c = c.Parent() {
        visit := func(f *pflag.Flag) {
            if !seen[f.Name] { seen[f.Name] = true; names = append(names, f.Name) }
        }
        if h.cfg.collectFlags == CollectFlagsAll {
            c.Flags().VisitAll(visit)
        } else {
            c.Flags().Visit(visit)
        }
    }
    return strings.Join(names, ",")
}

func commandPath(cmd *cobra.Command) string {
    if cmd == nil { return "" }
    var parts []string
    for c := cmd; c != nil; c = c.Parent() {
        name := c.CalledAs()
        if name == "" { name = c.Name() }
        parts = append([]string{name}, parts...)
    }
    return strings.Join(parts, " ")
}
```

## 測試策略 (Test Strategy)

`metric/cobra_test.go` 採用白箱測試 (同 package):
- `httptest.NewServer` 模擬 VictoriaMetrics endpoint,捕捉每次 POST 的 body
- 用 `promwrite.WriteRequest.Unmarshal` 解碼 binary protobuf 內容,逐 label 驗證
- 所有測試用 `WithCobraSync()` 確保送出後才能 assert

測試案例:
1. `TestCobraHook_EmitsCommandChainAndChangedFlags` — `myapp sub action --name=foo` 應送出 `cmd="myapp sub action"` 與 `flags="name"`
2. `TestCobraHook_CollectsFlagsFromParentChain` — 同時設 `--name` (action) 與 `--verbose` (sub),`flags` 應含兩個
3. `TestCobraHook_AllFlagsMode` — 切到 `CollectFlagsAll`,未設的 flag 也要列
4. `TestCobraHook_EmptyFlags` — 沒設任何 flag,`flags` 為空字串
5. `TestCobraHook_ExtraTags` — `WithCobraExtraTags` 附加的 tag 應出現在 metric 中
6. `TestCobraHook_OverridesMetricName` — `WithCobraMetricName` 改變 `__name__` label
7. `TestCobraHook_PreservesExistingPreRun` — 既有 `PersistentPreRunE` 仍會被呼叫
8. `TestCobraHook_PreservesExistingPreRunError` — 既有 hook 噴錯時錯誤會向上傳遞
9. `TestCommandPath` — 直接驗證 `commandPath` 對 root / 中間 / leaf 的輸出

## 與既有模組的關係 (Relationship to Existing Modules)

- 重用 `MetricService.Send()` (`metric/metric.go:131`) — 已是公開 API,內部走 `promwrite.WriteRequest` 序列化
- 重用 `NewVictoriaMetricsService()` (`metric/victoriametrics.go:11`) 作為預設後端
- 與 `cmd/versioning`、`cmd/gotmpl` 等既有 CLI 無侵入式整合 — 使用者只需在 `Execute()` 前呼叫 `metric.InstrumentCobra(rootCmd)`
- 不動既有檔案、不引入新依賴 (cobra v1.9.1 已是 go.mod 的直接依賴)

## 驗證 (Verification)

1. `make test` 全綠 (含新測試)
2. `go vet ./...` 無警告
3. `make build` 編譯通過
4. 跑既有 CLI 範例,手動驗證:
   - 起一個本地 VictoriaMetrics (或 `nc -l 8428` 簡單驗證)
   - 設定 `VICTORIAMETRICS_URL=http://localhost:8428/api/v1/write`
   - 跑 `bin/versioning patch --foo=bar`,預期在 VictoriaMetrics 看到 `cli_command_executed{cmd="versioning patch",flags="foo"}` 1 筆
