# gosdk

Go 語言通用開發工具包 (Shared SDK)，提供設定管理、HTTP 服務骨架、程式碼產生器、編碼轉換等可重用模組，作為 Go 專案的基礎函式庫使用。

## 業務領域 (Business Domains)

### 設定管理 (Configuration Management)

統一管理應用程式設定來源，支援 `.env`、YAML、JSON 及 `embed.FS` 四種格式，透過 Viper 實現階層式設定合併。根據 `PROFILE` 環境變數自動切換對應的設定檔（`local`/`dev`/`stage`/`prod`），並支援 `APP_` 前綴的環境變數自動綁定。同時提供資料庫連線工廠，透過設定驅動建立 GORM ORM 連線（支援 MySQL、SQLite）。

`領域流程 (Domain Flow):`

1. 呼叫 `config.Default()` 啟動設定載入，綁定 `CONFIG_DIR` 與 `PROFILE` 環境變數
2. `EnvConfig.Load()` 讀取 `.env` → `.env.<profile>` 並合併至全域 Viper
3. `YamlConfig.Load()` 讀取 `config.<profile>.yaml` 並合併至全域 Viper
4. 啟用 `APP_` 前綴環境變數自動綁定（`viper.AutomaticEnv()`）
5. `viper.Unmarshal()` 將設定反序列化為強型別 `ConfigSchema` 結構
6. 下游模組透過 `config.GlobalConfig` 或 `viper.GetString()` 取得設定值

`核心實體 (Key Entities):` `Config` 介面, `ConfigSchema`, `EnvConfig`, `YamlConfig`, `JsonConfig`, `FSConfig`, `DBConfig`, `ServerConfig`, `DBConnConfig`

`相關處理器 (Related Handlers):` `config.Default()`, `NewEnvConfig()`, `NewYamlConfig()`, `NewJsonConfig()`, `NewFSConfig()`, `db.NewDBConfig()`, `db.DatabaseFactory()`, `NewMySQL()`, `NewSQLite()`

---

### HTTP 服務 (HTTP Service)

基於 Gin 框架提供 HTTP 服務骨架，包含預設路由註冊、health check 端點、ping/pong 端點、stats 資訊端點，以及 Correlation ID 請求追蹤與 Helmet 安全性標頭中介層。設計為可嵌入其他 Go 專案的服務基礎模組。

`領域流程 (Domain Flow):`

1. `main()` 依序執行：`config.Default()` → `log.Init()` → 資料庫連線 → `HTTPServer()`
2. `HTTPServer()` 建立 `gin.Default()` 引擎，掛載 `mw.CorrelationID()` 與 `mw.Helmet()` 中介層
3. `router.Default(s)` 註冊 `/stats` 路由，回傳版本、Profile、設定檔路徑及 Correlation ID
4. `router.HealthRouterGroup(s)` 透過 `gin-healthcheck` 套件提供 `/healthz` 端點
5. `router.PingRouterGroup(s)` 提供 `/ping/` 端點，回應 `{"message": "pong"}`
6. 伺服器監聽 `config.GlobalConfig.Server` 指定的 host:port（預設 `:8080`）

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

`相關處理器 (Related Handlers):` `NewGBKDecoder()`, `NewBig5Decoder()`, `DecodeGBKBytes()`, `DecodeBig5Bytes()`, `ProcessCSVFile()`, `NewCSVFilelistCallback()`, `SaveFile()`, `SaveCSV()`, `ParseROCDate()`, `ParseTimeDuration()`, `ConfigSleep()`, `IsNil()`, `StringPointer()`, `IntPointer()`

---

### 日誌與觀測 (Logging & Observability)

基於 `zap` 的結構化日誌系統，根據 `PROFILE` 環境變數自動切換 development（帶 stacktrace）或 production（JSON 格式）模式，支援 `LOG_LEVEL` 動態調整日誌等級。透過 `init()` 自動初始化全域 Logger 並替換 `zap.ReplaceGlobals()`。

