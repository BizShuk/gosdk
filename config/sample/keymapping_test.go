// keymapping_test.go 用可執行的測試釘住 gosdk/config 的 key 對應規則。
//
// 這些規則常被誤解，最常見的誤解是「a.b.c 在 .env 或 yaml 裡會變成 a_b_c」。
// 事實正好相反：
//
//   - "." 是 viper 的 key 分隔符，代表巢狀層級。
//   - "_" 只是一般字元，不會被拆開。
//   - 因此 a.b.c 與 a_b_c 是兩個完全不同、互不相通的 key。
//
// 唯一會做 "_" ↔ "." 轉換的地方是 viper 的 EnvKeyReplacer，而 config.Default()
// 並未設定它（見 config/config.go）。所以 APP_ 環境變數只能覆蓋扁平 key
// （APP_A_B_C → a_b_c），無法覆蓋巢狀 key（a.b.c）。
//
// 執行:
//
//	go test ./config/sample/ -run TestKey -v
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/viper"
)

// loadFixture 在暫存目錄建立設定檔並跑一次 config.Default()。
//
// config.Default() 寫入的是全域 viper 單例，所以每個測試前後都必須 viper.Reset()，
// 這些測試也因此不能 t.Parallel()。
func loadFixture(t *testing.T, files map[string]string) {
	t.Helper()

	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	t.Chdir(dir)

	viper.Reset()
	config.SetAppName("")
	t.Cleanup(func() {
		viper.Reset()
		config.SetAppName("")
	})

	config.Default()
}

// assertGet 斷言 viper.Get(key) 的值。want 傳 nil 代表「這個 key 不該存在」。
func assertGet(t *testing.T, key string, want any) {
	t.Helper()
	if got := viper.Get(key); got != want {
		t.Errorf("viper.Get(%q) = %#v, want %#v", key, got, want)
	}
}

// TestKeyDotenvUnderscoreStaysFlat 驗證 .env 的底線寫法維持扁平。
//
// A_B_C=xyz 產生的是「一個」名為 a_b_c 的 key，不是三層巢狀結構。
// 用 a.b.c 讀取一定拿到 nil。
func TestKeyDotenvUnderscoreStaysFlat(t *testing.T) {
	loadFixture(t, map[string]string{
		".env": "A_B_C=underscore-form\n",
	})

	assertGet(t, "a_b_c", "underscore-form") // ✅ 唯一正確的讀法
	assertGet(t, "a.b.c", nil)               // ❌ 底線不會被當成分隔符
	assertGet(t, "a.b", nil)
	assertGet(t, "a", nil)
}

// TestKeyDotenvDotBecomesNested 驗證 .env 也支援 "." —— 而且會被還原成巢狀。
//
// dotenv 的 key 允許含 "."，viper 讀進來後 AllSettings() 會把它展開成
// map[a][b][c]，與 yaml 的巢狀縮排完全等價。
func TestKeyDotenvDotBecomesNested(t *testing.T) {
	loadFixture(t, map[string]string{
		".env": "X.Y.Z=dot-form\n",
	})

	assertGet(t, "x.y.z", "dot-form") // ✅ 巢狀路徑讀得到
	assertGet(t, "x_y_z", nil)        // ❌ 這是另一個 key，不存在

	// 中間層真的是 map，不是字串。
	if _, ok := viper.Get("x.y").(map[string]any); !ok {
		t.Errorf("viper.Get(%q) = %#v, want map[string]any", "x.y", viper.Get("x.y"))
	}
}

// TestKeyYamlFlatAndNestedCoexist 驗證同一份 yaml 裡，扁平與巢狀是兩個獨立的 key。
//
// 這也解釋了為什麼本專案的扁平慣例（SQLITE_PATH、LOG_LEVEL）可以和巢狀的
// server.host 和平共存 —— 它們本來就不在同一個命名空間。
func TestKeyYamlFlatAndNestedCoexist(t *testing.T) {
	loadFixture(t, map[string]string{
		"config.yaml": "" +
			"a:\n" +
			"  b:\n" +
			"    c: nested-yaml\n" +
			"a_b_c: flat-yaml\n" +
			"SQLITE_PATH: ./sample.db\n",
	})

	assertGet(t, "a.b.c", "nested-yaml") // 巢狀縮排
	assertGet(t, "a_b_c", "flat-yaml")   // 扁平單層，與上面互不干擾
	assertGet(t, "sqlite_path", "./sample.db")
}

