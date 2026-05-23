# gosdk

Go 語言通用開發工具包 (Shared SDK)，提供設定管理、HTTP 服務骨架、程式碼產生器、版本管理、通用通知、編碼轉換等可重用模組，作為 Go 專案的基礎函式庫使用。

## 業務領域 (Business Domains)

### 設定管理 (Configuration Management)

統一管理應用程式設定來源，支援 `.env`、YAML、JSON 及 `embed.FS` 四種格式，透過 Viper 實現階層式設定合併。各格式採用雙檔案載入模式：固定讀取 base 檔案（`.env`、`config.yaml`、`settings.json`），再合併同名 `.local` 覆寫檔（`.env.local`、`config.local.yaml`、`settings.local.json`），不再依賴 `PROFILE` 環境變數。同時支援 `APP_` 前綴環境變數自動綁定，並提供資料庫連線工廠，透過設定驅動建立 GORM ORM 連線（支援 MySQL、SQLite）。

`領域流程 (Domain Flow):`

1. 呼叫 `config.Default()` 或 `config.DefaultWithDir("/path")` 啟動設定載入，綁定/指定 `CONFIG_DIR`
2. `EnvConfig.Load()` 讀取 `.env` → `.env.local` 並合併至全域 Viper
3. `YamlConfig.Load()` 讀取 `config.yaml` → `config.local.yaml` 並合併至全域 Viper
4. `JsonConfig.Load()` 讀取 `settings.json` → `settings.local.json` 並合併至全域 Viper
5. 啟用 `APP_` 前綴環境變數自動綁定（`viper.AutomaticEnv()`）
6. 下游模組透過 `viper.GetString()` 或 `viper.Unmarshal()` 取得設定值

`核心實體 (Key Entities):` `Config` 介面, `ConfigSchema`, `EnvConfig`, `YamlConfig`, `JsonConfig`, `FSConfig`, `DBConfig`, `ServerConfig`, `DBConnConfig`

`相關處理器 (Related Handlers):` `config.Default()`, `config.DefaultWithDir()`, `NewEnvConfig()`, `NewYamlConfig()`, `NewJsonConfig()`, `NewFSConfig()`, `common.NewDBConfig()`, `common.DatabaseFactory()`, `NewMySQL()`, `NewSQLite()`

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
5. `outputgittag.sh` 腳本搭配使用，可將版本號轉換為 Git tag 格式

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

`相關處理器 (Related Handlers):` `NewGBKDecoder()`, `NewBig5Decoder()`, `DecodeGBKBytes()`, `DecodeBig5Bytes()`, `ProcessCSVFile()`, `NewCSVFilelistCallback()`, `SaveFile()`, `SaveCSV()`, `CreateIfNotExist()`, `ParseROCDate()`, `ParseTimeDuration()`, `ConfigSleep()`, `IsNil()`, `StringPointer()`, `IntPointer()`

---

### 日誌與觀測 (Logging & Observability)

基於 `zap` 的結構化日誌系統，根據環境自動切換 development（帶 stacktrace）或 production（JSON 格式）模式，支援 `LOG_LEVEL` 環境變數動態調整日誌等級。透過 `init()` 自動初始化全域 Logger 並替換 `zap.ReplaceGlobals()`。

`領域流程 (Domain Flow):`

1. `log.init()` → `log.Init()` 於套件載入時自動執行
2. 根據 `PROFILE` 環境變數選擇 `zap.NewDevelopmentConfig()` 或 `zap.NewProductionConfig()`
3. `GetLogLevel()` 從 `LOG_LEVEL` 環境變數解析日誌等級（`debug`/`info`/`warn`/`error`）
4. 設定 `timestamp` 時間格式為 `time.DateTime`
5. 建立 Logger 並透過 `zap.ReplaceGlobals()` 設定為全域實例
6. 下游模組透過 `log.Info()`、`log.Error()` 等封裝函式輸出日誌

`核心實體 (Key Entities):` `zap.Logger` (全域), `zapcore.Level`

`相關處理器 (Related Handlers):` `log.Init()`, `log.GetLogLevel()`, `log.Info()`, `log.Infof()`, `log.Error()`, `log.Errorf()`, `log.Debug()`, `log.Debugf()`, `log.Fatal()`, `log.Fatalf()`, `log.Panic()`, `log.Panicf()`

