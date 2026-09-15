---
name: golang-review
description: >
    Use when reviewing or refactoring Go — SOLID, error wrapping, context,
    constructor injection, unused symbols, import cycles and dependency
    boundaries, hot-path performance (pool, prealloc, boxing, GC), or
    measuring HeapAlloc, Peak RSS, or binary size.
    Triggers: review this Go, apply SOLID, dead code, unused, U1000,
    staticcheck, pprof, allocs/op, HeapAlloc, slow, GC, sync.Pool,
    import cycle, dependency graph, unused dependency, go.mod bloat,
    layer violation, go-dependency-analysis.
    Layout, naming, HTTP, and build flags live in golang-dev.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuestion
user-invocable: true
disable-model-invocation: false
context: fork
---

# golang-review

Review Go for simplicity, SOLID, dead code, and measured performance.

Clear is better than clever. A function should be obvious in under 60 seconds; a new feature should add code, not rewrite existing code.

Go files only. Skip `vendor/` and `// Code generated`. Load **only** the chapter that matches the task.

| Chapter | Load when |
| ------- | --------- |
| [solid.md](references/solid.md) | SOLID, fat interfaces, constructor vs concrete |
| [errors.md](references/errors.md) | wrap, sentinel, ctx, DI |
| [performance.md](references/performance.md) | hot path, pool, allocs, GC, boxing |
| [measure.md](references/measure.md) | HeapAlloc, Peak RSS, binary size, pprof |
| [dead-code.md](references/dead-code.md) | unused, U1000, staticcheck, deprecated |
| [dependencies.md](references/dependencies.md) | import cycle, layer violation, unused dependency, go.mod bloat |
| [findings.md](references/findings.md) | how to report / apply a finding |

Layout, naming, HTTP, TUI, build/escape: `golang-dev`.
