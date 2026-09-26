# Layers (MVC)

Gin is the default HTTP framework. A web service uses this layout and no other:

```tree
main.go                  # cmd.Execute() only
cmd/
  root.go
  web.go                 # WebCmd — the trigger: wire concretes, build the engine, serve
  web/                   # only when `web` has subcommands, one file per verb
    routes.go            #   RoutesCmd — print the route table
handler/
  route/
    route.go             # package route — New(Handlers) *gin.Engine; the only file that knows URLs
  middleware/
    auth.go              # package middleware — project gin.HandlerFunc
  user/                  # package user — one directory per domain
    handler.go           #   Handler, New, the service interface it consumes
    get.go               #   one file per action
svc/
  user/                  # package user — domain rules + every outgoing call (gorm, upstream HTTP, queue)
    service.go
model/
  app.go                 # APP_NAME — app name, config dir, identity audience
  user.go                # package model — structs shared across layers
```

| Layer | Package | Does | Must not import |
| ----- | ------- | ---- | --------------- |
| Trigger | `cmd/web.go` | `db.Init`, identity verifier, construct svc → handler, `route.New`, run `http.Server` | — (the only place concretes are wired) |
| Route | `handler/route` | engine, global middleware, gosdk routers, per-group auth / plan gates, `METHOD path → handler` | `svc/`, gorm |
| Middleware | `handler/middleware` | cross-cutting `gin.HandlerFunc`; deps via constructor params | `handler/<domain>`, `svc/` |
| Handler | `handler/<domain>` | bind + validate request, read claims, call svc, map error → status, render | gorm, DB drivers, viper, `http.Client` |
| Service | `svc/<domain>` | business rules, persistence, outgoing requests | gin, `handler/`, identity claims |
| Model | `model/` | structs, JSON (snake_case) + gorm tags | any project package |

Arrows point one way: `cmd → handler/route → handler/<domain> → svc/<domain> → model`. `gosdk/mw` (`CorrelationID`, `Helmet`) and `gosdk/router` (`/healthz`, `/ping`, `/stats`) are used as-is from `route`; `handler/middleware` holds only what the project adds. `model` stays one package unless >30 types, then split by domain.

## Middleware — auth via identity

Who is calling is answered by `github.com/bizshuk/identity/sdk/client-go` (skill `identity-sdk`): tokens are verified **locally** against cached JWKS, never by calling identity per request.

- API-only service (callers send `Authorization: Bearer`): use `ginmw.RequireAuth(verifier)` directly in `route` — **no** file in `handler/middleware`.
- Service that also serves browsers (the `identity_token` cookie on `.shuks.dev`): `ginmw.RequireAuth` reads only the header, so the project adds this:

```go
// handler/middleware/auth.go
package middleware

// Auth verifies the caller whether the token arrives as a bearer header or
// as the identity cookie. Claims go under ginmw's key, so ginmw.ClaimsFrom,
// RequirePlanOn and RequireFreshAuth chain after it unchanged.
func Auth(verifier *identity.Verifier) gin.HandlerFunc {
    return func(c *gin.Context) {
        claims, err := verifier.Verify(c.Request.Context(), identity.TokenFromRequest(c.Request))
        switch {
        case errors.Is(err, identity.ErrInsufficientAssurance):
            // a 401 without a challenge leaves the client retrying the same token forever
            c.Header("WWW-Authenticate", identity.StepUpChallenge(verifier.Requires(), 0))
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "insufficient_user_authentication"})
        case errors.Is(err, identity.ErrRevoked):
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token revoked"})
        case err != nil:
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
        default:
            c.Set(ginmw.CLAIMS_CONTEXT_KEY, claims)
            c.Next()
        }
    }
}
```

The middleware only verifies. Plan / role / fresh-auth gates are `ginmw.RequirePlanOn` / `RequireRoleOf` / `RequireFreshAuth` attached in `route`; `403` (valid identity, not entitled) comes from those, never from here. Browser renewal (`identity_hint` → `/auth/refresh`) and start-up reconciliation against `/.well-known/identity-configuration`: `identity-sdk`.

## Handler

The interface lives **where consumed**; sentinels live in the owning `svc`. Identity comes from the claims the middleware stored — never from a raw header — and reaches svc as a plain uid:

```go
// handler/user/handler.go
package user

type service interface {
    Get(ctx context.Context, id string) (*model.User, error)
}

type Handler struct{ svc service }

func New(svc service) *Handler { return &Handler{svc: svc} }

// handler/user/get.go
func (h *Handler) Me(c *gin.Context) {
    claims, ok := ginmw.ClaimsFrom(c)
    if !ok {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
        return
    }
    u, err := h.svc.Get(c.Request.Context(), claims.UID())
    switch {
    case errors.Is(err, usersvc.ErrNotFound):
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
    case err != nil:
        slog.ErrorContext(c, "get user", "uid", claims.UID(), "correlation_id", mw.GetCorrelationID(c), "err", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
    default:
        c.JSON(http.StatusOK, u)
    }
}
```

