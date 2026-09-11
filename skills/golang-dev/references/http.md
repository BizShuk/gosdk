# HTTP & networking

```go
s := gin.Default()
s.Use(mw.CorrelationID(), mw.Helmet())
router.Default(s)           // /stats
router.HealthRouterGroup(s) // /healthz
router.PingRouterGroup(s)   // /ping
s.Run(":8080")
```

Prefer `net/http` unless routing complexity needs gin. Production servers use `http.Server` with every timeout set — `s.Run` is samples only.

**Client.** Drain then close: `io.Copy(io.Discard, resp.Body)` then `resp.Body.Close()`. Never `http.Get` / `http.DefaultClient` — one `http.Client` per upstream, with `Timeout` and a tuned `Transport` (`MaxIdleConns`, `MaxIdleConnsPerHost`, `MaxConnsPerHost`, `IdleConnTimeout`). Retries via `gosdk/http` (import alias `gohttp`): only idempotent ops, cap + exponential backoff + jitter, honor `Retry-After`, pair with a circuit breaker. A slow dependency must not share a pool with a fast one.

**Server.** Set `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` — a zero `http.Server{}` is a slow-loris. Bound accept/handler concurrency (semaphore or pool); shed with `503` + `Retry-After` rather than unbounded `go handle(c)`. Handler work: `context.WithTimeout`.

**Conn.** Wrap `net.Conn` in `bufio` (4–8 KB), `Flush` before `Close`. Pool buffers; copy bytes out before they escape (`append([]byte(nil), b[:n]...)`). Deadlines on every blocking op; reset on activity for long-lived / WebSocket (ping/pong).

**TLS.** `MinVersion: tls.VersionTLS12`. Session resumption: persistent `SessionTicketKey` on the server, `ClientSessionCache` on the client. No `InsecureSkipVerify`.

Reuse connections — never dial per RPC. HTTP/2 for public APIs; gRPC for internal typed RPC; raw TCP only when you own framing. Go has no DNS cache — pre-resolve or TTL-cache at scale. Measure under load before tuning; OS limits (`ulimit`, `somaxconn`) are checklists, not magic numbers.
