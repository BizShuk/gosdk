---
name: golang-code-quality
description: Golang code quality reviewer focused on simplicity and scalability through SOLID principles, idiomatic package structure, error handling, context propagation, and dependency injection. Use when reviewing Go code, generating new Go code, or refactoring existing Go codebases. Triggers on requests like "review this Go code", "refactor this Go file", "is this Go idiomatic", or any new Go file/package creation.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuest
disable-model-invocation: false
user-invocable: false
context: fork
---

# golang-code-quality

A senior Go code quality review workflow, with a focus on **simplicity (簡潔性)** and **scalability (可擴展性)** — NOT testing. Guiding philosophy: code should be obvious to read, hard to misuse, and easy to extend without modification.

## Core Philosophy / 核心哲學

> "Clear is better than clever." — Rob Pike

Two non-negotiable goals when reviewing or writing Go:

1. **Simplicity**: A new engineer should understand any single function in under 60 seconds.
2. **Scalability**: Adding a new feature should mean adding new code, rarely modifying existing code (Open/Closed Principle).

If a piece of code violates either, flag it.

---

## 1. SOLID Principles in Go

Go has no inheritance — apply SOLID through **composition**, **small interfaces**, and **package boundaries**.

### S — Single Responsibility Principle

- One package = one reason to change. One struct = one responsibility.
- ❌ A `UserService` that handles auth, billing, and email notifications.
- ✅ Split into `auth.Service`, `billing.Service`, `notify.Service`.

### O — Open/Closed Principle

- Code should be open for extension, closed for modification.
- Achieved via **interfaces** + **strategy pattern**.
- ❌ `if paymentType == "stripe" { ... } else if paymentType == "paypal" { ... }`
- ✅ `type PaymentProvider interface { Charge(ctx, amount) error }` — add new providers without touching existing code.

### L — Liskov Substitution Principle

- Any implementation of an interface must honor its contract (including error semantics, nil-safety, idempotency).
- ❌ One `Storage.Get` returns `nil, nil` for missing keys; another panics.
- ✅ Define explicit sentinel errors like `ErrNotFound` in the interface's package and require all implementations to use them.

### I — Interface Segregation Principle

- **Define interfaces where they are consumed, not where they are implemented.**
- Prefer many small interfaces over one fat interface. `io.Reader` and `io.Writer` are the gold standard.
- ❌ `type UserRepo interface { Get; Create; Update; Delete; Search; Export; Import }` consumed by a handler that only needs `Get`.
- ✅ Handler defines `type userGetter interface { Get(ctx, id) (*User, error) }` locally.

### D — Dependency Inversion Principle

- High-level modules depend on **abstractions**, not concrete implementations.
- Constructor injection is the idiomatic Go way (no DI frameworks needed).
- ❌ `func NewHandler() *Handler { return &Handler{db: postgres.New()} }`
- ✅ `func NewHandler(repo UserRepo, logger Logger) *Handler { ... }`

---

## 2. Package Structure (MVC-style)

Enforce this layered architecture. **Dependencies flow downward only** — handler → service → model. No upward or sideways imports between siblings.

| Package                | Responsibility                                                                                      | What MUST NOT be there                          |
| ---------------------- | --------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `config/`              | Load configuration. Prefer `config.Default()` from `github.com/bizshuk/gosdk`, falling back to `viper` if not supported. Initialize external clients (DB, Redis, S3, HTTP clients). | Business logic, domain types                    |
| `handler/`             | Aggregate domain logic. Orchestrate calls to services. **All business rules live here.**            | Direct DB calls, raw HTTP calls, config loading |
| `service/` (or `svc/`) | Wrap external services. Basic error handling, retries, timeout enforcement. **Thin adapters only.** | Domain logic, business rules                    |
| `model/`               | Data structures + conversions between them (DTO ↔ domain ↔ persistence).                            | Behavior beyond conversion, I/O                 |
| `validation/`          | Common validation rules. **Each validator must have an explicit, descriptive name.**                | Business decisions, side effects                |
| `utils/`               | Non-business, non-functional concerns: metrics emission, callbacks, helper closures.                | Anything domain-specific                        |

### Validation naming rule

Validators must be named for **what they check**, not where they're used:

- ❌ `validateInput`, `checkUser`, `validate`
- ✅ `ValidateEmailFormat`, `ValidateAgeRange`, `ValidatePasswordComplexity`

### Architectural smell checks

- Does `service/` import `handler/`? → **Violation**, flag it.
- Does `model/` have any I/O? → **Violation**, models are pure data.
- Is there business logic inside `service/`? → Move to `handler/`.
- Is `utils/` becoming a junk drawer? → Split into named packages (`metrics/`, `callback/`).

---

## 3. Package Naming (per <https://go.dev/blog/package-names>)

Enforce these rules strictly:

- **Short, lowercase, no underscores, no mixedCaps.** `userauth` not `user_auth` or `userAuth`.
- **Singular, not plural.** `user` not `users`. (Use `model` (singular) as the default package for domain models, unless there are more than 30 models, in which case they must be split into domain-specific packages).
- **Avoid generic names**: `util`, `common`, `base`, `helpers`, `misc`, `shared`. These signal a missing abstraction.
- **No stutter.** If the package is `user`, the type is `User` not `user.UserModel`. Callers write `user.New()` not `user.NewUser()`. (Exception: the `model` package can contain a `model.User` or similar domain structs).
- **Name describes what it provides, not what it contains.** `http` not `httpfunctions`.
- The acceptable exception to `utils/` per the user's convention: it stays narrowly for non-business cross-cutting concerns (metrics, callbacks). If it grows beyond that, split it.