`領域流程 (Domain Flow):`

1. `log.init()` → `log.Init()` 於套件載入時自動執行
2. 根據 `PROFILE` 選擇 `zap.NewDevelopmentConfig()` 或 `zap.NewProductionConfig()`
3. `GetLogLevel()` 從 `LOG_LEVEL` 環境變數解析日誌等級（`debug`/`info`/`warn`/`error`）
4. 設定 `timestamp` 時間格式為 `time.DateTime`
5. 建立 Logger 並透過 `zap.ReplaceGlobals()` 設定為全域實例
6. 下游模組透過 `log.Info()`、`log.Error()` 等封裝函式輸出日誌

`核心實體 (Key Entities):` `zap.Logger` (全域), `zapcore.Level`

`相關處理器 (Related Handlers):` `log.Init()`, `log.GetLogLevel()`, `log.Info()`, `log.Infof()`, `log.Error()`, `log.Errorf()`, `log.Debug()`, `log.Debugf()`, `log.Fatal()`, `log.Fatalf()`, `log.Panic()`, `log.Panicf()`

---

## 領域關聯 (Domain Relationships)

- `設定管理` 為所有領域的基礎依賴：`HTTP 服務`、`日誌`、`編碼與資料處理` 皆透過 Viper 全域實例取得設定值
- `HTTP 服務` 依賴 `日誌與觀測` 提供請求日誌，依賴 `設定管理` 取得 host/port、Profile、版本號等資訊
- `程式碼產生` 為獨立工具領域，不依賴其他執行時期領域，但產出的程式碼（`*_string.go`）可能被其他領域使用
- `編碼與資料處理` 為獨立工具領域，提供底層工具函式給其他領域使用（如 CSV 處理、檔案操作、型別轉換）
- `日誌與觀測` 被所有需要記錄的領域引用，提供統一的結構化日誌出口

## 使用方式 (Usage)

### 設定管理

```go
import "github.com/bizshuk/gosdk/config"
import "github.com/bizshuk/gosdk/config/db"

config.Default()                      // 載入 .env + YAML 設定
dbCfg := db.NewDBConfig("default")    // 從 db.default 區段建立資料庫設定
conn, _ := dbCfg.Create()             // 工廠模式建立 GORM 連線
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

### 編碼與資料處理

```go
import roctime "github.com/bizshuk/gosdk/time"
import "github.com/bizshuk/gosdk/utils"
import encode "github.com/bizshuk/gosdk/encode/io"

t := roctime.ParseROCDate("100/08/07")              // 2011-08-07
utils.NewCSVFilelistCallback("data/*.csv", myProcessor)
utf8, _ := encode.DecodeGBKBytes(gbkData)            // GBK → UTF-8
```

## 改善建議 (Improvement Suggestions)

Based on codebase analysis:

- [x] 日誌一致性：`config/json.go` 與 `config/embedFS.go` 仍使用標準庫 `log` 與 `fmt.Println`，而非統一的 `gosdk/log` 套件，導致日誌格式不一致
- [x] 補充 `utils/` 測試覆蓋：`utils/file.go`、`utils/processor.go`（已移至 `encode/csv`）、`utils/string.go`、`utils/time.go` 等核心工具函式缺少單元測試
- [x] CSV Decoder 整合：`encode/csv/csv.go` 中的 `Decoder` 介面與 `utils/processor.go` 中的 CSV 處理邏輯分散在不同位置，考慮統一至 `encode/csv` 套件
- [x] `config/embedFS.go` 錯誤處理：`GetFSReader()` 中讀取失敗僅用 `fmt.Println` 印出錯誤，未回傳 error 或終止，可能導致後續空資料問題
- [x] 安全性增強：`mw.Helmet()` 中 `X-XSS-Protection` 標頭已被現代瀏覽器棄用，建議改用 `Permissions-Policy` 或 `Cross-Origin-Opener-Policy` 等現代標頭
