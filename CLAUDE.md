# gosdk — 技術脈絡 (Technical Context)

## 專案結構 (Project Structure)

```tree
gosdk/
├── main.go                  # HTTP 伺服器入口
├── go.mod                   # Go 1.26, module: github.com/bizshuk/gosdk
├── Makefile                 # build / test / generate / run / clean
├── version                  # 版本號檔案（語義版本，如 1.0.1）
├── outputgittag.sh          # 輸出 Git tag 格式的版本號腳本
├── build/
│   └── dockerfile           # Multi-stage Docker 建置（golang:1.26-alpine）
├── cmd/
│   ├── gotmpl/              # Cobra CLI 模板渲染工具
│   │   ├── main.go
│   │   ├── config.yaml
│   │   └── LICENSE
│   ├── stringer/            # 增強版 enum stringer CLI
│   │   └── main.go
│   ├── versioning/          # 語義版本管理 CLI
│   │   ├── main.go
│   │   └── cmd/
│   │       ├── root.go      # Version 結構、ReadVersion()、WriteVersion()
│   │       ├── major.go     # major 子命令
│   │       ├── minor.go     # minor 子命令
│   │       └── patch.go     # patch 子命令
│   └── cobrasample/         # metric/cobra hook 使用範例（可執行 demo）
│       └── main.go
├── config/                  # 設定管理模組
│   ├── config.go            # Config 介面、Default()、GetAppConfigDir()
│   ├── config_test.go       # 基本設定載入測試
│   ├── option.go            # ConfigOption：WithAppName / WithDefaultValue
│   ├── option_test.go       # option 測試
│   ├── env.go               # .env dotenv 載入器（雙檔案模式）
│   ├── env_test.go          # env 載入器測試
│   ├── yaml.go              # YAML 設定載入器（雙檔案模式）
│   ├── yaml_test.go         # yaml 載入器測試
│   ├── json.go              # JSON 設定載入器（雙檔案模式）
│   ├── embedFS.go           # embed.FS 設定載入器
│   └── sample/              # config 套件使用範例 (含 conf/ 設定檔及 SQLite 範例)
├── db/                      # 資料庫連線服務模組(per-storage singleton + flat viper keys)
│   ├── db.go                # Service 介面(DB() / Close())
│   ├── sqlite.go            # SQLite type + DefaultSQLite + InitSQLite(SQLITE_PATH)
│   ├── mysql.go             # MySQL  type + DefaultMySQL  + InitMySQL(MYSQL_DSN)
│   ├── postgres.go          # Postgres type + DefaultPostgres + InitPostgres(POSTGRES_DSN)
│   ├── sqlite_test.go       # SQLite 單元測試(viper 讀取、singleton 守衛、Service 方法)
│   ├── mysql_test.go        # MySQL 單元測試(白箱模擬已初始化、驗證守衛)
│   └── postgres_test.go     # PostgreSQL 單元測試(結構與 MySQL 對稱)
├── encode/                  # 編碼轉換模組
│   ├── csv/
│   │   ├── csv.go           # CSV Decoder 介面
│   │   ├── processor.go     # CSV RecordProcessor 與歸檔邏輯
│   │   └── processor_test.go
│   └── io/
│       ├── decode.go        # DecodeGBKBytes(), DecodeBig5Bytes()
│       ├── decode_test.go   # GBK/Big5 解碼測試
│       ├── gbk.go           # GBK 串流解碼器
│       └── big5.go          # Big5 串流解碼器
├── log/                     # 結構化日誌模組
│   ├── log.go               # slog 全域 logger 初始化（init() 自動初始化 + Init() 套用 LOG_LEVEL/LOG_FORMAT）
│   ├── levelSplitHandler.go # slog.Handler 實作，負責將 Warn/Error 與其餘層級日誌分流
│   ├── log_test.go          # 日誌與日誌等級單元測試
│   └── level.go             # LOG_LEVEL 環境變數解析
├── mw/                      # Gin 中介層
│   ├── correlationId.go     # X-Correlation-Id 請求追蹤
│   └── helmet.go            # 安全性標頭（CSP, X-Frame-Options 等）
├── metric/                  # 指標監控模組（remote write + OTel）
│   ├── metric.go            # 通用 Prometheus remote-write client（MetricService）
│   ├── metric_test.go       # MetricService 單元測試
│   ├── victoriametrics.go   # VictoriaMetrics 便利建構子（現行預設後端）
│   ├── mimir.go             # Mimir 便利建構子（alias → MetricService，MIMIR_URL 預設 :9009/api/v1/push）
│   ├── otel.go              # Go OpenTelemetry metrics/traces 封裝（OTLP HTTP）
│   ├── otel_test.go         # OTel provider 單元測試
│   ├── otel.py              # Python OpenTelemetry metrics 封裝
│   ├── cobra.go             # spf13/cobra hook：每次 CLI 執行送 command_line_trigger
│   ├── cobra_test.go        # cobra hook 單元測試
│   └── model.go             # Metric 資料結構
├── notify/                  # 通用通知模組
│   ├── notifier.go          # Notifier 介面定義
│   ├── multi.go             # Multi 組合通知器
│   ├── stdout.go            # StdoutNotifier 實作
│   ├── slack.go             # SlackNotifier 實作（Go）
│   ├── slack.py             # SlackNotifier 實作（Python）
│   ├── example.py           # 使用範例（Python）
│   ├── notifier_test.go     # 通知器整合測試
│   └── slack_test.go        # Slack 通知器單元測試
├── router/                  # HTTP 路由定義
│   ├── default.go           # /stats 路由註冊
│   ├── statsHandler.go      # Stats JSON 回應
│   ├── statsHandler_test.go # StatsHandler 單元測試
│   ├── health.go            # /healthz 端點 (gin-healthcheck)
│   └── ping.go              # /ping 端點
├── scheduler/               # 排程管理模組
│   ├── scheduler.go         # 排程器核心與啟動邏輯
│   ├── job.go               # 排程任務定義
│   └── scheduler_test.go    # 排程器單元測試
├── service/                 # 核心服務邏輯
│   ├── default.go           # 空 package 佔位
│   └── generator.go         # stringer 核心：AST 解析與程式碼產生
├── time/                    # 時間工具模組
│   ├── roc.go               # 民國曆日期解析
│   ├── roc_test.go          # ROC 日期解析測試
│   ├── sleep.go             # 設定驅動的延遲函式
│   └── sleep_test.go        # ConfigSleep 測試
├── utils/                   # 通用工具函式
│   ├── file.go              # 檔案操作、CSV 批次處理、CreateIfNotExist()
│   ├── file_test.go         # 檔案操作測試
│   ├── string.go            # 隨機字串產生、指標轉換
│   ├── string_test.go       # 字串工具測試
│   ├── int.go               # 整數指標轉換函式
│   ├── int_test.go          # 整數工具測試
│   ├── time.go              # HH:MM:SS 時間解析
│   ├── time_test.go         # 時間解析測試
│   ├── type.go              # IsNil() reflect 檢查
│   ├── type_test.go         # IsNil 測試
│   └── stringer.go          # stringer go:generate 範例
├── .claude-plugin/          # Claude Code plugin manifest
│   └── plugin.json          # plugin metadata（name=gosdk、version 對齊 version 檔）
├── plans/                   # 開發計畫文件
├── skills/                  # Agent skills（9 個：golang-dev、golang-gosdk、golang-mvc、golang-code-quality、golang-dead-code、golang-naming、golang-network、golang-performance-tuning、migrate-zap-to-slog）
├── agents/                  # Agent 定義（golang-refactor.md）
├── docs/                    # 其他文件（superpowers）
├── AGENTS.md                # Agent 入口說明
├── SPEC.md                  # 規格文件
├── .github/
│   └── workflows/
│       └── ci.yml           # GitHub Actions CI（vet, test, build）
├── .env                     # 預設環境變數
└── .gitignore
```