// TestKeyLookupIsCaseInsensitive 驗證 viper 的 key 查詢不分大小寫。
//
// 所以設定檔可以維持 SCREAMING_SNAKE_CASE 的可讀性（SQLITE_PATH），
// 程式端不論用哪種大小寫都讀得到。
func TestKeyLookupIsCaseInsensitive(t *testing.T) {
	loadFixture(t, map[string]string{
		"config.yaml": "SQLITE_PATH: ./sample.db\nServer:\n  Host: localhost\n",
	})

	assertGet(t, "SQLITE_PATH", "./sample.db")
	assertGet(t, "sqlite_path", "./sample.db")
	assertGet(t, "SeRvEr.HoSt", "localhost")
}

// TestKeyEnvOverridesFlatAndNestedKeys 驗證 OS 環境變數（如 A_B_C / SERVER_PORT）
// 可同時覆蓋扁平 key 與巢狀 key（由 bindAllEnvVars 進行 BindEnv）。
func TestKeyEnvOverridesFlatAndNestedKeys(t *testing.T) {
	t.Setenv("A_B_C", "from-envvar")

	loadFixture(t, map[string]string{
		"settings.local.json": `{"a_b_c":"from-file","a":{"b":{"c":"nested-from-file"}}}`,
	})

	// 扁平 key 與巢狀 key：環境變數覆蓋皆生效。
	assertGet(t, "a_b_c", "from-envvar")
	assertGet(t, "a.b.c", "from-envvar")
}

// TestKeySourcePrecedence 釘住六個設定檔的合併順序。
//
// 跨格式設定優先權（後載入者覆蓋先載入者）：
//
//	config.yaml < config.local.yaml < settings.json < settings.local.json < .env < .env.local < OS 環境變數
func TestKeySourcePrecedence(t *testing.T) {
	loadFixture(t, map[string]string{
		"config.yaml":         "shared: 3-yaml\nonly_yaml: yaml\n",
		"config.local.yaml":   "shared: 4-yaml-local\n",
		"settings.json":       `{"shared":"5-json","only_json":"json"}`,
		"settings.local.json": `{"shared":"6-json-local","only_json_local":"jsonlocal"}`,
		".env":                "SHARED=1-env\nONLY_ENV=env\n",
		".env.local":          "SHARED=2-env-local\n",
	})

	// .env.local 是檔案層的最後一關，勝出。
	assertGet(t, "shared", "2-env-local")

	// 各來源獨有的 key 都仍在，證明六個檔案確實都被載入了。
	assertGet(t, "only_env", "env")
	assertGet(t, "only_yaml", "yaml")
	assertGet(t, "only_json", "json")
	assertGet(t, "only_json_local", "jsonlocal")
}

// TestKeyEnvFileWinsOverOtherFiles 單獨釘住 .env.local / .env 檔案層最高優先權。
func TestKeyEnvFileWinsOverOtherFiles(t *testing.T) {
	loadFixture(t, map[string]string{
		"config.local.yaml":   "server:\n  host: from-yaml-local\n",
		"settings.json":       `{"server":{"host":"from-json"}}`,
		"settings.local.json": `{"server":{"host":"from-json-local"}}`,
		".env.local":          "SERVER.HOST=from-env-local\n",
	})

	assertGet(t, "server.host", "from-env-local")
}

// TestKeyEmptyParentIsInvisibleToViper 記錄一個 --delete 的副作用。
//
// cmd/config.go 的 --delete 只刪最底層的 key，保留變成空 map 的 parent。
// 但 viper 的 AllKeys() 不會為空 map 產生任何 key —— 也就是說，settings.local.json
// 裡留下的 "a": {"b": {}} 在檔案裡看得到，在 viper 裡完全不存在。
func TestKeyEmptyParentIsInvisibleToViper(t *testing.T) {
	loadFixture(t, map[string]string{
		"settings.local.json": `{"a":{"b":{}},"kept":"value"}`,
	})

	assertGet(t, "a.b.c", nil)
	assertGet(t, "a.b", nil) // 空 map 不產生 key
	assertGet(t, "kept", "value")

	for _, k := range viper.AllKeys() {
		if k == "a" || k == "a.b" {
			t.Errorf("viper.AllKeys() 不該包含空 map 產生的 key %q，實際: %v", k, viper.AllKeys())
		}
	}
}
