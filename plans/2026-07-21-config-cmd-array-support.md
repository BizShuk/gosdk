# Config Cmd 陣列支援 實作計畫

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 讓 `gosdk/cmd` 的 `ConfigCmd --update` 對陣列型別提供完整支援：整體取代（已能用）+ 元素級 append / remove。

**Architecture:**
- **Layer 1**（零程式碼）：在 cobra `Long` / `Example` 補上 JSON literal 陣列範例
- **Layer 2**（新增）：`--append x.y=value` 與 `--remove-from x.y=value` 兩個 element-level 旗標
- **Layer 3**（YAGNI）：`--update x.y[0]=c` 索引語法不做

**Tech Stack:** Go 1.26、`spf13/cobra`、`spf13/viper`、stdlib `encoding/json` / `slices`、既有 `runConfigChanges` 共用寫入路徑。

**WIP 適配（2026-07-21 對齊）**：本實作接續已進行的重構（uncommitted working tree）：
- 所有變動函式簽名沿用 `local bool` 參數（WIP 引入的 `--local` flag）
- 渲染邏輯在 `cmd/configRender.go`，新 `ChangeKind` 須補 switch case
- 測試拆分：`cmd/config_test.go`（CLI 整合）、`cmd/config_logic_test.go`（logic 單元）、`cmd/configRender_test.go`（render 契約）
- `runConfigChanges` 簽名擴充為 `(updates, deletes, appends, removes []string, local bool)` 以維持單一原子寫入

---

## 使用者審查要求

- [x] 確認採用 `--append` / `--remove-from` 命名（已於對話中與使用者達成共識）
- [x] 確認元素僅支援 string（JSON 元素 escape 太痛，留給 Layer 1 整體取代）
- [x] 確認混合 `--update` + `--append` 執行順序 = flag call order（先 update 後 append）
- [x] 確認接續 WIP，所有新函式沿用 `local bool` 簽名（2026-07-21 對齊）

## 全域約束 (Global Constraints)

- Go 1.26，`go vet ./...` 與 `gofmt` 必須通過
- 所有 const 使用 `SCREAMING_SNAKE_CASE`（含 unexported / block-scoped）
- 既有慣例：package-level exported var + `init()` 綁 flag；不引入新的 cobra command
- 不引入新依賴；用 stdlib `slices.Index` / `slices.Delete`
- 既有測試 `cmd/config_test.go` 全綠是基線（不可破壞）

---

## 檔案變動總覽

| 檔案 | 變動 | 說明 |
|------|------|------|
| `cmd/config.go` | 修改 | 新增 `ChangeKind`、flag、helper、`dispatchConfig` 路由、`RunConfigAppend` / `RunConfigRemoveFrom` |
| `cmd/config_test.go` | 修改 | 新增 6 個測試覆蓋所有新行為 |
| `plans/2026-07-21-config-cmd-array-support.md` | 新增 | 本計畫 |

---

## Task 1: 文件化既有 JSON literal 陣列支援（零 code 變動）

**Files:**
- Modify: `cmd/config.go:109-123`（Example 段落）

**Interfaces:** 無（純文件）

- [ ] **Step 1: 補上陣列範例**

編輯 `cmd/config.go` 的 `Example:` 區塊（109-123 行），在現有 `"1234"` 範例之後新增兩行：

```go
Example: `  # show the merged configuration
  app config

  # show which layer each value came from
  app config --source

  # set a nested field (creates intermediate levels)
  app config --update server.host=0.0.0.0

  # values that parse as JSON keep their type; quote to force a string
  app config --update server.port=8080
  app config --update build.number='"1234"'

  # JSON literals preserve their type: arrays become []any in the file
  app config --update 'tags=["a","b","c"]'
  app config --update 'ports=[80,443]'

  # remove a field
  app config --delete server.host`,
```

（僅文件變動，不需 commit、不需測試）

---

## Task 2: 新增 `--append` flag（TDD: RED → GREEN）

**Files:**
- Modify: `cmd/config.go`（新增 `ChangeKind`、`RunConfigAppend`、`appendArrayElement`、`splitKey`/`lookupPath` 既有）
- Test: `cmd/config_test.go`（新增 3 個 test）

