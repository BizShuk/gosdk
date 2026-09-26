# Findings

Cite `file:line`. Show smell + fix. Never "could be improved" without a diff.

```text
HIGH — handler/user.go:42
Issue: …
Why: …
Fix: <before / after>
```

| Severity | When |
| -------- | ---- |
| HIGH | leak, swallowed error, missing ctx on I/O, panic for recoverable, hot-path alloc with a cheap fix, `InsecureSkipVerify` |
| MED | fat interface, producer-side interface, missing `%w`, boxing on a warm path, no breaker |
| LOW | long function, magic number, micro-tuning, observability gap |

**Must-fix:** cyclic imports, business logic in `handler/`/`model/` (`golang-dev` layers chapter), concrete deps in handlers, `_ = err`, unbounded `go` in Accept/loops.

Preserve external behavior when applying a refactor. One principle per change when several exist — let the user prioritize unless they asked to apply all.
