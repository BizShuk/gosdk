# Dependencies

Import-graph review with `go-dependency-analysis`. Read-only — it never touches `go.mod` / `go.work`.

Need `go-dependency-analysis` on `PATH`. If missing, ask before `go install github.com/bizshuk/go-dependency-analysis@latest`.

```bash
mkdir -p tmp
# 1. facts (no policy) — the baseline every review starts from
go-dependency-analysis --workspace ./go.mod --format text --fail-on none | tee tmp/dependency-report.txt
go-dependency-analysis --workspace ./go.mod --format json --fail-on none --output tmp/dependency-report.json
# 2. confirmation pass — required before reporting any unused dependency
go-dependency-analysis --workspace ./go.mod --include-tests --format json --fail-on none --output tmp/dependency-report-tests.json
# 3. optional: boundary enforcement
go-dependency-analysis --workspace ./go.mod --policy <policy>.json --fail-on error
go-dependency-analysis --workspace ./go.mod --format mermaid --output tmp/dependency.mmd
```

Reports go in `tmp/`, never in the repo tree. Use `--workspace ./go.work` for a multi-module workspace, `--exclude` to drop sample/vendor modules.

## Evidence class

The tool labels every diagnostic. Report them differently.

| Class | Source | Treat as |
| ----- | ------ | -------- |
| `go-tool-fact` | `go list` / `go mod edit` | Verified — cite directly |
| `policy-heuristic` | your `--policy` file | Only as strong as the policy; cite the rule that fired |

Intrinsic diagnostics run without `--policy`. A policy can only re-rank their severity, not invent facts.

## Diagnostics → severity

| Diagnostic | Severity | Action |
| ---------- | -------- | ------- |
| module/package cycle inside the analyzed module | HIGH | Must-fix. Break with an interface at the consumer (`solid.md`) |
| cycle wholly among third-party modules | ignore | Not yours. Do not report |
| `layer-forbidden` on your own packages | HIGH | Wrong-direction import; invert the dependency |
| `unused-direct-candidate` | MED | **Confirm first** — see below |
| `multi-version` naming a module you require directly | LOW | Note the version spread; MVS still picks one |
| `multi-version` from transitive pressure only | ignore | Noise on any real dependency tree |

## Traps

`unused-direct-candidate` is a candidate, not a verdict. Run the `--include-tests` pass and diff: a candidate that disappears is a test-only import and is `correct as written`. Also clear build tags, platform files, generated code, `//go:build tools` and blank imports (`_ "driver"`) before reporting. Only report what survives all five.

`package_edges` are `intra-module only`. A cross-module import is recorded as a **module** edge instead, so a package's third-party fan-out never shows in `package_edges` — read `module_edges` for that. An empty external fan-out means nothing.

Stdlib nodes are filtered from `packages[]` but `package_edges` keeps stdlib→stdlib edges (`--show-stdlib` says *nodes*). Any graph built from the JSON must drop edges whose endpoints are absent from `packages[]`, or ~38% of a typical edge list is stdlib plumbing.

The graph reflects the host Go version, OS/arch and build tags. Cross-platform claims need one run per platform, kept as separate snapshots — never union them.

## Report

Group by evidence class, facts first. Cite module or package paths, not file lines — this chapter is the one place `findings.md`'s `file:line` rule does not apply.

```text
HIGH — package cycle (go-tool-fact)
Cycle: ./service -> ./handler -> ./service
Why: …
Fix: <before / after>
```

State the headline counts (modules, packages, edges) so a later run is comparable, and say which diagnostics were dismissed as third-party noise — a silent filter looks like a missed finding.
