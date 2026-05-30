# 實作計畫 (Implementation Plan) — 將 metric 套件重構並提取 SDK 核心

## 目的 (Goal Description)

我們需要對 `metric` 套件進行重構，將 `metric/metric.go` 中的通用 `OpenTelemetry` 初始化及關閉邏輯（SDK 部分）與應用層（Service/Application Layer）的指標（如 port 檢測的 Gauge 與 `UpdateStatuses`）進行解耦提取。此外，我們將在 `metric/otel_test.go` 中提供一個應用層如何使用此 SDK 的單元測試驗證範例。

> [!NOTE]
> 根據使用者指示，套件名稱與目錄名稱均保留為 `metric`，且核心 SDK 初始化邏輯已整併至 `metric/otel.go`。

## 使用者審查 (User Review Required)

> [!IMPORTANT]
>
> 1. 我們保留整個目錄與 Go 套件名稱為 `metric`，這避免了大規模修改 Go/Python 導入路徑。
> 2. 原未追蹤之 `metric/metric.go` 中的 `UpdateStatuses`、`statusGauge` 與 `latencyGauge` 將被徹底從 SDK 程式碼中移除，改在 `metric/otel_test.go` 中以單元測試形式實作。
> 3. 需要在 `go.mod` 中新增缺失的 `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`、`otlptrace/otlptracehttp` 與 `sdk/trace` 依賴。

## 調整內容 (Proposed Changes)

---

### SDK 目錄與套件重構

保持目錄 `metric/` 名稱，並將所有 Go 檔案套件名稱設為 `package metric`。

---

### SDK 核心實作

#### [MODIFY] [otel.go](file:///Users/shuk/projects/gosdk/metric/otel.go)

- 實作通用 `OpenTelemetry` SDK 初始化與關閉功能：
    - `InitMeterProvider(ctx context.Context) error` (建立 `MeterProvider`、註冊 global 實例並設置 periodic reader，預設自 viper 讀取 `MIMIR_URL`，若為空則跳過初始化)
    - `InitTracerProvider(ctx context.Context, tempoURL string) error` (建立 `TracerProvider`、註冊 global 實例並設置 batcher，若為空則預設自 viper 讀取 `TEMPO_URL`)
    - `ShutdownOTel(ctx context.Context) error` (同時關閉 global meter 與 tracer 實例以確保數據完整輸出)
    - `Meter(name string, opts ...metric.MeterOption) metric.Meter` (封裝 OTel global meter 的獲取方法，解決同名套件之命名衝突)
    - `Tracer(name string, opts ...trace.TracerOption) trace.Tracer` (封裝 OTel global tracer 的獲取方法)
    - 這些函數不包含任何具體的 port 狀態指標（如 `statusGauge`、`latencyGauge`）。

---

### 應用層範例與測試

#### [NEW] [otel_test.go](file:///Users/shuk/projects/gosdk/metric/otel_test.go)

- 建立範例測試/應用層驗證。
- 定義 `PortStatus` 結構。
- 在 `TestOtelSample` 函數中呼叫 `InitMeterProvider(...)` 與 `InitTracerProvider(...)` 進行初始化。
- 定義並註冊自訂指標 `statusGauge` 與 `latencyGauge`（使用 `Meter`）。
- 實作 `UpdateStatuses` 邏輯，並模擬定時記錄指標，同時使用 `Tracer` 建立 Trace Spans。
- 結束時呼叫 `ShutdownOTel(...)` 確保關閉。

---

### 依賴與設定檔更新

#### [MODIFY] [go.mod](file:///Users/shuk/projects/gosdk/go.mod)

- 使用 `go get` 新增 OTLP HTTP exporter 與 trace SDK 依賴。

#### [MODIFY] [CLAUDE.md](file:///Users/shuk/projects/gosdk/CLAUDE.md)

- 更新專案結構，加入 `otel.go` 作為 OTel SDK 封裝入口的說明。

#### [MODIFY] [SPEC.md](file:///Users/shuk/projects/gosdk/SPEC.md)

- 更新 Go/Python 的套件名稱與範例導入路徑。

---

## 驗證計畫 (Verification Plan)

### 自動化測試 (Automated Tests)

- 執行 `go test -v -run=TestOtelSample ./metric/...` 驗證套件功能與單元測試流程與 traces

