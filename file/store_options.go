package file

import "os"

// 選項預設值。
const (
	DEFAULT_DIR_PERM  os.FileMode = 0o755
	DEFAULT_FILE_PERM os.FileMode = 0o644
	DEFAULT_EXT       string      = ".json"
)

// Options 是 Store 的可調參數。
type Options struct {
	// DirPerm 是建立目錄時的權限。
	DirPerm os.FileMode
	// FilePerm 是建立檔案時的權限。
	FilePerm os.FileMode
	// Ext 是檔名副檔名,含前導點。缺點會自動補上。
	Ext string
	// Atomic 決定 Write 是否走 temp + rename。預設開啟。
	Atomic bool
	// DecodeHook 在 json.Unmarshal 之前改寫原始位元組,供 schema 遷移使用。
	DecodeHook func([]byte) ([]byte, error)
}

// Option 以函式選項模式調整 Options。刻意不帶型別參數,否則呼叫端得寫
// file.WithDirPerm[Cred](0o700),每個選項都要重複一次型別名。
type Option func(*Options)

// WithDirPerm 設定目錄權限。
func WithDirPerm(m os.FileMode) Option { return func(o *Options) { o.DirPerm = m } }

// WithFilePerm 設定檔案權限。
func WithFilePerm(m os.FileMode) Option { return func(o *Options) { o.FilePerm = m } }

// WithExt 設定副檔名,前導點可省略。
func WithExt(ext string) Option { return func(o *Options) { o.Ext = ext } }

// WithAtomicWrite 決定 Write 是否走 temp + rename。
func WithAtomicWrite(on bool) Option { return func(o *Options) { o.Atomic = on } }

// WithDecodeHook 註冊解碼前的位元組改寫函式。
func WithDecodeHook(fn func([]byte) ([]byte, error)) Option {
	return func(o *Options) { o.DecodeHook = fn }
}

// defaultOptions 回傳套用任何 Option 之前的基準設定。
func defaultOptions() Options {
	return Options{
		DirPerm:  DEFAULT_DIR_PERM,
		FilePerm: DEFAULT_FILE_PERM,
		Ext:      DEFAULT_EXT,
		Atomic:   true,
	}
}
