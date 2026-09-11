# Layers

| Layer | Package | Does | Must not import |
| ----- | ------- | ---- | --------------- |
| Handler | `handler/` | HTTP + business rules; map I/O | `repository/`, DB drivers, `config/` |
| Service | `service/` | Thin adapter over APIs/queues/caches | `handler/` |
| Repository | `repository/` | DB only; returns domain objects | `handler/`, `service/` |
| Model | `model/` | Structs + `ToDTO` / `FromRow` | handler/service/repository |
| Validation | `validation/` | Named validators | handler/service |
| Config | `config/` | Load + wire concretes | — |

Build order: model → repository/service → handler → validation → `main`/`bootstrap`. Interfaces live **where consumed**:

```go
// handler/user.go — not in repository/
type userGetter interface {
    GetByID(ctx context.Context, id string) (*model.User, error)
}
```

Wire concretes only in `main`/`bootstrap`. Inject interfaces everywhere else. ≤4 deps: constructor params; 5+: options struct; many knobs: `...Option`. `model` (singular) unless >30 types, then split by domain. JSON tags: snake_case. `Validate()` on request structs; errors are `validation.Error{Field, Reason}`.

Wrap at each boundary (`fmt.Errorf("handler.GetUser: %w", err)`). Sentinels in the owning package (`ErrNotFound`). HTTP status mapping and logging only in handler. `ctx` is the first I/O param; never store it on a struct; timeouts from config.

Tests: same package `_test.go`. Handler — mock interfaces + `httptest`. Repository — sqlmock or real test DB. Validation — table-driven.