## 技術棧 (Tech Stack)

- Language: Go 1.26
- Framework: `gin-gonic/gin` v1.11.0 (HTTP)
- Build tool: `Makefile` + `go build`
- Key dependencies:
    - `spf13/viper` v1.17.0 — 階層式設定管理
    - `spf13/cobra` v1.9.1 — CLI 框架（gotmpl、versioning）
    - `log/slog` (stdlib) — 結構化日誌（取代 zap）
    - `gorm.io/gorm` v1.31.1 — ORM（MySQL + SQLite + PostgreSQL，driver 各 v1.6.0）
    - `castai/promwrite` v0.6.0 — Prometheus remote-write client（MetricService）
    - `go.opentelemetry.io/otel` v1.44.0 — OpenTelemetry SDK（OTLP HTTP metrics/traces）
    - `slack-go/slack` v0.23.1 — Slack 通知
    - `golang.org/x/tools` v0.44.0 — Go AST 解析（stringer）
    - `golang.org/x/text` v0.37.0 — CJK 編碼轉換
    - `tavsec/gin-healthcheck` v1.2.2 — Health check 端點
    - `hairyhenderson/gomplate` v4.3.3 — 模板渲染函式

## 關鍵決策 (Key Decisions)

- 使用 Viper 全域單例管理設定：所有設定來源（.env、YAML、JSON、環境變數）合併至單一 `viper` 實例，簡化跨模組存取，但犧牲了可測試性
- 雙檔案載入模式：各設定格式固定載入 base 檔案 + `.local` 覆寫檔（`.env` + `.env.local`、`config.yaml` + `config.local.yaml`、`settings.json` + `settings.local.json`），不再依賴 `PROFILE` 環境變數切換
- 扁平 viper key 直讀：`config.Default()` 載入設定後透過 `viper.Get*()` 取值；不再維護強型別 `ConfigSchema` / `ServerConfig` / `DBConfig` 等聚合結構（2026-06 重構後 `config/common` 已廢除）
- 儲存型態採 per-service singleton：每種儲存是一個獨立 service（`db.SQLite` / `db.MySQL` / `db.Postgres`），各自有 `DefaultSQLite` / `DefaultMySQL` / `DefaultPostgres` 全域 singleton 與扁平 viper key（`SQLITE_PATH` / `MYSQL_DSN` / `POSTGRES_DSN`），守護函式 `InitSQLite()` / `InitMySQL()` / `InitPostgres()` 拒絕重複初始化以落實「micro-service: 同型態不可有兩個 instance」；MySQL 與 PostgreSQL 採單一 DSN 字串欄位而非拆 `HOST`/`PORT`/`USER`/`PASSWORD`，簡化設定並與舊 `url` 對齊(PostgreSQL 接受 URL 形式 `postgres://...` 或 keyword/value 形式 `host=... user=...`)
- `stringer` 以 `GeneratorEx` 組合模式擴充標準庫 `stringer`：嵌入 `service.Generator`，額外產生 `List()`、`ValueList()`、`Map()`、`ValueMap()` 四個輔助函式
- 日誌模組在 `init()` 時即以預設值初始化 slog 全域 logger：確保任何 import 此套件的模組都能立即使用套件層級 `slog.*`，`log.Init()` 可在設定載入後再次呼叫以套用 `LOG_LEVEL` / `LOG_FORMAT`。不提供 wrapper 函式，消費端直接使用 stdlib `log/slog`
- CSV 處理使用歸檔標記檔（`.archived`）防止重複處理：以簡單的檔案系統機制取代資料庫或 Redis 的已處理紀錄
- Helmet 中介層採用靜態安全標頭：直接在 response header 注入 `X-Content-Type-Options`、`X-Frame-Options`、`CSP` 等，不依賴外部套件
- 排程器採用極簡設計：`Scheduler` 僅負責按 `Interval` 觸發 `Job.Fn`，錯誤處理、日誌、重試等策略完全由呼叫方透過 `Job.OnError` 自行決定
- `Notifier` 介面保持單一方法（`Notify`）：不綁定特定訊息格式，呼叫方自行序列化 summary 字串，使通知器可輕易替換或組合
- `versioning` CLI 使用純文字 `version` 檔案：避免依賴 git tags 或外部服務，檔案格式為 `major.minor.patch`，可直接納入版本控制
- Cobra hook 採極簡設計（無 option、同步送出）：`CobraCMDHook(root)` 在 PreRun 送出 `command_line_trigger{cmd, flag}`（PreRun 而非 PostRun：永遠會送，即使 RunE 失敗）；`cmd` 為完整指令鏈（`cmd.CommandPath()`，root → leaf）；`flag` 收集使用者實際設定的 flags（走訪整條 chain、`seen` map 去重 persistent flag），字母排序後以 `-` 串接；發送走套件層級 `Send()`（全域 `MetricService`，首次使用時以 `METRIC_URL` 建立；測試以 `viper.Set` 覆寫）

