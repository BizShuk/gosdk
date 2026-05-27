"""
Notify package usage example demonstrating metric tracking and Slack notifications.
Run: python -m notify.example
"""

import time
from metric.otel import OtelMetrics, BatchSummary, format_batch_summary
from notify.slack import SlackNotifier


def fetch_ticker(ticker: str) -> str:
    """Mock ticker fetch - fails for specific tickers."""
    if ticker == "FAIL":
        raise TimeoutError(f"timeout fetching {ticker}")
    if ticker == "ERROR":
        raise ValueError(f"invalid ticker {ticker}")
    return f"data_for_{ticker}"


def main():
    # Configuration
    token = "your-slack-token"
    channel_id = "#your-channel"
    job_name = "ticker-fetch"
    instance = "worker-1"

    # Initialize notifiers
    notifier = SlackNotifier(token, channel_id)
    metrics = OtelMetrics(job_name, instance)

    tickers = ["AAPL", "GOOG", "MSFT", "FAIL", "TSLA", "ERROR"]
    summary = BatchSummary()

    print(f"Processing {len(tickers)} tickers...")

    for ticker in tickers:
        start = time.time()
        try:
            result = fetch_ticker(ticker)
            duration_ms = (time.time() - start) * 1000
            metrics.record_process(ticker, "success", duration_ms=duration_ms)
            summary.succeed += 1
            print(f"  ✓ {ticker}: {result}")
        except TimeoutError as e:
            duration_ms = (time.time() - start) * 1000
            metrics.record_process(ticker, "failure", error_type="timeout", duration_ms=duration_ms)
            summary.failed += 1
            summary.failed_list.append(f"{ticker}-timeout")
            notifier.notify(f"❌ {ticker} timeout: {e}")
            print(f"  ✗ {ticker}: timeout")
        except ValueError as e:
            duration_ms = (time.time() - start) * 1000
            metrics.record_process(ticker, "failure", error_type="validation", duration_ms=duration_ms)
            summary.failed += 1
            summary.failed_list.append(f"{ticker}-validation")
            notifier.notify(f"❌ {ticker} error: {e}")
            print(f"  ✗ {ticker}: validation error")

        summary.total += 1

    # Batch completion notification
    notifier.notify(format_batch_summary(job_name, summary))
    print(f"\nBatch complete: {summary.succeed}/{summary.total} succeeded")


if __name__ == "__main__":
    main()
