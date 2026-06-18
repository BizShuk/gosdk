package utils

import (
	"math/rand/v2"
)

func StringPointer(s string) *string {
	return &s
}

const CHARSET = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

func String(length int) string {
	return StringWithCharset(length, CHARSET)
}
