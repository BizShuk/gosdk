---
name: golang-naming
description: >-
    Go naming convention reviewer and safe renamer. Audits package, function,
    variable, struct, interface, and method names across the entire Go workspace
    for Go community naming idioms (acronym casing, stutter, receiver length,
    grab-bag packages like util/common, etc.). Produces a rename proposal report,
    and ONLY after the user explicitly approves applies renames safely via
    `gopls rename` so that every call site (and import path) in the workspace is
    updated atomically. Read-only until approval. Refuses non-Go files.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuest
model: sonnet
effort: high
context: fork
disable-model-invocation: true
user-invocable: false
---

# golang-naming

A Go naming convention reviewer and safe renamer workflow.

## Hard rules

1. **Invocation context matters.** When invoked as a standalone skill (not by a parent agent), confirm intent with the user before proceeding. When invoked by a parent agent (e.g., `golang-refactor`, `feature`), proceed directly — the parent agent has already determined the need.
2. **Two-phase: report → approval → apply.** Never rename anything without explicit per-batch user approval. Default to dry-run.
3. **`gopls rename` is the only renaming mechanism.** Never use Edit/sed/grep-replace to rename Go symbols. Textual replacement is forbidden because it cannot resolve Go scope, interface satisfaction, or cross-package references.
4. **Refuse non-Go files.** If asked to review `.py`, `.ts`, IDL, YAML, etc., decline and explain that this skill is Go-only.
5. **Skip generated and vendored code.** Exclude any file matching:
    - Contains header `// Code generated ... DO NOT EDIT.`
    - Under `vendor/`, `kitex_gen/`, `hertz_gen/`, `thrift_gen/`, `pb_gen/`, `mock/` (auto-generated mocks), `*_mock.go` (mockey-generated)
    - Path components matching `\.pb\.go$`, `_gen\.go$`

## Prerequisite check (run first, every invocation)

```bash
command -v gopls >/dev/null 2>&1 || { echo "gopls not installed"; exit 1; }
gopls version
```

If `gopls` is missing, stop and instruct the user to install:

```bash
go install golang.org/x/tools/gopls@latest
```

Also verify you are at a Go module root (presence of `go.mod`). If not, ask the user to point you at the correct directory.

## Scope

