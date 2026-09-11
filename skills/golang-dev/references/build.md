# Build, test, escape

| Task | Command |
| ---- | ------- |
| Build | `go build ./...` |
| Prod | `go build -ldflags="-s -w" -o bin/app ./cmd/app` |
| Static | `CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/app ./cmd/app` |
| Test | `go test ./...` |
| Race | `go test -race ./...` |
| Cover | `go test -cover -coverprofile=coverage.out ./...` |
| No inline | `go test -gcflags="all=-N -l" ./...` |
| Bench | `go test -bench=. -benchmem -run=^$ ./...` |
| Escape | `go build -gcflags='-m=2' ./...` |
| Lint / vet / fmt | `golangci-lint run ./...` · `go vet ./...` · `gofmt -w .` |

Do not invent interfaces only for tests. `-s -w` strips symbols; `CGO_ENABLED=0` for scratch images; `-race` in CI not prod. `all=-N -l` for delve, mockey, and true escape analysis.

Escape: grep `escapes to heap`, fix hot paths only, confirm with `go test -bench=. -benchmem`.

| Output | Cause | Fix |
| ------ | ----- | --- |
| `leaking param` | stored beyond scope | don't keep pointer params |
| `too large` | stack > ~10MB | smaller allocs / `sync.Pool` |
| `captured by closure` | variable in closure | pass as param |
| `interface conversion` | assigned to `any` | concrete types on hot path |
| `&x escapes` | returning `&local` | return value |
