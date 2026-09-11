# Dead code

Detect → classify → confirm → delete → `go build` + `go vet`. Never silent delete. Never auto-delete exported symbols.

Need `staticcheck`, `deadcode`, `unparam` on `PATH`. If missing, ask before `go install honnef.co/go/tools/cmd/staticcheck@latest` / `golang.org/x/tools/cmd/deadcode@latest` / `mvdan.cc/unparam@latest`.

```bash
staticcheck -checks=U1000,U1001,SA4006 ./...
# deadcode needs a package main; skip on library-only modules
deadcode ./...
unparam ./...
grep -rn "// Deprecated:" --include="*.go" .
```

| Bucket | Criteria | Default |
| ------ | -------- | ------- |
| Safe | unexported, no tests, no reflect/plugin | Delete after confirm |
| Deprecated-in-use | `// Deprecated:` with callers | List call sites; migrate, don't delete |
| Risky | exported, `reflect.`, `plugin.Open`, `//go:linkname`, build tags | Skip unless opted in |

Grep `\bsymbol\b` before deleting — tools miss interface methods and tags. Remove the doc comment with the symbol; `gofmt`/`goimports` after each file batch. Build fail → report, offer `git checkout -- <file>`, mark Risky, continue.

Keep: `init()`, `//go:linkname`, `go:embed` targets, reflect method names, interface methods, test helpers used only from `_test.go`.
