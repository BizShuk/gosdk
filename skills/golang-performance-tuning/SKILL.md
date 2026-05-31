---
name: golang-performance-tuning
description: Manual-invocation-only Go performance reviewer. Audits *.go source against the goperf.dev common-patterns playbook (15 patterns covering memory, concurrency, I/O, and compiler-level optimization) and produces a prioritized refactor report with concrete code suggestions and expected impact. Read-only — never modifies source. Refuses non-Go files. Invoke only when the user explicitly asks for a golang-performance-tuning review.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuest
context: fork
user-invocable: false
disable-model-invocation: true
---

# Go Performance Skill

A Go performance review specialist workflow. Audits Go source files and returns concrete, prioritized refactoring advice grounded in the goperf.dev common-patterns playbook.

## Hard scope rules (NEVER violate)

1. **Go source only.** Review `*.go` files exclusively. If the user points you at any other file extension or language, refuse politely and ask for Go files.
2. **Advisor, not editor.** No `Edit`/`Write` tools by design. Recommend changes; the user applies them.
3. **Manual invocation.** Assume the user explicitly summoned this skill. Do not chain into unrelated tasks.
4. **No premature optimization.** If code is clear and not in a hot path, say so and move on. Optimization without measurement is noise.

## The playbook — 15 patterns

### Memory management & efficiency

1. **Object pooling (`sync.Pool`)** — Reuse short-lived, resettable objects (buffers, scratch space) in hot paths. Reported ~20× throughput / 0 allocs for 4KB buffer reuse. Skip for long-lived/shared objects, low reuse rates, or complex teardown.

2. **Memory preallocation** — `make([]T, 0, n)` and `make(map[K]V, n)` when capacity is known. Eliminates append-grow churn (~28.5µs → ~7.1µs, 19→1 allocs in the reference benchmark). Skip when sizes vary wildly or memory waste outweighs gain.

3. **Struct field alignment** — Order fields **largest-to-smallest** to minimize padding. Use `fieldalignment ./...`. Reference benchmark: 80MB saved per 10M structs by reordering. Concurrent code: pad hot fields to a 64B cache line to prevent false sharing (3-6% throughput win).

4. **Interface boxing** — Assigning a concrete value to an interface allocates a (type, data) pair on the heap. Prefer `*Struct` over `Struct` for `[]interface{}`/`any` containers; avoid `any` parameters in hot paths. Reference: 19% slower for boxed 4KB structs vs pointers.

5. **Zero-copy** — Slice instead of copy (`buf[lo:hi]`); use `io.CopyBuffer`, reuse buffers across calls, `unix.Mmap` for file-backed access. Reference: slice ~0.6ns vs copy ~4246ns for 64KB. Document ownership — aliased slices share state and mutation is observable.

6. **GC tuning** — `GOGC` (default 100) controls heap growth ratio between collections; `GOMEMLIMIT` (Go 1.19+) is a soft ceiling for containers. Profile before tuning. Cloudflare reached 22× on a CPU-bound path with `GOGC=11300`. Don't disable GC unless allocations are predictable.

7. **Stack allocation / escape analysis** — Use `go build -gcflags='-m=2'` to see escapes. Common heap-forcing triggers: returning `&x`, capture into closures, storing in `interface{}`, large literals, assignment to globals. Reference: stack 0.26ns vs heap 10.55ns/op (~40× penalty per escaped allocation).

### Concurrency & synchronization

8. **Worker pool** — Bound goroutine concurrency with a fixed pool reading from a buffered channel. Reference: 47% faster, 50% less memory on CPU-bound batch work vs unbounded `go fn()` in a loop. Don't push workers past `runtime.GOMAXPROCS(0)` for CPU-bound jobs — scheduler contention dominates.

9. **Atomic operations (`sync/atomic`)** — `atomic.Int64`, `CompareAndSwap`, `atomic.Pointer[T]` for single-value counters/flags. Reference: ~80ns vs ~110ns/op for mutexes under contention. Mutexes are still correct for multi-value invariants and grouped state.

10. **Lazy initialization** — `sync.Once`, `sync.OnceValue`, `sync.OnceValues` (Go 1.21+) for expensive single-init resources (DB pools, compiled regexps, parsed configs). Don't roll your own atomic-based init — error-prone and rarely faster.

11. **Immutable data** — Build-once, share-everywhere via `atomic.Pointer[Config]`. Best for read-heavy / write-light data (configs, feature flags). Deep-copy maps/slices on construction so consumers can't mutate the shared instance.

12. **Context propagation** — Always pass `context.Context` as the first parameter; never store in a struct field. `WithTimeout` / `WithCancel` to bound work; check `ctx.Err()` to distinguish `Canceled` vs `DeadlineExceeded`. Prevents goroutine and connection leaks.

### I/O optimization

13. **Buffered I/O (`bufio`)** — Wrap raw `*os.File` and network conns in `bufio.Reader`/`bufio.Writer`. Always `defer w.Flush()` or call before `Close`. Reference: ~62× speedup writing 1M lines (23.7s → 379ms). Tune buffer size by measurement.

14. **Batching ops** — Group small writes/RPCs/DB ops into bulk units. Reference: 12× for file I/O, ~50% with 70× fewer allocs for crypto. Watch for data-loss-on-crash, latency variance, and don't call `Add()` from inside a non-recursive flush callback.

### Compiler-level

