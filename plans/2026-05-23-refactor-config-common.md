# 移轉 `config/config.go` 核心結構與 `config/db/*` 至 `config/common`

本計劃旨在重構 `組態管理 (Configuration Management)` 模組。我們將把 `config/config.go` 中定義的 `ServerConfig`、`DBConnConfig` 以及 `ConfigSchema` 結構，以及 `config/db/` 目錄下的所有資料庫連線代碼移轉至新套件 `config/common`。

## User Review Required

> [!IMPORTANT]
> 此次重構涉及多個套件的搬移與路徑變更：
> 1. `config/db` 套件將被完全刪除，其下的 `db.go`, `mysql.go`, `sqlite.go` 將移至 `config/common`。
> 2. `main.go` 與 `sample/config/main.go` 中對於 `config/db` 的引用需要同步修改為 `config/common`。
> 3. 為維持向下相容性 (Backward Compatibility)，我們將在 `config/config.go` 中保留 `ServerConfig`、`DBConnConfig` 與 `ConfigSchema` 的 `型別別名 (Type Alias)`。

## Open Questions

目前無開放性問題。我們將採取保留 `型別別名 (Type Alias)` 的方式來避免破壞現有對 `config.ConfigSchema` 的參考。

## Proposed Changes

---

### 組態共用模組 (Config Common Module)

#### [NEW] [config/common/config.go](file:///Users/shuk/projects/gosdk/config/common/config.go)
建立新檔案，定義從 `config/config.go` 移轉過來的核心結構：
- `ServerConfig` 結構
- `DBConfig` 結構（來自原 `config/db/db.go`）
- `DBConnConfig` 作為 `DBConfig` 的 `型別別名 (Type Alias)`，以確保 `ConfigSchema` 與其他程式的相容性
- `ConfigSchema` 結構

#### [NEW] [config/common/db.go](file:///Users/shuk/projects/gosdk/config/common/db.go)
移轉自 `config/db/db.go`，變更包名為 `common`。

#### [NEW] [config/common/mysql.go](file:///Users/shuk/projects/gosdk/config/common/mysql.go)
移轉自 `config/db/mysql.go`，變更包名為 `common`。

#### [NEW] [config/common/sqlite.go](file:///Users/shuk/projects/gosdk/config/common/sqlite.go)
移轉自 `config/db/sqlite.go`，變更包名為 `common`。

---

### 原組態模組 (Original Config Module)

#### [MODIFY] [config/config.go](file:///Users/shuk/projects/gosdk/config/config.go)
- 移除 `ServerConfig`、`DBConnConfig`、`ConfigSchema` 的具體定義。
- 引入 `github.com/bizshuk/gosdk/config/common`。
- 新增 `型別別名 (Type Alias)`：
  ```go
  type ServerConfig = common.ServerConfig
  type DBConnConfig = common.DBConnConfig
  type ConfigSchema = common.ConfigSchema
  ```

#### [DELETE] [config/db/db.go](file:///Users/shuk/projects/gosdk/config/db/db.go)
#### [DELETE] [config/db/mysql.go](file:///Users/shuk/projects/gosdk/config/db/mysql.go)
#### [DELETE] [config/db/sqlite.go](file:///Users/shuk/projects/gosdk/config/db/sqlite.go)

---

### 應用程式入口與範例 (Application Entry and Samples)

#### [MODIFY] [main.go](file:///Users/shuk/projects/gosdk/main.go)
- 將 `github.com/bizshuk/gosdk/config/db` 的 import 改為 `github.com/bizshuk/gosdk/config/common`。
- 將 `db.NewDBConfig` 的呼叫改為 `common.NewDBConfig`。

#### [MODIFY] [sample/config/main.go](file:///Users/shuk/projects/gosdk/sample/config/main.go)
- 將 `github.com/bizshuk/gosdk/config/db` 的 import 改為 `github.com/bizshuk/gosdk/config/common`。
- 將 `db.DBConfig` 與 `db.NewDBConfig` 的參考修改為 `common.DBConfig` 與 `common.NewDBConfig`。

---

### 專案文件 (Project Documents)

#### [MODIFY] [CLAUDE.md](file:///Users/shuk/projects/gosdk/CLAUDE.md)
更新 `專案結構 (Project Structure)` 與 `模組對應 (Module Mapping)` 以反映路徑變更。

#### [MODIFY] [README.md](file:///Users/shuk/projects/gosdk/README.md)
更新 `核心實體 (Key Entities)` 及說明文檔中的 `import` 與型別參考。

---

## Verification Plan

### Automated Tests
- 執行單元測試：`make test` 或 `go test -v ./...`。
- 執行組態範例：`go run ./sample/config`，驗證輸出是否正常且沒有編譯錯誤。
- 啟動伺服器：`go run main.go`，驗證資料庫連線初始化與伺服器啟動是否正常。

### Manual Verification
- 確認 `config/db/` 目錄已完全清除。
- 檢查專案中沒有殘留的 `config/db` 的 `import`。
