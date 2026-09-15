package metric

import (
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

// SendTest pushes a small batch of backfilled samples for manual verification.
func (s *MetricService) SendTest() error {
	var metrics []Metric
	now := time.Now().Unix()

	for i := 0; i <= 60; i += 10 {
		ts := now - int64(i*60)
		metrics = append(metrics, Metric{
			Name:      "stock.analysis.test",
			Timestamp: ts,
			Value:     90.0 + float64(i)/2.0,
			Tags: map[string]string{
				"host":    "shuk-mac-mini",
				"project": "stock",
				"status":  "multi-value-test",
				"mood":    "flirty-yuna",
			},
		})
	}

	slog.Debug("sending test metrics to remote-write backend", "count", len(metrics))
	if err := s.SendMulti(metrics); err != nil {
		slog.Error("SendTest failed", "err", err)
		return err
	}
	slog.Debug("SendTest completed successfully")
	return nil
}

// decodeSeries decompresses and unmarshals the most recent remote-write
// body captured by cap, returning every time series it carried.
func decodeSeries(t *testing.T, cap *captureServer) []prompb.TimeSeries {
	t.Helper()
	body := cap.lastBody()
	if body == nil {
		t.Fatal("no metric emitted")
	}
	decompressed, err := snappy.Decode(nil, body)
	if err != nil {
		t.Fatalf("snappy decode: %v\nbody=%q", err, body)
	}
	req := &prompb.WriteRequest{}
	if err := req.Unmarshal(decompressed); err != nil {
		t.Fatalf("unmarshal: %v\ndecompressed=%q", err, decompressed)
	}
	return req.Timeseries
}

func TestMetricService_SendTest(t *testing.T) {
	cap := &captureServer{}
	ts := httptest.NewServer(cap.handler())
	t.Cleanup(ts.Close)

	svc := NewMetricService(ts.URL)
	if err := svc.SendTest(); err != nil {
		t.Fatalf("SendTest() error = %v", err)
	}

	series := decodeSeries(t, cap)
	if got, want := len(series), 7; got != want {
		t.Fatalf("time series: got %d, want %d", got, want)
	}

	for i, s := range series {
		labels := labelsToMap(&series[i])
		// SendMulti replaces "." with "_" to satisfy Prometheus naming.
		if got, want := labels["__name__"], "stock_analysis_test"; got != want {
			t.Errorf("series %d name: got %q, want %q", i, got, want)
		}
		if got, want := labels["project"], "stock"; got != want {
			t.Errorf("series %d project label: got %q, want %q", i, got, want)
		}
		if len(s.Samples) != 1 {
			t.Errorf("series %d: got %d samples, want 1", i, len(s.Samples))
		}
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    float64
		wantErr bool
	}{
		{"float64", float64(1.23), 1.23, false},
		{"float32", float32(4.56), float64(float32(4.56)), false},
		{"int", int(10), 10.0, false},
		{"int64", int64(20), 20.0, false},
		{"int32", int32(30), 30.0, false},
		{"int16", int16(40), 40.0, false},
		{"int8", int8(50), 50.0, false},
		{"uint", uint(60), 60.0, false},
		{"uint64", uint64(70), 70.0, false},
		{"uint32", uint32(80), 80.0, false},
		{"uint16", uint16(90), 90.0, false},
		{"uint8", uint8(100), 100.0, false},
		{"string invalid", "1.23", 0, true},
		{"bool invalid", true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toFloat64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("toFloat64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("toFloat64() got = %v, want %v", got, tt.want)
			}
		})
	}
}
