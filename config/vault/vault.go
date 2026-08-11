// Package vault 是本機祕密保險庫:讀取 .env 內容,以主密碼
// (scrypt 金鑰衍生 + AES-256-GCM)加密成 .env.vault,需要時再解密還原。
// .env.vault 可安全提交至版本控制。
//
// 本套件`只`負責加解密、檔案格式與 token;把 vault 接成應用程式設定來源的是
// 上一層的 config 套件:config/vault.go 以 [Codec] 註冊 "vault" 這個 viper 格式,
// 因此祕密不需要先解密成 .env 落地。
//
// 基本用法:
//
//	v, _ := vault.New([]byte("master-password"))
//	v.Set("API_KEY", "sk-123")
//	data, _ := v.Marshal()               // 寫入 .env.vault 的內容
//
//	v2, _ := vault.Open(data, []byte("master-password"))
//	val, _ := v2.Get("API_KEY")          // "sk-123"
//	v2.Close()                           // 清除記憶體中的金鑰
//
// # 兩層金鑰
//
// 主密碼`不`直接加密資料。密碼經 scrypt 衍生出 KEK(key-encryption key),
// KEK 只用來包裹一把隨機產生的 DEK(data-encryption key),真正加密每個值的是
// DEK。多這一層是為了讓「換一種方式取得 DEK」成為可能——[Vault.IssueToken]
// 發出的限時 token 就是 DEK 的另一個包裹,詳見 token.go。
//
// # 檔案格式
//
//	{
//	  "magic": "ENV-VAULT-2",
//	  "kdf": {"name": "scrypt", "n": 32768, "r": 8, "p": 1},
//	  "salt": "base64...",                 // KEK 的鹽
//	  "dek":  "base64(nonce+ct)",          // 以 KEK 包裹的 DEK
//	  "secrets": {"API_KEY": "base64(nonce+ct)", "...": "..."}
//	}
//
// # 安全設計
//
//   - scrypt(N=32768, r=8, p=1)由主密碼衍生 256-bit KEK;密碼與金鑰皆不落地。
//   - AES-256-GCM 驗證加密,每個值使用獨立隨機 nonce。
//   - 變數名稱作為 AAD:即使攻擊者把兩個密文互換,解密也會直接失敗。
//   - 包裹的 DEK 本身就是密碼驗證器:密碼錯誤時 GCM 驗證直接失敗,
//     不需要額外的 verifier 欄位,也不會解出亂碼。
//   - 變數`名稱`不加密([KeysOfFile] 免密碼即可列出),只有`值`受保護。
//   - 解密輸出的 .env 與 .env.vault 檔案權限一律為 0600。
//   - 忘記主密碼即無法解密——這是設計上的取捨。
//
// 記憶體中的祕密如何清除,見 memory.go 的說明與 [Wipe]、[Vault.Close]。
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// Magic 為檔案格式版本標記。
const Magic = "ENV-VAULT-2"

// 預設 scrypt 參數。
const (
	scryptN = 1 << 15 // 32768
	scryptR = 8
	scryptP = 1
	keyLen  = 32 // AES-256
	saltLen = 16
)

// dekAAD 是包裹 DEK 時使用的 AAD,讓被包裹的金鑰無法被當成某個變數的密文使用。
const dekAAD = "__dek__"

// 可供呼叫端判斷的錯誤。
var (
	ErrWrongPassword = errors.New("vault: 主密碼不正確或檔案已損毀")
	ErrNotFound      = errors.New("vault: 找不到指定的變數")
	ErrBadFormat     = errors.New("vault: 不是有效的 vault 檔案")
	ErrClosed        = errors.New("vault: 保險庫已關閉")
	ErrEmptyPassword = errors.New("vault: 密碼不可為空")
)

type kdfParams struct {
	Name string `json:"name"`
	N    int    `json:"n"`
	R    int    `json:"r"`
	P    int    `json:"p"`
}

// fileFormat 對應 .env.vault 的 JSON 結構。
type fileFormat struct {
	Magic   string            `json:"magic"`
	KDF     kdfParams         `json:"kdf"`
	Salt    string            `json:"salt"`
	DEK     string            `json:"dek"`
	Secrets map[string]string `json:"secrets"`
}

// Vault 代表一個已解鎖的保險庫。零值不可用,請透過 New、Open 或 OpenWithToken
// 建立;用完請呼叫 [Vault.Close]。
type Vault struct {
	aead cipher.AEAD
	dek  []byte // 明文 DEK:發 token 與 Close 清除時需要

	// wrappedDEK 是檔案裡那份以 KEK 包裹的 DEK。原樣保留,才能在`只用 token
	// 解鎖`的情況下仍然寫回檔案——重新包裹需要主密碼,而 token 路徑沒有它。
	wrappedDEK string

	salt    []byte
	kdf     kdfParams
	secrets map[string]string // 變數名稱 -> base64(nonce + ciphertext)
}

