---
name: golang-dead-code
description: Detect and safely remove dead code in Go projects — unused functions/vars/types/consts, unreachable branches.
argument-hint: [path]
arguments: path
user-invocable: false
disable-model-invocation: true
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuest
effort: xhigh
context: fork
---

# Golang Dead Code Removal Skill

Detect and remove dead code in a Go project through a **4-phase workflow**: Detect → Classify → Apply → Verify. The workflow prioritizes safety: every candidate is classified by risk level, presented to the user for confirmation before deletion, and verified with `go build` + `go test` after each batch.

## Scope

Target path: `$1` (default: current working directory if not provided).

Handles four categories of dead code:

| Category               | How it's detected                       | Default action       |
| ---------------------- | --------------------------------------- | -------------------- |
| Unused symbols         | `staticcheck -checks U1000,U1001`       | Delete after confirm |
| Unreachable code       | `go vet` + `staticcheck -checks SA4006` | Delete after confirm |
| Module-level dead code | `golang.org/x/tools/cmd/deadcode`       | Delete after confirm |
| Deprecated APIs        | grep for `// Deprecated:` comments      | Migrate or delete    |

---

## Phase 0: Tool Installation Check

**Before anything else**, verify the required tools are installed. Run this exact sequence:

```bash
# Verify Go is installed and find GOPATH/bin
go version || { echo "Go is not installed. Install from https://go.dev/dl/"; exit 1; }
GOBIN="$(go env GOBIN)"
[ -z "$GOBIN" ] && GOBIN="$(go env GOPATH)/bin"
echo "Go binaries directory: $GOBIN"

# Check if GOBIN is on PATH
case ":$PATH:" in
  *":$GOBIN:"*) echo "PATH OK";;
  *) echo "WARNING: $GOBIN is not on PATH. Add it with: export PATH=\"\$PATH:$GOBIN\"";;
esac

# Check each required tool
for tool in staticcheck deadcode unparam; do
  if command -v "$tool" >/dev/null 2>&1; then
    echo "$tool: installed"
  else
    echo "$tool: MISSING"
  fi
done
```

### Installing Missing Tools

If any tool reports `MISSING`, install with the commands below. Ask the user before running `go install` (it modifies their environment).

```bash
# staticcheck — primary unused-symbol detector (U1000/U1001)
go install honnef.co/go/tools/cmd/staticcheck@latest

# deadcode — whole-program reachability analysis (requires main package entry points)
go install golang.org/x/tools/cmd/deadcode@latest

# unparam — finds unused function parameters and return values
go install mvdan.cc/unparam@latest
```

For **reproducible installs** (CI or team setups), pin versions instead of `@latest`:

```bash
go install honnef.co/go/tools/cmd/staticcheck@v0.4.7
go install golang.org/x/tools/cmd/deadcode@v0.21.0
go install mvdan.cc/unparam@v0.0.0-20240528143540-8a5130ca722f
```

After installing, re-run the check loop above to confirm everything is on `PATH`.

---

## Phase 1: Detect

Run the tools on `$1` (or `.`) and collect raw findings. **Do not delete anything yet.**

```bash
TARGET="${1:-.}"
cd "$TARGET" || exit 1

# 1a. staticcheck: unused identifiers + unreachable code
staticcheck -checks=U1000,U1001,SA4006 ./... 2>&1 | tee /tmp/dead-staticcheck.txt

# 1b. deadcode: whole-program callgraph analysis
#    NOTE: requires at least one `package main` entry point. Skip silently if none exist.
if grep -rl "^package main" --include="*.go" . >/dev/null 2>&1; then
  deadcode ./... 2>&1 | tee /tmp/dead-deadcode.txt
else
  echo "No main package found — skipping deadcode (library-only module)" | tee /tmp/dead-deadcode.txt
fi

# 1c. unparam: unused parameters / return values
unparam ./... 2>&1 | tee /tmp/dead-unparam.txt

# 1d. Deprecated APIs: grep for `// Deprecated:` markers
grep -rn "// Deprecated:" --include="*.go" . | tee /tmp/dead-deprecated.txt
```

Read `/tmp/dead-*.txt` and produce a consolidated list of candidate symbols, each with: file:line, symbol name, category, source tool.

---

## Phase 2: Classify

For **each candidate**, classify into one of three risk buckets using `Grep` to cross-check usage:

| Bucket                | Criteria                                                                            | Default         |
| --------------------- | ----------------------------------------------------------------------------------- | --------------- |
| **Safe**              | unexported (lowercase first letter), no test references, no reflect/plugin patterns | Auto-stage      |
| **Deprecated-in-use** | marked `// Deprecated:` but `grep` finds active callers                             | Show migration  |
| **Risky**             | exported symbol OR matches `reflect.`, `plugin.Open`, build tags, `//go:linkname`   | Skip by default |

For each candidate, also run:

```bash
grep -rn "\b<symbolName>\b" --include="*.go" .
```

to verify there are no callers the static tools missed (interface satisfaction, struct tags, generated code).

Present the classification as a table grouped by file, then use `AskUserQuestion` to confirm which buckets to act on:

- **Delete all Safe**
- **Review Deprecated-in-use one-by-one**
- **Skip Risky (default)** — let user opt-in per item

---

## Phase 3: Apply

For each confirmed deletion:

1. Use `Read` to load the surrounding context (±10 lines).
2. Use `Edit` to remove the symbol AND its associated:
    - Doc comment block immediately above
    - Trailing blank line if removal would leave a double-blank
3. For **deprecated symbols with callers**, do NOT delete — instead, list the call sites for the user to migrate first.
4. Process deletions in **batches per file** to keep diffs reviewable.

After each batch, run:

```bash
gofmt -w <modified-files>
goimports -w <modified-files>   # if available
```

to clean up imports that may now be unused.

---

## Phase 4: Verify

After all edits in a batch:

```bash
go build ./...     # must pass
go vet ./...       # should not regress
go test ./... -count=1 -short   # optional but recommended
```

If anything fails:

1. **Report which file/symbol broke the build.**
2. Offer to revert via `git checkout -- <file>` (ask first — never auto-revert).
3. Re-classify the broken symbol as Risky and continue with the rest.

---

## Usage Examples

```bash
# Whole module
/golang-dead-code

# Specific package
/golang-dead-code ./internal/auth

# Specific subdirectory
/golang-dead-code cmd/server
```

## Safety Rules

- **Never delete without explicit user confirmation** (per file or per batch is OK; never silent).
- **Never `--no-verify`, never bypass `go build`** failures.
- **Never auto-delete exported symbols** even if marked unused — they may be part of a public API consumed externally.
- **Always show the call-site grep output** for Deprecated-in-use symbols so the user can see migration cost.
- **Never modify generated files** (`*.pb.go`, `*_gen.go`, files starting with `// Code generated`).

## Known False-Positive Patterns to Watch For

| Pattern                                | Why static tools flag it       | What to do                            |
| -------------------------------------- | ------------------------------ | ------------------------------------- |
| `reflect.ValueOf(x).MethodByName(...)` | Method called by string name   | Mark Risky, keep                      |
| `//go:linkname foo bar`                | Linked from another package    | Mark Risky, keep                      |
| Methods satisfying an interface        | Tools may miss interface match | Verify with `grep -rn "<MethodName>"` |
| `init()` functions                     | Called implicitly              | Never delete                          |
| Test helpers (`testing.TB`)            | Only used in `_test.go`        | Check both file types                 |
| `go:embed` target functions            | Used via embed directives      | Mark Risky, keep                      |