---

## 領域關聯 (Domain Relationships)

- `設定管理` 為所有領域的基礎依賴：`HTTP 服務`、`日誌`、`排程管理`、`編碼與資料處理` 皆透過 Viper 全域實例取得設定值
- `HTTP 服務` 依賴 `日誌與觀測` 提供請求日誌，依賴 `設定管理` 取得 host/port、版本號等資訊
- `通用通知` 可被 `排程管理` 組合使用：定期任務完成後透過 `Notifier.Notify()` 推送摘要
- `程式碼產生` 為獨立工具領域，不依賴其他執行時期領域，但產出的程式碼（`*_string.go`）可能被其他領域使用
- `版本管理` 為獨立 CLI 工具領域，透過 `version` 檔案與 `outputgittag.sh` 和 CI/CD 流程整合
- `編碼與資料處理` 為獨立工具領域，提供底層工具函式給其他領域使用（如 CSV 處理、檔案操作、型別轉換）
- `日誌與觀測` 被所有需要記錄的領域引用，提供統一的結構化日誌出口

## 使用方式 (Usage)

### 設定管理

```go
import "github.com/bizshuk/gosdk/config"
import "github.com/bizshuk/gosdk/config/common"
import "github.com/spf13/viper"

// 方式 1：載入預設設定（從當前目錄或 CONFIG_DIR 環境變數載入）
config.Default()

// 方式 2：在載入前直接透過 viper.Set 自訂設定檔目錄（無須環境變數）
viper.Set("CONFIG_DIR", "/custom/path")
config.Default()

// 方式 3：直接呼叫帶有自訂路徑的初始化函式
config.DefaultWithDir("/custom/path")

dbCfg := common.NewDBConfig("default")    // 從 db.default 區段建立資料庫設定
conn, _ := dbCfg.Create()                 // 工廠模式建立 GORM 連線
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
# stringer: 從常數型別產生輔助方法
go run ./cmd/stringer -type=MyEnum -output=myenum_string.go -trimprefix=PREFIX_ .

# gotmpl: 從 YAML 設定渲染 Go 模板
go run ./cmd/gotmpl --config config.yaml
```

### 版本管理

```bash
# 查看目前版本
go run ./cmd/versioning

# 遞增 patch 版本（例如 1.0.1 → 1.0.2）
go run ./cmd/versioning patch

# 遞增 minor 版本（例如 1.0.1 → 1.1.0）
go run ./cmd/versioning minor

# 遞增 major 版本（例如 1.0.1 → 2.0.0）
go run ./cmd/versioning major
```

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

s := scheduler.New()
s.Add(scheduler.Job{
    Name:     "daily-report",
    Interval: 24 * time.Hour,
    Fn: func(ctx context.Context) error {
        // 業務邏輯
        return nil
    },
    OnError: func(name string, err error) {
        log.Errorf("job %s failed: %v", name, err)
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

## 改善建議 (Improvement Suggestions)

Based on codebase analysis:

- [ ] 排程器與通知器整合範例：`scheduler` 與 `notify` 兩個套件設計上可以搭配使用（週期任務完成後推送通知），但目前缺少官方範例或整合測試，建議在 `sample/` 目錄補充完整使用範例
- [ ] `versioning` CLI 缺少 reset 與 set 子命令：目前只能遞增版本號，若需要直接設定特定版本（如 release hotfix 時跳號）無法支援，建議新增 `set <version>` 子命令
- [ ] `ConfigSchema.DB` 為 `map[string]DBConfig`，但 `common.NewDBConfig()` 需要手動傳入 key 名稱：若設定中只有單一資料庫，呼叫方需知道預設 key 名稱（`"default"`），建議在文件或設定範例中明確定義約定的 key 名稱
- [ ] 安全性增強：`mw.Helmet()` 中 `X-XSS-Protection` 標頭已被現代瀏覽器棄用，建議改用 `Permissions-Policy` 或 `Cross-Origin-Opener-Policy` 等現代標頭
- [ ] `notify.SlackNotifier` 缺少重試機制：Slack API 呼叫若因網路問題失敗，目前直接回傳 error，建議加入指數退避重試或將重試邏輯委托給呼叫方約定
