# CHANGELOG

已完成的變更紀錄，由 `README.todo` 的 `## Archive` 章節搬入。只增不減 (append-only)。
條目原文照搬，`@` 標記為完成時間與分類。

## 2026-07-06

- [2026-05-22-fix-readme-problems.md](specs/2026-08-12-Summary.md)：合併重複 decode/sleep 邏輯、統一 log 系統、SQLite 錯誤訊息、CSV Processor 參數化、補充單元測試、建立 Makefile/Dockerfile/GitHub Actions CI。 @2026-07-06_13:05:04 @Completed @CI
- [architecture-gosdk.md](../plans/2026-08-12-Refresh.md) §3-5 + §6-階段四：將 `main.go` 中的 `HTTPServer()` 移至新建 `server/server.go`（`package server`），改為 `func Run(ctx context.Context) error` 支援 graceful shutdown；`main.go` 改為 `signal.NotifyContext` 觸發 cancel，可被外部專案作為程式庫引用（`server/server_test.go` 涵蓋 4 條路由 + ctx cancel）。 @2026-07-06_13:05:02 @Completed @Server
- [architecture-service-generator-migration.md](../plans/2026-08-12-Refresh.md) §4 遷移映射：將 `service/generator.go` 移至 `service/stringer/generator.go` 子套件，使根目錄結構清晰（已完成遷移且無遺留依賴）。 @2026-07-06_13:05:00 @Completed @Service
- [architecture-gosdk.md](../plans/2026-08-12-Refresh.md) §3-2 + §6-階段二：將 `encode/io/` 目錄下所有檔案（`decode.go`、`gbk.go`、`big5.go`、`decode_test.go`）移至 `encode/` 根目錄，套件名統一為 `encode`，消除目錄路徑與套件名稱不一致。 @2026-07-06_13:04:58 @Completed @Util
- [2026-05-23-refactor-and-test-coverage.md](specs/2026-08-12-Summary.md)：CSV 處理邏輯整合至 `encode/csv`，修正 `config/embedFS.go` 的 `GetFSReader` 錯誤處理，補充 `utils/`、`encode/csv` 單元測試；更新 `mw.Helmet()` 移除已棄用 `X-XSS-Protection`，新增 `Permissions-Policy`/`Cross-Origin-Opener-Policy`。 @2026-07-06_13:04:56 @Completed @Util
- [2026-06-14-create-file-with-backup.md](specs/2026-08-12-Summary.md)：實作 `WriteFile`/`CreateFile` 核心寫檔函式，支援 `WithBackup()`（遞迴備份 `*.bak`）與 `WithCreate()` 選項，重構 `SaveFile` 呼叫新介面。 @2026-07-06_13:04:54 @Completed @Util
- [architecture-gosdk.md](../plans/2026-08-12-Refresh.md) §3-6 + §6-階段四：在 `metric/otel.go` 實作 Go 版 `OtelMetrics` 結構、`NewOtelMetrics` 建構子，以及 `ProcessCounter`、`ProcessHistogram`、`RecordProcessWithDuration` 方法，對齊 Python `otel.py` 語義（命名採 `dot→underscore` 對齊 Prometheus；`error_type` 空字串省略；unit `ms`；採用 NoopProvider / ManualReader 白箱測試）。 @2026-07-06_13:04:52 @Completed @Metric
- [prancy-imagining-meteor.md](specs/2026-08-12-Summary.md)：定義跨語言 Process Metrics 規格（Counter/Histogram/Gauge 命名、標準 Tag Keys），Python 版 `OtelMetrics`（`otel.py`）、`SlackNotifier`（`slack.py`）已完成。 @2026-07-06_13:04:50 @Completed @Metric
- [cryptic-pondering-pie.md](specs/2026-08-12-Summary.md)：將 `config/common/` DB 連線工廠重構為獨立 `db/` 套件，採 per-storage singleton（`DefaultSQLite`/`DefaultMySQL`/`DefaultPostgres`）+ 扁平 Viper key（`SQLITE_PATH`/`MYSQL_DSN`/`POSTGRES_DSN`），廢除 `config/common`。 @2026-07-06_13:04:48 @Completed @DB
- [2026-06-22-level-split-handler.md](specs/2026-08-12-Summary.md)：實作 `LevelSplitHandler`，將 Debug/Info 導至 `os.Stdout`、Warn/Error 導至 `os.Stderr`，修正 `Enabled` 恆回傳 `true` 的問題。 @2026-07-06_13:04:46 @Completed @Log
- [2026-06-15-replace-zap-with-slog.md](specs/2026-08-12-Summary.md)：全面將 `gosdk/log` 與 13 個呼叫端從 `go.uber.org/zap` 遷移至 Go 標準 `log/slog`，以 Viper 注入 `LOG_LEVEL`/`LOG_FORMAT`。 @2026-07-06_13:04:44 @Completed @Log
- [architecture-config-dynamic-reload.md](../plans/2026-08-12-Refresh.md)：設計與實作設定檔變更監聽與回調重載機制，使系統可即時套用最新參數。 @2026-07-06_13:04:43 @Completed @Config
- [architecture-gosdk.md](../plans/2026-08-12-Refresh.md) 階段四：重構 `HTTPServer` 模組導出至 `server/` 子包（`func Run(ctx context.Context) error`，支援 graceful shutdown），並在 `metric/otel.go` 實作 Go 版 `OtelMetrics` 結構與 `ProcessCounter`/`ProcessHistogram`/`RecordProcessWithDuration` 方法。 @2026-07-06_13:04:41 @Completed @Architecture
- [architecture-gosdk.md](../plans/2026-08-12-Refresh.md) 階段三：提供資料庫 `db` 顯式參數建構子以解耦 `viper`，並重構 `log` 套件導出顯式初始化方法。 @2026-07-06_13:04:39 @Completed @Architecture
- [architecture-gosdk.md](../plans/2026-08-12-Refresh.md) 階段二：解耦公用庫 `utils` 與重構 `encode/` 目錄結構。 @2026-07-06_13:04:37 @Completed @Architecture
- [architecture-gosdk.md](../plans/2026-08-12-Refresh.md) 階段一：修正 `utils` 與 `metric` 的測試在沙盒 (Sandbox) 環境下寫入權限與網路連線受阻問題。 @2026-07-06_13:04:31 @Completed @Architecture
