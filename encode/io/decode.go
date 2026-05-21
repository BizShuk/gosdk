package encode

import (
	"bytes"
	"io"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// Decoder defines the interface for character set decoding.
type Decoder interface {
	Decode() io.Reader
}

// DecodeGBKBytes converts GBK bytes to UTF-8 bytes
func DecodeGBKBytes(s []byte) ([]byte, error) {
	r := bytes.NewReader(s)
	tr := transform.NewReader(r, simplifiedchinese.GBK.NewDecoder())
	return io.ReadAll(tr)
}

// DecodeBig5Bytes converts Big5 bytes to UTF-8 bytes
func DecodeBig5Bytes(s []byte) ([]byte, error) {
	r := bytes.NewReader(s)
	tr := transform.NewReader(r, traditionalchinese.Big5.NewDecoder())
	return io.ReadAll(tr)
}
