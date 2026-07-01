# 階段四實作計畫 — HTTPServer 模組導出 + Go 版 OtelMetrics

## Context

`architecture-gosdk.md` 階段四需完成兩項目標：

1. **`HTTPServer` 模組導出**：目前 `HTTPServer()` 函式在 `package main`（`main.go:50-72`），外部 Go 專案無法將其作為程式庫引用。把此函式搬至新建 `server/` 子包（`package server`），使 SDK 可被嵌入重用。
2. **Go 版 `OtelMetrics` 高階封裝**：目前 `metric/otel.go` 僅有低階 Provider 初始化，缺少對齊 Python `metric/otel.py` 與 `SPEC.md` 規格的「業務語意化」API（`OtelMetrics` 結構、`NewOtelMetrics`、`ProcessCounter` / `ProcessHistogram` / `RecordProcessWithDuration`）。需要讓外部 repo 能直接 `import "github.com/bizshuk/gosdk/metric"` 並使用統一 Tag Keys。

預期成果：

- 外部專案能 `import "github.com/bizshuk/gosdk/server"` 後呼叫 `server.Run(ctx)`
- Go 端 `metric.OtelMetrics` 與 Python `OtelMetrics` 語義完全對齊（`process.ticker.fetch` 等命名、`job_name` / `instance` / `ticker` / `status` / `error_type` 標準 Tag）
- `BatchSummary` + `FormatBatchSummary` 提供批次通知的格式化標準，與 `notify` 套件組合使用
- 既有 `metric/MetricService`（Prometheus remote-write）路線保留不動，作為 `CobraCMDHook` 等低頻場景使用；新 `OtelMetrics` 走 OTLP 路線

## 使用者已確認之決策

| 主題 | 決策 |
|---|---|
| `NewOtelMetrics` 簽章 | `(jobName, instance string) (*OtelMetrics, error)`；內建 OTLP HTTP exporter，endpoint 走 viper `OTLP_METRIC_URL` |
| `ProcessCounter` / `ProcessHistogram` 回傳 | 原生 OTel instrument 物件（`metric.Int64Counter` / `metric.Float64Histogram`），呼叫端決定 attributes 與時機 |
| `BatchSummary` 與 `FormatBatchSummary` | `BatchSummary` 為公開結構；`FormatBatchSummary(jobName string, summary BatchSummary) string` 為套件層級函式 |
| `HTTPServer` 形態 | 改為 `func Run(ctx context.Context) error`；支援 ctx 取消與 graceful shutdown；**不留 Deprecated 別名**（直接 break，README.todo §server 已標待辦） |
| Signal 職責 | main 自行呼叫 `signal.NotifyContext`；`server.Run` 僅接 ctx，不處理 signal |
| Histogram bucket | 採 OTel 預設 bucket（`sdkmetric.DefaultHistogramBucketBoundaries`） |
| Histogram unit | `"ms"`，呼叫端負責單位轉換 |
| `error_type` 空字串 | 省略 `error_type` attribute，不寫入空字串 label |
| Resource 屬性 | `service.name = "gosdk"` + `job_name` + `instance` |
| 測試隔離 | 新白箱測試採 `metric.NewNoopMeterProvider()` 注入；既有 `TestOtelSample` 整合測試保留不動 |

---

## 變更清單

### 新增檔案

| 檔案 | 用途 |
|---|---|
| `server/server.go` | `package server` 對外導出 `Run(ctx)` 與內部 `assembleEngine()` |
| `server/server_test.go` | 黑箱測試：4 條路由 + ctx cancel + graceful shutdown |
| `metric/summary.go` | `BatchSummary` 公開結構 + `FormatBatchSummary` 套件函式 |
| `metric/summary_test.go` | `FormatBatchSummary` 4 case table-driven 測試 |
| `metric/otel_metrics.go` | `OtelMetrics` 結構 + `NewOtelMetrics` + 6 個 domain method + `RecordProcessWithDuration` + `Shutdown` |
| `metric/otel_metrics_test.go` | NoopProvider 白箱測試（instrument 命名、attribute 組合、count 累加） |

### 修改檔案

| 檔案 | 修改內容 |
|---|---|
| `main.go` | 移除 `HTTPServer()`；`main()` 改為 `config.Default()` → `log.Init()`（若需要）→ `signal.NotifyContext` → `server.Run(ctx)` |
| `metric/otel.go` | 不修改既有 `InitMeterProvider` / `InitTracerProvider` / `ShutdownOTel`；保留作 SDK 全域 Provider 入口 |
| `go.mod` | 確認 `go.opentelemetry.io/otel/attribute v1.44.0` 已存在；缺則 `go get` 補上 |
| `README.todo` | §server 與 §metric/§OtelMetrics 條目改為 `[x]`；§config/§log 不變動 |

