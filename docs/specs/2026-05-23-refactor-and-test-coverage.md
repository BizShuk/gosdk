# 解決 README.md 中的待辦事項與測試覆蓋率提升 (Resolving README.todo Items and Improving Test Coverage)

此實作計畫 (Implementation Plan) 旨在解決 `README.md` 中所列出的五個技術債與待辦事項：
1. 統一 `config/json.go` 與 `config/embedFS.go` 的日誌 (Logging) 輸出，改用全域的 `gosdk/log` 套件。
2. 修正 `config/embedFS.go` 中 `GetFSReader()` 的錯誤處理 (Error Handling)，避免在讀取失敗時傳回空資料。
3. 重構並整合 `utils/processor.go` 中的 CSV 處理邏輯與 `encode/csv/csv.go` 中的 `Decoder` 介面至統一的 `encode/csv` 套件。
4. 更新安全中介軟體 (Security Middleware) `mw.Helmet()`，移除已被現代瀏覽器棄用的 `X-XSS-Protection` 標頭，並引入現代的 `Permissions-Policy` 與 `Cross-Origin-Opener-Policy` 標頭。
5. 針對 `utils/` 套件底下的工具函式與重構後的 `encode/csv` 套件撰寫完整的單元測試 (Unit Test) 以提升測試覆蓋率 (Test Coverage)。

## 使用者審查請求 (User Review Required)

無重大的破壞性變更 (Breaking Changes)。以下為需要確認的設計細節：
- 安全性標頭 (Security Headers)：在 `mw.Helmet()` 中新增的 `Permissions-Policy` 與 `Cross-Origin-Opener-Policy` 將使用預設的安全設定（例如限制地理位置、麥克風、相機等權限，並限制跨來源視窗）。

## 開放性問題 (Open Questions)

- 目前無其他未決的開放性問題。

## 建議變更 (Proposed Changes)

---

### 設定管理模組 (Configuration Module)

#### [MODIFY] [json.go](file:///Users/shuk/projects/gosdk/config/json.go)
- 移除標準庫的 `log` 與 `fmt` 引用，改為引入 `github.com/bizshuk/gosdk/log`。
- 修改 `Load()` 函式中的日誌輸出，使其調用 `log.Info`、`log.Infof` 和 `log.Fatalf`。

#### [MODIFY] [embedFS.go](file:///Users/shuk/projects/gosdk/config/embedFS.go)
- 移除標準庫的 `log` 與 `fmt` 引用，改為引入 `github.com/bizshuk/gosdk/log`。
- 重構 `GetFSReader(fs embed.FS, filename string)` 函式，使其傳回 `(*bytes.Reader, error)`。
- 修改 `Load()` 函式，當 `GetFSReader()` 傳回錯誤時直接呼叫 `log.Fatalf` 終止程式，防止後續因空資料而產生問題。

---

### 編碼與 CSV 資料處理模組 (Encoding and CSV Processing Module)

#### [MODIFY] [csv.go](file:///Users/shuk/projects/gosdk/encode/csv/csv.go)
- 將套件名稱從 `encode` 修改為 `csv`，以符合 Go 語言 (Go Language) 的目錄命名慣例。

#### [NEW] [processor.go](file:///Users/shuk/projects/gosdk/encode/csv/processor.go)
- 新增 `encode/csv/processor.go`，將原先位於 `utils/processor.go` 中的 `RecordProcessor` 型別與 `ProcessCSVFile` 函式移至此處。
- 套件名稱設為 `csv`。
- 移除原先對 `utils` 的相依性，使用內部小寫的檔名解析輔助函式。

#### [NEW] [processor_test.go](file:///Users/shuk/projects/gosdk/encode/csv/processor_test.go)
- 針對 `ProcessCSVFile` 與 CSV 的處理邏輯撰寫單元測試，驗證檔案讀取、歸檔標記檔 (Archived Mark File) 的生成與略過、以及錯誤處理等邏輯。

#### [DELETE] [processor.go](file:///Users/shuk/projects/gosdk/utils/processor.go)
- 刪除此檔案，其邏輯已移至 `encode/csv/processor.go`。

---

### 安全中介軟體模組 (Security Middleware Module)

#### [MODIFY] [helmet.go](file:///Users/shuk/projects/gosdk/mw/helmet.go)
- 移除 `c.Header("X-XSS-Protection", "1; mode=block")`。
- 新增 `c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")`。
- 新增 `c.Header("Cross-Origin-Opener-Policy", "same-origin")`。

---

### 通用工具套件與測試 (Utilities Package and Testing)

#### [MODIFY] [file.go](file:///Users/shuk/projects/gosdk/utils/file.go)
- 引入 `github.com/bizshuk/gosdk/encode/csv`。
- 將 `NewCSVFilelistCallback` 函式參數中的 `rowProcessor RecordProcessor` 修改為 `rowProcessor csv.RecordProcessor`。
- 將內部呼叫的 `ProcessCSVFile` 修改為 `csv.ProcessCSVFile`。

#### [NEW] [file_test.go](file:///Users/shuk/projects/gosdk/utils/file_test.go)
- 新增對 `utils/file.go` 的單元測試，覆蓋 `FileExists`、`SaveFile`、`SaveCSV`、`ParseCSVFile`、`GetFileName`、`NewFilelistCallback` 以及 `NewCSVFilelistCallback` 等函式。

#### [NEW] [string_test.go](file:///Users/shuk/projects/gosdk/utils/string_test.go)
- 新增對 `utils/string.go` 的單元測試，覆蓋 `StringPointer`、`StringWithCharset` 與 `String`。

#### [NEW] [time_test.go](file:///Users/shuk/projects/gosdk/utils/time_test.go)
- 新增對 `utils/time.go` 的單元測試，覆蓋 `ParseTimeDuration` 函式在各種正常與異常格式下的解析行為。

#### [NEW] [int_test.go](file:///Users/shuk/projects/gosdk/utils/int_test.go)
- 新增對 `utils/int.go` 的單元測試，覆蓋各種整數與無號整數指標 (Integer and Unsigned Integer Pointers) 轉換函式。

---

### 專案結構與文件更新 (Project Structure and Documentation Updates)

#### [MODIFY] [CLAUDE.md](file:///Users/shuk/projects/gosdk/CLAUDE.md)
- 更新專案結構圖，移除 `utils/processor.go` 並新增 `encode/csv/processor.go` 等檔案，同時更新相關模組說明。

#### [MODIFY] [README.md](file:///Users/shuk/projects/gosdk/README.md)
- 更新 `README.md` 中的專案結構描述，並將已完成的待辦事項勾選完成。

## 驗證計畫 (Verification Plan)

### 自動化測試 (Automated Tests)
- 於終端機執行 `go test -v ./...` 確保所有的單元測試（包含新增的測試檔案）均能順利通過，且無編譯錯誤。
- 執行 `go vet ./...` 檢查靜態語法。
- 執行 `go generate ./...` 驗證程式碼產生。

### 手動驗證 (Manual Verification)
- 執行預設的 config 範例：`go run sample/config/main.go`，觀察日誌格式是否符合預期，且印出內容是否皆使用 zap logger 格式。
