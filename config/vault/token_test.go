package vault

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// forgeExpiry rewrites the plaintext expiry prefix of a token, which is what
// an attacker holding one would try first.
func forgeExpiry(token string, exp time.Time) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	binary.BigEndian.PutUint64(raw[:expLen], uint64(exp.Unix()))
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// deviceKeyFixture points DefaultDeviceKeyPath at a temp file and returns a
// freshly generated device key.
func deviceKeyFixture(t *testing.T) (path string, key []byte) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "device.key")
	t.Setenv(DEVICE_KEY_ENV, path)

	key, err := EnsureDeviceKey(path)
	if err != nil {
		t.Fatalf("EnsureDeviceKey: %v", err)
	}
	return path, key
}

func sealedVault(t *testing.T, password string, entries map[string]string) ([]byte, *Vault) {
	t.Helper()
	v, err := New([]byte(password))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := v.SetAll(entries); err != nil {
		t.Fatalf("SetAll: %v", err)
	}
	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return data, v
}

func TestTokenUnlocksWithoutPassword(t *testing.T) {
	_, key := deviceKeyFixture(t)
	data, v := sealedVault(t, "pw", map[string]string{"API_KEY": "sk-123"})
	defer v.Close()

	token, err := v.IssueToken(key, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	opened, err := OpenWithToken(data, key, token)
	if err != nil {
		t.Fatalf("OpenWithToken: %v", err)
	}
	defer opened.Close()

	if got, _ := opened.Get("API_KEY"); got != "sk-123" {
		t.Errorf("API_KEY = %q, want sk-123", got)
	}
}

func TestTokenExpires(t *testing.T) {
	_, key := deviceKeyFixture(t)
	data, v := sealedVault(t, "pw", map[string]string{"A": "1"})
	defer v.Close()

	// 直接發一個到期時間在過去的 token,不必等待也不必注入時鐘。
	token, err := v.issueTokenAt(key, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("issueTokenAt: %v", err)
	}

	if _, err := OpenWithToken(data, key, token); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("OpenWithToken err = %v, want ErrTokenExpired", err)
	}
}

// 到期時間同時參與金鑰衍生與 AAD,所以把它改長不會讓 token 活得更久,
// 只會讓它解不開——這是「過期無法被單純竄改繞過」的那道保證。
func TestTokenExpiryCannotBeExtended(t *testing.T) {
	_, key := deviceKeyFixture(t)
	data, v := sealedVault(t, "pw", map[string]string{"A": "1"})
	defer v.Close()

	token, err := v.issueTokenAt(key, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("issueTokenAt: %v", err)
	}
	forged, err := forgeExpiry(token, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("forgeExpiry: %v", err)
	}

	// 過期檢查會放行(時間戳已被改成未來),真正擋下來的是金鑰不符。
	if _, err := OpenWithToken(data, key, forged); !errors.Is(err, ErrBadToken) {
		t.Fatalf("OpenWithToken err = %v, want ErrBadToken", err)
	}
}

