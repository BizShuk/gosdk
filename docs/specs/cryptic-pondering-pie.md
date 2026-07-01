# Plan — 將 `config/common` 改寫為 `db` 套件(per-storage singleton + flat viper keys)

## Context(為什麼要改)

現有的 `config/common/` 套件把「DB 連線工廠」與「config schema 型別」混在一起,而且把 driver 用 runtime 字串 (`"sqlite"` / `"mysql"`) 切換 factory:

```go
// 現行用法
gormDB, _ := common.NewDBConfig("default").Create()
// viper: db.default.driver=sqlite  /  db.default.url=./sample.db
```

這個模式有幾個問題,跟專案「micro-service」取向衝突:

1. **driver 是 runtime 決定的字串** — 一個 process 只能「用一個 driver」,但程式碼卻允許任意 driver 字串流動,失去型別安全
2. **巢狀 viper key (`db.default.driver` / `db.default.url`)** — 跟專案其他模組(如 `metric.METRIC_URL`、`MIMIR_URL`、`VICTORIAMETRICS_URL`)的扁平 `<UPPER>_<KEY>` 命名不一致
3. **「建立連線」是函式呼叫,不是 service** — 沒有 singleton,呼叫端每次拿到新的 `*gorm.DB`,且無從統一管理生命週期
4. **`config/common` 同時放 DB 與 config schema (`ServerConfig`、`ConfigSchema`)** — 變成 junk drawer,移除 DB 時連這些型別都要跟著評估

**預期結果**: 每種儲存型態是一個 service,有自己型別、自己的 global singleton、自己的 viper key;移除 `config/common/` 整個目錄;`config/` 不再依賴任何 sub-package。

## 設計決策(已與使用者確認)

| 議題 | 決定 | 理由 |
| --- | --- | --- |
| 套件佈局 | 單一 `db` 套件,內部多檔案 | 使用者選擇;扁平,`db/sqlite.go` + `db/mysql.go` + `db/db.go` |
| MySQL 連線欄位 | 保留單一 DSN 字串 (`MYSQL_DSN`) | 使用者選擇;與舊 `url` 欄位對齊,最簡單 |
| Init 模式 | 明確呼叫 `InitSQLite()` / `InitMySQL()` | 使用者選擇;啟動順序透明 |
| Viper 變數命名 | `<TYPE>_<FIELD>` 扁平大寫 | `SQLITE_PATH`、`MYSQL_DSN`(env var 前綴 `APP_` 沿用 `config.Default()` 慣例,實際對應 `APP_SQLITE_PATH`、`APP_MYSQL_DSN`) |
| `ServerConfig` / `ConfigSchema` / `DBConnConfig` | 連同 alias 一起移除 | 整個 repo grep 確認這三個型別除了 `config/config.go` 自己的 alias 外沒有任何外部使用者;`config/sample/main.go` 已用本地 `ServerConfig` 定義 |

## 新套件結構

```
db/
├── db.go           # Service 介面
├── sqlite.go       # SQLite type + DefaultSQLite + InitSQLite
├── mysql.go        # MySQL  type + DefaultMySQL  + InitMySQL
├── sqlite_test.go  # SQLite 測試
└── mysql_test.go   # MySQL 測試
```

## 公開 API

```go
// db/db.go
package db

import "gorm.io/gorm"

type Service interface {
    DB() *gorm.DB
    Close() error
}
```

```go
// db/sqlite.go
package db

type SQLite struct {
    db *gorm.DB
    path string
}

func (s *SQLite) DB() *gorm.DB { return s.db }
func (s *SQLite) Close() error { /* closes underlying *sql.DB */ }

// DefaultSQLite 是 SQLite 服務的全域 singleton,需透過 InitSQLite() 初始化。
// 命名說明:「Default」前綴強調這是慣例存取點,而非「唯一存取點」——
// 仍然不鼓勵建立第二個 *SQLite(違背 micro-service 概念)。
var DefaultSQLite *SQLite

// InitSQLite 從 viper 讀取 SQLITE_PATH,開啟連線,並設定 DefaultSQLite。
// 已經初始化過會回傳 error 以避免覆蓋(守護 singleton 不變性)。
func InitSQLite() error { ... }
```

`db/mysql.go` 結構完全對稱,欄位為 `dsn`、viper key 為 `MYSQL_DSN`。