- **Default**: the entire current Go workspace (every package in `go.mod`'s module).
- Enumerate `.go` files with `find . -name '*.go' -not -path './vendor/*' -not -path './kitex_gen/*' -not -path './hertz_gen/*'` (extend exclusions as needed) and filter out generated/mock files by reading their first 5 lines.
- If the user specifies a path/package, narrow to that subtree but keep the _rename impact analysis_ workspace-wide (gopls handles that automatically).

## Naming rules to enforce

Apply these idioms (Effective Go + Google Go Style Guide + community consensus):

| #   | Rule                                                                                                                                                      | Bad                                                           | Good                                                                                 |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| 1   | **No stutter** between package and exported name                                                                                                          | `region.RegionManager`, `user.UserService`                    | `region.Manager`, `user.Service`                                                     |
| 2   | **Acronyms in full caps** for ID/URL/API/HTTP/JSON/XML/SQL/TCC/IDC/PSM/RPC/IDL/UUID                                                                       | `userId`, `httpUrl`, `tccConfig`                              | `userID`, `httpURL`, `tccConfig` (TCC is already caps; flag `Tcc`/`tcc` mixed cases) |
| 3   | **Receiver names short** (1–3 lowercase letters), consistent across all methods of the same type                                                          | `func (this *UserService)`, `func (userService *UserService)` | `func (s *UserService)` (consistently `s`)                                           |
| 4   | **Exported vs unexported casing** correct relative to intended visibility                                                                                 | unused-outside-package `Helper`                               | `helper`                                                                             |
| 5   | **Verb-first for functions/methods that perform actions**                                                                                                 | `UserGet`, `ConfigLoad`                                       | `GetUser`, `LoadConfig`                                                              |
| 6   | **No redundant suffixes** on functions                                                                                                                    | `GetUserFunc`, `ValidateMethod`                               | `GetUser`, `Validate`                                                                |
| 7   | **No redundant type suffixes** on struct names when context already conveys it                                                                            | `UserStruct`, `OptionsType`                                   | `User`, `Options`                                                                    |
| 8   | **Avoid unclear abbreviations**; prefer clarity over brevity for package-level names. Short names are OK inside short scopes (loop variables, receivers). | package-level `cfg`, `mgr`, `ctx` field                       | `config`, `manager` (keep `ctx` for `context.Context`, `err` for errors)             |
| 9   | **Boolean names read as predicates**: `is/has/can/should` prefix                                                                                          | `enable`, `success` (as flag)                                 | `isEnabled`, `hasSucceeded`                                                          |
| 10  | **Interface naming**: single-method interfaces use `-er` suffix; multi-method use noun. `I` prefix is allowed.                                            | `ReadInterface`                                               | `Reader`; for multi-method something like `Store`; `IReader` is acceptable           |
| 11  | **Errors**: variables start with `Err`, types end with `Error`                                                                                            | `NotFound` (error sentinel)                                   | `ErrNotFound`, `NotFoundError` (type)                                                |
| 12  | **Constants**: use `SCREAMING_SNAKE_CASE` (all caps with underscores). This is a deliberate team convention.                                              | `MixedCase` constants                                         | `MAX_RETRIES`, `DEFAULT_TIMEOUT`, `COLOR_RED`                                        |
| 13  | **Use `any` instead of `interface{}`** (Go 1.18+ style)                                                                                                   | `interface{}`, `map[string]interface{}`                       | `any`, `map[string]any`                                                              |

Tailor to the project: if the codebase already uses a documented convention (e.g. CLAUDE.md says X), defer to it and call out conflicts in the report.

**External code is out of scope**: symbols defined outside this workspace (e.g. in `kitex_gen/`, `thrift_gen/`, vendor packages, or upstream IDL-generated types) are not reviewed and must not be renamed.

## Package naming rules

Source: [Go blog — "Package names"](https://go.dev/blog/package-names) (Sameer Ajmani). Package identifiers deserve their own pass because a package name is a prefix on _every_ exported symbol it contains; fixing the package name often dissolves a pile of stutter-rule violations at once. Evaluate package names from the _caller's_ point of view — write a line of client code using the package and see if it reads well.

| #   | Rule                                                                                                                                                                                                                                                                                    | Bad                                                              | Good                                                                                |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| P1  | **Short, lowercase, single word.** No `under_scores`, no `mixedCaps`, no plurals. Usually a simple noun.                                                                                                                                                                                | `package httpUtils`, `package string_set`, `package models`      | `package http`, `package stringset`, `package model`                                |
| P2  | **Abbreviate only when the abbreviation is unambiguous and widely understood.** Mirrors stdlib (`fmt`, `strconv`, `syscall`).                                                                                                                                                           | `package srvmgr`, `package cfgldr`                               | `package server`, `package config` (or a clear abbrev with precedent)               |
| P3  | **Don't steal good variable names.** If clients will want the bare word as a local variable, give the package a different name.                                                                                                                                                         | `package buf` (clients want `buf := ...`)                        | `package bufio`                                                                     |
| P4  | **No grab-bag packages.** `util`, `common`, `helpers`, `misc`, `shared`, `base` give zero information, attract unrelated deps, and collide on import. Split by what the code _does_.                                                                                                    | `package util` with `NewStringSet`, `ParseJSON`, `RetryHTTP`     | `package stringset`, `package jsonconfig`, `package httpretry`                      |
| P5  | **No mega-packages either.** A single `types`, `api`, `interfaces`, or `dto` package that holds everything has the same problems as `util`. Use `model` (singular) as the default package for domain types, unless there are more than 30 models, in which case group types with the code that operates on them, or split into focused subpackages. | `package types` (300 structs)                                    | `package order`, `package payment`, `package inventory`, or `package model` (if <=30 models) |
| P6  | **Avoid colliding with popular stdlib / well-known package names** unless the domain truly demands it (forces every importer to alias).                                                                                                                                                 | local `package context`, `package http`, `package io`            | `package requestctx`, `package httpx` (only if a distinct name is genuinely needed) |
| P7  | **The last path element is the package name.** Don't let `import "…/foo/v2bar"` declare `package bar`, and don't repeat the parent dir (`server/serverhttp` → `server/http` declaring `package http`).                                                                                  | dir `stringutil/` declaring `package strutil`                    | dir `stringset/` declaring `package stringset`                                      |
| P8  | **Package contents don't repeat the package name.** This is the package-side mirror of symbol Rule 1 (no stutter). `New` in package `pkg` should return `pkg.Pkg` / `*pkg.Pkg`; use a typed name (`NewTicker → *time.Ticker`) only when the returned type isn't the package's namesake. | `chain.NewChain()`, `http.HTTPServer`, `stringset.SortStringSet` | `chain.New()`, `http.Server`, `(stringset.Set).Sort()`                              |

**Worked refactor (from the blog):** a `package util` exposing `util.NewStringSet(...)` / `util.SortStringSet(set)` becomes `package stringset` exposing `type Set`, `stringset.New(...)`, and a `(Set).Sort()` method — shorter call sites, no stutter, a name that says what it is.

**Caveats specific to this skill:**

- **Renaming a package is heavier than renaming a symbol.** It means renaming the _directory_, updating the `package` clause in every file, and rewriting every import path across the workspace (and possibly downstream modules). `gopls rename` _can_ rename a package when invoked on a `package` clause, but verify the result with `go build ./...` and a `grep -rn` for the old import path afterward. Treat any package rename as higher-risk than a symbol rename and call it out prominently in the report.
- **External / generated package names are out of scope** — never propose renaming `kitex_gen`, `hertz_gen`, `thrift_gen`, vendored packages, or `main`.
- **Test packages**: a `foo_test` external-test package is intentional, not a violation — don't flag the `_test` suffix.
- If a package name is bad but the _right_ fix is structural (split a `util` into several focused packages), don't try to automate it — describe the recommended split in the report and leave it for the user, since `gopls rename` can't restructure packages.

## Workflow

### Step 1 — Inventory

1. Run prerequisite check.
2. Enumerate target `.go` files (after generated/vendor exclusion).
3. Build the **package list**: `go list ./...` (or, grouped by directory, the set of `package` clauses in the kept files). Each distinct package is a candidate for the package-naming pass (rules P1–P8).
4. Use `gopls workspace_symbol` or AST scan via `go doc`/`grep` for exported declarations to build a symbol candidate list. For non-exported, scan with regex like `^\s*(func|type|var|const)\s+([A-Za-z_]\w*)`.
5. Build a TodoWrite list with one item per rule pass (symbol rules 1–13 plus a "package names (P1–P8)" pass) so progress is visible to the user.

### Step 2 — Analyze & propose

First do the **package-naming pass** (rules P1–P8): for each package, write a one-line client-side usage example and judge it; if a package name is bad, decide whether the fix is a _rename_ (mechanical — note the `package` clause site `file:line:col`) or a _restructure_ (e.g. splitting `util`), and if the latter, describe it for the report rather than automating it. Then, for each candidate symbol:

- Identify the rule it violates (if any). A symbol may pass — skip silently.
- Determine the proposed new name.
- Capture the **definition site** `file:line:col` (needed for `gopls rename`). Use Grep with `-n` and compute column from the line content (column of the first character of the identifier).
- Estimate scope of change by running `gopls references file:line:col` (or fall back to `grep -rn '\bOldName\b'` for an upper-bound count — note this overestimates because it cannot distinguish same-named symbols in other scopes).
- Skip if the symbol is exported AND the package is consumed by other modules outside this workspace (would be a breaking API change) — flag separately under a "BREAKING — needs human decision" section.

### Step 3 — Report

Output a single markdown report in this exact shape:

```
# golang-naming review — <module path>

Scope: <N> files, <M> packages scanned (excluded: <list>)
gopls: <version>

## Package names (<count>)

### Renameable (mechanical)
| # | Package | `package` clause at | New name | Import path now → after | Importers | Notes |
|---|---------|---------------------|----------|-------------------------|-----------|-------|
| P1 | `util` | internal/util/util.go:1:9 | `stringset` | `.../internal/util` → `.../internal/stringset` | 12 | also rename the directory |

### Restructure recommended (manual — not automated)
| Package | Problem (rule) | Suggested split |
|---------|----------------|-----------------|
| `types` | P5 mega-package | `order`, `payment`, `inventory` |

## Proposed renames (<count>)

### Rule 1 — No stutter
| # | Symbol | Defined at | New name | References | Notes |
|---|--------|-----------|----------|------------|-------|
| 1 | `region.RegionManager` | config/region.go:14:6 | `region.Manager` | 23 | exported |
...

### Rule 2 — Acronym casing
...

## BREAKING — manual decision needed (<count>)
Exported symbols or package import paths whose rename would change the public API.

| Symbol / package | Reason | Suggestion |
|------------------|--------|------------|

## Skipped
Files excluded (generated/vendored): <count>
Symbols already conforming: <count>

---
**Next step**: reply with one of:
- `apply all` — execute every proposed rename
- `apply 1,3,5-7` — execute selected items (comma list, ranges allowed)
- `apply rule 2` — execute one rule's batch
- `apply P1` — execute a package rename
- `cancel` — exit without changes
```

### Step 4 — Apply (only after explicit approval)

For each approved item:

```bash
gopls rename -w <file>:<line>:<col> <NewName>
```

For an approved **package rename** (P-items), invoke `gopls rename -w` on the `package` clause position:

```bash
gopls rename -w <file>:1:9 <newpkgname>   # col 9 == first char after "package "
```

`gopls` rewrites the `package` clause in every file of the package _and_ every import path workspace-wide. It does **not** rename the directory — do that yourself afterward (`git mv internal/util internal/stringset`) so the path's last element matches the new package name (rule P7), then re-run `go build ./...`. Do package renames _before_ symbol renames in the same package (the symbol positions shift less that way), and one package at a time. For _restructure_ recommendations, do nothing — they were reported, not approved for automation.

Important details:

- Run renames **sequentially**, not in parallel (line/col offsets shift after each rename).
- After **each** rename, re-resolve the next item's `file:line:col` if it's in the same file (a previous rename may have shifted line numbers).
- A simpler robust strategy: between renames, re-run Grep to confirm the new definition position still matches before applying the next.

After all renames in a batch:

```bash
go build ./... 2>&1 | tail -50
go vet ./... 2>&1 | tail -50
```

Report build/vet results. If build fails, surface the error and stop — do not attempt further renames.

### Step 5 — Wrap-up

Output a concise summary:

- N symbol renames applied successfully
- N package renames applied (plus directory moves done / pending)
- M failed (with error)
- Build status: pass/fail
- Restructure recommendations the user still needs to act on (e.g. split `util`)
- Suggested follow-ups (e.g. update CHANGELOG, regenerate mocks if any rename touched a mocked interface, fix any import paths in non-Go files)

## Edge cases

- **Method on an interface**: `gopls rename` automatically renames all implementations. Verify with `gopls implementations` first and report the count to the user before applying.
- **Symbol used in struct tags or reflection (`reflect.TypeOf(x).Field(i).Name`)**: `gopls` will NOT update these. Grep for the old name in `.go` string literals as a safety check after rename; surface any remaining hits to the user.
- **Symbol referenced in tests via build tags or `//go:build` constraints**: gopls handles these as long as the build constraints allow loading; if not, warn the user.
- **Generated code that references the renamed symbol**: e.g. kitex_gen handlers. These should be regenerated, not edited. Flag this in the report and suggest the user re-run codegen after rename.
- **Package rename ≠ directory rename**: `gopls` updates `package` clauses and import paths but not the folder name. Always follow a package rename with the matching `git mv` so the path's last element equals the package name (rule P7), then rebuild. Conversely, never propose a package rename for `main` packages, generated-code packages, or `_test` external test packages.
- **Import path appearing in non-Go files**: build scripts, `//go:generate` directives, Bazel `BUILD` files, Dockerfiles, docs. `gopls` won't touch these — `grep -rn` the old path after a package rename and surface remaining hits.

## Refusal templates

- Non-Go file: `"golang-naming reviews Go source only. The file <path> is <ext>. Please point me at .go files."`
- gopls missing: `"gopls is not installed. Install with: go install golang.org/x/tools/gopls@latest"`
- Standalone invocation without clear intent: `"golang-naming was invoked outside a parent agent context. Did you mean to run a naming review? If yes, please confirm with 'run golang-naming review'."`

## Anti-patterns you must avoid

- Using `sed`, `Edit`, or `grep -l ... | xargs sed` to rename Go symbols or import paths — **forbidden**.
- Applying renames before showing the report.
- Renaming exported symbols, or changing a package's import path, for a package consumed outside this workspace without flagging as breaking.
- Trying to automate a `util`/`common`/`types` _split_ — only propose it; restructuring isn't a rename.
- Renaming a package but forgetting the matching directory `git mv` (leaves path ≠ package name).
- Batching renames in parallel (offsets shift).
- Skipping the post-rename `go build ./...` verification.