**Interfaces:**
- Consumes: `RunConfigAppend(specs []string) (ChangeReport, error)`
- Produces: `Change{Kind: ChangeAppended, Key, Old, New}`，`Old` 為 append 前陣列、`New` 為 append 後陣列
- 新 `ChangeKind = "append"`

### Step 1: 寫失敗的測試

在 `cmd/config_test.go` 末尾新增：

```go
func TestAppendCreatesArrayIfMissing(t *testing.T) {
    dir := fixture(t, nil)

    out, err := run(t, "--append", "tags=a", "--append", "tags=b")
    if err != nil {
        t.Fatalf("append failed: %v\n%s", err, out)
    }

    tags := readLocal(t, dir)["tags"].([]any)
    if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
        t.Errorf("tags = %#v, want [a b]", tags)
    }
}

func TestAppendToExistingArray(t *testing.T) {
    dir := fixture(t, map[string]string{
        "settings.local.json": `{"tags":["a"]}`,
    })

    out, err := run(t, "--append", "tags=b")
    if err != nil {
        t.Fatalf("append failed: %v\n%s", err, out)
    }

    tags := readLocal(t, dir)["tags"].([]any)
    if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
        t.Errorf("tags = %#v, want [a b]", tags)
    }
}

func TestAppendRejectsScalarBlockingPath(t *testing.T) {
    fixture(t, map[string]string{
        "settings.local.json": `{"tags":"not-an-array"}`,
    })

    if _, err := run(t, "--append", "tags=a"); err == nil {
        t.Error("expected an error when path holds a non-array value")
    }
}
```

### Step 2: 跑測試確認 RED

Run: `go test ./cmd/ -run TestAppend -v`
Expected: FAIL — `RunConfigAppend` 與 `--append` flag 都不存在

### Step 3: 最小實作

在 `cmd/config.go` 新增：

```go
const (
    ChangeAdded   ChangeKind = "add"
    ChangeUpdated ChangeKind = "update"
    ChangeDeleted ChangeKind = "delete"
    ChangeAppended ChangeKind = "append"
)
```

在 flag 區塊新增（既有 `init()` 函式內）：

```go
configAppends []string
```

```go
f.StringArrayVar(&configAppends, "append", nil,
    "append a string element to an array field, as a.b.c=value (repeatable)")
```

在 `dispatchConfig` 內 `case len(updates) > 0` 之前新增 `case len(appends) > 0`，並把 append 與 update 合併處理（兩個都允許，append 在 update 之後執行）：

```go
case len(appends) > 0 && (len(updates) > 0 || len(deletes) > 0):
    report, err := runConfigChanges(updates, deletes, appends)
    if err != nil { return "", err }
    return renderChangeReport(report), nil
case len(appends) > 0:
    report, err := RunConfigAppend(appends)
    if err != nil { return "", err }
    return renderChangeReport(report), nil
```

新增公開函式：

```go
func RunConfigAppend(appends []string) (ChangeReport, error) {
    return runConfigChanges(nil, nil, appends)
}
```

擴充既有 `runConfigChanges` 簽名並加入 append 處理（在 `delete` 迴圈之後、寫檔之前）：

```go
func runConfigChanges(updates, deletes, appends []string) (ChangeReport, error) {
    // ... 既有 updates + deletes 邏輯 ...

    for _, spec := range appends {
        key, value, ok := strings.Cut(spec, "=")
        if !ok {
            return ChangeReport{}, fmt.Errorf("invalid append %q: expected a.b.c=value", spec)
        }
        segs, err := splitKey(key)
        if err != nil { return ChangeReport{}, err }
        old, _ := lookupPath(settings, segs)
        newArr, err := appendArrayElement(settings, segs, value)
        if err != nil { return ChangeReport{}, err }
        changes = append(changes, Change{Kind: ChangeAppended, Key: key, Old: old, New: newArr})
    }

    if err := writeLocalSettings(path, settings); err != nil { return ChangeReport{}, err }
    return ChangeReport{
        Changes:  changes,
        Path:     path,
        Warnings: envShadowWarnings(updates, deletes),
    }, nil
}
```

新增 helper（在 `deletePath` 之後）：

