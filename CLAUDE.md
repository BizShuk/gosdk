# gosdk — 技術脈絡 (Technical Context)

## 專案結構 (Project Structure)

```tree
gosdk/
├── main.go                  # HTTP 伺服器入口
├── go.mod                   # Go 1.24.5, module: github.com/bizshuk/gosdk
├── Makefile                 # build / test / generate / run / clean
├── build/
│   └── dockerfile           # Multi-stage Docker 建置（golang:1.24-alpine）
├── cmd/
│   ├── gotmpl/              # Cobra CLI 模板渲染工具
│   │   ├── main.go
│   │   ├── config.yaml
│   │   └── LICENSE
│   └── stringer/            # 增強版 enum stringer CLI
│       └── main.go
├── config/                  # 設定管理模組
│   ├── config.go            # Config 介面、Default()
│   ├── config_test.go       # 基本設定載入測試
│   ├── env.go               # .env dotenv 載入器
│   ├── yaml.go              # YAML 設定載入器
│   ├── json.go              # JSON 設定載入器
│   ├── embedFS.go           # embed.FS 設定載入器
│   └── db/                  # 資料庫連線工廠
│       ├── db.go            # DBConfig, DatabaseFactory()
│       ├── mysql.go         # MySQL GORM 驅動
│       └── sqlite.go        # SQLite GORM 驅動
├── encode/                  # 編碼轉換模組
│   ├── csv/
│   │   ├── csv.go           # CSV Decoder 介面
│   │   └── processor.go     # CSV RecordProcessor 與歸檔邏輯
│   └── io/
│       ├── decode.go        # DecodeGBKBytes(), DecodeBig5Bytes()
│       ├── decode_test.go   # GBK/Big5 解碼測試
│       ├── gbk.go           # GBK 串流解碼器
│       └── big5.go          # Big5 串流解碼器
├── log/                     # 結構化日誌模組
│   ├── log.go               # zap Logger 初始化與封裝
│   └── level.go             # LOG_LEVEL 環境變數解析
├── mw/                      # Gin 中介層
│   ├── correlationId.go     # X-Correlation-Id 請求追蹤
│   └── helmet.go            # 安全性標頭（CSP, X-Frame-Options 等）
├── router/                  # HTTP 路由定義
│   ├── default.go           # /stats 路由註冊
│   ├── statsHandler.go      # Stats JSON 回應
│   ├── statsHandler_test.go # StatsHandler 單元測試
│   ├── health.go            # /healthz 端點 (gin-healthcheck)
│   └── ping.go              # /ping 端點
├── service/                 # 核心服務邏輯
│   ├── default.go           # 空 package 佔位
│   └── generator.go         # stringer 核心：AST 解析與程式碼產生
├── time/                    # 時間工具模組
│   ├── roc.go               # 民國曆日期解析
│   ├── roc_test.go          # ROC 日期解析測試
│   ├── sleep.go             # 設定驅動的延遲函式
│   └── sleep_test.go        # ConfigSleep 測試
├── utils/                   # 通用工具函式
│   ├── file.go              # 檔案操作、CSV 批次處理
│   ├── string.go            # 隨機字串產生、指標轉換
│   ├── int.go               # 整數指標轉換函式
│   ├── time.go              # HH:MM:SS 時間解析
│   ├── type.go              # IsNil() reflect 檢查
│   └── type_test.go         # IsNil 測試
├── sample/                  # 使用範例
│   ├── stringer.go          # stringer go:generate 範例
│   └── config/
│       └── main.go          # config 套件完整使用範例
├── .github/
│   └── workflows/
│       └── ci.yml           # GitHub Actions CI（vet, test, build）
├── .env                     # 預設環境變數
├── .gitignore
└── version                  # 版本號檔案
```

## 技術棧 (Tech Stack)

- Language: Go 1.24.5
- Framework: `gin-gonic/gin` v1.11.0 (HTTP)
- Build tool: `Makefile` + `go build`
- Key dependencies:
    - `spf13/viper` v1.17.0 — 階層式設定管理
    - `spf13/cobra` v1.9.1 — CLI 框架（gotmpl）
    - `go.uber.org/zap` v1.27.1 — 結構化日誌
    - `gorm.io/gorm` v1.31.1 — ORM（MySQL + SQLite）
    - `golang.org/x/tools` v0.39.0 — Go AST 解析（stringer）
    - `golang.org/x/text` v0.32.0 — CJK 編碼轉換
    - `tavsec/gin-healthcheck` v1.2.2 — Health check 端點
    - `hairyhenderson/gomplate` v4.3.3 — 模板渲染函式