### Remote Write 與 OpenTelemetry 指標發送差異 (Remote Write vs OpenTelemetry Metrics)

使用 `MetricService`（透過 `gosdk/metric`）與使用 `otel`（OpenTelemetry）發送指標的主要差異在於 `系統複雜度` 與 `傳輸協定`。

以下是兩者的詳細對照表：

| 特性 (Feature)            | 使用 Remote Write 發送 (`gosdk/metric`)                          | 使用 OpenTelemetry 發送 (`go.opentelemetry.io/otel`)               |
| :------------------------ | :--------------------------------------------------------------- | :----------------------------------------------------------------- |
| `傳輸協定 (Protocol)`     | Prometheus 遠端寫入 (Prometheus remote-write)                    | OpenTelemetry 協定 (OTLP)                                          |
| `依賴大小 (Dependency)`   | 極度輕量（僅需 HTTP 協定與 Protobuf 定義）                       | 較為龐大（需要完整的 OTel SDK 與相關插件）                         |
| `生命週期 (Lifecycle)`    | 無需特殊管理，隨呼叫發送，不需要 `Shutdown` 釋放資源             | 需要配置 `MeterProvider`、`Exporter` 並於程式結束前進行 `Shutdown` |
| `批次發送 (Batching)`     | 由開發者在程式碼中主動呼叫 `SendMulti` 控制批次邊界              | 由 SDK 的觀測週期 (`PeriodicReader`) 背景自動收集並定期發送        |
| `指標轉換 (Sanitization)` | `gosdk` 自動將指標名稱中的 `.` 轉換為 `_` 以符合 Prometheus 規範 | 開發者必須手動定義符合 OTel 與 Prometheus 相容的指標名稱與屬性     |

