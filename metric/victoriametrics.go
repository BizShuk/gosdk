package metric

import "github.com/spf13/viper"

func init() {
	viper.SetDefault("VICTORIAMETRICS_URL", "http://localhost:8428/api/v1/write")
}

// NewVictoriaMetricsService creates a MetricService targeting
// VictoriaMetrics, honoring the VICTORIAMETRICS_URL config if set.
func NewVictoriaMetricsService() *MetricService {
	return NewMetricService(viper.GetString("VICTORIAMETRICS_URL"))
}