## 關鍵決策 (Key Decisions)

- 使用 Viper 全域單例管理設定：所有設定來源（.env、YAML、環境變數）合併至單一 `viper` 實例，簡化跨模組存取，但犧牲了可測試性
- 強型別 `ConfigSchema` 結合 flat key 存取：`config.Default()` 載入設定後透過 `viper.Unmarshal()` 或直接 `viper.Get*()` 取值
- `stringer` 以 `GeneratorEx` 組合模式擴充標準庫 `stringer`：嵌入 `service.Generator`，額外產生 `List()`、`ValueList()`、`Map()`、`ValueMap()` 四個輔助函式
- 日誌模組在 `init()` 時即初始化：確保任何 import 此套件的模組都能立即使用 `log.Info()` 等函式，`log.Init()` 可在設定載入後再次呼叫以更新等級
- 雙檔案載入模式：各設定格式固定載入 base 檔案 + `.local` 覆寫檔（`.env` + `.env.local`、`config.yaml` + `config.local.yaml`、`settings.json` + `settings.local.json`），不再依賴 `PROFILE` 環境變數切換
- CSV 處理使用歸檔標記檔（`.archived`）防止重複處理：以簡單的檔案系統機制取代資料庫或 Redis 的已處理紀錄
- Helmet 中介層採用靜態安全標頭：直接在 response header 注入 `X-Content-Type-Options`、`X-Frame-Options`、`CSP` 等，不依賴外部套件

## 模組對應 (Module Mapping)

| 業務領域 (Domain)     | 套件/模組 (Package/Module)              | 進入點 (Entry Point)   |
| --------------------- | --------------------------------------- | ---------------------- |
| 設定管理              | `config/`, `config/db/`                 | `config.Default()`     |
| HTTP 服務             | `router/`, `mw/`, `main.go`             | `HTTPServer()`         |
| 程式碼產生 — stringer | `cmd/stringer/`, `service/generator.go` | `cmd/stringer/main.go` |
| 程式碼產生 — gotmpl   | `cmd/gotmpl/`                           | `cmd/gotmpl/main.go`   |
| 編碼與資料處理        | `encode/`, `utils/`, `time/`            | 各函式獨立呼叫         |
| 日誌與觀測            | `log/`                                  | `log.Init()`           |

## 開發指南 (Development Guide)

### 前置需求 (Prerequisites)

- Go 1.24+ 已安裝
- CGO 支援（SQLite 驅動需要 `gcc`）
- Git

### 安裝 (Installation)

```bash
go mod download
```

### 建置 (Build)

```bash
make build          # 編譯為 bin/server
# 或
go build -o bin/server main.go
```

### 測試 (Test)

```bash
make test           # 等同 go test -v ./...
# 或
go test -v ./...
```

### 程式碼產生 (Code Generation)

```bash
make generate       # 等同 go generate ./...
```

### 執行 (Run)

```bash
make run            # build + 執行 bin/server
# 或
PROFILE=local go run main.go
```

### Docker 建置 (Docker Build)

```bash
docker build -f build/dockerfile -t gosdk .
docker run -p 8080:8080 gosdk
```

### CI/CD

GitHub Actions workflow 定義於 `.github/workflows/ci.yml`，於 push/PR 至 `main`/`master` 時自動執行：

1. `go mod download`
2. `go vet ./...`
3. `go test -v ./...`
4. `go build -v ./...`

### 部署 (Deploy)

透過 Dockerfile multi-stage build 產生最小化 Alpine 映像，暴露 port 8080。

## 慣例 (Conventions)

- Naming: 套件名稱使用小寫單字（`config`, `log`, `mw`, `router`）；檔案名稱使用 camelCase（`correlationId.go`, `statsHandler.go`）；常數使用 `UPPER_SNAKE_CASE`
- Error handling: 使用 `fmt.Errorf("...: %w", err)` 進行 error wrapping；設定載入失敗區分 `ConfigFileNotFoundError`（允許 fallback）與其他錯誤（`log.Fatal` 終止）
- Logging: 統一使用 `gosdk/log` 封裝的 `zap` 函式（`Info`, `Infof`, `Error`, `Errorf` 等）；部分模組直接使用 `zap.L()` 全域實例
- Testing: 測試檔案與被測檔案放在同一 package 內（白盒測試）；使用 `testing.T` 標準庫；測試前透過 `viper.Set()` 或 `os.Setenv()` 注入設定
- Configuration: `CONFIG_DIR` 控制設定檔搜尋路徑；雙檔案模式（base + `.local`）自動載入；`APP_` 前綴環境變數自動覆蓋設定
