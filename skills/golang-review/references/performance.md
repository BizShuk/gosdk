# Performance (hot paths)

Measure first (`pprof`, `go test -bench=. -benchmem`). Speculative numbers are noise. Escape analysis and `bufio` on conns: `golang-dev` http / build chapters. Do not tune cold code.

| Pattern | Smell | Do |
| ------- | ----- | -- |
| Pool | `make([]byte, N)` / `bytes.Buffer{}` per request | `sync.Pool` for short-lived resettable buffers |
| Prealloc | `var s []T` + `append` when `n` is known | `make([]T, 0, n)`, `make(map[K]V, n)` |
| Align | `bool` next to `int64` | largest-to-smallest; `fieldalignment ./...` |
| Boxing | `any` / `[]any` on the hot path | concrete types or `*T` |
| Zero-copy | `string(b)` / `[]byte(s)` round-trips | slice `buf[lo:hi]`; copy out before the buffer escapes |
| GC | huge caches; container OOM | profile, then `GOMEMLIMIT`; `GOGC` only with evidence |
| Workers | `go fn(item)` in a loop | bounded pool; CPU-bound ≤ `GOMAXPROCS` |
| Atomic | mutex around one int/flag | `atomic.Int64` / `atomic.Pointer[T]` |
| Lazy | `regexp.MustCompile` per call; heavy `init()` | `sync.OnceValue` |
| Immutable | `RWMutex` on rarely written config | `atomic.Pointer[Config]` + deep copy on build |
| Batch | per-row insert / per-event RPC | bulk units |

PGO: `-pgo=default.pgo` on production hot binaries.
