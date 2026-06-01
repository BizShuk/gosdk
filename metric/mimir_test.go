package metric

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func (s *MimirService) SendTest() error {
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

	zap.S().Infof("sending %d test metrics to Mimir", len(metrics))
	if err := s.SendMulti(metrics); err != nil {
		zap.S().Errorf("SendTest failed: %v", err)
		return err
	}
	zap.S().Infof("SendTest completed successfully")
	return nil
}

func TestMimirService_SendTest(t *testing.T) {
	svc := NewMimirService()
	if err := svc.SendTest(); err != nil {
		t.Errorf("SendTest() error = %v", err)
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
