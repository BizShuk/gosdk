package metric

type Metric struct {
	Name      string
	Timestamp int64
	Value     float64
	Tags      map[string]string
}