// ---------------------------------------------------------------- 建立與開啟

// New 以主密碼建立一個空的保險庫(自動產生隨機鹽與隨機 DEK)。
//
// password 由呼叫端擁有:本函式不會保留它的參考,衍生完 KEK 後即可 [Wipe]。
func New(password []byte) (*Vault, error) {
	if len(password) == 0 {
		return nil, ErrEmptyPassword
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	dek := make([]byte, keyLen)
	if _, err := rand.Read(dek); err != nil {
		return nil, err
	}

	kdf := kdfParams{Name: "scrypt", N: scryptN, R: scryptR, P: scryptP}
	wrapped, err := wrapDEK(password, salt, kdf, dek)
	if err != nil {
		return nil, err
	}
	return newVault(dek, wrapped, salt, kdf, nil)
}

// Open 解析 .env.vault 內容並以主密碼解鎖;密碼錯誤時回傳 ErrWrongPassword。
func Open(data, password []byte) (*Vault, error) {
	f, salt, err := parseFile(data)
	if err != nil {
		return nil, err
	}
	if len(password) == 0 {
		return nil, ErrEmptyPassword
	}

	kek, err := deriveKEK(password, salt, f.KDF)
	if err != nil {
		return nil, err
	}
	defer Wipe(kek)

	dek, err := unsealWith(kek, dekAAD, f.DEK)
	if err != nil {
		return nil, ErrWrongPassword
	}
	return newVault(dek, f.DEK, salt, f.KDF, f.Secrets)
}

// OpenFile 是 Open 的檔案版本。
func OpenFile(path string, password []byte) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Open(data, password)
}

// newVault 以明文 DEK 組出可用的 Vault。dek 的所有權轉移給 Vault,
// 由 [Vault.Close] 負責清除。
func newVault(dek []byte, wrappedDEK string, salt []byte, kdf kdfParams, secrets map[string]string) (*Vault, error) {
	aead, err := newAEAD(dek)
	if err != nil {
		return nil, err
	}
	if secrets == nil {
		secrets = map[string]string{}
	}
	return &Vault{
		aead:       aead,
		dek:        dek,
		wrappedDEK: wrappedDEK,
		salt:       salt,
		kdf:        kdf,
		secrets:    secrets,
	}, nil
}

// parseFile 檢查檔案結構並解出鹽。所有「這不是一份 vault」的判斷集中在這裡,
// 密碼路徑與 token 路徑才不會對格式錯誤給出不同的答案。
func parseFile(data []byte) (fileFormat, []byte, error) {
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil || f.Magic != Magic {
		return f, nil, ErrBadFormat
	}
	if f.KDF.Name != "scrypt" {
		return f, nil, fmt.Errorf("%w:不支援的 KDF %q", ErrBadFormat, f.KDF.Name)
	}
	if f.DEK == "" {
		return f, nil, ErrBadFormat
	}
	salt, err := base64.StdEncoding.DecodeString(f.Salt)
	if err != nil {
		return f, nil, ErrBadFormat
	}
	return f, salt, nil
}

// ---------------------------------------------------------------- 讀寫祕密

// Set 新增或更新一個變數(每次都使用新的隨機 nonce)。
func (v *Vault) Set(key, value string) error {
	if v.aead == nil {
		return ErrClosed
	}
	blob, err := sealWith(v.aead, key, []byte(value))
	if err != nil {
		return err
	}
	v.secrets[key] = blob
	return nil
}

// SetAll 一次寫入多個變數。
func (v *Vault) SetAll(entries map[string]string) error {
	for k, val := range entries {
		if err := v.Set(k, val); err != nil {
			return err
		}
	}
	return nil
}

// Get 解密單一變數;不存在時回傳 ErrNotFound。
//
// 回傳的 string 無法事後清除。祕密的生命週期若需要控制,請改用 [Vault.GetBytes]。
func (v *Vault) Get(key string) (string, error) {
	b, err := v.GetBytes(key)
	if err != nil {
		return "", err
	}
	defer Wipe(b)
	return string(b), nil
}

// Delete 移除一個變數。
func (v *Vault) Delete(key string) {
	delete(v.secrets, key)
}

