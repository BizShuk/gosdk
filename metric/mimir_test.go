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
