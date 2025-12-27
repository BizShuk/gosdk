package encode

import (
	"io"
)

// Decoder defines the interface for character set decoding.
type Decoder interface {
	Decode() io.Reader
}
