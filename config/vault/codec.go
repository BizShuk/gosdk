package vault

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNoCredential 表示呼叫端既沒給主密碼也沒給 token:vault 沒有無憑證的讀寫
// 路徑,缺憑證是設定錯誤而不是空的保險庫。
var ErrNoCredential = errors.New("vault: 未提供主密碼或 token")

// Codec 把一份 .env.vault 當成`扁平 map`來解密與加密。
//
// 方法組合與 viper.Codec 一致,因此設定載入器可以直接把它註冊成一種格式,
// 而本套件不需要 import viper:
//
//	reg.RegisterCodec("vault", vault.Codec{Password: pw})
//
// Codec 是 Vault 之上的`文件層`:Vault 面向「逐一存取某個祕密」,Codec 面向
// 「整份檔案換成一張表」——這正是設定系統需要的粒度。也因為如此,Codec 一定會
// 把所有值變成無法清除的 string;只要一兩個祕密的呼叫端請走 [Vault.GetBytes]。
//
// vault 沒有巢狀結構,所有值都是字串。Decode 出來的 map 一律 flat,
// Encode 時非字串的值會轉成 JSON 文字後再加密。
type Codec struct {
	// Password 是主密碼。
	Password []byte

	// Token 是 [Vault.IssueToken] 發出的限時憑證,設定後`優先於` Password:
	// 呼叫端特地提供一個範圍更小、會過期的憑證,就是希望用它。
	// 使用 Token 時 DeviceKey 必填。
	Token     string
	DeviceKey []byte
}

// Decode 解密整份 vault 並填入 v。憑證錯誤時回傳 ErrWrongPassword 或
// ErrBadToken,內容不是 vault 檔時回傳 ErrBadFormat。
func (c Codec) Decode(b []byte, v map[string]any) error {
	vault, err := c.open(b)
	if err != nil {
		return err
	}
	defer vault.Close()

	entries, err := vault.DecryptAll()
	if err != nil {
		return err
	}
	for k, val := range entries {
		v[k] = val
	}
	return nil
}

// Encode 以 Password 建立一份新的 vault(隨機新鹽、新 DEK)並輸出檔案內容。
//
// 只接受主密碼:token 是既有 DEK 的包裹,拿它「建立一份新的保險庫」沒有意義,
// 那會產出一份沒有任何密碼能開啟的檔案。
//
// 每次 Encode 都重新衍生金鑰與 nonce,所以同樣的輸入不會產生相同的位元組——
// 這是加密的必然結果,不是不穩定的輸出。
func (c Codec) Encode(v map[string]any) ([]byte, error) {
	if len(c.Password) == 0 {
		return nil, ErrNoCredential
	}
	vault, err := New(c.Password)
	if err != nil {
		return nil, err
	}
	defer vault.Close()

	for k, val := range v {
		if err := vault.Set(k, stringifyValue(val)); err != nil {
			return nil, err
		}
	}
	return vault.Marshal()
}

// open 依 Codec 持有的憑證解鎖保險庫。
func (c Codec) open(b []byte) (*Vault, error) {
	switch {
	case c.Token != "":
		return OpenWithToken(b, c.DeviceKey, c.Token)
	case len(c.Password) > 0:
		return Open(b, c.Password)
	default:
		return nil, ErrNoCredential
	}
}

// stringifyValue 把任意值轉成 vault 能存放的字串形式:字串原樣保留,
// 其餘型別走 JSON,讓數字與布林值 round trip 後仍可辨識。
func stringifyValue(val any) string {
	if s, ok := val.(string); ok {
		return s
	}
	b, err := json.Marshal(val)
	if err != nil {
		return fmt.Sprintf("%v", val)
	}
	return string(b)
}