```go
// appendArrayElement appends value to the array at segs, creating []any{} when
// the path is missing. A non-array value blocking the path is reported.
func appendArrayElement(m map[string]any, segs []string, value string) ([]any, error) {
    full := strings.Join(segs, ".")
    cur := m
    for i, seg := range segs[:len(segs)-1] {
        k, ok := matchKey(cur, seg)
        if !ok {
            child := map[string]any{}
            cur[seg] = child
            cur = child
            continue
        }
        child, ok := cur[k].(map[string]any)
        if !ok {
            return nil, fmt.Errorf("cannot append to %q: %q already holds a scalar value",
                full, strings.Join(segs[:i+1], "."))
        }
        cur = child
    }
    last := segs[len(segs)-1]
    if k, ok := matchKey(cur, last); ok { last = k }
    arr, ok := cur[last].([]any)
    if !ok {
        if cur[last] != nil {
            return nil, fmt.Errorf("cannot append to %q: existing value is not an array (%T)", full, cur[last])
        }
        arr = []any{}
    }
    arr = append(arr, value)
    cur[last] = arr
    return arr, nil
}
```

更新既有兩個呼叫端 `RunConfigUpdate` / `RunConfigDelete` 的簽名：

```go
func RunConfigUpdate(updates []string) (ChangeReport, error) {
    return runConfigChanges(updates, nil, nil)
}

func RunConfigDelete(deletes []string) (ChangeReport, error) {
    return runConfigChanges(nil, deletes, nil)
}
```

並更新 `RunConfig` 內的 slice 變數處理與既有 `dispatchConfig` 的呼叫：

```go
updates := append(append([]string{}, configUpdates...), configAdds...)
output, err := dispatchConfig(configSource, updates, configDeletes, configAppends)
```

更新 `TestPublicConfigFunctions`（既有測試）— 它呼叫 `RunConfigUpdate([]string{"keep=new"})` 與 `RunConfigDelete([]string{"remove"})`，新簽名相容，無需改動。

### Step 4: 跑測試確認 GREEN

Run: `go test ./cmd/ -run TestAppend -v`
Expected: PASS

### Step 5: Commit

```bash
git add cmd/config.go cmd/config_test.go
git commit -m "feat(cmd): support --append for array elements in config command"
```

---

## Task 3: 新增 `--remove-from` flag（TDD: RED → GREEN）

**Files:**
- Modify: `cmd/config.go`
- Test: `cmd/config_test.go`

**Interfaces:**
- `RunConfigRemoveFrom(specs []string) (ChangeReport, error)`
- `Change{Kind: ChangeRemoved, Key, Old, New}`，`Old` 為移除前陣列、`New` 為移除後陣列
- 新 `ChangeKind = "remove"`

### Step 1: 寫失敗的測試

在 `cmd/config_test.go` 末尾新增：

```go
func TestRemoveFromArrayRemovesFirstMatch(t *testing.T) {
    dir := fixture(t, map[string]string{
        "settings.local.json": `{"tags":["a","b","a"]}`,
    })

    out, err := run(t, "--remove-from", "tags=a")
    if err != nil {
        t.Fatalf("remove-from failed: %v\n%s", err, out)
    }

    tags := readLocal(t, dir)["tags"].([]any)
    if len(tags) != 2 || tags[0] != "b" || tags[1] != "a" {
        t.Errorf("tags = %#v, want [b a]", tags)
    }
}

func TestRemoveFromArrayMissingValueIsNoop(t *testing.T) {
    dir := fixture(t, map[string]string{
        "settings.local.json": `{"tags":["a","b"]}`,
    })

    out, err := run(t, "--remove-from", "tags=z")
    if err != nil {
        t.Fatalf("remove-from failed: %v\n%s", err, out)
    }

    tags := readLocal(t, dir)["tags"].([]any)
    if len(tags) != 2 {
        t.Errorf("tags = %#v, want unchanged [a b]", tags)
    }
}

func TestRemoveFromArrayRejectsNonArray(t *testing.T) {
    fixture(t, map[string]string{
        "settings.local.json": `{"tags":"scalar"}`,
    })

    if _, err := run(t, "--remove-from", "tags=a"); err == nil {
        t.Error("expected an error when path holds a non-array value")
    }
}
```

### Step 2: 跑測試確認 RED

Run: `go test ./cmd/ -run TestRemoveFrom -v`
Expected: FAIL — `--remove-from` flag 與 helper 都不存在