func TestTokenRejectedByDifferentDeviceKey(t *testing.T) {
	_, key := deviceKeyFixture(t)
	data, v := sealedVault(t, "pw", map[string]string{"A": "1"})
	defer v.Close()

	token, err := v.IssueToken(key, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	other := make([]byte, keyLen)
	other[0] = 0xff
	if _, err := OpenWithToken(data, other, token); !errors.Is(err, ErrBadToken) {
		t.Fatalf("OpenWithToken err = %v, want ErrBadToken", err)
	}
}

// 同一台機器上的兩個 vault:A 的 token 不該打得開 B。
func TestTokenIsBoundToItsVault(t *testing.T) {
	_, key := deviceKeyFixture(t)
	_, a := sealedVault(t, "pw-a", map[string]string{"A": "1"})
	defer a.Close()
	dataB, b := sealedVault(t, "pw-b", map[string]string{"B": "2"})
	defer b.Close()

	token, err := a.IssueToken(key, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := OpenWithToken(dataB, key, token); !errors.Is(err, ErrBadToken) {
		t.Fatalf("A 的 token 打開了 B:err = %v, want ErrBadToken", err)
	}
}

func TestRotateDeviceKeyInvalidatesTokens(t *testing.T) {
	path, key := deviceKeyFixture(t)
	data, v := sealedVault(t, "pw", map[string]string{"A": "1"})
	defer v.Close()

	token, err := v.IssueToken(key, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	rotated, err := RotateDeviceKey(path)
	if err != nil {
		t.Fatalf("RotateDeviceKey: %v", err)
	}

	if _, err := OpenWithToken(data, rotated, token); !errors.Is(err, ErrBadToken) {
		t.Fatalf("輪換後 err = %v, want ErrBadToken", err)
	}
	// 主密碼不受影響:輪換的是 token 的包裹金鑰,不是 vault 本身。
	reopened, err := Open(data, []byte("pw"))
	if err != nil {
		t.Fatalf("輪換後主密碼失效: %v", err)
	}
	reopened.Close()
}

// 寫回檔案不需要主密碼:被包裹的 DEK 原樣保留,所以 token 也能改內容。
func TestTokenOpenedVaultCanBeSaved(t *testing.T) {
	_, key := deviceKeyFixture(t)
	data, v := sealedVault(t, "pw", map[string]string{"A": "1"})
	defer v.Close()
	token, err := v.IssueToken(key, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	opened, err := OpenWithToken(data, key, token)
	if err != nil {
		t.Fatalf("OpenWithToken: %v", err)
	}
	if err := opened.Set("B", "2"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	saved, err := opened.Marshal()
	opened.Close()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// 主密碼仍然打得開改過的檔案。
	reopened, err := Open(saved, []byte("pw"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer reopened.Close()
	if got, _ := reopened.Get("B"); got != "2" {
		t.Errorf("B = %q, want 2", got)
	}
}

func TestLoadDeviceKeyDoesNotCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.key")

	if _, err := LoadDeviceKey(path); !errors.Is(err, ErrNoDeviceKey) {
		t.Fatalf("LoadDeviceKey err = %v, want ErrNoDeviceKey", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("LoadDeviceKey 建立了金鑰檔:讀取路徑不該產生新金鑰")
	}
}

func TestEnsureDeviceKeyIsStableAndPrivate(t *testing.T) {
	path, first := deviceKeyFixture(t)

	second, err := EnsureDeviceKey(path)
	if err != nil {
		t.Fatalf("EnsureDeviceKey: %v", err)
	}
	if string(first) != string(second) {
		t.Error("EnsureDeviceKey 第二次回傳了不同的金鑰")
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", st.Mode().Perm())
	}
}

func TestDefaultDeviceKeyPathHonoursOverride(t *testing.T) {
	t.Setenv(DEVICE_KEY_ENV, "/tmp/custom-device.key")
	if got := DefaultDeviceKeyPath(); got != "/tmp/custom-device.key" {
		t.Errorf("DefaultDeviceKeyPath() = %q, want the override", got)
	}
}

func TestTokenExpiryReportsDeadline(t *testing.T) {
	_, key := deviceKeyFixture(t)
	_, v := sealedVault(t, "pw", map[string]string{"A": "1"})
	defer v.Close()

	want := time.Now().Add(90 * time.Minute)
	token, err := v.issueTokenAt(key, want)
	if err != nil {
		t.Fatalf("issueTokenAt: %v", err)
	}
	got, err := TokenExpiry(token)
	if err != nil {
		t.Fatalf("TokenExpiry: %v", err)
	}
	if got.Unix() != want.Unix() {
		t.Errorf("TokenExpiry = %v, want %v", got, want)
	}
}

func TestIssueTokenRejectsNonPositiveTTL(t *testing.T) {
	_, key := deviceKeyFixture(t)
	_, v := sealedVault(t, "pw", map[string]string{"A": "1"})
	defer v.Close()

	if _, err := v.IssueToken(key, 0); err == nil {
		t.Error("ttl=0 應該被拒絕")
	}
}
