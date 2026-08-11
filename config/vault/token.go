package vault

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 這個檔案回答一個實務問題:解鎖一次之後,能不能拿到一個`會過期`的憑證,
// 之後用它解密而不必重打主密碼?
//
// # 為什麼不能只用時間戳
//
// 純粹由時間推導出來的 token(TOTP 那一類)在密碼學上`無法`用來解密:
// 能解密就代表 token 帶著金鑰資訊,而如果金鑰單純從時間算得出來,任何人都算得
// 出來。TOTP 是身分驗證機制,不是加密金鑰機制。
//
// # 實際的做法:包裹 DEK
//
// 真正加密資料的是 DEK(見 vault.go 的兩層金鑰)。token 就是 DEK 的`另一個
// 包裹`,只是這個包裹的金鑰綁定了到期時間:
//
//	exp      = now + ttl                                   // 寫進 token 明文段
//	tokenKey = HKDF-SHA256(deviceKey, info = "…|" + exp)
//	token    = base64url( exp ‖ nonce ‖ AES-GCM(tokenKey, DEK, AAD = exp) )
//
// exp 同時參與`金鑰衍生`與`AAD`,所以把 token 裡的到期時間改長是無效的——
// 改了就衍生出不同的金鑰,解不開。deviceKey 是存在本機的隨機金鑰檔
// (見 [DefaultDeviceKeyPath]),不在 token 裡,所以 token 單獨外洩者無法使用。
//
// # 威脅模型(請照實理解)
//
// 在沒有硬體支援的單機環境裡,`過期是軟性防護`:過期檢查由本程序執行,同時取得
// token`與`deviceKey 的攻擊者可以繞過檢查、用當時的 exp 直接解開。它真正防的是
// token 單獨外洩(例如出現在 CI log 或 shell history)後被長期利用。
//
// 要讓過期具有強制力,有三條升級路徑,依成本排序:
//
//  1. [RotateDeviceKey] 定期輪換裝置金鑰,一次讓所有既發 token 失效。
//  2. 把 DEK 交給 TPM / macOS Keychain / 雲端 KMS,由外部元件判斷過期。
//  3. agent 模式(類似 ssh-agent):常駐程序持有 DEK 並設 TTL,CLI 透過 unix
//     socket 請求解密,到期即清除——這種過期是真的,因為金鑰只存在於該程序。
//
// 本套件實作的是第 1 條;2 與 3 需要本機以外的元件,不屬於這一層。

// TOKEN_INFO 是 HKDF 的 info 前綴,把衍生出的金鑰綁定在「vault token」這個用途
// 上,避免同一把 deviceKey 在別處被衍生出相同的金鑰。
const TOKEN_INFO = "gosdk-vault-token-v1"

// DEVICE_KEY_ENV 可覆寫裝置金鑰檔的位置,讓宿主應用把它放進自己的設定目錄。
const DEVICE_KEY_ENV = "VAULT_DEVICE_KEY_FILE"

// DEVICE_KEY_FILE 是裝置金鑰的預設檔名。
const DEVICE_KEY_FILE = "vault-device.key"

// expLen 是 token 前綴中 Unix 秒數的位元組長度。
const expLen = 8

var (
	ErrTokenExpired = errors.New("vault: token 已過期")
	ErrBadToken     = errors.New("vault: token 無效或與本機裝置金鑰不符")
	ErrNoDeviceKey  = errors.New("vault: 找不到本機裝置金鑰")
)

// ---------------------------------------------------------------- 裝置金鑰

// DefaultDeviceKeyPath 回傳裝置金鑰的預設路徑:
// $VAULT_DEVICE_KEY_FILE,否則 <XDG_CONFIG_HOME|~/.config>/gosdk/vault-device.key。
//
// 這裡自行解析設定根目錄而不呼叫 config.GetAppConfigDir():config 套件 import
// 本套件,反向依賴會形成循環。裝置金鑰本來也不隨應用程式而異——它代表`這台機器
// 上的這個使用者`——所以固定在 gosdk 目錄下反而是對的預設值。
func DefaultDeviceKeyPath() string {
	if p := strings.TrimSpace(os.Getenv(DEVICE_KEY_ENV)); p != "" {
		return p
	}
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return DEVICE_KEY_FILE
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gosdk", DEVICE_KEY_FILE)
}

// LoadDeviceKey 讀取裝置金鑰;檔案不存在時回傳 ErrNoDeviceKey。
//
// 讀取路徑`不會`順手建立金鑰:自動生成一把新的只會讓 token 驗證失敗,卻在磁碟上
// 留下一個看起來正常的檔案,把「沒發過 token」誤導成「token 壞掉」。
func LoadDeviceKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNoDeviceKey, path)
	}
	if err != nil {
		return nil, err
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("%w:長度 %d,預期 %d", ErrNoDeviceKey, len(key), keyLen)
	}
	return key, nil
}

// EnsureDeviceKey 讀取裝置金鑰,不存在時產生一把(目錄 0700、檔案 0600)。
// 發放 token 的路徑用這個。
func EnsureDeviceKey(path string) ([]byte, error) {
	key, err := LoadDeviceKey(path)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, ErrNoDeviceKey) {
		return nil, err
	}
	return RotateDeviceKey(path)
}

