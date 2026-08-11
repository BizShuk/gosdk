package vault

import (
	"errors"
	"testing"
)

func encodeVault(t *testing.T, password []byte, entries map[string]any) []byte {
	t.Helper()
	data, err := Codec{Password: password}.Encode(entries)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return data
}

func TestCodecRoundTrip(t *testing.T) {
	data := encodeVault(t, []byte("pw"), map[string]any{
		"API_KEY": "sk-123",
		"PORT":    8080,
		"DEBUG":   true,
	})

	got := map[string]any{}
	if err := (Codec{Password: []byte("pw")}).Decode(data, got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// vault 只存字串:非字串的值以 JSON 文字形式 round trip。
	want := map[string]any{"API_KEY": "sk-123", "PORT": "8080", "DEBUG": "true"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("Decode[%q] = %v, want %v", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("Decode 回傳 %d 個 key, want %d", len(got), len(want))
	}
}

func TestCodecWrongPassword(t *testing.T) {
	data := encodeVault(t, []byte("right"), map[string]any{"A": "1"})

	err := (Codec{Password: []byte("wrong")}).Decode(data, map[string]any{})
	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("Decode 錯誤密碼 err = %v, want ErrWrongPassword", err)
	}
}

func TestCodecNoCredential(t *testing.T) {
	if err := (Codec{}).Decode([]byte("{}"), map[string]any{}); !errors.Is(err, ErrNoCredential) {
		t.Errorf("Decode 無憑證 err = %v, want ErrNoCredential", err)
	}
	if _, err := (Codec{}).Encode(map[string]any{}); !errors.Is(err, ErrNoCredential) {
		t.Errorf("Encode 無憑證 err = %v, want ErrNoCredential", err)
	}
}

func TestCodecRejectsPlainDotenv(t *testing.T) {
	err := (Codec{Password: []byte("pw")}).Decode([]byte("A=1\nB=2\n"), map[string]any{})
	if !errors.Is(err, ErrBadFormat) {
		t.Fatalf("Decode 明文 .env err = %v, want ErrBadFormat", err)
	}
}

// 每次 Encode 都用新的鹽與 nonce,所以位元組必然不同,但都能解回同一份內容。
func TestCodecEncodeIsNotDeterministic(t *testing.T) {
	entries := map[string]any{"A": "1"}
	first := encodeVault(t, []byte("pw"), entries)
	second := encodeVault(t, []byte("pw"), entries)

	if string(first) == string(second) {
		t.Error("兩次 Encode 產生相同位元組,表示 nonce 或鹽沒有重新產生")
	}
	for _, data := range [][]byte{first, second} {
		got := map[string]any{}
		if err := (Codec{Password: []byte("pw")}).Decode(data, got); err != nil || got["A"] != "1" {
			t.Fatalf("Decode = %v, err = %v", got, err)
		}
	}
}
