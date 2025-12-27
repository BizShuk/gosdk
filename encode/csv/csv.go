package encode

// Decoder defines the interface for parsing a raw CSV row into a struct.
type Decoder interface {
	Decode(row []string) error
}
