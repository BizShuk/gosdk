# 術語表 (Terminology)

本檔是領域名詞的`單一定義來源`。程式碼的 identifier 以此為準。
名詞的`行為`與`理由`寫在 [CLAUDE.md](../CLAUDE.md) 的關鍵決策，本檔只定義`是什麼`。

## 設定管理 (Configuration)

| Term | 定義 |
| --- | --- |
| `Config Source` | 一個實際存在的設定檔絕對路徑。`config.Sources()` 依 merge 順序（低到高）列出 6 個 source。 |
| `Layer` | 設定的格式層級：`yaml` → `json` → `env`，後者覆蓋前者。與 `Priority` 不同，Layer 只描述`檔案格式`。 |
| `Priority` | 跨來源的覆蓋順序，共 4 層：`OS env` > `.env` > `settings.json` > `config.yaml`。 |
| `Dual-file Mode` | 每種格式固定載入 base 檔 + `.local` 覆寫檔（`config.yaml` + `config.local.yaml`）。取代已廢除的 `PROFILE` 切換。 |
| `Search Path` | 設定檔的目錄搜尋順序 `.` → `./conf` → app config dir。`同一檔名只有第一個命中的目錄生效`（fallback chain，非跨目錄 merge）。 |
| `App Config Dir` | `~/.config/<appName>`，由 `WithAppName` 決定，可被 `WithConfigDir` 強制覆寫。其下固定有 `data/` 與 `logs/`。 |
| `Provenance` | 「某個 key 的值來自哪一個檔案」的可追溯性。由 `Sources()` + `LoadFile()` 提供，`app config --files` 是其 CLI 出口。 |
| `Flat Viper Key` | 直接以 `viper.Get*()` 讀取的扁平鍵（如 `SQLITE_PATH`）。取代已廢除的強型別 `ConfigSchema` 聚合結構。 |
| `JSONC` | 允許註解與 trailing comma 的 JSON。僅`讀取`路徑接受，寫回一律 strict JSON。 |

## 儲存 (Storage)

| Term | 定義 |
| --- | --- |
| `Storage Service` | 一種資料庫型態的連線服務（`db.SQLite` / `db.MySQL` / `db.Postgres`），實作 `DB()` / `Close()`。 |
| `Per-storage Singleton` | 每種 Storage Service 全域`只允許一個 instance`；`InitXxx()` 是拒絕重複初始化的守護函式。 |
| `DSN` | MySQL / PostgreSQL 的單一連線字串欄位。刻意不拆成 HOST / PORT / USER / PASSWORD。 |
| `Store[T]` | `gosdk/file` 的泛型檔案儲存庫。本體是`一個目錄`，檔名由呼叫端傳入。 |
| `Document` | Store 的「整檔一份 JSON」存取型態，寫入採 atomic temp + rename。 |
| `JSONL Log` | Store 的「每行一筆」追加型態。「尚未建立」與「空的」視為同一狀態。 |
| `Scan` | JSONL 基礎層：交出`原始位元組`，容許同一檔案混有多種記錄型別。 |
| `Query Layer` | 建在 Scan 之上、自動解成 `T` 的便利層（`Find` / `Filter` / `Count`）。 |
| `Containment Check` | `safeName` 的最終防線：算出實際路徑後確認其父目錄等於 Store 目錄，不靠枚舉名稱形狀。 |

## 觀測 (Observability)

| Term | 定義 |
| --- | --- |
| `MetricService` | 通用 Prometheus remote-write client。輕量、無生命週期管理、批次邊界由呼叫端決定。 |
| `Remote Write` | Prometheus 遠端寫入協定。端點由 `METRIC_URL` 注入，預設 VictoriaMetrics。 |
| `OTLP` | OpenTelemetry 協定路徑，需 `MeterProvider` / `Shutdown`，批次由 SDK 週期性背景送出。 |
| `Sanitization` | remote-write 路徑自動把指標名稱的 `.` 轉為 `_`；OTLP 路徑`不做`，由開發者自負。 |
| `Cobra Hook` | `CobraCMDHook(root)` 在 PreRun 送出 `command_line_trigger`。用 PreRun 是為了`即使 RunE 失敗也會送`。 |
| `Level Split` | slog handler 行為：Warn/Error 與其餘層級分流輸出。 |

## CLI 與版本 (CLI & Versioning)

| Term | 定義 |
| --- | --- |
| `Subcommand Directory` | `gosdk/cmd` 的定位：可被宿主應用 `AddCommand()` 的子命令`目錄`，不是可執行程式。 |
| `Package-level Cmd Var` | 子命令的宣告形式：exported package-level var + `init()` 綁 flag，不用 `NewXxxCmd()` 建構子。 |
| `VERSION File` | 工作目錄下的純文字 `major.minor.patch` 檔。版本事實不依賴 git tag 或外部服務。 |

## 通用元件 (Shared Primitives)

| Term | 定義 |
| --- | --- |
| `Retryable` | 標記某個 error 值得重試的包裝；`IsRetryable` 判定。可被再次包裝而不失去標記。 |
| `Retry Policy` | 重試預算與退避策略（指數 / 固定），預設 `DEFAULT_MAX_ATTEMPTS=5`。 |
| `Notifier` | 單一方法 `Notify` 的通知介面。不綁訊息格式，呼叫端自行序列化。 |
| `Job` | Scheduler 的排程單位：`Interval` + `Fn` + `OnError`。重試與日誌策略由呼叫端擁有。 |
| `GeneratorEx` | 以組合模式擴充標準庫 `stringer`，額外產生 `List()` / `ValueList()` / `Map()` / `ValueMap()`。 |
| `Archive Marker` | CSV 處理的 `.archived` 標記檔，以檔案系統取代資料庫作為「已處理」紀錄。 |

## 命名慣例 (Naming Conventions)

| 對象 | 慣例 | 範例 |
| --- | --- | --- |
| Package | 小寫單字 | `config`, `log`, `mw`, `notify` |
| 檔案 | camelCase | `correlationId.go`, `levelSplitHandler.go` |
| 常數 | `SCREAMING_SNAKE_CASE`（含 unexported 與 block-scoped） | `DEFAULT_MAX_ATTEMPTS`, `TEMP_FILE_PREFIX` |
| 變數 | MixedCaps | `configBaseDir` |
| 環境變數 | 對應 viper key 的大寫形式 | `server.port` → `SERVER_PORT` |
