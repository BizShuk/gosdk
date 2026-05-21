package encode

import (
	"testing"
)

func TestDecodeGBKBytes(t *testing.T) {
	// GBK bytes for "中文"
	gbkBytes := []byte{0xd6, 0xd0, 0xce, 0xc4}
	res, err := DecodeGBKBytes(gbkBytes)
	if err != nil {
		t.Fatalf("Failed to decode GBK bytes: %v", err)
	}
	if string(res) != "中文" {
		t.Errorf("Expected 中文, got %s", string(res))
	}
}

func TestDecodeBig5Bytes(t *testing.T) {
	// Big5 bytes for "中文"
	big5Bytes := []byte{0xa4, 0xa4, 0xa4, 0xe5}
	res, err := DecodeBig5Bytes(big5Bytes)
	if err != nil {
		t.Fatalf("Failed to decode Big5 bytes: %v", err)
	}
	if string(res) != "中文" {
		t.Errorf("Expected 中文, got %s", string(res))
	}
}