### 不修改

- `metric/metric.go`（`MetricService` remote-write 路線）
- `metric/cobra.go`、`metric/mimir.go`、`metric/victoriametrics.go`、`metric/model.go`
- `router/`、`mw/`、`log/`、`config/`、`db/`
- `metric/otel_test.go` 既有 `TestOtelSample` 整合測試

---

## 實作細節

### A. `server/server.go`

**套件**：`package server`

**匯入**：

- `context`（ctx 取消）
- `fmt`、`net/http`、`os/signal`、`syscall` — 由 caller（main）使用；server 內部只需 `context`、`net/http`、`time`、`log/slog`、`github.com/gin-gonic/gin`、`github.com/spf13/viper`、`github.com/bizshuk/gosdk/router`、`github.com/bizshuk/gosdk/mw`

**匯出**：

```go
// Run 啟動 HTTP 伺服器並阻塞至 ctx 取消，之後進行 graceful shutdown。
// 錯誤回傳：listener 綁定失敗或 Shutdown 期間錯誤。
func Run(ctx context.Context) error
```

**私有**：

```go
// assembleEngine 建立並設定 gin.Engine（middleware + router 群組）。
// 匯出供測試呼叫，不對外 commit（package-private）
func assembleEngine() *gin.Engine
```

**`Run` 內部流程**：

1. `engine := assembleEngine()`
2. 讀 viper `server.host` / `server.port`（預設 `:8080`）
3. 建立 `http.Server{Addr: addr, Handler: engine}`
4. goroutine 內呼叫 `srv.ListenAndServe()`，錯誤送 channel
5. `select` 等 `ctx.Done()` 或 listener err
6. ctx 取消後建立 `shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)`
7. `defer cancel()`；呼叫 `srv.Shutdown(shutdownCtx)`
8. 若 listener 早於 ctx 結束，回傳 listener 錯誤（`http.ErrServerClosed` 過濾）

**`assembleEngine` 內部**：

- 移植 `main.go:51-57` 既有邏輯：`gin.Default()` → `mw.CorrelationID()` → `mw.Helmet()` → `router.Default(s)` → `router.HealthRouterGroup(s)` → `router.PingRouterGroup(s)`

### B. `main.go` 重寫

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "github.com/bizshuk/gosdk/config"
    "github.com/bizshuk/gosdk/db"
    "github.com/bizshuk/gosdk/log"
    "github.com/bizshuk/gosdk/server"
    "github.com/spf13/viper"
)

