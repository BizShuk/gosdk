# Dependencies

Import-graph and module review using standard `go` tooling (`go list`, `go mod`). Read-only — never mutate `go.mod` / `go.work` during review.

```bash
# 1. package dependency graph inside the module (cycles fail go list / go build immediately)
go list -f '{{.ImportPath}} -> {{.Imports}}' ./...
# 2. module graph & direct-dependency bloat / unused check
go mod graph
go mod tidy -diff
# 3. explain why any indirect or suspect module is pulled in
go mod why -m <module>
# 4. confirmation pass including tests (required before reporting an unused dependency)
go list -test -f '{{.ImportPath}} -> {{.Imports}}' ./...
```

In a multi-module workspace (`go.work`), run `go list` from the workspace root or pass `-m all` to inspect resolved module versions (`go list -m -json all`).

## Evidence class

| Class | Source | Treat as |
| ----- | ------ | -------- |
| `go-tool-fact` | `go list` / `go mod graph` / `go mod why` / `go mod tidy -diff` | Verified — cite directly |
| `boundary-rule` | repo architecture / layer convention (`AGENTS.md` / `golang-dev`) | Only as strong as the documented boundary; cite the violated rule |

## Diagnostics → severity

| Diagnostic | Severity | Action |
| ---------- | -------- | ------ |
| module/package cycle inside the analyzed module | HIGH | Must-fix. Break with an interface at the consumer (`solid.md`) |
| cycle wholly among third-party modules | ignore | Not yours. Do not report |
| wrong-direction import across own layers (e.g. `model/` → `svc/` or `handler/`) | HIGH | Invert the dependency via a consumer-side interface |
| direct `require` dropped by `go mod tidy -diff` | MED | **Confirm first** — see below |
| duplicate major/minor versions visible in `go mod graph` for a direct dep | LOW | Note the version spread; MVS picks the highest |
| transitive-only version spread in `go mod graph` | ignore | Noise on any real dependency tree |

## Traps

`go mod tidy -diff` and `go list ./...` without `-test` do not clear every edge case automatically. Before reporting an unused dependency, check build tags, platform-specific files (`_linux.go`, `_darwin.go`), generated code, `//go:build tools` / `tool` directives, and blank imports (`_ "driver"`). Only report what survives all five.

`go list -f '{{.ImportPath}} -> {{.Imports}}' ./...` includes stdlib imports; filter to the module prefix (e.g. `grep` for the module path from `go.mod`) when mapping internal package boundaries.

The graph reflects the host Go version, `GOOS`/`GOARCH`, and active build tags. Cross-platform claims need separate checks per target (`GOOS=linux go list ./...`, `GOOS=darwin go list ./...`).

## Report

Group by evidence class, facts first. Cite module or package paths, not file lines — this chapter is the one place `findings.md`'s `file:line` rule does not apply.

```text
HIGH — package cycle (go-tool-fact)
Cycle: ./service -> ./handler -> ./service
Why: …
Fix: <before / after>
```

State the headline counts (modules, internal packages, direct dependencies) so a later run is comparable, and note any third-party noise that was dismissed.
