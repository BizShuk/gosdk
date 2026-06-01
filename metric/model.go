package metric

type Metric struct {
	Name      string
	Timestamp int64
	Value     any
	Tags      map[string]string
}
