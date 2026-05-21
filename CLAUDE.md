# gosdk — 技術脈絡 (Technical Context)

## 專案結構 (Project Structure)

```
gosdk/
├── main.go                     # 主程式進入點（含空 main 與 HTTPServer）
├── go.mod                       # Go 模組定義（github.com/bizshuk/gosdk, Go 1.24.5）
├── .env                         # 環境變數設定（PROFILE, viper.file）
├── outputgittag.sh              # Git tag 版本號輸出腳本
├── LICENSE
│
├── config/                      # 設定管理模組
│   ├── config.go                # Config 介面 + Default() 設定載入
│   ├── env.go                   # .env / .env.<profile> 載入
│   ├── yaml.go                  # config.<profile>.yaml 載入
│   ├── json.go                  # JSON 設定檔載入
│   ├── embedFS.go               # embed.FS 設定載入
│   └── db/                      # 資料庫連線管理
│       ├── db.go                # DBConfig + DatabaseFactory
│       ├── sqlite.go            # SQLite 連線
│       └── mysql.go             # MySQL 連線
│
├── router/                      # HTTP 路由模組
│   ├── default.go               # 預設路由（/stats）
│   ├── statsHandler.go          # Stats API Handler
│   ├── statsHandler_test.go     # 測試（空 TODO）
│   ├── health.go                # Health Check 端點
│   └── ping.go                  # Ping/Pong 端點
│
├── mw/                          # HTTP 中介層
│   ├── correlationId.go         # X-Correlation-Id 追蹤
│   └── helmet.go                # 安全性標頭（未實作）
│
├── log/                         # 日誌模組
│   ├── log.go                   # zap Logger 初始化 + Sugar 封裝
│   └── level.go                 # LOG_LEVEL 解析
│
├── encode/                      # 編碼轉換模組
│   ├── io/
│   │   ├── decode.go            # Decoder 介面
│   │   ├── gbk.go               # GBK → UTF-8
│   │   └── big5.go              # Big5 → UTF-8
│   └── csv/
│       └── csv.go               # CSV Row Decoder 介面
│
├── time/                        # 時間工具
│   ├── roc.go                   # 民國曆日期解析
│   └── sleep.go                 # 可配置延遲
│
├── utils/                       # 通用工具
│   ├── file.go                  # 檔案操作（SaveFile, SaveCSV, Glob 回呼）
│   ├── processor.go             # CSV 逐行處理器
│   ├── string.go                # 隨機字串產生
│   ├── int.go                   # 整數指標轉換
│   ├── type.go                  # IsNil 反射式 nil 檢查
│   ├── type_test.go             # IsNil 測試
│   ├── decode.go                # GBK/Big5 解碼（與 encode/io 重複）
│   ├── sleep.go                 # ConfigSleep（與 time/sleep.go 重複）
│   └── time.go                  # HH:MM:SS 解析
│
├── service/                     # 程式碼產生器核心
│   ├── generator.go             # Go AST 解析 + stringer 產生引擎
│   └── default.go               # 空 package
│
├── sample/
│   └── stringer.go              # go:generate 範例
│
└── cmd/                         # CLI 工具
    ├── stringer/
    │   └── main.go              # 增強版 stringer CLI
    └── gotmpl/
        ├── main.go              # gotmpl CLI 進入點
        ├── config.yaml          # 範例設定
        ├── gotmpl               # 預編譯二進位檔
        ├── cmd/
        │   ├── root.go          # Cobra root command
        │   └── loadTemplate.go  # 模板渲染邏輯
        └── tmpl/
            ├── tmpl.go          # embed.FS 嵌入
            └── sample.go.tmpl   # Go 程式碼模板
```

## 技術棧 (Tech Stack)

- Language: Go 1.24.5
- Framework: Gin v1.11.0 (HTTP), Cobra v1.9.1 (CLI)
- Build tool: `go build`（無 Makefile）
- Key dependencies:
  - `github.com/spf13/viper` v1.17.0 — 設定管理
  - `go.uber.org/zap` v1.27.1 — 結構化日誌
  - `gorm.io/gorm` v1.31.1 — ORM
  - `gorm.io/driver/sqlite` + `gorm.io/driver/mysql` — 資料庫驅動
  - `golang.org/x/text` v0.32.0 — 字元編碼
  - `golang.org/x/tools` v0.39.0 — Go AST 分析
  - `github.com/hairyhenderson/gomplate/v4` v4.3.3 — 模板函式
  - `github.com/satori/go.uuid` v1.2.0 — UUID 產生

## 關鍵決策 (Key Decisions)