### Step 3: 最小實作

在 `cmd/config.go` `ChangeKind` 區塊新增：

```go
const (
    ChangeAdded   ChangeKind = "add"
    ChangeUpdated ChangeKind = "update"
    ChangeDeleted ChangeKind = "delete"
    ChangeAppended ChangeKind = "append"
    ChangeRemoved  ChangeKind = "remove"
)
```

在 flag 區塊新增變數：

```go
configRemoves []string
```

```go
f.StringArrayVar(&configRemoves, "remove-from", nil,
    "remove the first matching element from an array field, as a.b.c=value (repeatable)")
```

擴充 `dispatchConfig`：

```go
case len(removes) > 0 && (len(updates) > 0 || len(deletes) > 0 || len(appends) > 0):
    report, err := runConfigChanges(updates, deletes, appends)
    if err != nil { return "", err }
    // removes 走 RunConfigRemoveFrom 確保 ChangeRemoved 而非 ChangeAppended
    rmReport, err := RunConfigRemoveFrom(removes)
    if err != nil { return "", err }
    return renderChangeReport(mergeReports(report, rmReport)), nil
case len(removes) > 0:
    report, err := RunConfigRemoveFrom(removes)
    if err != nil { return "", err }
    return renderChangeReport(report), nil
```

新增公開函式：

```go
func RunConfigRemoveFrom(removes []string) (ChangeReport, error) {
    return runConfigRemoves(removes)
}

func runConfigRemoves(removes []string) (ChangeReport, error) {
    path := localSettingsPath()
    settings, err := readLocalSettings(path)
    if err != nil { return ChangeReport{}, err }

    var changes []Change
    for _, spec := range removes {
        key, value, ok := strings.Cut(spec, "=")
        if !ok {
            return ChangeReport{}, fmt.Errorf("invalid remove-from %q: expected a.b.c=value", spec)
        }
        segs, err := splitKey(key)
        if err != nil { return ChangeReport{}, err }
        old, _ := lookupPath(settings, segs)
        newArr, err := removeArrayElement(settings, segs, value)
        if err != nil { return ChangeReport{}, err }
        changes = append(changes, Change{Kind: ChangeRemoved, Key: key, Old: old, New: newArr})
    }

    if err := writeLocalSettings(path, settings); err != nil { return ChangeReport{}, err }
    return ChangeReport{
        Changes:  changes,
        Path:     path,
        Warnings: envShadowWarnings(nil, nil, nil, removes),
    }, nil
}
```

新增 helper：

```go
func removeArrayElement(m map[string]any, segs []string, value string) ([]any, error) {
    full := strings.Join(segs, ".")
    cur := m
    for i, seg := range segs[:len(segs)-1] {
        k, ok := matchKey(cur, seg)
        if !ok {
            return nil, fmt.Errorf("cannot remove from %q: %q not found", full, strings.Join(segs[:i+1], "."))
        }
        child, ok := cur[k].(map[string]any)
        if !ok {
            return nil, fmt.Errorf("cannot remove from %q: %q holds a scalar value", full, strings.Join(segs[:i+1], "."))
        }
        cur = child
    }
    last := segs[len(segs)-1]
    actual, ok := matchKey(cur, last)
    if !ok {
        return nil, fmt.Errorf("cannot remove from %q: key not found", full)
    }
    arr, ok := cur[actual].([]any)
    if !ok {
        return nil, fmt.Errorf("cannot remove from %q: existing value is not an array (%T)", full, cur[actual])
    }
    idx := slices.Index(arr, value)
    if idx == -1 {
        return arr, nil // noop
    }
    arr = slices.Delete(arr, idx, idx+1)
    cur[actual] = arr
    return arr, nil
}
```

擴充 `envShadowWarnings` 簽名新增 removes 參數：

```go
func envShadowWarnings(updates, deletes, appends, removes []string) []string {
    // ... 既有邏輯 + 把 removes 與 updates/deletes 一起納入 key list ...
}
```

更新既有 `runConfigChanges` 呼叫 `envShadowWarnings` 的點。

新增 `mergeReports` helper（用於 append+remove 混合時合併報告）：