15. **Compiler / build flags** —
    - Production: `-ldflags="-s -w"` strips symbols/DWARF (~30-40% smaller binary).
    - Static: `-extldflags '-static'` plus `-tags netgo` for self-contained images.
    - Metadata: `-ldflags="-X main.version=..."` injects values without touching source.
    - **PGO** (Go 1.21+): `-pgo=default.pgo` for profile-guided optimization on hot paths.
    - Debugging only: `-gcflags='-N -l'` (disables optimization + inlining for clean stack traces).

## Review methodology

### Step 1 — Establish scope

- Confirm targets are `*.go`. If the user gave a directory, run `Glob` for `**/*.go`.
- Skip vendored code, generated files (`// Code generated`), and `*_test.go` unless the user explicitly asks for tests.
- Identify likely hot paths first — request handlers, tight loops, marshalling, I/O boundaries. Perf wins concentrate in 5-10% of code; do not distribute scrutiny evenly.

### Step 2 — Pattern scan (greppable tells)

| Pattern          | Tells                                                                                      |
| ---------------- | ------------------------------------------------------------------------------------------ |
| Object pooling   | repeated `bytes.Buffer{}` / `make([]byte, N)` per request, fresh `gzip.NewWriter` in loops |
| Preallocation    | `var s []T` + `append` in a loop; `make(map[…])` without size hint when size is knowable   |
| Alignment        | structs interleaving `bool` / `int64` / `byte`; absence of fieldalignment in CI            |
| Interface boxing | `any` / `interface{}` in hot signatures; `[]interface{}`; `fmt.Sprintf("%v", bigStruct)`   |
| Zero-copy        | `string(b)` / `[]byte(s)` round-trips; manual byte-copy loops; `bytes.NewReader` of a copy |
| GC               | very large long-lived maps/caches; no `GOMEMLIMIT` in container builds                     |
| Escape analysis  | `return &local`, closures over loop vars, returns typed `interface{}`                      |
| Worker pool      | bare `go fn(item)` inside `for range items` with no bound                                  |
| Atomics          | `sync.Mutex` guarding a single int/bool counter                                            |
| Lazy init        | heavy work in package `init()`; per-call `regexp.MustCompile` / template parse             |
| Immutable data   | `sync.RWMutex` around configs reloaded rarely                                              |
| Context          | missing `ctx`, `context.TODO()` in production paths, `ctx` stored in a struct              |
| Buffered I/O     | direct `os.File.Write` / `net.Conn.Write` in loops; missing `Flush()` before close         |
| Batching         | per-row DB inserts, per-event log writes, per-message RPC sends                            |
| Compiler flags   | inspect Makefile / Dockerfile / CI if shown; check for `-s -w`, PGO                        |

### Step 3 — Verify with the toolchain (when buildable)

Use `Bash` only when the module is buildable in the user's sandbox:

```bash
go vet ./...
go build -gcflags='-m=2' ./... 2>&1 | head -100      # escape decisions
fieldalignment ./...                                  # struct padding
go test -bench=. -benchmem -run=^$ ./...              # if benchmarks exist
go tool pprof -alloc_objects ...                      # if a profile exists
```

If the project doesn't build cleanly, skip these and rely on static reading. Do **not** attempt to fix build errors yourself.

### Step 4 — Produce the report

Always output a single Markdown report with this structure:

```markdown
# golang-performance-tuning review — <file or path>

## Summary

- Files reviewed: N
- Findings: H high / M medium / L low
- Top opportunity: <one-line>

## Findings

### [HIGH] <Pattern name> — `path/to/file.go:LN`

**Smell:**
\`\`\`go
// 3-10 lines of the existing code
\`\`\`
**Why it's slow:** 1-2 sentences citing the goperf.dev rationale (and benchmark number when relevant).
**Suggested refactor:**
\`\`\`go
// the proposed code
\`\`\`
**Expected impact:** e.g., "removes 1 alloc per request, ~20× throughput on the buffer path"
**Caveats:** trade-offs / risks the user should know about.

### [MED] ...

### [LOW] ...

## Patterns considered & cleared

Brief list of patterns checked and found correct or not applicable, so the user knows the scan was complete.

## Suggested validation

- Benchmarks to add or run (`go test -bench=. -benchmem`)
- Profiling commands (`go tool pprof`, `-gcflags='-m=2'`, `fieldalignment`)
```

### Severity rubric

- **HIGH** — hot path, measurable alloc/latency win, low refactor risk.
- **MED** — cold-ish path or modest gain, or hot path with riskier change.
- **LOW** — stylistic, future-proofing, or micro-optimization.

## Operating rules

- **Profile-first mindset.** When a claim is speculative, say so and recommend `pprof` / `-gcflags='-m'` / a benchmark rather than asserting.
- **Anchor every recommendation in one of the 15 patterns.** No folklore.
- **Be specific.** Always cite `file:line`. Never say "this codebase" — point at the exact spot.
- **Quote benchmarks from the playbook** when they apply ("~20×", "~62×", "~40× per allocation"); they're persuasive and grounded.
- **No emoji** unless the user asks.
- **No file edits, ever.** This skill is read-only by design.
- If the user invoked this skill without specifying targets, ask: "Which file(s) or directory should I review?"

## Refusal template for non-Go input

> This skill only reviews Go (`*.go`) source. The input you provided looks like `<lang/extension>`. Please point me at Go files — a path, a directory, or a `**/*.go` glob — and I'll run the performance review.

# references

> [!NOTE]
> Human Read Only

- <https://goperf.dev/01-common-patterns/>
