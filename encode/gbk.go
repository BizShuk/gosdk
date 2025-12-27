package encode

import (
	"io"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type gbkDecoder struct {
	r io.Reader
}

// NewGBKDecoder creates a new Decoder for GBK encoded content.
func NewGBKDecoder(r io.Reader) Decoder {
	return &gbkDecoder{r: r}
}

func (d *gbkDecoder) Decode() io.Reader {
	return transform.NewReader(d.r, simplifiedchinese.GBK.NewDecoder())
}
