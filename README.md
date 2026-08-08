# gosdk

Go 語言通用開發工具包 (Shared SDK)，提供設定管理、HTTP 服務骨架、程式碼產生器、版本管理、通用通知、編碼轉換等可重用模組，作為 Go 專案的基礎函式庫使用。

本 repo 同時是 `Claude Code plugin`：除了作為 Go 函式庫被 `import`，也可安裝為 Claude Code plugin 取得 9 個 Go 專用 skills 與 `golang-refactor` agent。安裝方式見最下方 [Claude Code Plugin](#claude-code-plugin-安裝).

## 業務領域 (Business Domains)

### 設定管理 (Configuration Management)

統一管理應用程式設定來源，支援 `.env`、YAML、JSON（讀取接受 JSONC：註解與 trailing commas；檔名仍為 `.json`）及 `embed.FS` 四種格式，透過 Viper 實現階層式設定合併。各格式採用雙檔案載入模式：固定讀取 base 檔案（`.env`、`config.yaml`、`settings.json`），再合併同名 `.local` 覆寫檔（`.env.local`、`config.local.yaml`、`settings.local.json`），不再依賴 `PROFILE` 環境變數。同時支援 `APP_` 前綴環境變數自動綁定。DB 連線另由獨立的 `db` 套件負責（見下一節），不在 `config/` 範圍。寫回 `settings.json` 時仍輸出標準 JSON（註解不會被保留）。

`領域流程 (Domain Flow):`

1. 呼叫 `config.Default()`（可加 `WithAppName("myapp")` 等 option）啟動設定載入；搜尋目錄固定為 `.`、`./conf`、`~/.config/<appName>`；app 目錄可用 `WithConfigDir("~/.config/myapp")` 強制指定，`XDG_CONFIG_HOME` 因此不再左右設定位置
2. `EnvConfig.Load()` 讀取 `.env` → `.env.local` 並合併至全域 Viper
3. `YamlConfig.Load()` 讀取 `config.yaml` → `config.local.yaml` 並合併至全域 Viper
4. `JsonConfig.Load()` 讀取 `settings.json` → `settings.local.json` 並合併至全域 Viper
5. 啟用 `APP_` 前綴環境變數自動綁定（`viper.AutomaticEnv()`）
6. 下游模組透過 `viper.GetString()` 或 `viper.Unmarshal()` 取得設定值

`核心實體 (Key Entities):` `Config` 介面, `EnvConfig`, `YamlConfig`, `JsonConfig`, `FSConfig`

`相關處理器 (Related Handlers):` `config.Default()`, `GetAppConfigDir()`, `WithAppName()`, `WithConfigDir()`, `WithDefaultValue()`, `SetConfigDir()`, `NewEnvConfig()`, `NewYamlConfig()`, `NewJsonConfig()`, `NewFSConfig()`

---

### 資料庫連線 (Database Services)

每種儲存型態是一個獨立的 service:有自己的型別、自己的全域 singleton、自己的扁平 viper key (例如 `SQLITE_PATH`、`MYSQL_DSN`、`POSTGRES_DSN`)。micro-service 概念下,一個 process 內不應該存在兩個同型態的 service;`InitSQLite()` / `InitMySQL()` / `InitPostgres()` 在第二次呼叫時會回傳 error,守護 singleton 不變性。

`領域流程 (Domain Flow):`

1. `config.Default()` 載入設定後,呼叫端用 `viper.IsSet("SQLITE_PATH")` / `viper.IsSet("MYSQL_DSN")` / `viper.IsSet("POSTGRES_DSN")` 判斷是否啟用該儲存
2. 呼叫 `db.InitSQLite()` / `db.InitMySQL()` / `db.InitPostgres()` 從 viper 讀取設定、開啟連線、設為 singleton
3. 任何地方透過 `db.DefaultSQLite.DB()` / `db.DefaultMySQL.DB()` / `db.DefaultPostgres.DB()` 取得 `*gorm.DB`
4. 結束時呼叫 `db.DefaultSQLite.Close()` / `db.DefaultMySQL.Close()` / `db.DefaultPostgres.Close()` 釋放連線

`核心實體 (Key Entities):` `Service` 介面, `SQLite` struct, `MySQL` struct, `Postgres` struct, `DefaultSQLite` / `DefaultMySQL` / `DefaultPostgres` singleton

`相關處理器 (Related Handlers):` `db.InitSQLite()`, `db.InitMySQL()`, `db.InitPostgres()`, `db.DefaultSQLite.DB()`, `db.DefaultSQLite.Close()`, `db.DefaultMySQL.DB()`, `db.DefaultMySQL.Close()`, `db.DefaultPostgres.DB()`, `db.DefaultPostgres.Close()`

---

### HTTP 服務 (HTTP Service)

基於 Gin 框架提供 HTTP 服務骨架，包含預設路由註冊、health check 端點、ping/pong 端點、stats 資訊端點，以及 Correlation ID 請求追蹤與 Helmet 安全性標頭中介層。設計為可嵌入其他 Go 專案的服務基礎模組。

`領域流程 (Domain Flow):`

1. `main()` 依序執行：`config.Default()` → `log.Init()` → 資料庫連線 → `HTTPServer()`
2. `HTTPServer()` 建立 `gin.Default()` 引擎，掛載 `mw.CorrelationID()` 與 `mw.Helmet()` 中介層
3. `router.Default(s)` 註冊 `/stats` 路由，回傳版本、Profile、設定檔路徑及 Correlation ID
4. `router.HealthRouterGroup(s)` 透過 `gin-healthcheck` 套件提供 `/healthz` 端點
5. `router.PingRouterGroup(s)` 提供 `/ping/` 端點，回應 `{"message": "pong"}`
6. 伺服器監聽 `server.host:server.port` 設定值（預設 `:8080`）

`核心實體 (Key Entities):` `Stats` 結構, `CorrelationHeader` 常數

`相關處理器 (Related Handlers):` `HTTPServer()`, `router.Default()`, `StatsHandler()`, `HealthRouterGroup()`, `PingRouterGroup()`, `mw.CorrelationID()`, `mw.GetCorrelationID()`, `mw.Helmet()`

---

### 程式碼產生 (Code Generation)

提供兩個 CLI 工具：`stringer` 為增強版常數列舉產生器，從 Go AST 解析常數定義並自動產生 `String()`、`List()`、`ValueList()`、`Map()`、`ValueMap()` 五個方法；`gotmpl` 為模板渲染引擎，透過 Cobra CLI 讀取 YAML 設定，搭配 gomplate 函式庫渲染嵌入的 Go 模板。

`領域流程 (Domain Flow) — stringer:`

1. CLI 解析 `-type`、`-output`、`-trimprefix`、`-linecomment`、`-tags` 等參數
2. `Generator.ParsePackage()` 使用 `golang.org/x/tools/go/packages` 載入目標套件語法樹
3. `File.GenDecl()` 遍歷 AST 常數宣告，收集符合目標型別的 `Value` 清單
4. `Generator.Generate()` 根據連續序列數量選擇 `buildOneRun` / `buildMultipleRuns` / `buildMap` 策略產生 `String()` 方法
5. `GeneratorEx.generate()` 額外呼叫 `buildListFn` / `buildValueListFn` / `buildMapFn` / `buildValueMapFn` 產生四個輔助函式
6. `Generator.Format()` 格式化並輸出至指定檔案

`領域流程 (Domain Flow) — gotmpl:`

1. Cobra root command 解析 `--config` 參數，透過 Viper 讀取 YAML 設定
2. 設定反序列化為 `TemplateLoader` 結構
3. `TemplateLoader.Load()` 從 `embed.FS` 載入 `.tmpl` 模板
4. 使用 `gomplate.CreateFuncs` 提供進階模板函式進行渲染，輸出至 `stdout`

`核心實體 (Key Entities):` `Generator`, `GeneratorEx`, `File`, `Package`, `Value`, `TemplateLoader`

`相關處理器 (Related Handlers):` `Generator.ParsePackage()`, `Generator.Generate()`, `GeneratorEx.generate()`, `SplitIntoRuns()`, `File.GenDecl()`, `TemplateLoader.Load()`

---

### 版本管理 (Version Management)

提供 `versioning` CLI 工具，用於管理專案的語義版本號（Semantic Versioning）。從 `version` 檔案讀取與寫入版本資訊，支援 `major`、`minor`、`patch` 三個子命令分別遞增對應的版本號碼。設計為 CI/CD 流程中的版本自動化工具。

`領域流程 (Domain Flow):`

1. CLI 執行 `versioning` root command 或子命令（`major` / `minor` / `patch`）
2. `ReadVersion()` 從 `version` 檔案讀取目前版本；若檔案不存在則回傳零值
3. 根據子命令遞增對應的版本欄位（`Major++` / `Minor++` / `Patch++`）
4. `WriteVersion()` 將新版本號寫回 `version` 檔案，並輸出至 stdout

`核心實體 (Key Entities):` `Version` 結構, `versionFile` 常數

`相關處理器 (Related Handlers):` `Execute()`, `ReadVersion()`, `WriteVersion()`, `ParseVersion()`, `Version.String()`

---

### 通用通知 (Notification)

提供可組合的通知器架構，透過統一的 `Notifier` 介面傳送訊息摘要。支援 Slack 頻道通知（`SlackNotifier`）與標準輸出（`StdoutNotifier`），並透過 `Multi` 組合器將多個通知器串聯。Slack client 與 channel ID 為可選配置，未設定時靜默略過。

`領域流程 (Domain Flow):`

1. 建立各通知器實例（`NewSlackNotifier(token, channelID)` / `&StdoutNotifier{}`）
2. 透過 `NewMulti(notifiers...)` 組合多個通知器（可選）
3. 呼叫 `Notifier.Notify(ctx, summary)` 傳遞訊息摘要
4. `SlackNotifier` 使用 `slack.Client.PostMessageContext()` 發送至指定頻道；若未配置則記錄警告並略過
5. `Multi.Notify()` 串列呼叫所有子通知器，使用 `errors.Join()` 彙整多個錯誤

`核心實體 (Key Entities):` `Notifier` 介面, `SlackNotifier`, `StdoutNotifier`, `Multi`

`相關處理器 (Related Handlers):` `NewSlackNotifier()`, `NewMulti()`, `Notifier.Notify()`

---

### 排程管理 (Scheduler)

提供最小化的週期性任務排程器，不依賴外部框架。以固定間隔執行已註冊的 `Job`，每個 Job 在獨立的 goroutine 中執行，並透過 `context` 取消信號統一停止所有任務。設計上刻意保持極簡，將日誌、錯誤處理等策略完全交由呼叫方決定。

`領域流程 (Domain Flow):`

1. `scheduler.New()` 建立空的排程器實例
2. `Scheduler.Add(job)` 鏈式呼叫註冊多個 `Job`（含 `Name`、`Interval`、`Fn`、`OnError`）
3. `Scheduler.Start(ctx)` 為每個 Job 啟動獨立 goroutine，並以 `time.Ticker` 驅動定期執行
4. 每次 tick 呼叫 `job.Fn(ctx)`；若回傳 error 且 `job.OnError` 非 nil，呼叫錯誤處理函式
5. `ctx` 取消後所有 goroutine 優雅退出，`Start()` 等待所有 Job 完成後回傳

`核心實體 (Key Entities):` `Scheduler`, `Job`

`相關處理器 (Related Handlers):` `scheduler.New()`, `Scheduler.Add()`, `Scheduler.Start()`

---

### 編碼與資料處理 (Encoding & Data Processing)

提供 CJK 字元編碼轉換（GBK ↔ UTF-8、Big5 ↔ UTF-8）、CSV 檔案批次處理與歸檔機制、檔案操作工具、民國曆 (ROC calendar) 日期解析，以及各種型別輔助函式（指標轉換、nil 檢查、隨機字串、時間解析等）。設計為可獨立引用的工具模組。

`領域流程 (Domain Flow) — CSV 處理:`

1. `NewCSVFilelistCallback()` 使用 Glob pattern 取得檔案清單
2. `ProcessCSVFile()` 逐檔開啟 CSV，跳過 header，逐行呼叫 `RecordProcessor` 回呼
3. 若 `archive` 參數為 `true`，處理完成後建立 `.archived` 標記檔避免重複處理

`領域流程 (Domain Flow) — 編碼轉換:`

1. 建立 `Decoder` 實例（`NewGBKDecoder` / `NewBig5Decoder`）
2. 呼叫 `Decode()` 回傳以 `transform.NewReader` 包裝的 UTF-8 串流
3. 或直接使用 `DecodeGBKBytes()` / `DecodeBig5Bytes()` 轉換位元組

`核心實體 (Key Entities):` `Decoder` 介面 (io), `Decoder` 介面 (csv), `RecordProcessor`, `FileCallback`

`相關處理器 (Related Handlers):` `NewGBKDecoder()`, `NewBig5Decoder()`, `DecodeGBKBytes()`, `DecodeBig5Bytes()`, `ProcessCSVFile()`, `NewCSVFilelistCallback()`, `SaveFile()`, `WriteCSV()`, `CreateIfNotExist()`, `ParseROCDate()`, `ParseTimeDuration()`, `ConfigSleep()`, `IsNil()`, `StringPointer()`, `IntPointer()`

---

### 日誌與觀測 (Logging & Observability)

基於 `zap` 的結構化日誌系統，根據環境自動切換 development（帶 stacktrace）或 production（JSON 格式）模式，支援 `LOG_LEVEL` 環境變數動態調整日誌等級。透過 `init()` 自動初始化全域 Logger 並替換 `zap.ReplaceGlobals()`。

`領域流程 (Domain Flow):`

1. `log.init()` → `log.Init()` 於套件載入時自動執行
2. 根據 `PROFILE` 環境變數選擇 `zap.NewDevelopmentConfig()` 或 `zap.NewProductionConfig()`
3. `GetLogLevel()` 從 `LOG_LEVEL` 環境變數解析日誌等級（`debug`/`info`/`warn`/`error`）
4. 設定 `timestamp` 時間格式為 `time.DateTime`
5. 建立 Logger 並透過 `zap.ReplaceGlobals()` 設定為全域實例
6. 下游模組透過 `zap.L()`（結構化）或 `zap.S()`（sugar）直接輸出日誌

`核心實體 (Key Entities):` `zap.Logger` (全域), `zapcore.Level`

`相關處理器 (Related Handlers):` `log.Init()`, `log.GetLogLevel()`

---

### 指標監控 (Metrics & Tracing)

提供兩種指標發送方式：(A) 透過 `MetricService` 以 Prometheus remote-write 協定直接推送至 VictoriaMetrics、Mimir 等後端；(B) 透過 OpenTelemetry SDK（`InitMeterProvider` + `InitTracerProvider`）以 OTLP HTTP 協定發送 metrics 與 traces。兩者後端 URL 皆由 Viper 設定注入。

`環境變數 (Config Keys):`

| 變數 | 預設值 | 用途 |
| --- | --- | --- |
| `METRIC_URL` | `http://localhost:8428/api/v1/write` | Remote write — 通用後端 |
| `VICTORIAMETRICS_URL` | `http://localhost:8428/api/v1/write` | Remote write — VictoriaMetrics |
| `MIMIR_URL` | `http://localhost:9009/api/v1/push` | Remote write — Mimir (compat) |
| `OTLP_METRIC_URL` | `http://localhost:8428/opentelemetry/v1/metrics` | OTLP metrics 端點 |
| `OTLP_TRACE_URL` | `""` (空 = OTLP 預設 `localhost:4318`) | OTLP traces 端點 |

`領域流程 (Domain Flow) — Remote Write:`

1. 呼叫 `NewMetricService(url)`（空字串 = 讀 `METRIC_URL`）/ `NewVictoriaMetricsService()` / `NewMimirService()` 建立服務
2. 呼叫 `svc.Send(metric)` 或 `svc.SendMulti(metrics)` 批次推送
3. `MetricService` 自動將 metric name 中 `.` 轉換為 `_`（Prometheus 規範）
4. Timestamp 使用 epoch seconds (`time.Now().Unix()`)

`領域流程 (Domain Flow) — OpenTelemetry:`

1. `metric.InitMeterProvider(ctx)` — 初始化全域 `MeterProvider`，讀取 `OTLP_METRIC_URL`
2. `metric.InitTracerProvider(ctx)` — 初始化全域 `TracerProvider`，讀取 `OTLP_TRACE_URL`
3. `metric.Meter(name)` / `metric.Tracer(name)` 取得 meter/tracer 實例
4. 應用程式結束前呼叫 `metric.ShutdownOTel(ctx)` 排空緩衝資料

`領域流程 (Domain Flow) — Cobra CLI Hook:`

1. `metric.CobraCMDHook(root)` — 在 root command 掛上 PersistentPreRunE hook
2. 每次 CLI 執行送出 `command_line_trigger{cmd="root sub leaf", flag="a-b-c"}`（flag 為使用者實際設定的 flags，字母排序以 `-` 串接）
3. 發送走套件層級 `Send()`（全域 `MetricService`），後端由 `METRIC_URL` 控制

`核心實體 (Key Entities):` `MetricService`, `MimirService` (alias), `Metric`, `IMetric`

`相關處理器 (Related Handlers):` `NewMetricService()`, `NewVictoriaMetricsService()`, `NewMimirService()`, `MetricService.Send()`, `MetricService.SendMulti()`, `Send[T]()`, `CobraCMDHook()`, `InitMeterProvider()`, `InitTracerProvider()`, `ShutdownOTel()`, `Meter()`, `Tracer()`

---

## 領域關聯 (Domain Relationships)

- `設定管理` 為所有領域的基礎依賴：`HTTP 服務`、`日誌`、`排程管理`、`編碼與資料處理` 皆透過 Viper 全域實例取得設定值
- `HTTP 服務` 依賴 `日誌與觀測` 提供請求日誌，依賴 `設定管理` 取得 host/port、版本號等資訊
- `通用通知` 可被 `排程管理` 組合使用：定期任務完成後透過 `Notifier.Notify()` 推送摘要
- `程式碼產生` 為獨立工具領域，不依賴其他執行時期領域，但產出的程式碼（`*_string.go`）可能被其他領域使用
- `版本管理` 為獨立 CLI 工具領域，不依賴其他執行時期領域，操作純文字版本檔
- `編碼與資料處理` 為獨立工具領域，提供底層工具函式給其他領域使用（如 CSV 處理、檔案操作、型別轉換）
- `日誌與觀測` 被所有需要記錄的領域引用，提供統一的結構化日誌出口

## 使用方式 (Usage)

### 設定管理

```go
import "github.com/bizshuk/gosdk/config"

// 方式 1：載入預設設定（搜尋 .、./conf、~/.config/<appName>）
config.Default()

// 方式 2：以 option 設定 app 名稱（載入 ~/.config/<app_name>/）
config.Default(config.WithAppName("myapp"))

// 方式 3：以 option 設定 app 名稱與預設值（載入 ~/.config/<app_name>/，
// settings.json 不存在時以 defaultJSON 建立）
config.Default(
    config.WithAppName("myapp"),
    config.WithDefaultValue(defaultJSON),
)

// 方式 4：強制指定設定目錄（"~" 會展開為家目錄）。
// app 名稱不變，只有目錄被釘住 —— 別的工具設了 XDG_CONFIG_HOME
// 也不會讓這個應用程式的設定跟著搬家。
// data/ 與 logs/（GetAppDataDir / GetAppLogsDir）一併跟著移動。
config.Default(
    config.WithAppName("myapp"),
    config.WithConfigDir("~/.config/myapp"),
)
```

### 資料庫連線

```go
import "github.com/bizshuk/gosdk/config"
import "github.com/bizshuk/gosdk/db"
import "github.com/spf13/viper"

config.Default()

// 僅在對應 viper key 有設定時才初始化
if viper.IsSet("SQLITE_PATH") {
    if err := db.InitSQLite(); err != nil { /* 處理錯誤 */ }
}
if viper.IsSet("MYSQL_DSN") {
    if err := db.InitMySQL(); err != nil { /* 處理錯誤 */ }
}
if viper.IsSet("POSTGRES_DSN") {
    if err := db.InitPostgres(); err != nil { /* 處理錯誤 */ }
}

// 之後任何地方透過 singleton 取用 *gorm.DB
gormDB := db.DefaultSQLite.DB()
defer db.DefaultSQLite.Close()
```

### HTTP 服務

```go
import "github.com/bizshuk/gosdk/router"
import "github.com/bizshuk/gosdk/mw"

s := gin.Default()
s.Use(mw.CorrelationID())             // 啟用請求追蹤
s.Use(mw.Helmet())                    // 啟用安全性標頭
router.Default(s)                      // 註冊預設路由
router.HealthRouterGroup(s)            // 註冊 health check
router.PingRouterGroup(s)              // 註冊 ping
s.Run("localhost:8080")
```

### 程式碼產生

```bash
# stringer：測試 flag parsing、package parsing、程式碼產生與檔案寫入
go test ./cmd/sample/stringer -run TestRunGeneratesStringerCode -v

# gotmpl：測試 YAML 設定載入與 Go template 渲染
go test ./cmd/sample/gotmpl -run TestRun -v
```

### 內建子命令 (Built-in Subcommands)

`gosdk/cmd` 是一組現成的 cobra 子命令，由宿主應用自行掛上 root command：

```go
import (
    "github.com/bizshuk/gosdk/cmd"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{Use: "myapp"}

func init() {
    rootCmd.AddCommand(
        cmd.ConfigCmd,                            // 檢視/修改設定
        cmd.MajorCmd, cmd.MinorCmd, cmd.PatchCmd, // 操作 VERSION 檔
    )
}
```

`config` 子命令：

```bash
myapp config                              # 顯示合併後的設定
myapp config --source                     # 附帶來源層（env / yaml / json）
myapp config --update server.host=0.0.0.0 # 寫入 settings.local.json
myapp config --delete server.host
```

key 使用 viper 的點號路徑（`a.b.c` 代表巢狀三層）；`_` 是一般字元，`a_b_c` 是另一個扁平 key。

`major` / `minor` / `patch` 子命令操作工作目錄下的純文字 `VERSION` 檔（格式 `major.minor.patch`），也可直接呼叫 `cmd.ReadVersion()` / `cmd.WriteVersion()`。

### 通用通知

```go
import "github.com/bizshuk/gosdk/notify"

// Slack 通知
slackN := notify.NewSlackNotifier("xoxb-token", "C1234567890")

// 標準輸出
stdN := &notify.StdoutNotifier{}

// 組合多個通知器
multi := notify.NewMulti(slackN, stdN)
multi.Notify(ctx, "任務完成摘要")
```

### 排程管理

```go
import "github.com/bizshuk/gosdk/scheduler"
import "go.uber.org/zap"

s := scheduler.New()
s.Add(scheduler.Job{
    Name:     "daily-report",
    Interval: 24 * time.Hour,
    Fn: func(ctx context.Context) error {
        // 業務邏輯
        return nil
    },
    OnError: func(name string, err error) {
        zap.S().Errorf("job %s failed: %v", name, err)
    },
})
s.Start(ctx)
```

### 編碼與資料處理

```go
import roctime "github.com/bizshuk/gosdk/time"
import "github.com/bizshuk/gosdk/utils"
import encode "github.com/bizshuk/gosdk/encode/io"

t := roctime.ParseROCDate("100/08/07")              // 2011-08-07
utils.NewCSVFilelistCallback("data/*.csv", myProcessor)
utf8, _ := encode.DecodeGBKBytes(gbkData)            // GBK → UTF-8
utils.CreateIfNotExist("~/config.yaml", defaultYAML) // 若不存在則建立預設值
```

### 指標監控

```go
import "github.com/bizshuk/gosdk/metric"

// Remote write：推送單筆或批次指標（後端由 METRIC_URL 控制）
svc := metric.NewMetricService("")
svc.Send(metric.Metric{
    Name:      "app.operation.duration", // "." 自動轉為 "_"
    Timestamp: time.Now().Unix(),        // epoch seconds
    Value:     15.4,
    Tags:      map[string]string{"env": "prod"},
})

// Cobra CLI hook：每次指令執行送出 command_line_trigger{cmd, flag}
metric.CobraCMDHook(rootCmd)
```

完整可執行範例見 `cmd/sample/`。

## Claude Code Plugin 安裝

本 repo 已附 `.claude-plugin/plugin.json`，可直接作為 Claude Code plugin 載入，提供下列元件：

- `Skills (9)`：`golang-dev`、`golang-gosdk`、`golang-mvc`、`golang-code-quality`、`golang-dead-code`、`golang-naming`、`golang-network`、`golang-performance-tuning`、`golang-gosdk-migrate`
- `Agents (1)`：`golang-refactor`

`安裝方式 (Installation):`

```bash
# 方式 1：本地開發載入（指向 clone 出來的目錄）
claude --plugin-dir /path/to/gosdk

# 方式 2：透過 git source 從遠端安裝（需先建立 marketplace 或直接以 plugin URL）
claude --plugin-url https://github.com/bizshuk/gosdk

# 安裝後 skill 透過 plugin 名稱 namespace 觸發
/gosdk:golang-dev
/gosdk:golang-gosdk-migrate
```

---

## 改善建議 (Improvement Suggestions)

Based on codebase analysis:

- [ ] 排程器與通知器整合範例：`scheduler` 與 `notify` 兩個套件設計上可以搭配使用（週期任務完成後推送通知），但目前缺少官方範例或整合測試，建議在 `sample/` 目錄補充完整使用範例
- [ ] `versioning` CLI 缺少 reset 與 set 子命令：目前只能遞增版本號，若需要直接設定特定版本（如 release hotfix 時跳號）無法支援，建議新增 `set <version>` 子命令
- [ ] `notify.SlackNotifier` 缺少重試機制：Slack API 呼叫若因網路問題失敗，目前直接回傳 error，建議加入指數退避重試或將重試邏輯委托給呼叫方約定
- [ ] `db` 套件目前支援 SQLite、MySQL、PostgreSQL,若需要 Redis 等 key-value 儲存或 MongoDB 等文件資料庫,因 GORM 慣例不適用,需另開套件(`cache/` / `document/`)並定義新 service 介面
