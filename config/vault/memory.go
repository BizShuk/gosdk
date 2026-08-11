package vault

import "runtime"

// 這個檔案處理一個問題:解密完之後,祕密還留在記憶體裡多久。
//
// # 能保證什麼、不能保證什麼
//
// Go 沒辦法讓你完全掌控祕密的記憶體。三個限制是本質性的,不是這裡少寫了什麼:
//
//  1. string 不可變:祕密一旦成為 string,就`無法`覆寫,只能等 GC 回收,
//     而回收不會歸零。
//  2. GC 會搬移與複製:[]byte 轉 string、map 寫入、slice 成長都可能留下你
//     摸不到的副本。
//  3. AES 金鑰排程:aes.NewCipher(key) 會把金鑰展開成 round key 存在 cipher
//     物件內,清掉原始 key 不會清掉展開後的排程——因此 [Vault.Close] 除了
//     Wipe,還把 aead 設為 nil 讓整個物件可被回收。
//
// 所以這裡的目標不是「絕對清除」,而是`把可清除的範圍最大化`,並讓 API 的形狀
// 引導呼叫端做正確的事:
//
//   - 主密碼全程走 []byte(term.ReadPassword 本來就回 []byte),衍生出 KEK 後
//     立刻 Wipe——這是最有價值的一段,因為主密碼是所有其他祕密的上游。
//   - [Vault.GetBytes] 讓「只要一個值」的呼叫端拿到可清除的位元組,
//     而不是被迫走 [Vault.DecryptAll]。
//   - [Vault.Close] 清除 DEK 並讓保險庫停止服務。
//
// 需要更強的保證(mlock 防止換頁到磁碟、guard page、canary)就得引入
// memguard 之類的套件並處理各平台的 RLIMIT_MEMLOCK;在明文終究會交給 viper 或
// 環境變數的前提下,那層額外複雜度買到的東西有限,因此`刻意不做`。
//
// 真正的結論是設計層面的:祕密解密後餵進 viper、os.Setenv 或 log 就離開了控制
// 範圍,所以「用到才解密單一值、用完即清」比「啟動時全部攤開」更值得追求。

// Wipe 把位元組歸零,用於清除金鑰材料或解密後的祕密。
//
// runtime.KeepAlive 不是裝飾:清零迴圈的結果沒有人讀,編譯器有權把整段消掉,
// 這一行讓 b 在迴圈結束前保持存活,清零就不能被視為無用而移除。
func Wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}

// GetBytes 解密單一變數並回傳`可清除`的位元組;呼叫端用完應自行 [Wipe]。
//
// 與 [Vault.Get] 的差別只在型別,而型別就是重點:string 交出去之後你再也拿不
// 回那塊記憶體。
func (v *Vault) GetBytes(key string) ([]byte, error) {
	if v.aead == nil {
		return nil, ErrClosed
	}
	blob, ok := v.secrets[key]
	if !ok {
		return nil, ErrNotFound
	}
	return openWith(v.aead, key, blob)
}

// Close 清除記憶體中的 DEK 並停用保險庫。之後任何讀寫都回 [ErrClosed]。
//
// 可重複呼叫,方便直接 defer。已加密的 secrets 不清除:它們是密文,留著不構成
// 洩漏,而清掉會讓「Close 之後仍可 Marshal」這種誤用變成靜默的資料遺失。
func (v *Vault) Close() {
	if v == nil {
		return
	}
	Wipe(v.dek)
	v.dek = nil
	v.aead = nil
}