// Keys 回傳所有變數名稱(排序後)。此操作不需解密。
func (v *Vault) Keys() []string {
	keys := make([]string, 0, len(v.secrets))
	for k := range v.secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Len 回傳變數數量。
func (v *Vault) Len() int { return len(v.secrets) }

// KeysOfFile 讀出 vault 檔內的變數名稱(排序後),`不需要主密碼`。
//
// 只有`值`被加密,名稱是明文——因此「這個應用程式需要哪些祕密」在一台無法解密的
// 機器上依然回答得出來。這是刻意保留的能力,不是格式的疏漏。
func KeysOfFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil || f.Magic != Magic {
		return nil, ErrBadFormat
	}
	keys := make([]string, 0, len(f.Secrets))
	for k := range f.Secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// DecryptAll 解密全部變數,回傳明文 map。
//
// 這是`最寬`的取用方式:map 裡的 string 之後無法清除。設定載入等需要整份內容的
// 場景才用它,只要一兩個值請用 [Vault.GetBytes]。
func (v *Vault) DecryptAll() (map[string]string, error) {
	out := make(map[string]string, len(v.secrets))
	for k := range v.secrets {
		val, err := v.Get(k)
		if err != nil {
			return nil, fmt.Errorf("解密 %s 失敗: %w", k, err)
		}
		out[k] = val
	}
	return out, nil
}

// ---------------------------------------------------------------- 序列化

// Marshal 輸出 .env.vault 的 JSON 內容(可安全提交至版本控制)。
//
// 包裹過的 DEK 原樣寫回,所以以 token 解鎖的保險庫一樣能存檔:改寫祕密不需要
// 主密碼,只有`更換`主密碼才需要。
func (v *Vault) Marshal() ([]byte, error) {
	if v.aead == nil {
		return nil, ErrClosed
	}
	f := fileFormat{
		Magic:   Magic,
		KDF:     v.kdf,
		Salt:    base64.StdEncoding.EncodeToString(v.salt),
		DEK:     v.wrappedDEK,
		Secrets: v.secrets,
	}
	return json.MarshalIndent(f, "", "  ")
}

// SaveFile 將保險庫寫入檔案(權限 0600)。
func (v *Vault) SaveFile(path string) error {
	data, err := v.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// ---------------------------------------------------------------- .env 工具

var envLine = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)

// ParseEnv 解析 .env 內容:支援註解、export 前綴與單/雙引號。
func ParseEnv(data []byte) map[string]string {
	result := map[string]string{}
	for raw := range strings.SplitSeq(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := envLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, value := m[1], strings.TrimSpace(m[2])
		if len(value) >= 2 && value[0] == value[len(value)-1] &&
			(value[0] == '\'' || value[0] == '"') {
			value = value[1 : len(value)-1]
		}
		result[key] = value
	}
	return result
}

// MarshalEnv 將明文 map 輸出成 .env 格式(key 排序,必要時加引號)。
func MarshalEnv(entries map[string]string) []byte {
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(quoteEnvValue(entries[k]))
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func quoteEnvValue(value string) string {
	if value == "" || strings.ContainsAny(value, " \t#'\"\\$") {
		escaped := strings.ReplaceAll(value, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return value
}

// ---------------------------------------------------------------- 加解密核心

// deriveKEK 以 scrypt 從主密碼衍生 key-encryption key。回傳的位元組由呼叫端
// 負責 [Wipe]。
func deriveKEK(password, salt []byte, p kdfParams) ([]byte, error) {
	return scrypt.Key(password, salt, p.N, p.R, p.P, keyLen)
}

// wrapDEK 以主密碼衍生的 KEK 包裹 DEK。
func wrapDEK(password, salt []byte, kdf kdfParams, dek []byte) (string, error) {
	kek, err := deriveKEK(password, salt, kdf)
	if err != nil {
		return "", err
	}
	defer Wipe(kek)

	aead, err := newAEAD(kek)
	if err != nil {
		return "", err
	}
	return sealWith(aead, dekAAD, dek)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealWith 加密單一值:隨機 nonce + AES-GCM,並以 aad 綁定用途或變數名稱,
// 防止密文被互相調換仍能成功解密。
func sealWith(aead cipher.AEAD, aad string, plaintext []byte) (string, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nil, nonce, plaintext, []byte(aad))
	return base64.StdEncoding.EncodeToString(append(nonce, ct...)), nil
}

func openWith(aead cipher.AEAD, aad, blob string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(blob)
	if err != nil || len(raw) < aead.NonceSize() {
		return nil, ErrBadFormat
	}
	n := aead.NonceSize()
	pt, err := aead.Open(nil, raw[:n], raw[n:], []byte(aad))
	if err != nil {
		return nil, ErrWrongPassword
	}
	return pt, nil
}

// unsealWith 以 key 直接解開一份包裹,省去呼叫端自行建立 AEAD。
func unsealWith(key []byte, aad, blob string) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return openWith(aead, aad, blob)
}
