package metric

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/castai/promwrite"
	"github.com/spf13/viper"
)

type MimirService struct {
	client *promwrite.Client
}

func NewMimirService() *MimirService {
	mimirURL := viper.GetString("MIMIR_URL")
	if mimirURL == "" {
		mimirURL = "http://localhost:9009/api/v1/push"
	}
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	return &MimirService{
		client: promwrite.NewClient(mimirURL, promwrite.HttpClient(httpClient)),
	}
}

func sanitizeMetricName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func (s *MimirService) SendMulti(metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	var req promwrite.WriteRequest
	for _, m := range metrics {
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
				Value: m.Value,
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

func (s *MimirService) Send(metric Metric) error {
	return s.SendMulti([]Metric{metric})
}
