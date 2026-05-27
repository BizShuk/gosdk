# Process Metrics Specification

## Overview

通用程序監控規格，支援批次處理（Batch）、服務監控（Service）、佇列/Worker（Queue）三種領域的 metrics 追蹤。

## Metric Naming Convention

### Counter（計數器）

```
{domain}.{name}.{operation}
```

| Domain    | Example                | Description  |
| --------- | ---------------------- | ------------ |
| `process` | `process.ticker.fetch` | 批次資料處理 |
| `queue`   | `queue.job.execute`    | 佇列任務執行 |
| `service` | `service.api.request`  | API/服務請求 |

### Histogram（分布）

```
{domain}.{name}.{operation}.duration
```

### Gauge（指標）

```
{domain}.{resource}.current
```

---

## Standard Tag Keys

### 通用維度（所有領域共享）

| Tag Key    | Type   | Description   | Example                                    |
| ---------- | ------ | ------------- | ------------------------------------------ |
| `job_name` | string | 任務/服務名稱 | `ticker-fetch`, `user-api`                 |
| `instance` | string | 實例識別      | `host:port`, `worker-1`                    |
| `status`   | string | 執行狀態      | `success`, `failure`, `timeout`, `skipped` |

### 批次處理維度（process.\*）

| Tag Key      | Type   | Description        | Example                               |
| ------------ | ------ | ------------------ | ------------------------------------- |
| `ticker`     | string | 標的物/項目ID      | `AAPL`, `BTC-USD`                     |
| `error_type` | string | 錯誤分類（失敗時） | `timeout`, `not_found`, `auth_failed` |

### 佇列/Worker 維度（queue.\*）

| Tag Key      | Type   | Description | Example                     |
| ------------ | ------ | ----------- | --------------------------- |
| `worker_id`  | string | Worker 識別 | `worker-1`, `consumer-3`    |
| `job_type`   | string | 任務類型    | `email`, `webhook`, `sync`  |
| `queue_name` | string | 佇列名稱    | `email-queue`, `task-queue` |

### 服務/網路維度（service.\*）

| Tag Key       | Type   | Description | Example                        |
| ------------- | ------ | ----------- | ------------------------------ |
| `endpoint`    | string | API 端點    | `/api/users`, `/healthz`       |
| `method`      | string | HTTP 方法   | `GET`, `POST`, `PUT`, `DELETE` |
| `status_code` | string | HTTP 狀態碼 | `200`, `404`, `500`            |
| `source`      | string | 來源服務    | `client`, `internal`           |

---

## Status Values

| Status    | Description |
| --------- | ----------- |
| `success` | 執行成功    |
| `failure` | 執行失敗    |
| `timeout` | 逾時        |
| `skipped` | 跳過        |
| `retry`   | 重試中      |

---

## Error Types

| Error Type    | Description   |
| ------------- | ------------- |
| `timeout`     | 網路/作業逾時 |
| `not_found`   | 資源不存在    |
| `auth_failed` | 認證失敗      |
| `rate_limit`  | 速率限制      |
| `validation`  | 驗證失敗      |
| `internal`    | 內部錯誤      |

---

## Usage Examples

### Go

```go
// 初始化
m, _ := metric.NewOtelMetrics("ticker-fetch", "instance-1", mimirService)

// 建立 Counter
counter, _ := m.ProcessCounter("ticker", "fetch")
histogram, _ := m.ProcessHistogram("ticker", "fetch")

// 記錄成功
m.RecordProcessWithDuration(counter, histogram, "AAPL", "success", "", 150.5)

// 記錄失敗
m.RecordProcessWithDuration(counter, histogram, "AAPL", "failure", "timeout", 3000.0)

// 批次完成通知
summary := metric.BatchSummary{
    Total: 100, Succeed: 95, Failed: 5,
    FailedList: []string{"AAPL-timeout", "GOOG-rate_limit"},
}
notifier.Notify(ctx, metric.FormatBatchSummary("ticker-fetch", summary))
```

### Python

```python
# 初始化
m = OtelMetrics("ticker-fetch", "instance-1")

# 記錄成功
m.record_process("AAPL", "success", error_type="")

# 記錄失敗
m.record_process("AAPL", "failure", error_type="timeout")

# 批次完成通知
summary = BatchSummary(
    total=100, succeed=95, failed=5,
    failed_list=["AAPL-timeout", "GOOG-rate_limit"]
)
notifier.notify(format_batch_summary("ticker-fetch", summary))
```

---

## Query Examples (PromQL)

```promql
# 查詢失敗率最高的 ticker
sum by (ticker) (rate(process_ticker_fetch{status="failure"}[5m]))

# 查詢平均處理時間
histogram_quantile(0.95, sum by (ticker) (rate(process_ticker_fetch_duration_bucket[5m])))

# 查詢佇列失敗任務
sum by (queue_name, job_type) (rate(queue_job_execute{status="failure"}[5m]))
```

---

## Files

| File              | Description                        |
| ----------------- | ---------------------------------- |
| `metric/otel.go`  | Go OpenTelemetry metrics 封裝      |
| `metric/otel.py`  | Python OpenTelemetry metrics 封裝  |
| `metric/mimir.go` | Mimir client 實作                  |
| `metric/model.go` | Metric 資料結構                    |
| `notify/slack.go` | Go Slack 通知器                     |
| `notify/slack.py` | Python Slack 通知器                 |
| `notify/example.py` | 使用範例（Python）             |

---

## External Import（外部 repo 使用）

### Go

```go
import "github.com/bizshuk/gosdk/metric"
```

### Python

```bash
# 開發時（local path）
pip install -e /path/to/gosdk/metric

# 或設定 PYTHONPATH
export PYTHONPATH="/path/to/gosdk/metric:$PYTHONPATH"
```

```python
from metric.otel import OtelMetrics, BatchSummary, format_batch_summary
```

### 發布後（建議日後拆分）

```bash
pip install gosdk-metric
```
