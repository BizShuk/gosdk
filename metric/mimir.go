package metric

import "github.com/spf13/viper"

func init() {
	viper.SetDefault("MIMIR_URL", "http://localhost:9009/api/v1/push")
}

// MimirService is a type alias for MetricService targeting Mimir.
// Mimir and VictoriaMetrics share the same remote-write protocol; only the
// endpoint URL differs (:9009/api/v1/push vs :8428/api/v1/write).
type MimirService = MetricService

// NewMimirService creates a MetricService targeting Mimir, honoring the
// MIMIR_URL config (default: http://localhost:9009/api/v1/push).
func NewMimirService() *MimirService {
	return NewMetricService(viper.GetString("MIMIR_URL"))
}
