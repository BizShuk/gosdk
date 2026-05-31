---
name: golang-mvc
description: >
    Go MVC architecture conventions for feature implementation and refactoring.
    Defines layer rules (handler/service/repository/model), interface placement,
    constructor injection, error wrapping, context propagation, and test patterns
    for Go projects. Actively enforces and applies layer conventions — modifies
    and refactors code that violates MVC boundaries. Distinct from
    golang-code-quality which focuses on SOLID and code-level patterns.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuest
disable-model-invocation: true
user-invocable: false
context: fork
---

# golang-mvc

Go MVC implementation conventions for **feature work and refactoring**.
This skill **guides code generation and actively enforces layer boundaries** —
it modifies and refactors code that violates MVC conventions.

---

## Layer Map

| Layer      | Package       | Responsibility                                                              | May import                          | Must NOT import                       |
| ---------- | ------------- | --------------------------------------------------------------------------- | ----------------------------------- | ------------------------------------- |
| Handler    | `handler/`    | HTTP entry point; orchestrate business rules; validate and map input/output | `service/`, `model/`, `validation/` | `config/`, DB drivers, `repository/`  |
| Service    | `service/`    | Thin adapter over external APIs, queues, caches                             | `model/`, `config/`                 | `handler/`                            |
| Repository | `repository/` | DB access only; returns domain objects                                      | `model/`, `config/`                 | `handler/`, `service/`                |
| Model      | `model/`      | Pure data: domain structs + conversion methods (`ToDTO`, `FromRow`)         | nothing internal                    | `handler/`, `service/`, `repository/` |
| Validation | `validation/` | Named validators for domain rules                                           | `model/`                            | `handler/`, `service/`                |
| Config     | `config/`     | Load env/files; construct and wire concrete types                           | all                                 | —                                     |

---

## Feature Implementation Checklist

Build in this order. Do not skip steps.

### Step 1 — Model first (`model/`)

- Define new domain structs in `model/` (use `model` (singular) as the default package for domain models, unless there are more than 30 models, in which case they must be split into domain-specific packages).
- No behavior beyond conversion methods (`ToDTO()`, `FromRow()`).
- Every exported field: snake_case JSON tag.

### Step 2 — Repository / Service

- DB access → `repository/` package. One file per aggregate root.
- External API/queue → `service/` package. Retry + timeout logic only; no business decisions.
- Both MUST define an interface in the **consumer** package (see Interface Placement Rule below).

### Step 3 — Handler (`handler/`)

- One handler struct per resource or domain area.
- Constructor: `func NewUserHandler(getter UserGetter, notifier NotifySender, log Logger) *UserHandler`
- All business decisions live here. If a handler method exceeds ~100 lines, split into named helpers.
- Route registration: a separate `RegisterRoutes(r *gin.RouterGroup)` method; never inline in `main.go`.

### Step 4 — Validation (`validation/`)

- Every request struct gets a `Validate() error` method.
- Validator names describe what they check: `ValidateCreateOrderRequest`, not `validate`.
- Validation errors: return `validation.Error{Field: "...", Reason: "..."}` — never raw strings.

### Step 5 — Wiring (`main.go` or `bootstrap.go`)

- Concrete types only in `main.go`/`bootstrap.go`.
- Inject interfaces everywhere else. (Exception: global state is acceptable/good for client, handler, and configuration if they are immutable).
- Configuration loading: Prefer `config.Default()` from `github.com/bizshuk/gosdk`, falling back to raw `viper` manual setup only if the SDK is not supported/available.
- Initialization order: config → DB connection → repositories → services → handlers → router.

---

## Interface Placement Rule (critical for testability)

Interfaces are defined **where consumed**, not where implemented.

```go
// In handler/user.go — NOT in repository/user.go
type userGetter interface {
    GetByID(ctx context.Context, id string) (*model.User, error)
}
```

This means handler tests can mock the interface without importing the repository package,
breaking the import cycle and enabling true unit isolation.

---

## Constructor Injection Rules

| Dependency count    | Pattern                        |
| ------------------- | ------------------------------ |
| ≤ 4 deps            | Plain constructor params       |
| 5+ deps             | `HandlerOptions` struct        |
| Many optional knobs | Functional options `...Option` |

---

## Error & Constant Conventions

- Wrap at every layer boundary: `fmt.Errorf("handler.GetUser: %w", err)`
- Sentinel errors defined in the package that owns the concept: `var ErrNotFound = errors.New("not found")`
- HTTP status mapping lives in the handler layer only — never map errors in service or repository
- Log once at the handler boundary; service and repository only wrap and return
- All constants must use `SCREAMING_SNAKE_CASE` (e.g. `MAX_RETRIES`, `DEFAULT_TIMEOUT`).

---

## Context Conventions

- `ctx context.Context` is always the **first parameter** of any function that does I/O
- Never store `ctx` in a struct
- Wrap every DB query and external call: `ctx, cancel := context.WithTimeout(ctx, cfg.DBTimeout); defer cancel()`
- Timeouts come from config — never hardcode duration literals

---

## Test Patterns

| Layer      | Test approach                                                                       |
| ---------- | ----------------------------------------------------------------------------------- |
| Handler    | Mock all interfaces; use `httptest.NewRecorder()`; assert status + response body    |
| Repository | Use `sqlmock` or a real test DB via docker-compose; never mock the DB driver itself |
| Service    | Mock external HTTP/gRPC clients with interface mocks                                |
| Validation | Table-driven tests required; cover valid, missing, and out-of-range inputs          |

All test files: `_test.go` suffix, same package as the code under test (prefer white-box
tests for internal helpers, `_test` package suffix for public API contracts).