### viper key 對照

| 舊 key (巢狀) | 新 key (扁平) | 對應 env var |
| --- | --- | --- |
| `db.default.driver` | (移除;由 InitSQLite/InitMySQL 決定) | — |
| `db.default.url` | `SQLITE_PATH` | `APP_SQLITE_PATH` |
| `db.default.url` (driver=mysql 時) | `MYSQL_DSN` | `APP_MYSQL_DSN` |

### 典型呼叫模式

```go
config.Default()
log.Init()

if viper.IsSet("SQLITE_PATH") {
    if err := db.InitSQLite(); err != nil { zap.S().Errorf(...) }
}
if viper.IsSet("MYSQL_DSN") {
    if err := db.InitMySQL(); err != nil { ... }
}

// 之後任何地方
gormDB := db.DefaultSQLite.DB()
```

## 修改/建立/刪除的檔案

### 刪除 (4 個檔案,整個 `config/common/` 目錄)

- `config/common/config.go`
- `config/common/db.go`
- `config/common/sqlite.go`
- `config/common/mysql.go`

### 建立 (5 個檔案)

- `db/db.go` — `Service` 介面
- `db/sqlite.go` — `SQLite` type、`DefaultSQLite`、`InitSQLite()`
- `db/mysql.go` — `MySQL` type、`DefaultMySQL`、`InitMySQL()`
- `db/sqlite_test.go` — 涵蓋 viper 讀取、連線開啟、singleton 保護、Service 方法
- `db/mysql_test.go` — 涵蓋 viper 讀取、singleton 保護(MySQL 連線測試用 `sqlmock` 模擬,避免 CI 需要 MySQL instance)

### 修改

- `main.go` (line 7 import, line 24-33 DB 區塊) — 移除 `config/common` import,DB 區塊改為 `viper.IsSet` + `db.InitSQLite/InitMySQL`
- `config/config.go` (line 9 import, lines 14-18 type alias block) — 移除整個 `type ( ... )` alias block 與 `config/common` import
- `config/sample/main.go`
  - line 22 移除 `config/common` import
  - line 34 移除 `DB map[string]common.DBConfig` 欄位(`AppConfig` 不再代表 DB 結構,新設計每個儲存有自己的 key)
  - line 113-123 「用法 5」改為呼叫 `db.InitSQLite()` 並透過 `db.DefaultSQLite.DB()` 取得 `*gorm.DB`
- `config/sample/conf/config.local.yaml` — 移除 `db:` 區塊,新增:
  ```yaml
  sqlite:
    path: ./sample.db
  ```
- `README.md` — line 20, 22, 235, 335 移除 `common.*` 引用、修改章節說明以反映新設計
- `CLAUDE.md` — 專案結構 tree 移除 `config/common/` 子節;Module Mapping 表格中「設定管理」列從 `config/, config/common/` 改為 `config/`,新增「資料庫連線」列指向 `db/`;「Key Decisions」段落新增關於此重構的決策紀錄
- `skills/golang-gosdk/SKILL.md` line 402 — 反模式表格的 Hardcoding viper keys 列改為建議使用 `db.InitSQLite()` / `db.InitMySQL()`

### 不動

- `config/sample/conf/.env` / `.env.local` — 權限被擋,使用者須自行確認是否有 `db.*` 相關設定;若僅含 `server.*` 等則不需動
- `config/sample/sample.db` — 既有 SQLite 檔,新設計仍會用相同路徑產生

## 關鍵實作細節

### Singleton 保護

`InitSQLite()` 內:

```go
func InitSQLite() error {
    if DefaultSQLite != nil {
        return fmt.Errorf("db.SQLite already initialized")
    }
    path := viper.GetString("SQLITE_PATH")
    if path == "" {
        return fmt.Errorf("SQLITE_PATH not set")
    }
    gormDB, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
    if err != nil {
        return fmt.Errorf("sqlite open: %w", err)
    }
    DefaultSQLite = &SQLite{db: gormDB, path: path}
    return nil
}
```

`InitMySQL()` 結構對稱,使用 `gorm.io/driver/mysql` 與 `MYSQL_DSN`。

### 為什麼 `*SQLite` 欄位不匯出