---

## 4. Error Handling Conventions

- **Errors are values.** Return them, don't panic. `panic` only for truly unrecoverable programmer errors (nil pointer in init, etc.).
- **Wrap with context** using `fmt.Errorf("doing X: %w", err)`. The `%w` verb preserves the chain for `errors.Is` / `errors.As`.
- **Define sentinel errors** at the package level for known conditions: `var ErrNotFound = errors.New("not found")`.
- **Define error types** when callers need structured info: `type ValidationError struct { Field string; Reason string }`.
- **Don't log and return.** Pick one. Logging at every layer creates noise; log once at the top (handler boundary).
- **Don't ignore errors with `_`** unless you've written a comment explaining why.
- **Check errors immediately**, on the line after the call. Never let an error variable live across multiple statements.

```go
// ❌ Bad
result, err := svc.Fetch(ctx, id)
log.Printf("fetch failed: %v", err)
return nil, err

// ✅ Good — at service layer, just wrap
result, err := svc.Fetch(ctx, id)
if err != nil {
    return nil, fmt.Errorf("fetch user %s: %w", id, err)
}
```

---

## 5. Context (`context.Context`) Conventions

- **`ctx` is always the first parameter** of any function that does I/O, blocks, or might be cancelled.
- **Never store `ctx` in a struct field.** Pass it through explicitly.
- **Never pass `nil` context.** Use `context.TODO()` if you genuinely don't have one yet (and add a comment).
- **`context.Value` is for request-scoped data only** (request ID, auth user, trace span). Not for optional parameters.
- Use **typed keys** for context values to avoid collisions:

    ```go
    type ctxKey struct{ name string }
    var userKey = ctxKey{"user"}
    ```

- Always **honor cancellation**: check `ctx.Done()` in loops, pass `ctx` down to every downstream call.

---

## 6. Dependency Injection Patterns

Go DI is **constructor injection**. No frameworks required.

### Pattern A: Plain constructor (preferred for ≤4 deps)

```go
type Handler struct {
    repo   UserRepo
    mailer Mailer
    logger Logger
}

func NewHandler(repo UserRepo, mailer Mailer, logger Logger) *Handler {
    return &Handler{repo: repo, mailer: mailer, logger: logger}
}
```

### Pattern B: Options struct (when ≥5 deps or many optional)

```go
type HandlerOptions struct {
    Repo    UserRepo
    Mailer  Mailer
    Logger  Logger
    Metrics MetricsEmitter // optional
}

func NewHandler(opts HandlerOptions) *Handler { ... }
```

### Pattern C: Functional options (for libraries with many optional knobs)

```go
func NewHandler(repo UserRepo, opts ...Option) *Handler { ... }
```

### Rules

- **Wire dependencies in `main.go` or a single `wire.go` / `bootstrap.go`.** This is the only place concrete types meet.
- **Accept interfaces, return concrete types.** This lets callers narrow what they depend on.
- **No global state.** Inject databases/services. (Exception: global state is acceptable/good for client, handler, and configuration if they are immutable).
- **No service locator pattern.** No `container.Get("UserService")`.

---

## Review Workflow

When asked to review Go code:

1. Run `git diff --name-only` (if in a repo) to scope changes, otherwise read the file/package the user pointed to.
2. Walk through the checklist below and produce findings organized by severity.
3. For each finding: cite the file and line, explain WHY it's a problem (link to the principle), and show the fix.

## Review Checklist

**🔴 Critical (must fix)**

- Cyclic imports between packages
- Business logic in `service/` or `model/`
- Concrete types injected instead of interfaces
- Missing context propagation in I/O paths
- Errors swallowed (`_ = err`) without justification
- `panic` used for recoverable conditions

**🟡 Warnings (should fix)**

- Fat interfaces (>3 methods unless genuinely cohesive)
- Interfaces defined in the producer package instead of consumer
- Generic package names (`util`, `common`, `helpers`)
- Stutter in naming (`user.UserService`)
- Errors returned without `%w` wrapping
- Validators with non-descriptive names
- Constants not using `SCREAMING_SNAKE_CASE`

**🟢 Suggestions (consider)**

- Functions exceeding ~50 lines (signal of mixed responsibilities)
- Structs with >7 fields (consider splitting)
- Repeated `if err != nil` blocks that could become a helper
- Magic numbers/strings that should be named constants

## Output Format

Always structure findings like this:

```
🔴 CRITICAL — handler/user.go:42
Issue: Direct DB query in handler bypasses service layer
Why: Violates layered architecture — handler should orchestrate, not access storage
Fix:
  // before
  rows, err := h.db.Query("SELECT ...")
  // after
  user, err := h.userSvc.GetByID(ctx, id)
```

Be specific, cite line numbers, and show the diff. Never say "this could be improved" without showing how.

## When generating new code

- Start by sketching the package layout and interface boundaries before writing implementations.
- Generate one package at a time, top-down: `model/` → `service/` → `handler/` → wiring in `main.go`.
- Every exported function gets a doc comment starting with the function name (`// NewHandler returns ...`).
- Prefer returning early. Avoid nesting beyond 2 levels.

## When refactoring

- Identify the violation, name the principle being broken, propose the smallest change that fixes it.
- Preserve external behavior. Refactors are not feature changes.
- Suggest one refactor at a time when multiple exist; let the user prioritize.
