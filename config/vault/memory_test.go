package vault

import (
	"bytes"
	"errors"
	"testing"
)

func TestWipeZeroesBytes(t *testing.T) {
	b := []byte("super-secret")
	Wipe(b)

	if !bytes.Equal(b, make([]byte, len(b))) {
		t.Errorf("Wipe 後 = %v, want 全 0", b)
	}
}

// Close 的重點:DEK 不再留在記憶體裡,而且保險庫停止服務。
func TestCloseWipesTheDataKey(t *testing.T) {
	v, err := New([]byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("A", "1"); err != nil {
		t.Fatal(err)
	}

	dek := v.dek
	before := append([]byte(nil), dek...)
	v.Close()

	if bytes.Equal(dek, before) {
		t.Error("Close 沒有清除 DEK")
	}
	if !bytes.Equal(dek, make([]byte, len(dek))) {
		t.Errorf("DEK = %v, want 全 0", dek)
	}
	if v.aead != nil {
		t.Error("Close 後 aead 仍非 nil:AES 金鑰排程應該一併釋出")
	}
}

func TestClosedVaultRefusesWork(t *testing.T) {
	v, err := New([]byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("A", "1"); err != nil {
		t.Fatal(err)
	}
	v.Close()

	if err := v.Set("B", "2"); !errors.Is(err, ErrClosed) {
		t.Errorf("Set 後 err = %v, want ErrClosed", err)
	}
	if _, err := v.GetBytes("A"); !errors.Is(err, ErrClosed) {
		t.Errorf("GetBytes err = %v, want ErrClosed", err)
	}
	if _, err := v.Marshal(); !errors.Is(err, ErrClosed) {
		t.Errorf("Marshal err = %v, want ErrClosed", err)
	}
}

// 重複 Close 必須安全,否則 defer 就不能無腦使用。
func TestCloseIsIdempotent(t *testing.T) {
	v, err := New([]byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	v.Close()
}

func TestGetBytesReturnsWipeableCopy(t *testing.T) {
	v, err := New([]byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.Set("API_KEY", "sk-123"); err != nil {
		t.Fatal(err)
	}

	b, err := v.GetBytes("API_KEY")
	if err != nil {
		t.Fatalf("GetBytes: %v", err)
	}
	if string(b) != "sk-123" {
		t.Fatalf("GetBytes = %q, want sk-123", b)
	}
	Wipe(b)

	// 清掉回傳值不能影響保險庫內容:交出去的必須是副本,不是內部狀態。
	again, err := v.GetBytes("API_KEY")
	if err != nil || string(again) != "sk-123" {
		t.Errorf("第二次 GetBytes = %q, err = %v", again, err)
	}
}

func TestGetBytesMissingKey(t *testing.T) {
	v, err := New([]byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	if _, err := v.GetBytes("NOPE"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetBytes err = %v, want ErrNotFound", err)
	}
}

// New 不保留呼叫端密碼的參考:呼叫端 Wipe 之後保險庫仍然可用,
// 而同一個密碼(重新提供)仍然打得開。
func TestPasswordCanBeWipedByCaller(t *testing.T) {
	pw := []byte("master-password")
	v, err := New(pw)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("A", "1"); err != nil {
		t.Fatal(err)
	}
	data, err := v.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	Wipe(pw)

	reopened, err := Open(data, []byte("master-password"))
	if err != nil {
		t.Fatalf("清除密碼副本後無法開啟: %v", err)
	}
	defer reopened.Close()
	if got, _ := reopened.Get("A"); got != "1" {
		t.Errorf("A = %q, want 1", got)
	}
}