欄位 `db` 與 `path` 用小寫不匯出,符合「singleton 不可繞過建構式」的精神——外部呼叫端只能透過 `DB()` 與 `Close()` 與 singleton 互動,無法直接讀取 `path` 或操作 `db` 指標。

### Close 行為

```go
func (s *SQLite) Close() error {
    sqlDB, err := s.db.DB()
    if err != nil {
        return err
    }
    return sqlDB.Close()
}
```

## 測試策略

### `db/sqlite_test.go`

| Test | 目的 |
| --- | --- |
| `TestInitSQLite_ReadsSQLitePathFromViper` | 設 `viper.Set("SQLITE_PATH", ":memory:")` → InitSQLite 成功,`DefaultSQLite.path == ":memory:"` |
| `TestInitSQLite_EmptyPathFails` | 不設 viper → InitSQLite 回傳 error 包含 `SQLITE_PATH not set` |
| `TestInitSQLite_RefusesDoubleInit` | 第一次成功,第二次回傳 error 包含 `already initialized` |
| `TestSQLite_DB_ReturnsGormDB` | 呼叫 `DefaultSQLite.DB()` 取得非 nil `*gorm.DB` 且能 ping |
| `TestSQLite_Close_ClosesUnderlyingDB` | 呼叫 `Close()` 後 `db.DB()` 呼叫底層 sql 操作回傳錯誤 |

### `db/mysql_test.go`

| Test | 目的 |
| --- | --- |
| `TestInitMySQL_ReadsDSNFromViper` | 設 `viper.Set("MYSQL_DSN", "user:pass@tcp(...)")` → InitMySQL 嘗試連線(以無效 DSN 驗證讀取邏輯) |
| `TestInitMySQL_EmptyDSNFails` | 不設 viper → 回傳 error 包含 `MYSQL_DSN not set` |
| `TestInitMySQL_RefusesDoubleInit` | 第一次成功或失敗後,第二次的 `already initialized` 邏輯(注意:第一次若連線失敗,singleton 不應被設定) |

(實際 MySQL 連線測試在 CI 環境通常不跑,僅驗證「讀到 DSN 字串並呼叫 gorm.Open」的邏輯,允許連線失敗但仍確認 viper 解析正確。)

### 全套驗證

```bash
go test ./...        # 所有 package 測試包含新 db/* 與現有 config/*
go vet ./...         # 無 vet 警告
go build ./...       # 編譯通過(main.go 與 config/sample 都會觸及新 db 套件)
go run ./config/sample  # 跑 sample:設定檔含 sqlite.path,確認 InitSQLite 成功、sample.db 寫入
go run main.go       # 根目錄無 sqlite/mysql 設定,Init 不被呼叫、伺服器正常啟動
```

## 風險與緩解

| 風險 | 緩解 |
| --- | --- |
| `.env` 內可能含舊 `db.default.*` 設定,新設計讀不到 | 計畫明確標注需手動確認;若發現則搬移到 `SQLITE_PATH` / `MYSQL_DSN` |
| `config.Default()` 內 `viper.SetEnvPrefix("APP")` + `AutomaticEnv()` 對全大寫 key 的 env 變數處理可能不如預期(目前看 `metric.METRIC_URL` 採同樣 pattern 可運作) | 沿用 `metric/` 既有 pattern,測試會驗證 `viper.Set` + `viper.GetString` 路徑;env var 行為不在單元測試範圍 |
| 移除 `ServerConfig` / `ConfigSchema` / `DBConnConfig` 屬於 breaking change | grep 已確認無外部使用者;README / SKILL 同步更新避免文件誤導 |
| README / SKILL 文件散落多處,可能漏改 | 計畫明確列出每個要改的檔案與行號,實作時用 Grep 複查一次 `common\.` 與 `ServerConfig` 是否還有殘留 |

## 不在本次範圍

- 不新增除了 SQLite / MySQL 以外的 storage(如 Postgres、Redis)
- 不動 `metric/` 套件(雖然命名風格類似,但屬於另一個 domain)
- 不動 `config/sample/sample.db` 既有檔案內容(新設計仍會以 `SQLITE_PATH=./sample.db` 寫入同位置)
- 不新增 connection pool 設定(若未來需要,再以 `SQLITE_MAX_OPEN_CONNS` 等欄位延伸)
