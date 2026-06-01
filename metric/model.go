package metric

// IMetric is an interface for types that can be converted to Metric
type IMetric interface {
	ConvertToMetric() Metric
}

type Metric struct {
	Name      string
	Timestamp int64
	Value     any
	Tags      map[string]string
}
