package encode

import (
	"io"

	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

type big5Decoder struct {
	r io.Reader
}

// NewBig5Decoder creates a new Decoder for Big5 encoded content.
func NewBig5Decoder(r io.Reader) Decoder {
	return &big5Decoder{r: r}
}

func (d *big5Decoder) Decode() io.Reader {
	return transform.NewReader(d.r, traditionalchinese.Big5.NewDecoder())
}
