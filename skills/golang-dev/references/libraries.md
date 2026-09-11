# Libraries

| Category | Library | When |
| -------- | ------- | ---- |
| Logging | `log/slog` + `gosdk/log` | [logging.md](logging.md) |
| Testing | `stretchr/testify` | `assert` continues; `require` stops |
| Mocking | `bytedance/mockey` | inside `mockey.PatchConcurrently` |
| Lint | `golangci-lint` | `golangci-lint run ./...` |
| HTTP | `net/http` (+ gin when routing is complex); `gosdk/http` retries | [http.md](http.md) |
| CLI | cobra | [cli.md](cli.md) |
| TUI | bubbletea + lipgloss | [tui.md](tui.md); static tables: `tui.Table` |
| Config | viper via `config.Default` | [config.md](config.md) |
| Hot reload | `air-verse/air` | `air` instead of `go run` |

```go
require.NoError(err)
assert.Equal("Alice", user.Name)

mockey.PatchConcurrently(t, func() {
    mockey.Mock(FetchUserFromDB).Return(&User{ID: "123"}, nil).Build()
})
```