Request structs: `c.ShouldBindJSON` then `Validate()` built on `gosdk/validator`. HTTP status mapping and error logging happen **only** in the handler.

## Service

```go
// svc/user/service.go
package user

var ErrNotFound = errors.New("user not found")

type Service struct {
    db     *gorm.DB
    client *http.Client // one per upstream, see http.md
}

func New(db *gorm.DB, client *http.Client) *Service { return &Service{db: db, client: client} }
```

Takes `*gorm.DB`, never calls `db.Init()`. Takes a uid, never `*model.Claims` — svc stays callable from CLI and jobs where no token exists. Wrap at the boundary (`fmt.Errorf("user.Get: %w", err)`). `ctx` is the first I/O param; never store it on a struct.

## Route

```go
// handler/route/route.go
package route

type Handlers struct {
    Auth gin.HandlerFunc
    User *user.Handler
}

func New(h Handlers) *gin.Engine {
    r := gin.New() // not gin.Default — its logger bypasses slog
    r.Use(gin.Recovery(), mw.CorrelationID(), mw.Helmet())
    router.HealthRouterGroup(r) // public
    router.PingRouterGroup(r)

    api := r.Group("/api/v1", h.Auth)
    api.GET("/me", h.User.Me)
    api.POST("/export", ginmw.RequirePlanOn(model.APP_NAME, "pro"), h.User.Export)
    return r
}
```

## Trigger

```go
// cmd/web.go
const IDENTITY_BASE = "https://identity.shuks.dev"

var WebCmd = &cobra.Command{
    Use: "web", Short: "Start the HTTP server",
    RunE: func(c *cobra.Command, args []string) error {
        ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
        defer stop()

        if err := db.Init(); err != nil {
            return err
        }

        // process-wide: one verifier, one JWKS cache, one revocation feed
        outbound := client.New(client.WithUserAgent(model.APP_NAME + "/1"))
        revocations := identity.NewRevocationCache(IDENTITY_BASE+"/v1/revocations", outbound)
        go revocations.Watch(ctx)
        keys := identity.NewRemoteKeySource(IDENTITY_BASE+"/.well-known/jwks.json", outbound)
        verifier := identity.NewVerifier(keys, IDENTITY_BASE, model.APP_NAME).WithRevocations(revocations)

        users := usersvc.New(db.Default.DB(), &http.Client{Timeout: 10 * time.Second})
        engine := route.New(route.Handlers{
            Auth: middleware.Auth(verifier),
            User: user.New(users),
        })

        srv := &http.Server{
            Addr:              fmt.Sprintf(":%d", viper.GetInt("port")),
            Handler:           engine,
            ReadHeaderTimeout: 5 * time.Second,
            ReadTimeout:       15 * time.Second,
            WriteTimeout:      30 * time.Second,
            IdleTimeout:       60 * time.Second,
        }
        go func() { <-ctx.Done(); srv.Shutdown(context.Background()) }()
        if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
            return err
        }
        return nil
    },
}
```

`model.APP_NAME` (also passed to `config.WithAppName`) is the identity **audience** — it must match the service's entry in identity's `SERVICES`, or every token fails as `invalid token`. A verifier per request refetches JWKS every call; build it here once. Without `revocations.Watch` a suspended user keeps access for a full token TTL.

`handler/user` and `svc/user` are both `package user`: import the handler side bare, alias the service side `<domain>svc` (`usersvc`). `web` only serves — migrations, seed, and route listing are subcommands under `cmd/web/` (two-layer rule in [cli.md](cli.md)). `RoutesCmd` builds the same engine with `route.New` and prints `engine.Routes()`.

≤4 deps: constructor params; 5+: options struct; many knobs: `...Option`.

## Tests

Same package `_test.go`. Handler — fake `service`, put claims on the context with `c.Set(ginmw.CLAIMS_CONTEXT_KEY, &idmodel.Claims{…})` (`idmodel` = `identity/model`), drive it with `httptest.NewRecorder`; never mint real tokens for handler tests. Middleware — a verifier over a static key source, pinning the five rejections `identity-sdk` lists (expired, foreign key, wrong audience, revoked, insufficient assurance) plus bearer-vs-cookie. Service — sqlite `:memory:` `*gorm.DB`, `httptest.Server` for upstreams. Validation — table-driven.