// RotateDeviceKey 產生一把新的裝置金鑰並覆寫檔案,`一次讓所有既發 token 失效`。
// 這是本套件對「撤銷」的答案:token 沒有中央註冊表,能作廢的只有解開它們的金鑰。
func RotateDeviceKey(path string) ([]byte, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	key := make([]byte, keyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// ---------------------------------------------------------------- token

// IssueToken 發出一個 ttl 之後失效的 token,持有者可用它解鎖本保險庫而不需要
// 主密碼。ttl 必須為正數。
//
// token 是敏感資料:它等同於「到期前的主密碼」。請像對待密碼一樣處理它,
// 不要寫進 log 或版本控制。
func (v *Vault) IssueToken(deviceKey []byte, ttl time.Duration) (string, error) {
	if v.aead == nil {
		return "", ErrClosed
	}
	if len(deviceKey) != keyLen {
		return "", ErrNoDeviceKey
	}
	if ttl <= 0 {
		return "", errors.New("vault: token 的存活時間必須為正數")
	}
	return v.issueTokenAt(deviceKey, time.Now().Add(ttl))
}

// issueTokenAt 以明確的到期時間發 token。測試用它產生一個`已經過期`的 token,
// 不必等待也不必注入時鐘。
func (v *Vault) issueTokenAt(deviceKey []byte, exp time.Time) (string, error) {
	expBytes := make([]byte, expLen)
	binary.BigEndian.PutUint64(expBytes, uint64(exp.Unix()))

	tokenKey, err := deriveTokenKey(deviceKey, expBytes)
	if err != nil {
		return "", err
	}
	defer Wipe(tokenKey)

	aead, err := newAEAD(tokenKey)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nil, nonce, v.dek, expBytes)

	raw := make([]byte, 0, len(expBytes)+len(nonce)+len(ct))
	raw = append(raw, expBytes...)
	raw = append(raw, nonce...)
	raw = append(raw, ct...)
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// OpenWithToken 以 token 解鎖 vault 內容,不需要主密碼。
// token 過期回傳 ErrTokenExpired,與本機裝置金鑰不符回傳 ErrBadToken。
func OpenWithToken(data, deviceKey []byte, token string) (*Vault, error) {
	f, salt, err := parseFile(data)
	if err != nil {
		return nil, err
	}
	dek, err := unwrapToken(deviceKey, token)
	if err != nil {
		return nil, err
	}
	v, err := newVault(dek, f.DEK, salt, f.KDF, f.Secrets)
	if err != nil {
		return nil, err
	}
	// token 解出的 DEK 必須真的屬於這份檔案:同一台機器上的兩個 vault,
	// A 的 token 不該打得開 B。任一祕密解不開就是配錯了。
	if err := v.checkDEK(); err != nil {
		v.Close()
		return nil, err
	}
	return v, nil
}

// OpenFileWithToken 是 OpenWithToken 的檔案版本。
func OpenFileWithToken(path string, deviceKey []byte, token string) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return OpenWithToken(data, deviceKey, token)
}

// TokenExpiry 讀出 token 標示的到期時間,`不驗證`任何東西——它只解析明文前綴,
// 供 CLI 顯示「還剩多久」。真正的判定發生在 OpenWithToken。
func TokenExpiry(token string) (time.Time, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(raw) < expLen {
		return time.Time{}, ErrBadToken
	}
	return time.Unix(int64(binary.BigEndian.Uint64(raw[:expLen])), 0), nil
}

// unwrapToken 檢查到期時間並解出 DEK。
func unwrapToken(deviceKey []byte, token string) ([]byte, error) {
	if len(deviceKey) != keyLen {
		return nil, ErrNoDeviceKey
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(raw) < expLen+1 {
		return nil, ErrBadToken
	}
	expBytes := raw[:expLen]
	exp := time.Unix(int64(binary.BigEndian.Uint64(expBytes)), 0)
	if time.Now().After(exp) {
		return nil, fmt.Errorf("%w(於 %s)", ErrTokenExpired, exp.Format(time.RFC3339))
	}

	tokenKey, err := deriveTokenKey(deviceKey, expBytes)
	if err != nil {
		return nil, err
	}
	defer Wipe(tokenKey)

	aead, err := newAEAD(tokenKey)
	if err != nil {
		return nil, err
	}
	n := aead.NonceSize()
	if len(raw) < expLen+n {
		return nil, ErrBadToken
	}
	dek, err := aead.Open(nil, raw[expLen:expLen+n], raw[expLen+n:], expBytes)
	if err != nil {
		return nil, ErrBadToken
	}
	return dek, nil
}

// deriveTokenKey 由裝置金鑰與到期時間衍生 token 專用金鑰。exp 進入 info,
// 因此每個到期時間都是一把不同的金鑰——竄改 exp 等於換一把解不開的鑰匙。
func deriveTokenKey(deviceKey, expBytes []byte) ([]byte, error) {
	info := fmt.Sprintf("%s|%d", TOKEN_INFO, binary.BigEndian.Uint64(expBytes))
	return hkdf.Key(sha256.New, deviceKey, nil, info, keyLen)
}

// checkDEK 確認手上的 DEK 能解開這份檔案的內容。空的保險庫沒有可驗證的密文,
// 視為通過:一個沒有祕密的 vault 本來就沒有「配錯」這回事。
func (v *Vault) checkDEK() error {
	for k := range v.secrets {
		b, err := v.GetBytes(k)
		if err != nil {
			return ErrBadToken
		}
		Wipe(b)
		return nil
	}
	return nil
}
