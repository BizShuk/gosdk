# gosdk — 技術脈絡 (Technical Context)

## 專案結構 (Project Structure)

```tree
gosdk/
├── main.go                  # HTTP 伺服器入口
├── go.mod                   # Go 1.26, module: github.com/bizshuk/gosdk
├── Makefile                 # build / test / generate / run / clean
├── build/
│   └── dockerfile           # Multi-stage Docker 建置（golang:1.26-alpine）
├── cmd/                     # package cmd：可被宿主應用 AddCommand 的子命令目錄
│   ├── config.go            # ConfigCmd：檢視/修改設定（--files/--source/--update/--add/--delete）
│   ├── config_test.go
│   ├── config/              # ConfigCmd 的邏輯層（不含 cobra）：Show/Apply/Default + Render*
│   │                        # show.go 依 sdkconfig.Sources() 逐檔合併，每個 key 帶實際來源路徑
│   ├── vault.go             # VaultCmd：命令樹、憑證取得（VAULT_TOKEN > VAULT_PASSWORD > prompt）
│   ├── vaultEncrypt.go      # .env -> .env.vault（保留明文檔，刪不刪由呼叫端決定）
│   ├── vaultDecrypt.go      # .env.vault -> .env（0600），decryptAll 共用helper
│   ├── vaultShow.go         # 解密後印出，不落地
│   ├── vaultGet.go          # 只印單一變數的值（供 $(...) 取用）
│   ├── vaultList.go         # 列出變數名稱（免密碼）
│   ├── vaultSet.go          # 新增/更新單一變數並寫回
│   ├── vaultToken.go        # 發出限時 token（--ttl，預設 8h；token 印 stdout、到期印 stderr）
│   ├── vaultRevoke.go       # 輪換裝置金鑰，一次作廢所有既發 token
│   ├── vault_test.go        # CLI 接線：預設檔名、輸出去向、免密碼 list、錯密碼失敗
│   ├── major.go             # MajorCmd：VERSION 主版號 +1
│   ├── minor.go             # MinorCmd：VERSION 次版號 +1
│   ├── patch.go             # PatchCmd：VERSION 修訂號 +1
│   ├── version.go           # Version 結構、ParseVersion()、ReadVersion()、WriteVersion()
│   ├── version_test.go
│   └── sample/              # 範例與測試工具（gotmpl/stringer 僅由測試執行）
│       ├── main.go          # metric/cobra hook 使用範例（可執行）
│       ├── gotmpl/          # Cobra 模板渲染測試工具（非 package main）
│       │   ├── main.go      # run() 測試入口
│       │   ├── cmd/         # RootCmd + TemplateLoader（保留：含 metadata）
│       │   ├── config.yaml
│       │   └── LICENSE
│       └── stringer/        # 增強版 enum stringer 測試工具（非 package main）
│           └── main.go      # run() 測試入口
├── config/                  # 設定管理模組
│   ├── config.go            # Config 介面、Default()、configBaseDir()/appConfigDirFor()
│   │                        # GetAppConfigDir() / GetAppDataDir() / GetAppLogsDir()
│   │                        # GetConfigDir() / SetConfigDir()（WithConfigDir 的命令式版本）
│   ├── config_test.go       # 基本設定載入測試
│   ├── appdir_test.go       # XDG_CONFIG_HOME 解析、空 appName 契約、seed 與讀取同目錄
│   │                        # WithConfigDir 覆寫（含 ~ 展開、data/logs 跟隨、Default 清除）
│   ├── option.go            # ConfigOption：WithAppName / WithConfigDir / WithDefaultValue
│   ├── option_test.go       # option 測試
│   ├── sources.go           # SearchPaths() / Sources()（8 個設定檔依 merge 順序解析出實際路徑）
│   │                        # LoadFile(path, layer)：單檔載入，key 正規化與 loader 一致
│   │                        # mergeNamedFiles()：以「確切檔名」解析並合併，不交給 viper 猜副檔名
│   ├── sources_test.go      # 搜尋順序、base/.local 各自解析、.yml 別名、forced dir、dotenv 解析
│   ├── env.go               # .env dotenv 載入器（雙檔案模式）
│   ├── env_test.go          # env 載入器測試
│   ├── yaml.go              # YAML 設定載入器（雙檔案模式）
│   ├── yaml_test.go         # yaml 載入器測試
│   ├── json.go              # JSON 設定載入器（雙檔案模式；讀取經 encode.JSONCCodec）
│   ├── vault.go             # 加密設定載入器（.env.vault + .env.local.vault，經 vault.Codec）
│   │                        # VAULT_FORMAT / VAULT_PASSWORD_ENV / VAULT_TOKEN_ENV、registerVaultExt()
│   ├── vault_test.go        # 雙檔載入、缺/錯密碼、不吃明文 .env、跨格式優先權
│   ├── vault/               # sub-package：祕密保險庫本體（scrypt + AES-256-GCM，KEK/DEK 兩層）
│   │   ├── vault.go         # Vault 型別：New/Open/OpenFile、Set/Get/Delete、Marshal/SaveFile
│   │   │                    # KeysOfFile（免密碼列名）、ParseEnv/MarshalEnv；套件註解含檔案格式與安全設計
│   │   ├── memory.go        # 記憶體衛生：Wipe / GetBytes / Close，含 Go 能與不能保證什麼的說明
│   │   ├── token.go         # 限時 token：裝置金鑰（Load/Ensure/RotateDeviceKey）、IssueToken
│   │   │                    # OpenWithToken / TokenExpiry；含威脅模型與三條升級路徑
│   │   ├── codec.go         # Codec{Password,Token,DeviceKey}：整份 vault ⇄ flat map，對齊 viper.Codec
│   │   ├── vault_test.go
│   │   ├── memory_test.go
│   │   ├── token_test.go
│   │   └── codec_test.go
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
├── file/                    # 泛型檔案儲存庫(目錄為單位,單檔文件 + JSONL 兩用)
│   ├── store.go             # Store[T] 型別、NewStore、safeName、Path/Dir、Custom
│   ├── store_options.go     # Options/Option、With* 選項、DEFAULT_* 預設值
│   ├── utils.go             # 套件內通用純函式:resolvePath / isNil / matchAll
│   │                        # (刻意不依賴 gosdk/utils,避開 gocsv 相依)
│   ├── document.go          # 單檔文件:Write / Read / ReadOr(atomic temp+rename)
│   ├── dir.go               # 目錄層:List(回名稱字串) / Delete / Exists / Sub
│   ├── jsonl.go             # JSONL 基礎層:Append / Scan(交出原始位元組)
│   ├── query.go             # JSONL 便利層:Find / Filter / Count / TruncateWhile
│   ├── sample/
│   │   └── main.go          # 可執行範例(go run ./file/sample):憑證庫、單一設定檔、
│   │                        # 事件日誌、混型別日誌、巢狀目錄五個場景
│   ├── store_test.go        # 建構、選項、名稱守衛、Custom
│   ├── document_test.go     # 單檔讀寫、atomic、權限、validator、decode hook
│   ├── dir_test.go          # 列表、刪除、存在、子目錄
│   ├── jsonl_test.go        # 追加、掃描、提早中止、長行
│   └── query_test.go        # 條件查詢、計數、前綴壓縮
├── encode/                  # 編碼轉換模組
│   ├── jsonc.go             # JSONCCodec + ToJSON（JSON with comments / trailing commas）
│   ├── jsonc_test.go
│   ├── csv/
│   │   ├── csv.go           # CSV Decoder 介面
│   │   ├── processor.go     # CSV RecordProcessor 與歸檔邏輯
│   │   └── processor_test.go
│   └── io/
│       ├── decode.go        # DecodeGBKBytes(), DecodeBig5Bytes()
│       ├── decode_test.go   # GBK/Big5 解碼測試
│       ├── gbk.go           # GBK 串流解碼器
│       └── big5.go          # Big5 串流解碼器
├── http/                    # HTTP client 端輔助（package 名遮蔽 net/http，慣例別名 gohttp）
│   ├── retry.go             # Retry[T] 泛型重試迴圈、Retryable/IsRetryable 標記、
│   │                        # RetryPolicy（DEFAULT_MAX_ATTEMPTS=5 指數退避 / Constant）、
│   │                        # IsRetryableStatus（429 與 5xx）
│   └── retry_test.go        # 重試預算、permanent 不重試、ctx 取消優先、標記可再包裝
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
├── tui/                     # 終端輸出渲染
│   ├── table.go             # Table：Unicode 框線表格（ANSI 色彩、多行 cell、逐欄對齊）
│   └── table_test.go        # Table 渲染測試
├── validator/               # 通用驗證框架（IValidator + composite Validator）
│   ├── validator.go         # IValidator 介面 + 通用 Validator(struct, 短路回傳第一個錯誤，可遞迴嵌套)
│   ├── validator_test.go    # composite / 遞迴 / defensive copy 等測試
│   ├── string/              # sub-package：字串驗證器實作（每個 validator 都有 typed *Error + sentinel + Is() 橋接）
│   │   ├── notEmpty.go      # NotEmpty (空字串檢查)
│   │   ├── notEmpty_test.go
│   │   ├── minLen.go        # MinLen (最小長度檢查, error 帶實際/要求長度)
│   │   ├── minLen_test.go
│   │   ├── maxLen.go        # MaxLen (最大長度檢查)
│   │   ├── maxLen_test.go
│   │   ├── pattern.go       # Pattern (regex match, 接受預編譯 *regexp.Regexp; nil 不 panic)
│   │   ├── pattern_test.go
│   │   ├── email.go         # Email (net/mail.ParseAddress; EmailError.Cause 透過 Unwrap 暴露底層錯誤)
│   │   ├── email_test.go
│   │   ├── url.go           # URL (net/url.Parse + scheme/host 必要檢查; URLError.Cause/Reason)
│   │   ├── url_test.go
│   │   ├── oneOf.go         # OneOf (值須在 allowed 集合內, defensive copy, case-sensitive)
│   │   ├── oneOf_test.go
│   │   ├── equalTo.go       # EqualTo (form: 密碼 vs 確認密碼)
│   │   ├── equalTo_test.go
│   │   ├── notEqualTo.go    # NotEqualTo (form: 拒絕 placeholder/sentinel)
│   │   └── notEqualTo_test.go
│   └── numeric/             # sub-package：int 驗證器實作（每個 validator 都有 typed *Error + sentinel + Is() 橋接）
│       ├── positive.go      # Positive (> 0)
│       ├── positive_test.go
│       ├── nonNegative.go   # NonNegative (>= 0)
│       ├── nonNegative_test.go
│       ├── negative.go      # Negative (< 0)
│       ├── negative_test.go
│       ├── min.go           # Min (>= floor, error 帶 actual/want)
│       ├── min_test.go
│       ├── max.go           # Max (<= ceiling, error 帶 actual/want)
│       ├── max_test.go
│       ├── range.go         # Range (closed interval [min, max])
│       └── range_test.go
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
│   └── plugin.json          # plugin metadata（name=gosdk；version 於 release 時人工對齊 tag）
├── plans/                   # 開發計畫文件
├── skills/                  # Agent skills（11 個：golang-dev、golang-gosdk、golang-mvc、golang-code-quality、golang-dead-code、golang-naming、golang-network、golang-performance-tuning、golang-gosdk-migrate、golang-runtime-profiling、golang-tui）
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
    - `spf13/viper` v1.21.0 — 階層式設定管理（CodecRegistry + JSONC for json）
    - `spf13/cobra` v1.9.1 — CLI 框架（gotmpl、versioning）
    - `log/slog` (stdlib) — 結構化日誌（取代 zap）
    - `gorm.io/gorm` v1.31.1 — ORM（MySQL + SQLite + PostgreSQL，driver 各 v1.6.0）
    - `castai/promwrite` v0.6.0 — Prometheus remote-write client（MetricService）
    - `go.opentelemetry.io/otel` v1.44.0 — OpenTelemetry SDK（OTLP HTTP metrics/traces）
    - `slack-go/slack` v0.23.1 — Slack 通知
    - `golang.org/x/tools` v0.44.0 — Go AST 解析（stringer）
    - `golang.org/x/text` v0.37.0 — CJK 編碼轉換
    - `golang.org/x/crypto` v0.51.0 — scrypt 金鑰衍生（config/vault）
    - `golang.org/x/term` v0.45.0 — 終端機密碼輸入不回顯（cmd.VaultCmd）
    - `tavsec/gin-healthcheck` v1.2.2 — Health check 端點
    - `hairyhenderson/gomplate` v4.3.3 — 模板渲染函式

## 關鍵決策 (Key Decisions)

- 使用 Viper 全域單例管理設定：所有設定來源（.env、YAML、JSON、環境變數）合併至單一 `viper` 實例，簡化跨模組存取，但犧牲了可測試性
- 雙檔案載入模式：各設定格式固定載入 base 檔案 + `.local` 覆寫檔（`.env` + `.env.local`、`config.yaml` + `config.local.yaml`、`settings.json` + `settings.local.json`），不再依賴 `PROFILE` 環境變數切換
- JSON 讀取接受 JSONC：codec 在 `encode/jsonc.go`（`encode.JSONCCodec`），僅 `JsonConfig` / `LoadFile(json)` / `ParseJSON` 使用；yaml/env/embed 仍 `viper.New()`。檔名維持 `.json`；寫回 strict JSON
- 跨格式設定優先權（5 層）：`OS env` > `.env` > `.env.vault` > `settings.json` > `config.yaml`（高者覆蓋低者）。`config.Default()` 內 `loadAllConfigs()` 依序 merge `yaml → json → vault → env`（後者覆蓋前者），最後呼叫 `viper.AutomaticEnv()` + `bindAllEnvVars()` 啟用 OS env 動態查詢。`bindAllEnvVars()` 透過 reflection 走完 `viper.AllSettings()` 並對每個 leaf 呼叫 `viper.BindEnv(key, UPPER(key))`，確保 flat key 與 nested key（如 `server.port` $\rightarrow$ `SERVER_PORT`）都能直接被對應名稱的 OS env 覆寫（驗證見 [config_test.go:TestDefault_EnvVarOverridesAllFiles](file:///Users/shuk/projects/platform/gosdk/config/config_test.go) + `TestDefault_NestedKeyEnvOverride`）。
- 設定來源可追溯（provenance）：`config.Sources()` 依 merge 順序（低到高）列出 6 個設定檔並解析出實際絕對路徑，`config.LoadFile(path, layer)` 單檔載入且 key 正規化與 loader 一致。目錄搜尋順序為 `.` → `./conf` → app config dir，且`同一檔名只有第一個命中的目錄生效`（fallback chain，非跨目錄 merge）；base 與 `.local` 各自獨立解析，可分別落在不同目錄。`cmd/config.Show()` 建在這兩者之上，因此 CLI 顯示的來源與 runtime 實際載入不會分歧；`app config --files` 直接列出搜尋結果與缺檔
- 加密設定是`一種 viper 格式`而非一支解密工具：`config/vault` 這個 sub-package 只做加解密與檔案格式，`Codec` 實作 `Decode/Encode`（方法組合對齊 `viper.Codec`，本身不 import viper），上一層的 `config/vault.go` 把它註冊成 `vault` 格式並載入 `.env.vault` + `.env.local.vault`。切法與 `cmd/config.go` + `cmd/config/` 相同：外層是接線，內層是本體。因此祕密不需要先解密成 `.env` 落地，直接是 `viper.GetString()` 的一個來源。三個相關決定：(1) 憑證`只`從 OS 環境變數讀（`VAULT_TOKEN` 優先於 `VAULT_PASSWORD`），不從 viper 讀——viper 裡的東西不是明文檔案就是環境變數，從它取鑰匙等於把鑰匙放進要保護的箱子；(2) codec registry `per-load` 建立而非行程共用，因為 registry 會持有密碼，共用等於讓密碼活到行程結束，也讓「兩個不同密碼的 vault」無法表達；(3) 缺密碼時整層跳過且不報錯，沒在用保險庫的應用程式不需要為此設定任何東西
- `兩層金鑰（KEK/DEK）`：主密碼經 scrypt 衍生的 KEK `不`直接加密資料，只用來包裹一把隨機 DEK，真正加密每個值的是 DEK。多這一層換到兩件事：(1) 限時 token 可以是 DEK 的`另一個包裹`，不必碰主密碼；(2) 被包裹的 DEK 本身就是密碼驗證器，因此 v1 的 `verifier` 欄位直接刪除。檔案格式為 `ENV-VAULT-2`，`不`保留 v1 讀取路徑，也因此`不再與 Python 版 vault.py 互通`
- `限時 token 的能力邊界`：純時間戳（TOTP 那類）在密碼學上無法解密資料——能解密就代表 token 帶著金鑰，而金鑰若能由時間推導則人人可推。實作採「金鑰包裹 + 時間視窗金鑰」：`tokenKey = HKDF(deviceKey, info=TOKEN_INFO|exp)`，`token = base64url(exp ‖ nonce ‖ AES-GCM(tokenKey, DEK, AAD=exp))`。exp 同時進 info 與 AAD，改長到期時間就解不開（已驗證）。`威脅模型`：單機無硬體支援時過期是`軟性防護`——同時拿到 token 與 deviceKey 者可繞過檢查，它防的是 token 單獨外洩後被長期利用。升級路徑：`vault revoke` 輪換裝置金鑰（已實作）、TPM/Keychain/KMS 託管 DEK、ssh-agent 式常駐程序（後兩者需本層以外的元件）
- `token 不能生 token`：`vault token` 只接受主密碼，不吃 `VAULT_TOKEN`。否則持有者可無限延長期限，過期形同虛設
- `記憶體衛生做到「可清除範圍最大化」而非「絕對清除」`：密碼全程 `[]byte`（`term.ReadPassword` 本來就回 bytes），衍生完 KEK 立刻 `Wipe`；`GetBytes` 讓只取一兩個值的呼叫端拿到可清除的位元組；`Close` 清 DEK 並釋放 aead。三個限制寫在 `memory.go`：string 不可變、GC 會留副本、AES round key 已展開。`刻意不引入` mlock/memguard——明文終究會進 viper 與環境變數，那層複雜度買到的有限
- 自訂 viper 格式必須同時登記副檔名：viper 在查 codec registry 之前會先比對 package-level `viper.SupportedExts`，只註冊 codec 會得到 `Unsupported Config Type`。`registerVaultExt()` 是冪等的，且在`每次` load 時重跑而不只在 `init()`——`viper.Reset()` 會把該清單還原成內建值，一個因為無關的 Reset 就安靜失效的格式極難追查
- 設定檔一律以`確切檔名`解析（`mergeNamedFiles`）：交給 viper 的 `SetConfigName` 會把名稱當成字根去比對所有支援的副檔名，因此 `.vault` 一登記，原本讀 `.env` 的 dotenv loader 就會改抓同目錄的 `.env.vault` 並解析失敗（實際踩到，非假設）。loader 定址的是`檔案`不是名稱字根
- 扁平 viper key 直讀：`config.Default()` 載入設定後透過 `viper.Get*()` 取值；不再維護強型別 `ConfigSchema` / `ServerConfig` / `DBConfig` 等聚合結構（2026-06 重構後 `config/common` 已廢除）
- 儲存型態採 per-service singleton：每種儲存是一個獨立 service（`db.SQLite` / `db.MySQL` / `db.Postgres`），各自有 `DefaultSQLite` / `DefaultMySQL` / `DefaultPostgres` 全域 singleton 與扁平 viper key（`SQLITE_PATH` / `MYSQL_DSN` / `POSTGRES_DSN`），守護函式 `InitSQLite()` / `InitMySQL()` / `InitPostgres()` 拒絕重複初始化以落實「micro-service: 同型態不可有兩個 instance」；MySQL 與 PostgreSQL 採單一 DSN 字串欄位而非拆 `HOST`/`PORT`/`USER`/`PASSWORD`，簡化設定並與舊 `url` 對齊(PostgreSQL 接受 URL 形式 `postgres://...` 或 keyword/value 形式 `host=... user=...`)
- `stringer` 以 `GeneratorEx` 組合模式擴充標準庫 `stringer`：嵌入 `service.Generator`，額外產生 `List()`、`ValueList()`、`Map()`、`ValueMap()` 四個輔助函式
- 日誌模組在 `init()` 時即以預設值初始化 slog 全域 logger：確保任何 import 此套件的模組都能立即使用套件層級 `slog.*`，`log.Init()` 可在設定載入後再次呼叫以套用 `LOG_LEVEL` / `LOG_FORMAT`。不提供 wrapper 函式，消費端直接使用 stdlib `log/slog`
- CSV 處理使用歸檔標記檔（`.archived`）防止重複處理：以簡單的檔案系統機制取代資料庫或 Redis 的已處理紀錄
- Helmet 中介層採用靜態安全標頭：直接在 response header 注入 `X-Content-Type-Options`、`X-Frame-Options`、`CSP` 等，不依賴外部套件
- 排程器採用極簡設計：`Scheduler` 僅負責按 `Interval` 觸發 `Job.Fn`，錯誤處理、日誌、重試等策略完全由呼叫方透過 `Job.OnError` 自行決定
- `Notifier` 介面保持單一方法（`Notify`）：不綁定特定訊息格式，呼叫方自行序列化 summary 字串，使通知器可輕易替換或組合
- `gosdk/cmd` 定位為「可被宿主應用 `AddCommand()` 的子命令目錄」，不是可執行程式：每個子命令一個檔案、檔名對應命令名（`config.go` → `ConfigCmd`、`major.go` → `MajorCmd`），一律採 package-level var + `init()` 綁 flag，root command 不上提（各 CLI 自己的 `main.go` 組裝）。子子命令以 prefix 命名（`deployLocal.go`）
- 版本管理改為 SDK 子命令：`cmd.MajorCmd` / `MinorCmd` / `PatchCmd` 操作工作目錄下的純文字 `VERSION` 檔（`major.minor.patch`），不依賴 git tag 或外部服務；`Version` 結構與 `ReadVersion()` / `WriteVersion()` 一併公開於 `cmd/version.go`。原先的獨立 `cmd/versioning` binary 已移除 —— 需要 CLI 的專案自行組 root command
- `cmd/sample/` 收攏範例與測試工具：只有 `cmd/sample/main.go` 是可執行程式，`gotmpl` 與 `stringer` 改由 `_test.go` 呼叫 `run()` 驗證功能，與 SDK 的子命令目錄分離
- Cobra hook 採極簡設計（無 option、同步送出）：`CobraCMDHook(root)` 在 PreRun 送出 `command_line_trigger{cmd, flag}`（PreRun 而非 PostRun：永遠會送，即使 RunE 失敗）；`cmd` 為完整指令鏈（`cmd.CommandPath()`，root → leaf）；`flag` 收集使用者實際設定的 flags（走訪整條 chain、`seen` map 去重 persistent flag），字母排序後以 `-` 串接；發送走套件層級 `Send()`（全域 `MetricService`，首次使用時以 `METRIC_URL` 建立；測試以 `viper.Set` 覆寫）
- `gosdk/file` 的 `Store[T]` 本體是`一個目錄`，檔名一律由呼叫端傳入，同時支援「整檔一份 JSON 文件」與「每行一筆的 JSONL 追加日誌」兩種存取型態 —— 兩種都提供但都不強制使用。JSONL 分兩層：`Scan` 是把原始位元組交給呼叫端的基礎層，因此能處理同一檔案內混有多種記錄型別的情況（例如首行 meta、其餘 turn），也讓不需解碼的呼叫端直接做位元組比對；`Find`/`Filter`/`Count` 是建在其上、會自動解成 `T` 的便利層。`Read` 對缺檔回 `ErrNotFound`，但 JSONL 讀取路徑把「尚未建立」與「空的」視為同一狀態、回空結果與 `nil` —— 對追加日誌而言這兩者確無語意差別。`safeName` 除了擋 `/` 與 `\`，另外明確擋掉 `..`（`filepath.Base("..")` 是 `".."`，會通過 `Base(name) != name` 的檢查，但它指的是上層目錄），並以「算出實際路徑後確認其父目錄等於 Store 目錄」作為最終防線 —— 這道 containment 檢查不依賴對名稱形狀的枚舉，能擋掉靠副檔名組合出來的邊界情況，例如 `WithExt("")` 時 `Path(".")` 會 Clean 成目錄本身、放行則 `Delete(".")` 會刪掉整個 Store 目錄。`.` 與其他點開頭的名稱是`合法`的：它們只是隱藏檔、不是穿越，因此 `List` 也只跳過 `TEMP_FILE_PREFIX` 暫存檔而非所有點檔案，維持「寫得進去就列得出來」的對稱。`file` 刻意`不 import gosdk/utils`：`utils/file.go` 內含 `gocarina/gocsv`，而 Go 以 package 為單位載入，任何引用都會把 CSV 函式庫拖進相依圖，代價是 `file/path.go` 自帶約 35 行的 `resolvePath`/`isNil`

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

| 業務領域 (Domain)     | 套件/模組 (Package/Module)                              | 進入點 (Entry Point)                                                                                                      |
| --------------------- | ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| 設定管理              | `config/`                                               | `config.Default()`                                                                                                        |
| 加密設定 / 祕密保險庫 | `config/vault/`, `config/vault.go`                      | `config.NewVaultConfig()`, `vault.Codec`, `vault.OpenFile()`                                                              |
| 限時解密憑證          | `config/vault/token.go`                                 | `vault.IssueToken()` / `vault.OpenWithToken()` / `cmd.VaultTokenCmd`                                                      |
| 保險庫 CLI            | `cmd/` (`vault*.go`)                                    | `cmd.VaultCmd`                                                                                                            |
| 資料庫連線            | `db/`                                                   | `db.InitSQLite()` / `db.InitMySQL()` / `db.InitPostgres()`                                                                |
| HTTP 服務             | `router/`, `mw/`, `main.go`                             | `HTTPServer()`                                                                                                            |
| 程式碼產生 — stringer | `cmd/sample/stringer/`, `service/generator.go`          | `go test ./cmd/sample/stringer -run TestRunGeneratesStringerCode`                                                         |
| 程式碼產生 — gotmpl   | `cmd/sample/gotmpl/`                                    | `go test ./cmd/sample/gotmpl -run TestRun`                                                                                |
| 版本管理              | `cmd/` (`version.go`)                                   | `cmd.MajorCmd` / `MinorCmd` / `PatchCmd`, `cmd.ReadVersion()`                                                             |
| 設定檢視/修改 CLI     | `cmd/` (`config.go`)                                    | `cmd.ConfigCmd`                                                                                                           |
| Cobra Hook 範例       | `cmd/sample/`                                           | `cmd/sample/main.go`                                                                                                      |
| 通用通知              | `notify/`                                               | 各通知器獨立建構與呼叫                                                                                                    |
| 排程管理              | `scheduler/`                                            | `scheduler.New()`                                                                                                         |
| 編碼與資料處理        | `encode/`, `utils/`, `time/`                            | 各函式獨立呼叫                                                                                                            |
| 通用驗證              | `validator/`, `validator/string/`, `validator/numeric/` | `validator.New()` (composite), `string.NewNotEmpty()` / `string.NewEmail()` / `string.NewEqualTo()`, `numeric.NewRange()` |
| 檔案儲存              | `file/`                                                 | `file.NewStore[T]()`                                                                                                      |
| 日誌與觀測            | `log/`                                                  | `log.Init()`                                                                                                              |
| Remote Write 指標     | `metric/`                                               | `NewMetricService()` / `NewVictoriaMetricsService()`                                                                      |
| Cobra CLI Hook 指標   | `metric/`                                               | `metric.CobraCMDHook()`                                                                                                   |
| OTel 指標             | `metric/`                                               | `metric.InitMeterProvider()`                                                                                              |
| OTel Tracer           | `metric/`                                               | `metric.InitTracerProvider()`                                                                                             |

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
- Configuration: 設定檔搜尋路徑固定為 `.`、`./conf`、`~/.config/<appName>`（需 `WithAppName`）；第三個目錄可用 `WithConfigDir(dir)` 強制覆寫（`~` 會展開，優先於 appName 與 `XDG_CONFIG_HOME`，`GetAppDataDir` / `GetAppLogsDir` 一併跟隨）；雙檔案模式（base + `.local`）自動載入；`APP_` 前綴環境變數自動覆蓋設定
