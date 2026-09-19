package metric

import "time"

// IMetric is an interface for types that can be converted to Metric
type IMetric interface {
	ConvertToMetric() []Metric
}

type Metric struct {
	Name string
	// Timestamp is the sample time in unix seconds. Ignored when Time is set.
	Timestamp int64
	// Time is the sample time at millisecond precision (the remote-write
	// resolution). When non-zero it takes precedence over Timestamp, so
	// sub-second samples of one series no longer collapse onto one point.
	Time  time.Time
	Value any
	Tags  map[string]string
}

// sampleTime resolves the time sent on the wire: Time first, then Timestamp.
func (m Metric) sampleTime() time.Time {
	if !m.Time.IsZero() {
		return m.Time
	}
	return time.Unix(m.Timestamp, 0)
}

func (m Metric) ConvertToMetric() []Metric {
	return []Metric{m}
}
