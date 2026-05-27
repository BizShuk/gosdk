# 實作計畫：為 config 套件新增 ConfigOption 與 Option 模式

為了支援在載入設定時透過 Functional Option 模式 (functional options pattern) 進行自訂配置，我們將在 `config` 套件下新增 `option.go`。本計畫依據使用者需求，新增 `ConfigOption` 設計，提供 `WithConfigPath` 與 `WithDefaultValue` 選項。

## 變更項目

1. **[NEW] [option.go](file:///Users/shuk/projects/gosdk/config/option.go)**
    - 定義 `configOptions` 結構體與 `ConfigOption` 函式型別。
    - 實作 `WithConfigPath(path string) ConfigOption`。
    - 實作 `WithDefaultValue(defaultValue string) ConfigOption`。
        - 若 `CONFIG_DIR` 或指定路徑下的 jsonConfig (即 `settings.json`) 不存在，則自動建立並寫入此 `defaultValue`。此邏輯利用 `utils.CreateIfNotExist` 進行，且只對 `jsonConfig` (settings.json) 生效。

2. **[MODIFY] [config.go](file:///Users/shuk/projects/gosdk/config/config.go)**
    - 重構 `Default` 函式，使其可接受可變參數 `opts ...ConfigOption`。
    - 為了維持向下相容性，`Default()` 仍可無參數呼叫。
    - 在 `Default` 中套用選項：
        - 若有提供 `WithConfigPath`，則將其設為 `CONFIG_DIR`。
        - 若有提供 `WithDefaultValue` 且 `settings.json` 不存在，則自動在指定目錄下建立 `settings.json` 並寫入預設值。

3. **[NEW] [option_test.go](file:///Users/shuk/projects/gosdk/config/option_test.go)**
    - 撰寫測試以驗證 `WithConfigPath` 與 `WithDefaultValue` 的正確性。
    - 測試在 `settings.json` 不存在時，是否正確自動建立並寫入預設值，且驗證非 JSON 格式不會受到此預設值影響。

## 驗證規劃

- 執行 `go test -v ./config/...` 驗證所有測試。
