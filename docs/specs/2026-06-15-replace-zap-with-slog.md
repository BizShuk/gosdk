# 將 `log/` 模組從 zap 遷移至 Go 標準 `log/slog`

本計畫旨在把 `gosdk/log` 模組及其 13 個呼叫端檔案從 `go.uber.org/zap` 全面替換為 Go 1.21+ 內建的 `log/slog`，並以 Viper 注入 `LOG_LEVEL` 與 `LOG_FORMAT` 兩個設定鍵，附帶合理預設值。

## 結論先行 (Conclusion First)

- 替換目標：把 `zap.L()` / `zap.S()` 全部改為 `slog.Info()` / `slog.Error()` 等標準 API；54 處呼叫點一次到位。
- 設定來源：`LOG_LEVEL` 與 `LOG_FORMAT` 透過 Viper 讀取（與既有 `LOG_LEVEL` 行為一致），不破壞現有部署。
- 預設值：`LOG_LEVEL=info`、`LOG_FORMAT=text`，無設定時自動 fallback。
- 相依性：`go.uber.org/zap` 保留在 `go.mod` 作為向後相容層，不主動刪除；本次只確保新程式碼不再 import zap。

## 設計決策 (Design Decisions)

### 1. 模組介面

`log/log.go` 公開最小集合：

| 函式                | 用途                                                                  |
| ------------------- | --------------------------------------------------------------------- |
| `Init()`            | 依 Viper 設定初始化全域 `slog.Default`；可在 `config.Default()` 後再呼叫以套用 `LOG_LEVEL` |
| `GetLogLevel()`     | 回傳 `slog.Level`；保留供測試與外部檢查使用                              |
| `SetDefault()` 內部 | 使用 `slog.SetDefault` 註冊至標準全域                                  |

不再提供 `Info` / `Error` / `Fatal` 等 wrapper 函式 — 全部走 `slog.Info()` / `slog.Error()` 的 package-level 函式。

### 2. 設定鍵

| 環境變數 / Viper key | 合法值 (case-insensitive) | 預設值 | 說明                            |
| -------------------- | ------------------------- | ------ | ------------------------------- |
| `LOG_LEVEL`          | `debug`/`info`/`warn`/`error` | `info` | 空字串或未設定皆走預設             |
| `LOG_FORMAT`         | `text`/`json`             | `text` | 空字串或未設定皆走預設；其他值 fallback 為 `text` |

### 3. Level / Handler 對照

| 設定值      | `slog.Level`     | 對應 handler 行為                                  |
| ----------- | ---------------- | -------------------------------------------------- |
| `debug`     | `LevelDebug` (-4) | 全部層級輸出                                       |
| `info`      | `LevelInfo`  (0)  | `info`/`warn`/`error` 輸出；`debug` 過濾掉          |
| `warn`      | `LevelWarn`  (4)  | `warn`/`error` 輸出                                |
| `error`     | `LevelError` (8)  | 僅 `error` 輸出                                     |

| 設定值   | Handler                 | 輸出格式                |
| -------- | ----------------------- | ----------------------- |
| `text`   | `slog.NewTextHandler`   | `time=... level=INFO msg=... key=value` |
| `json`   | `slog.NewJSONHandler`   | `{"time":"...","level":"INFO","msg":"...","key":"value"}` |

### 4. 預設值注入時機

`log.Init()` 內部邏輯：

1. 讀 `LOG_LEVEL`；空字串或不合法值 → `slog.LevelInfo`
2. 讀 `LOG_FORMAT`；空字串或不合法值 → `"text"`
3. 構造 `*slog.HandlerOptions{Level: level}`，再依 `format` 選 handler
4. `slog.SetDefault(slog.New(handler))`

`init()` 仍會在 import 時呼叫一次 `Init()`，確保 `viper` 尚未載入時也有 logger（預設 `info`/`text`），與現有行為一致。

### 5. slog 沒有 Fatal 等級的處理

`zap.Fatal` / `zap.S().Fatal` 會呼叫 `os.Exit(1)`，但 `slog` 沒有 `Fatal` level。處理原則：

