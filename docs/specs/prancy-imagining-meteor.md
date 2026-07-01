# Process Metrics Spec - 通用程序監控規格

## Context

用户需要一個通用 metrics 系統，能夠：
1. 追蹤所有類型的程序（批次處理、服務監控、佇列/Worker）
2. 同時支援靜態維度（程式碼中定義）+ 動態 Tag（runtime 產生）
3. 失敗時立即通知 + 批次完成後總結通知

目標是建立一個 **跨語言的 Metric Tag 規格**，讓 Python 和 Go 都能用相同的結構追蹤。

---

## 1. Metric 命名與類型

### 1.1 Counter（計數器）- 事件統計

```promql
# Naming: {domain}.{operation}.{status}
# Domain: process, service, queue
# Status: success, failure, timeout, cancelled

# 批次處理 ticker
process.ticker.fetch{status="success"}
process.ticker.fetch{status="failure"}

# 佇列任務
queue.job.execute{status="success|failed|timeout"}

# 服務 API
service.api.request{status="success|failure", method="GET|POST"}
```

### 1.2 Histogram（分布）- 延遲/耗時

```promql
# Naming: {domain}.{operation}.duration
process.ticker.fetch.duration
queue.job.execute.duration
service.api.request.duration
```

### 1.3 Gauge（指標）- 現存量

```promql
# Naming: {domain}.{resource}.current
queue.worker.current    # 目前 worker 數量
queue.job.pending.current # 等待中的任務
```

---

## 2. Standard Tag Keys（標準維度）

### 2.1 通用維度（所有類型共享）

| Tag Key | 說明 | 範例值 |
|---|---|---|
| `domain` | 程序領域 | `process`, `service`, `queue` |
| `service` | 服務名稱 | `ticker-fetch`, `user-api` |
| `instance` | 實例識別 | `host:port`, `worker-1` |
| `version` | 版本（可選） | `v1.2.3` |

### 2.2 批次處理維度

| Tag Key | 說明 | 範例值 |
|---|---|---|
| `job_name` | 任務名稱 | `import-csv`, `export-report` |
| `ticker` | 標的物/項目 | `AAPL`, `BTC-USD` |
| `status` | 執行狀態 | `success`, `failure`, `skipped` |
| `error_type` | 錯誤分類（失敗時） | `timeout`, `not_found`, `auth_failed` |

### 2.3 佇列/Worker 維度

| Tag Key | 說明 | 範例值 |
|---|---|---|
| `worker_id` | Worker 識別 | `worker-1`, `consumer-3` |
| `job_type` | 任務類型 | `email`, `webhook`, `sync` |
| `queue_name` | 佇列名稱 | `email-queue`, `task-queue` |
| `status` | 執行狀態 | `success`, `failure`, `timeout`, `retry` |

### 2.4 服務/網路維度

| Tag Key | 說明 | 範例值 |
|---|---|---|
| `endpoint` | API 端點 | `/api/users`, `/healthz` |
| `method` | HTTP 方法 | `GET`, `POST`, `PUT` |
| `status_code` | HTTP 狀態碼 | `200`, `404`, `500` |
| `source` | 來源服務 | `client`, `internal` |

---

## 3. 失敗通知時機

### 3.1 即時通知（On-failure）
- 子流程失敗時立即發送
- 包含：失敗的 sub-process identifier + 錯誤類型 + 錯誤訊息

### 3.2 批次完成通知（Batched Summary）
- 批次完成後一次性發送
- 包含：總數、成功數、失敗數、失敗列表、總耗時

---

## 4. 實作檔案

### 4.1 Go（本專案）
- `notify/slack.go` — 已存在
- 新增 `metric/metric.go` — OpenTelemetry metrics 封裝

### 4.2 Python
- 新增 `notify/slack.py` — 已建立
- 新增 `notify/metric.py` — OpenTelemetry metrics 封裝

### 4.3 共同規格文件
- `SPEC.md` — Metric Tag Keys/Values 定義（可供團隊共享）

---

## 5. Verification

1. Python 測試：`python -m pytest notify/ -v`
2. Go 測試：`go test -v ./notify/...`
3. 手動驗證：建立一個 mock ticker fetch 迴圈，模擬失敗並確認 Slack 收到通知