```go
func mergeReports(a, b ChangeReport) ChangeReport {
    return ChangeReport{
        Changes:  append(a.Changes, b.Changes...),
        Path:     b.Path,
        Warnings: append(a.Warnings, b.Warnings...),
    }
}
```

### Step 4: 跑測試確認 GREEN

Run: `go test ./cmd/ -run TestRemoveFrom -v`
Expected: PASS

### Step 5: Commit

```bash
git add cmd/config.go cmd/config_test.go
git commit -m "feat(cmd): support --remove-from for array elements in config command"
```

---

## Task 4: 混合操作與順序保證測試

**Files:**
- Test: `cmd/config_test.go`

### Step 1: 寫測試

```go
func TestMixedUpdateAndAppendOrder(t *testing.T) {
    dir := fixture(t, map[string]string{
        "settings.local.json": `{"tags":["old"]}`,
    })

    // --update 全部覆寫後，--append 接到尾端
    out, err := run(t, "--update", "tags=[\"a\"]", "--append", "tags=b")
    if err != nil {
        t.Fatalf("mixed failed: %v\n%s", err, out)
    }

    tags := readLocal(t, dir)["tags"].([]any)
    if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
        t.Errorf("tags = %#v, want [a b]", tags)
    }
}

func TestMixedAppendAndRemoveFrom(t *testing.T) {
    dir := fixture(t, map[string]string{
        "settings.local.json": `{"tags":["x"]}`,
    })

    out, err := run(t, "--append", "tags=y", "--remove-from", "tags=x")
    if err != nil {
        t.Fatalf("mixed failed: %v\n%s", err, out)
    }

    tags := readLocal(t, dir)["tags"].([]any)
    if len(tags) != 1 || tags[0] != "y" {
        t.Errorf("tags = %#v, want [y]", tags)
    }
}
```

### Step 2: 跑測試確認 PASS

Run: `go test ./cmd/ -run TestMixed -v`
Expected: PASS（既有混合 dispatch 路徑應已 cover；若失敗，調整 `dispatchConfig`）

### Step 3: Commit（如有新增 helper）

若此 task 沒新增 helper，無需 commit。否則：

```bash
git add cmd/config_test.go
git commit -m "test(cmd): cover --update + --append + --remove-from mixed operations"
```

---

## Task 5: 更新 cobra Long 說明

**Files:**
- Modify: `cmd/config.go`（`Long:` 區塊，第 88-108 行）

### Step 1: 擴充 Long 說明

把：

```
--update, --add and --delete write to settings.local.json only. ...
```

改為：

```
--update, --add and --delete write to settings.local.json only. That is
the last file in the merge order, so a value written there overrides every other
config file. It does not override an APP_ environment variable.

--append and --remove-from work at the array-element level: --append a.b.c=value
adds value to the array at a.b.c, creating []any if missing; --remove-from
a.b.c=value removes the first matching element. The value is always treated as
a plain string; use --update with a JSON literal for typed arrays.`,
```

### Step 2: Commit

```bash
git add cmd/config.go
git commit -m "docs(cmd): document --append and --remove-from in config long help"
```

---

## Task 6: 完整驗證

### Step 1: 全套測試

Run: `go test -v ./...`
Expected: ALL PASS（含既有 11 個 config 測試 + 新增 8 個）

### Step 2: go vet

Run: `go vet ./...`
Expected: no warnings

### Step 3: gofmt

Run: `gofmt -l cmd/`
Expected: no output（檔案已格式化）

### Step 4: 建置

Run: `go build ./...`
Expected: success

### Step 5: 人工 smoke test（可選）

```bash
echo '{"tags":["a"]}' > /tmp/test-settings.json
cd /tmp
APP_SETTINGS_DIR=. go run ./cmd/sample config --append tags=b
cat settings.local.json  # 應為 {"tags":["a","b"]}
```

---

## 驗收條件 (Definition of Done)

- [ ] 所有 6 個 task 完成
- [ ] `go test ./...` 全綠
- [ ] `go vet ./...` 無警告
- [ ] `gofmt -l cmd/` 無輸出
- [ ] `go build ./...` 成功
- [ ] 新增 8 個測試全部 PASS（3 append + 3 remove-from + 2 mixed）
- [ ] 既有測試未破壞
- [ ] 計畫文檔已歸檔至 `plans/`