- 若訊息本身就要終止程式（罕見，通常只在啟動失敗），呼叫 `slog.Error(...)` 後接 `os.Exit(1)`
- 一般錯誤處理維持 `slog.Error(...)`，由呼叫端決定是否退出
- `zap.S().Fatalf` 在 `main.go` / `embedFS.go` / `cmd/gotmpl` 中各出現 1 次，需逐一替換

### 6. zap field helper → slog key-value

`zap` 使用強型別 field 建構子（`zap.String`/`zap.Int`/`zap.Error`/`zap.Any`）；`slog` 改用 variadic `key, value` 配對。

對照：

| zap                                    | slog                                    |
| -------------------------------------- | --------------------------------------- |
| `zap.L().Info("m", zap.String("k","v"))` | `slog.Info("m", "k", "v")`              |
| `zap.L().Info("m", zap.Int("n", 1))`     | `slog.Info("m", "n", 1)`                |
| `zap.L().Info("m", zap.Any("o", obj))`   | `slog.Info("m", "o", obj)`              |
| `zap.L().Error("m", zap.Error(err))`     | `slog.Error("m", "err", err)`           |
| `zap.S().Infof("Server %s", addr)`       | `slog.Info("Server starting", "addr", addr)` |
| `zap.S().Errorf("err: %v", err)`         | `slog.Error("operation failed", "err", err)` |
| `zap.L().Sugar().Errorf("warn %v", err)` | `slog.Warn("message", "err", err)`      |

## 開放性問題 (Open Questions)

無 — 兩個關鍵設計選擇（呼叫端直接用 slog、保留 zap 相依）已於先前確認。

## 建議變更 (Proposed Changes)

---

### `log/` 模組重寫

#### [MODIFY] `log/log.go`
- 移除 `go.uber.org/zap` 與 `zapcore` import。
- 改 import `"log/slog"` 與 `"os"`、`"strings"`、`"github.com/spf13/viper"`。
- `Init()` 改為：
    - 呼叫內部 `parseLevel(viper.GetString("LOG_LEVEL"))` 取得 `slog.Level`
    - 呼叫內部 `parseFormat(viper.GetString("LOG_FORMAT"))` 取得 handler factory
    - 構造 `*slog.HandlerOptions{Level: level}`
    - `slog.SetDefault(slog.New(handler(os.Stdout, opts)))`
- 移除 `PROFILE=prod` 分支（`LOG_FORMAT=json` 已涵蓋 production 需求；如需保留環境變數切換可另開 issue）。
- `init()` 維持呼叫 `Init()`。

#### [MODIFY] `log/level.go`
- 函式簽章改為 `func GetLogLevel() slog.Level`，內部委派給 `parseLevel`。
- 新增 `func parseLevel(s string) slog.Level`：
    - `strings.ToLower` 後比對 `debug`/`info`/`warn`/`error`，預設 `slog.LevelInfo`
- 同檔新增 `func parseFormat(s string) string`：
    - 回傳 `"json"` 或 `"text"`，預設 `"text"`

#### [MODIFY] `log/log_test.go`
- `TestGetLogLevel` 改斷言 `slog.Level` 常數。
- `TestInitWithLogLevels` 改用 `slog.Default().Enabled(context.Background(), slog.LevelDebug)` 判斷輸出與否。
- 新增 `TestInitWithLogFormat` 覆蓋 `text` / `json` 兩種 handler（透過 `slog.Default().Handler()` 檢查型別）。
- 新增預設值測試：`viper.Reset()` 後 `Init()`，斷言 `LOG_LEVEL=""` 與 `LOG_FORMAT=""` 都得到 `info` / `text`。

#### [DELETE] 不適用 — 模組結構維持

---

### 呼叫端遷移 (13 個檔案，54 處呼叫)

> 每個檔案都會：
> 1. 移除 `go.uber.org/zap` import
> 2. 新增 `"log/slog"` import
> 3. 機械替換（見 §6 對照表）
> 4. 對 `Fatal` / `Fatalf` 額外加 `os.Exit(1)`

