"""
OtelMetrics provides process/queue/service metrics with OpenTelemetry.
Python version of metric/otel.go pattern.
"""

from dataclasses import dataclass, field
from typing import Optional
from opentelemetry import metrics
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.resource import ResourceAttributes


@dataclass
class BatchSummary:
    """Holds the result of a batch process."""
    total: int = 0
    succeed: int = 0
    failed: int = 0
    failed_list: list = field(default_factory=list)
    duration_ms: float = 0.0


class OtelMetrics:
    """Provides metrics for process/queue/service operations."""

    def __init__(self, job_name: str, instance: str):
        self.job_name = job_name
        self.instance = instance

        # Initialize OpenTelemetry Meter
        resource = Resource.create({
            ResourceAttributes.SERVICE_NAME: "gosdk",
            "job_name": job_name,
            "instance": instance,
        })
        provider = MeterProvider(resource=resource)
        metrics.set_meter_provider(provider)

        self.meter = provider.get_meter("gosdk")

        # Counters
        self._process_counter = self.meter.create_counter(
            name="process.count",
            description="Batch process counter",
            unit="1",
        )
        self._queue_counter = self.meter.create_counter(
            name="queue.count",
            description="Queue job counter",
            unit="1",
        )
        self._service_counter = self.meter.create_counter(
            name="service.count",
            description="Service API counter",
            unit="1",
        )

        # Histograms
        self._process_histogram = self.meter.create_histogram(
            name="process.duration",
            description="Batch process duration",
            unit="ms",
        )
        self._queue_histogram = self.meter.create_histogram(
            name="queue.duration",
            description="Queue job duration",
            unit="ms",
        )
        self._service_histogram = self.meter.create_histogram(
            name="service.duration",
            description="Service request duration",
            unit="ms",
        )

    def record_process(
        self,
        ticker: str,
        status: str,
        error_type: str = "",
        duration_ms: float = 0.0,
    ) -> None:
        """Record a process event with standard tags."""
        attributes = {
            "job_name": self.job_name,
            "instance": self.instance,
            "ticker": ticker,
            "status": status,
        }
        if error_type:
            attributes["error_type"] = error_type

        self._process_counter.add(1, attributes)
        if duration_ms > 0:
            self._process_histogram.record(duration_ms, attributes)

    def record_queue(
        self,
        worker_id: str,
        job_type: str,
        queue_name: str,
        status: str,
        duration_ms: float = 0.0,
    ) -> None:
        """Record a queue event with standard tags."""
        attributes = {
            "job_name": self.job_name,
            "instance": self.instance,
            "worker_id": worker_id,
            "job_type": job_type,
            "queue_name": queue_name,
            "status": status,
        }

        self._queue_counter.add(1, attributes)
        if duration_ms > 0:
            self._queue_histogram.record(duration_ms, attributes)

    def record_service(
        self,
        endpoint: str,
        method: str,
        status_code: str,
        source: str = "client",
        duration_ms: float = 0.0,
    ) -> None:
        """Record a service event with standard tags."""
        attributes = {
            "job_name": self.job_name,
            "instance": self.instance,
            "endpoint": endpoint,
            "method": method,
            "status_code": status_code,
            "source": source,
        }

        self._service_counter.add(1, attributes)
        if duration_ms > 0:
            self._service_histogram.record(duration_ms, attributes)


def format_batch_summary(job_name: str, summary: BatchSummary) -> str:
    """Format a BatchSummary into a notification string."""
    parts = [
        f"{job_name} batch complete:",
        f"{summary.total} total,",
        f"{summary.succeed} succeeded,",
        f"{summary.failed} failed",
    ]
    if summary.failed_list:
        parts.append(f"[{', '.join(summary.failed_list)}]")
    return " ".join(parts)