func main() {
    if err := config.Default(); err != nil {
        slog.Error("config load failed", "err", err)
        os.Exit(1)
    }
    if err := log.Init(); err != nil {
        slog.Error("log init failed", "err", err)
        os.Exit(1)
    }
    if err := initDB(); err != nil {
        slog.Error("db init failed", "err", err)
        os.Exit(1)
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    if err := server.Run(ctx); err != nil {
        slog.Error("server exited with error", "err", err)
        os.Exit(1)
    }
}

func initDB() error {
    if viper.IsSet("SQLITE_PATH") {
        if err := db.InitSQLite(); err != nil { return fmt.Errorf("sqlite: %w", err) }
    }
    if viper.IsSet("MYSQL_DSN") {
        if err := db.InitMySQL(); err != nil { return fmt.Errorf("mysql: %w", err) }
    }
    if viper.IsSet("POSTGRES_DSN") {
        if err := db.InitPostgres(); err != nil { return fmt.Errorf("postgres: %w", err) }
    }
    return nil
}
```

注意：`log.Init()` 是階段三的既有導出函式（`log/log.go`）；若日後發現 main.go 不需要顯式呼叫，請於實作時確認 `log.Init` 是否已導出。**預期存在**，因為 README.todo §log 階段三標為 `[x]`。

### C. `metric/summary.go`

```go
package metric

type BatchSummary struct {
    Total      int      `json:"total"`
    Succeed    int      `json:"succeed"`
    Failed     int      `json:"failed"`
    FailedList []string `json:"failed_list,omitempty"`
    DurationMs float64  `json:"duration_ms,omitempty"`
}

func FormatBatchSummary(jobName string, summary BatchSummary) string
```

**`FormatBatchSummary` 輸出格式**：

- 永遠輸出：`[jobName] batch finished: X total, Y succeed, Z failed`
- `summary.DurationMs > 0` 時附加：`(took D.Dms)`
- `len(summary.FailedList) > 0` 時換行附加：`failed items: a, b, c`

### D. `metric/otel_metrics.go`

**結構**：

```go
type OtelMetrics struct {
    jobName       string
    instance      string
    meterProvider *sdkmetric.MeterProvider
    meter         metric.Meter
    mu            sync.RWMutex
    counters      map[string]metric.Int64Counter       // key: domain+"."+name
    histograms    map[string]metric.Float64Histogram
}
```

**`NewOtelMetrics(jobName, instance string) (*OtelMetrics, error)` 內部**：

1. 讀 viper `OTLP_METRIC_URL`（預設 `http://localhost:8428/opentelemetry/v1/metrics`）
2. `otlpmetrichttp.New(ctx, WithEndpointURL(url), WithInsecure())` 條件性 insecure
3. `resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("gosdk"), attribute.String("job_name", jobName), attribute.String("instance", instance))`
4. `sdkmetric.NewMeterProvider(WithResource(res), WithReader(sdkmetric.NewPeriodicReader(exp, WithInterval(30*time.Second))))`
5. `otel.SetMeterProvider(mp)`
6. `mp.Meter("gosdk")`
7. 回傳 `&OtelMetrics{...}`

**6 個 domain method**（共用私有 helper）：

```go
func (m *OtelMetrics) ProcessCounter(name string) (metric.Int64Counter, error)  // 對齊 SPEC 範例
func (m *OtelMetrics) ProcessHistogram(name string) (metric.Float64Histogram, error)
func (m *OtelMetrics) QueueCounter(name string) (metric.Int64Counter, error)
func (m *OtelMetrics) QueueHistogram(name string) (metric.Float64Histogram, error)
func (m *OtelMetrics) ServiceCounter(name string) (metric.Int64Counter, error)
func (m *OtelMetrics) ServiceHistogram(name string) (metric.Float64Histogram, error)
```

> 簽章簡化決策：因 `domain` 已由 method 本身標明（Process/Queue/Service），不再傳 `domain` 參數；呼叫端傳入 `name` 即可（例：`m.ProcessCounter("ticker.fetch")`）。

**底層共用 helper**（私有）：

- `getOrCreateCounter(instrumentName string) (metric.Int64Counter, error)`
- `getOrCreateHistogram(instrumentName string) (metric.Float64Histogram, error)`
- instrument 名稱規則：`strings.ReplaceAll(name, ".", "_")`（例：`"ticker.fetch"` → `"ticker_fetch"`）

**`RecordProcessWithDuration` 簽章**（對齊 SPEC.md L109）：

```go
func (m *OtelMetrics) RecordProcessWithDuration(
    ctx context.Context,
    counter metric.Int64Counter,
    histogram metric.Float64Histogram,
    ticker, status, errorType string,
    durationMs float64,
)
```

- attributes：`job_name=m.jobName`, `instance=m.instance`, `ticker=ticker`, `status=status`
- `errorType != ""` 時加 `error_type=errorType`
- `counter.Add(ctx, 1, metric.WithAttributes(...))`
- `durationMs > 0` 時 `histogram.Record(ctx, durationMs, metric.WithAttributes(...))`

**`Shutdown(ctx context.Context) error`**：呼叫 `m.meterProvider.Shutdown(ctx)`。

### E. 測試

#### `server/server_test.go` 覆蓋點

| 測試函式 | 驗證內容 |
|---|---|
| `TestAssembleEngine_Ping` | `assembleEngine().ServeHTTP(rec, GET /ping)` → 200 "pong" |
| `TestAssembleEngine_Health` | `GET /healthz` → 200 |
| `TestAssembleEngine_Stats` | `GET /stats` → 200，body 為 Stats JSON |
| `TestAssembleEngine_NotFound` | `GET /nonexistent` → 404 |
| `TestRun_GracefulShutdown` | 啟動 `Run(ctx)`，0.5s 後 cancel ctx，驗證 2s 內 return nil |

**Shutdown 測試技巧**：用 `httptest.NewServer(assembleEngine())` + goroutine 包 `Run`，比直接呼叫 `Run` 易驗證 listener 綁定。

#### `metric/otel_metrics_test.go` 覆蓋點

| 測試函式 | 驗證內容 |
|---|---|
| `TestOtelMetrics_ProcessCounter_Caching` | 同 name 兩次呼叫回傳同一 instrument（指標物件相等） |
| `TestOtelMetrics_ProcessCounter_Naming` | `manualReader.Collect()` 取得 metric，名稱 = `"ticker_fetch"`（dot→underscore） |
| `TestOtelMetrics_RecordProcessWithDuration_Success` | 驗證：counter +1、histogram 收到 durationMs、attributes 完整且不含 `error_type` |
| `TestOtelMetrics_RecordProcessWithDuration_Failure` | 驗證：counter +1、histogram 收到 durationMs、attributes 含 `error_type="timeout"` |
| `TestOtelMetrics_RecordProcessWithDuration_NoDuration` | `durationMs=0` 時 histogram 不被呼叫（用 mock 或 instrument 收集） |

**白箱測試技巧**：

- 使用 `sdkmetric.NewManualReader` 抓取 metric
- 使用 `metric.NewMeterProvider(sdkmetric.WithReader(mr))` 建立隔離 provider
- **繞過 `NewOtelMetrics` 的 `otel.SetMeterProvider` 副作用**：用 `mp.Meter("gosdk")` 直接建構 `OtelMetrics{meter: ...}` 測試內部邏輯；或新增 `newOtelMetricsWithMeter(meter, jobName, instance)` 私有 helper

#### `metric/summary_test.go` 覆蓋點

| 測試函式 | 驗證內容 |
|---|---|
| `TestFormatBatchSummary_AllSucceed` | output 含 `"10 total"`, `"10 succeed"`, `"0 failed"`，不含 `took` 或 `failed items` |
| `TestFormatBatchSummary_WithFailures` | output 含失敗項目列表 |
| `TestFormatBatchSummary_WithDuration` | output 含 `(took 123.4ms)` |
| `TestFormatBatchSummary_Empty` | 全零值不 panic |

---

## 依賴驗證

實作前第一步：

```bash
grep -E "go.opentelemetry.io/otel/(attribute|metric|sdk|sdk/metric)" go.mod
```

若 `go.opentelemetry.io/otel/attribute` 缺席或版本錯位：

```bash
go get go.opentelemetry.io/otel/attribute@v1.44.0
go mod tidy
```

---

## 執行順序（提交粒度）

每步可單獨 `go test -race ./...` 通過。

1. **Step 1**：驗證 go.mod，補 `attribute` 依賴；`go mod tidy`
2. **Step 2**：`metric/summary.go` + `metric/summary_test.go`
3. **Step 3**：`metric/otel_metrics.go` + `metric/otel_metrics_test.go`
4. **Step 4**：`server/server.go` + `server/server_test.go`
5. **Step 5**：重寫 `main.go`
6. **Step 6**：`go test -race ./...`、`go vet ./...`、`gofmt -l .` 全綠
7. **Step 7**：更新 `README.todo` 階段四條目為 `[x]`

---

## 風險與緩解

| 風險 | 緩解 |
|---|---|
| `server.Run` 取代 `HTTPServer()` 為 breaking change | repo 內僅 `main.go` 單一呼叫；無外部 consumer；README.todo §server 已標待辦，視為已知 |
| `otel.SetMeterProvider` 全域汙染 | 文件明示「呼叫 `NewOtelMetrics` 會註冊全域 MeterProvider」；白箱測試用 `mp.Meter("gosdk")` 直注入 meter 結構欄位繞過副作用 |
| 多個 `OtelMetrics` 實例建立多份 `MeterProvider` 浪費 | 文件明示「同進程應只建立一個 `OtelMetrics` 實例」；`Shutdown` 由 owner 負責 |
| `assembleEngine` 私有導致測試覆蓋受限 | 採 package-internal 測試（`package server` 而非 `server_test`）即可直接呼叫 |
| `signal.NotifyContext` 與 `os/signal` 互斥 | main 內 `defer stop()` 確保 signal handler 被釋放 |
| 既有 `TestOtelSample` 整合測試在沙盒失敗 | 既有失敗已記錄於 `architecture-gosdk.md` §3-7；本次新測試採 NoopProvider 規避 |

---

## Verification

執行下列指令確認階段四完成：

```bash
# 1. 編譯
go build ./...

# 2. 既有測試不迴歸
go test -race ./... 2>&1 | tee /tmp/gosdk-test.log

# 3. 新測試覆蓋
go test -race -v ./metric/... ./server/... 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|FAIL|PASS|ok)"

# 4. 靜態分析
go vet ./...
gofmt -l .        # 應為空輸出

# 5. 啟動驗證
go run . &
SERVER_PID=$!
sleep 1
curl -s http://localhost:8080/ping       # 預期 {"message":"pong"}
curl -s http://localhost:8080/healthz    # 預期 200
curl -s http://localhost:8080/stats      # 預期 200 + JSON
kill -TERM $SERVER_PID
wait $SERVER_PID
echo "exit code: $?"                     # 預期 0

# 6. README.todo 更新
grep -E "^\- \[" README.todo | head -20
```

驗證標準：

- `go test -race ./...` 全綠
- `gofmt -l .` 無輸出
- `go vet ./...` 無警告
- `curl` 三條路由皆回預期
- 收 `SIGTERM` 後 1s 內退出且 exit code = 0
- `README.todo` 階段四條目改為 `[x]`