#### [MODIFY] `main.go`
- 8 處 `zap.S().Info/Error/Errorf/Infof/Fatalf` → `slog.Info/Error` + 必要時 `os.Exit(1)`
- `import "go.uber.org/zap"` 移除；`log/slog` 加入

#### [MODIFY] `cmd/gotmpl/cmd/root.go`
- 2 處 `zap.S().Fatal` / `zap.S().Info` 替換
- `Fatal` 需加 `os.Exit(1)`；但此檔目前已有 `os` import，無需新增

#### [MODIFY] `config/config.go`
- 2 處 `zap.L().Info/Infof` 替換

#### [MODIFY] `config/env.go`
- 2 處 `zap.S().Warnf` 替換

#### [MODIFY] `config/yaml.go`
- 2 處 `zap.S().Warnf` 替換

#### [MODIFY] `config/json.go`
- 2 處 `zap.S().Warnf` 替換

#### [MODIFY] `config/embedFS.go`
- 4 處 `zap.S().Fatal/Fatalf/Infof` 替換；`Fatal*` 加 `os.Exit(1)`

#### [MODIFY] `config/sample/main.go`
- 多處 `zap.L().Info/Error` 替換（注意保留 `zap.String("k", v)` 結構）

#### [MODIFY] `db/sqlite.go`、`db/mysql.go`、`db/postgres.go`
- 連線失敗與初始化訊息改用 `slog.Error` / `slog.Info`（具體行數以實際為準）

#### [MODIFY] `encode/csv/processor.go`
- 3 處 `zap.L().Error` 替換；`zap.Any` 對應到裸 value

#### [MODIFY] `metric/cobra.go`
- 1 處 `zap.L().Warn(..., zap.String(...), zap.Error(err))` → `slog.Warn(..., "cmd", m.Tags["cmd"], "err", err)`

#### [MODIFY] `metric/metric_test.go`、`metric/otel_test.go`
- 若有用到 zap，套相同替換

#### [MODIFY] `notify/slack.go`
- 1 處 `zap.S().Warn` 替換

#### [MODIFY] `utils/file.go`
- 6 處 `zap.L().Info/Error` 替換；含 `zap.L().Sugar().Errorf` 兩處

---

### go.mod 處理

#### [MODIFY] `go.mod` 與 `go.sum`
- 保留 `go.uber.org/zap` 條目（依使用者決定）
- 不主動移除；後續若 CI 報 unused 可在另一個 PR 處理

## 測試計畫 (Test Plan)

1. **單元測試**：
    - `log/log_test.go` 涵蓋 `parseLevel` / `parseFormat` / `Init` 全部路徑
    - 既有 `TestInitWithLogLevels` 改寫以驗證 slog 的 `Enabled()` 行為
    - 新增 `TestInitDefaultValues` 確保 viper 為空時套用預設
2. **迴歸測試**：
    - `go test -v ./...` 跑完整專案
    - `go vet ./...` 確認無 unused import
3. **行為驗證**：
    - 手動啟動 `bin/server`，觀察 `LOG_LEVEL=debug` 時多出 debug log
    - 切 `LOG_FORMAT=json`，確認 stdout 為 JSON 格式

## 驗證 (Verification)

- [ ] `go vet ./...` 無錯誤
- [ ] `go test -v ./...` 全部通過
- [ ] `go build -v ./...` 成功
- [ ] `go.mod` 中 `go.uber.org/zap` 仍存在（向後相容）
- [ ] `log/log.go`、`log/level.go` 不再 import `zap`
- [ ] 13 個呼叫端檔案不再 import `zap`
- [ ] 預設 `LOG_LEVEL` / `LOG_FORMAT` 行為與規格一致
- [ ] `LOG_FORMAT=json` 與 `LOG_LEVEL=debug` 動態生效

## 風險與回滾 (Risks & Rollback)

- 風險：slog 的 text handler 預設格式與 zap 不同，外部 log collector 若有 regex 解析需要更新
- 緩解：slog text handler 格式為 `time=... level=... msg=... key=value`，貼近 k=v 風格，主流 collector 應可解析
- 回滾：本次變更為單一 PR，git revert 即可回到 zap 版本
