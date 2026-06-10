package metric

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/castai/promwrite"
	"github.com/spf13/viper"
)

// MetricService pushes metrics to any Prometheus remote-write compatible
// backend. Backends differ only in the endpoint URL.
//
// Reference endpoints — remote write (this service, REMOTE_WRITE_URL) vs
// OTLP HTTP (otel.go, METRIC_URL):
//
//	VictoriaMetrics
//	  remote write: http://localhost:8428/api/v1/write
//	  OTLP:         http://localhost:8428/opentelemetry/v1/metrics
//	Mimir
//	  remote write: http://localhost:9009/api/v1/push
//	  OTLP:         http://localhost:9009/otlp/v1/metrics
//	Prometheus
//	  remote write: http://localhost:9090/api/v1/write           (--web.enable-remote-write-receiver)
//	  OTLP:         http://localhost:9090/api/v1/otlp/v1/metrics (--web.enable-otlp-receiver)
type MetricService struct {
	client *promwrite.Client
}

var globalMetricService *MetricService

func init() {
	viper.SetDefault("REMOTE_WRITE_URL", "http://localhost:8428/api/v1/write")
}

// NewMetricService creates a service for the given remote-write endpoint.
// An empty url falls back to the REMOTE_WRITE_URL config (default: VictoriaMetrics).
func NewMetricService(url string) *MetricService {
	if url == "" {
		url = viper.GetString("REMOTE_WRITE_URL")
	}
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	return &MetricService{
		client: promwrite.NewClient(url, promwrite.HttpClient(httpClient)),
	}
}

func sanitizeMetricName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int16:
		return float64(val), nil
	case int8:
		return float64(val), nil
	case uint:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case uint16:
		return float64(val), nil
	case uint8:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported metric value type: %T", v)
	}
}

func (s *MetricService) SendMulti(metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	var req promwrite.WriteRequest
	for _, m := range metrics {
		val, err := toFloat64(m.Value)
		if err != nil {
			return err
		}

		labels := []promwrite.Label{
			{Name: "__name__", Value: sanitizeMetricName(m.Name)},
		}
		for k, v := range m.Tags {
			labels = append(labels, promwrite.Label{Name: k, Value: v})
		}

		req.TimeSeries = append(req.TimeSeries, promwrite.TimeSeries{
			Labels: labels,
			Sample: promwrite.Sample{
				Time:  time.Unix(m.Timestamp, 0),
				Value: val,
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := s.client.Write(ctx, &req); err != nil {
		return err
	}

	return nil
}

func (s *MetricService) Send(metric Metric) error {
	return s.SendMulti([]Metric{metric})
}

// Send sends metrics to the remote-write backend configured by
// REMOTE_WRITE_URL via IMetric interface. Point REMOTE_WRITE_URL at another
// backend (e.g. Mimir's :9009/api/v1/push) to switch targets.
func Send[T IMetric](metrics []T) error {
	if len(metrics) == 0 {
		return nil
	}

	if globalMetricService == nil {
		globalMetricService = NewMetricService("")
	}

	const batchSize = 50
	for i := 0; i < len(metrics); i += batchSize {
		end := min(i+batchSize, len(metrics))

		var toSend []Metric
		for _, m := range metrics[i:end] {
			toSend = append(toSend, m.ConvertToMetric()...)
		}

		if err := globalMetricService.SendMulti(toSend); err != nil {
			return err
		}
	}

	return nil
}
