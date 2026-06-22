# 日誌分流處理器實作計劃

本計劃旨在將自訂的 `LevelSplitHandler` 整合至 `gosdk/log` 套件中，以達成將不同層級的日誌分流到 `os.Stdout` (如 Debug, Info) 與 `os.Stderr` (如 Warn, Error) 的功能。同時修正原始設計中 `Enabled` 恆回傳 `true` 導致日誌過濾失效的潛在問題。

## 使用者審查要求

無。

## 待解決問題

無。

## 預期變更

---

### 結構化日誌模組

#### [MODIFY] [log.go](file:///Users/shuk/projects/tmp/gosdk/log/log.go)
- 新增 `LevelSplitHandler` 結構體與其實作 `slog.Handler` 的四個方法 (`Enabled`, `Handle`, `WithAttrs`, `WithGroup`)。
- 修正 `Enabled` 邏輯：根據輸入層級，轉發給對應 of `stdoutHandler` 或 `stderrHandler` 進行其 `Enabled` 判斷。
- 修改套件層級的 `init()` 函數，分別使用 `os.Stdout` 與 `os.Stderr` 建立基礎 Handler，再透過 `LevelSplitHandler` 組合後註冊為全域預設 Logger。

#### [MODIFY] [log_test.go](file:///Users/shuk/projects/tmp/gosdk/log/log_test.go)
- 新增 `TestLevelSplitHandler` 單元測試。
- 測試使用自訂的 `bytes.Buffer` 捕獲 `stdout` 與 `stderr` 的輸出，以驗證：
  1. 日誌等級為 `Info` 時，`slog.Info` 僅輸出至 `stdout`，`slog.Error` 僅輸出至 `stderr`。
  2. 日誌等級為 `Warn` 時，`slog.Info` 不會被輸出 (確保 `Enabled` 邏輯正確)。

## 驗證計劃

### 自動化測試
- 執行 `go test -v ./log` 以驗證日誌分流功能及 Enabled 邏輯是否正確。
- 執行 `go test -v ./...` 以驗證整個專案沒有破壞性變更。
