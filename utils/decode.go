package utils

import (
	"bytes"
	"io"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// convert GBK to UTF-8
func Decodegbk(s []byte) ([]byte, error) {
	I := bytes.NewReader(s)
	O := transform.NewReader(I, simplifiedchinese.GBK.NewDecoder())
	d, e := io.ReadAll(O)
	if e != nil {
		return nil, e
	}
	return d, nil
}

// convert BIG5 to UTF-8
func Decodebig5(s []byte) ([]byte, error) {
	I := bytes.NewReader(s)
	O := transform.NewReader(I, traditionalchinese.Big5.NewDecoder())
	d, e := io.ReadAll(O)
	if e != nil {
		return nil, e
	}
	return d, nil
}

// convert GBK reader to UTF-8 reader
func DecodegbkReader(r io.ReadCloser) io.Reader {
	return transform.NewReader(r, simplifiedchinese.GBK.NewDecoder())
}

// convert BIG5 reader to UTF-8 reader
func Decodebig5Reader(r io.ReadCloser) io.Reader {
	return transform.NewReader(r, traditionalchinese.Big5.NewDecoder())
}