後端選擇透過 URL 注入：`MetricService` 的 remote-write 端點由 `METRIC_URL` 控制（預設 VictoriaMetrics `:8428/api/v1/write`），對 VictoriaMetrics（`VICTORIAMETRICS_URL`）、Mimir（`MIMIR_URL`，`:9009/api/v1/push`）等任何 remote-write 相容後端通用；OTLP metrics 路徑由 `OTLP_METRIC_URL` 控制（預設 VictoriaMetrics `:8428/opentelemetry/v1/metrics`）；OTLP traces 路徑由 `OTLP_TRACE_URL` 控制（空字串 = OTLP 預設 `localhost:4318`）。`MimirService` / `NewMimirService()` 保留為 Deprecated 相容層。

## 模組對應 (Module Mapping)

| 業務領域 (Domain)     | 套件/模組 (Package/Module)              | 進入點 (Entry Point)                                       |
| --------------------- | --------------------------------------- | ---------------------------------------------------------- |
| 設定管理              | `config/`                               | `config.Default()`                                         |
| 資料庫連線            | `db/`                                   | `db.InitSQLite()` / `db.InitMySQL()` / `db.InitPostgres()` |
| HTTP 服務             | `router/`, `mw/`, `main.go`             | `HTTPServer()`                                             |
| 程式碼產生 — stringer | `cmd/stringer/`, `service/generator.go` | `cmd/stringer/main.go`                                     |
| 程式碼產生 — gotmpl   | `cmd/gotmpl/`                           | `cmd/gotmpl/main.go`                                       |
| 版本管理              | `cmd/versioning/`                       | `cmd/versioning/main.go`                                   |
| Cobra Hook 範例       | `cmd/cobrasample/`                      | `cmd/cobrasample/main.go`                                  |
| 通用通知              | `notify/`                               | 各通知器獨立建構與呼叫                                     |
| 排程管理              | `scheduler/`                            | `scheduler.New()`                                          |
| 編碼與資料處理        | `encode/`, `utils/`, `time/`            | 各函式獨立呼叫                                             |
| 日誌與觀測            | `log/`                                  | `log.Init()`                                               |
| Remote Write 指標     | `metric/`                               | `NewMetricService()` / `NewVictoriaMetricsService()`       |
| Cobra CLI Hook 指標   | `metric/`                               | `metric.CobraCMDHook()`                                    |
| OTel 指標             | `metric/`                               | `metric.InitMeterProvider()`                               |
| OTel Tracer           | `metric/`                               | `metric.InitTracerProvider()`                              |

## 開發指南 (Development Guide)

### 前置需求 (Prerequisites)

- Go 1.26+ 已安裝
- CGO 支援（SQLite 驅動需要 `gcc`）
- Git

### 安裝 (Installation)

```bash
go mod download
```

### Python 依賴（用於 metric/otel.py, notify/slack.py）

```bash
source /Users/shuk/.venv/bin/activate
# 或使用專案 venv
source .venv/bin/activate

pip install opentelemetry-api opentelemetry-sdk slack-sdk
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
go run main.go
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

- Naming: 套件名稱使用小寫單字（`config`, `log`, `mw`, `router`, `notify`, `scheduler`）；檔案名稱使用 camelCase（`correlationId.go`, `statsHandler.go`）；常數使用 `UPPER_SNAKE_CASE`
- Error handling: 使用 `fmt.Errorf("...: %w", err)` 進行 error wrapping；設定載入失敗區分 `ConfigFileNotFoundError`（允許 fallback）與其他錯誤（`log.Warn` 或 `log.Fatal` 終止）
- Logging: `gosdk/log` 在 `init()` 自動初始化、並提供 `Init()` 套用 `LOG_LEVEL` / `LOG_FORMAT`；所有日誌記錄統一使用 stdlib 套件層級 `slog.*`（結構化 key/value），不使用 wrapper 函式
- Testing: 測試檔案與被測檔案放在同一 package 內（白盒測試）；使用 `testing.T` 標準庫；測試前透過 `viper.Set()` 或 `os.Setenv()` 注入設定
- Configuration: 設定檔搜尋路徑固定為 `.`、`./conf`、`~/.config/<appName>`（需 `WithAppName`）；雙檔案模式（base + `.local`）自動載入；`APP_` 前綴環境變數自動覆蓋設定
