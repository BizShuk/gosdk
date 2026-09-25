# SOLID

Composition, small interfaces, package boundaries. No inheritance.

| | Smell | Do |
| - | ----- | -- |
| S | One type owns auth + billing + mail | Split by reason to change |
| O | `if kind == "stripe"` chains | Strategy interface; add impls without editing callers |
| L | One `Get` returns `nil, nil`, another panics | Same contract + same sentinel (`ErrNotFound`) |
| I | Fat repo interface, handler needs `Get` | Interface **where consumed** (`userGetter`) |
| D | `NewRepo()` calls `gorm.Open` or reads `DB_DSN` itself | Constructor injection; only `main`/`bootstrap` calls `db.Init()` and passes `db.Default.DB()` |

Accept interfaces, return concrete types. No service locator. Global state only if immutable (config, clients).
