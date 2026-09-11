# Metrics, notify, files

**Metrics.** Default `metric.Send([]T)` (global `MetricService`, `METRIC_URL`, batches of 50). Timestamp = epoch **seconds**. `.` in names → `_` automatically. Implement `IMetric` on domain types instead of flattening by hand. Explicit `NewMetricService(url)` / `NewVictoriaMetricsService()` only for a custom URL or tests — not `NewMimirService()`. OTel: `InitMeterProvider` / `InitTracerProvider` (`OTLP_METRIC_URL` / `OTLP_TRACE_URL`); always `defer metric.ShutdownOTel(ctx)`.

**Notify.** `Notifier.Notify(ctx, summary string)`. Slack token empty → no-op. `NewMulti` calls every notifier and `errors.Join`s.

**Files.** `csv.ProcessCSVFile` (`.archived` marker). `utils.WriteFile` / `OpenFile` with `WithCreate`, `WithBackup`, `WithReturnWriter`. Expand `~` with `homedir.Expand` inline — no wrapper.