- 採用 Viper 多來源合併策略: `.env` → `.env.<profile>` → `config.<profile>.yaml` → `APP_*` 環境變數，每層可覆寫前層設定
- 設定載入器使用介面抽象: `Config` 介面定義 `Load()` 和 `GetConfigName()`，`EnvConfig` / `YamlConfig` / `JsonConfig` / `FSConfig` 皆可獨立使用或擴展
- 資料庫採工廠模式: `DatabaseFactory` 根據 `driver` 欄位選擇 SQLite 或 MySQL，設定從 Viper `db.<key>` 區段反序列化
- 日誌依環境自動切換: `PROFILE != "prod"` → development mode（console encoder），否則 production mode（JSON encoder）
- stringer 擴展標準工具: `GeneratorEx` 嵌入 `service.Generator`，在原有 `String()` 之外產生 `List()` / `ValueList()` / `Map()` / `ValueMap()`
- Correlation ID 中介層: 以 `X-Correlation-Id` header 追蹤請求，自動以 UUID v4 填充
- gotmpl 採 embed.FS: 模板檔透過 `//go:embed` 嵌入二進位檔，使用 `gomplate.CreateFuncs` 提供進階模板函式

## 模組對應 (Module Mapping)

| 業務領域 (Domain)                        | 套件/模組 (Package/Module)                             | 進入點 (Entry Point)                           |
| ---------------------------------------- | ------------------------------------------------------ | ---------------------------------------------- |
| 設定管理 (Configuration Management)      | `config/`, `config/db/`                                | `config.Default()`, `db.NewDBConfig()`         |
| HTTP 服務 (HTTP Service)                 | `router/`, `mw/`                                       | `HTTPServer()`, `router.Default()`             |
| 程式碼產生 (Code Generation) — stringer  | `service/`, `cmd/stringer/`                            | `cmd/stringer/main.go`, `Generator.Generate()` |
| 程式碼產生 (Code Generation) — gotmpl    | `cmd/gotmpl/`, `cmd/gotmpl/cmd/`, `cmd/gotmpl/tmpl/`  | `cmd/gotmpl/main.go`, `TemplateLoader.Load()`  |
| 編碼與資料處理 (Encoding & Data)         | `encode/io/`, `encode/csv/`, `utils/`, `time/`         | `NewGBKDecoder()`, `ProcessCSVFile()`          |
| 日誌與觀測 (Logging & Observability)     | `log/`                                                 | `log.Init()`, `log.Info()`                     |

## 開發指南 (Development Guide)

### 前置需求 (Prerequisites)

- Go 1.24.5 或以上
- Git（pre-commit hooks 需要）
- CGO 支援（SQLite 驅動 `mattn/go-sqlite3` 需要 CGO enabled）

### 安裝 (Installation)

```bash
# 作為模組依賴引入
go get github.com/bizshuk/gosdk

# 或 clone 後安裝依賴
git clone <repo-url>
go mod download
```

### 建置 (Build)

```bash
go build -o gosdk .
go build -o stringer ./cmd/stringer
go build -o gotmpl ./cmd/gotmpl
```

### 測試 (Test)

```bash
go test ./...
```

目前僅 `utils/type_test.go` 有實際測試案例，`router/statsHandler_test.go` 為空殼（TODO 註解）。

### 部署 (Deploy)

未偵測到部署設定 (No deployment config detected)。

`outputgittag.sh` 可將 Git tag/commit hash 輸出至 `version` 檔案，搭配 `-ldflags` 注入版本號：

```bash
bash outputgittag.sh
go build -ldflags="-X 'github.com/bizshuk/gosdk/config.Version=$(cat version)'"
```

### 環境變數 (Environment Variables)

| 變數 | 用途 | 預設值 |
|------|------|--------|
| `PROFILE` | 設定檔環境切換（`local`/`dev`/`stage`/`prod`） | `local` |
| `CONFIG_DIR` | 設定檔搜尋目錄 | `.` |
| `LOG_LEVEL` | 日誌等級（`debug`/`info`/`warn`/`error`） | `info` |
| `APP_*` | 自動綁定至 Viper 設定（`_` 視為 `.`） | — |

## 慣例 (Conventions)

- Naming: 套件使用小寫單字（`config`, `router`, `mw`, `log`），型別 PascalCase，建構函式以 `New` 前綴（`NewEnvConfig`, `NewDBConfig`）
- Error handling: 設定載入失敗以 `log.Fatal` 終止；資料庫連線失敗回傳 `fmt.Errorf` 包裝的 error；CSV 處理器內部 error 以 `zap.L().Error` 記錄後 `continue`
- Logging: `log/` 模組使用 `zap` + `zap.ReplaceGlobals()`，但 `config/` 子模組仍混用標準庫 `log.Println` 及 `fmt.Println`
- Testing: 使用標準 `testing` 套件 + table-driven tests 模式（`utils/type_test.go`），測試覆蓋率極低
- Code generation: `//go:generate` 註解觸發，產出的 `*_string.go` 檔案已加入 `.gitignore`
- Pre-commit: `pre-commit-golang` 確保提交前執行 `go mod tidy`
