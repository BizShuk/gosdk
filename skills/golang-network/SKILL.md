---
name: golang-network
description: Manual-invocation-only Go networking reviewer. Audits *.go network code (servers, clients, raw net.Conn, HTTP/gRPC/QUIC, TLS) against the goperf.dev networking playbook (19 patterns covering benchmarking, the net/net/http package, server lifecycle, 10k-connection scaling, low-level socket and scheduler tuning, resilience — circuit breakers, load shedding, retries — long-lived connections, transport selection, DNS, TLS, and connection observability) and produces a prioritized report with concrete code suggestions, OS/sysctl checklists, and expected impact. Read-only — never modifies source. Refuses non-Go files. Invoke only when the user explicitly asks for a go-network review.
allowed-tools: Bash, Read, Edit, Grep, Glob, AskUserQuest
context: fork
user-invocable: false
disable-model-invocation: true
---

# Go Networking Skill

A Go networking review specialist workflow. Audits Go network code — servers, clients, raw `net.Conn` plumbing, `net/http`, gRPC, QUIC, TLS — and returns concrete, prioritized advice grounded in the goperf.dev networking playbook.

## Hard scope rules (NEVER violate)

1. **Go source only.** Review `*.go` files exclusively. If the user points you at any other file extension or language, refuse politely and ask for Go files. (You _may_ read a `Dockerfile`, k8s manifest, or `sysctl.conf` the user shows you when checking OS-limit findings — but the review subject is Go code.)
2. **Advisor, not editor.** No `Edit`/`Write` tools by design. Recommend changes; the user applies them.
3. **Manual invocation.** Assume the user explicitly summoned this skill. Do not chain into unrelated tasks. If invoked without an explicit networking-review request, stop and ask the user to confirm intent.
4. **Establish the workload first.** Ask (or infer and state) whether the code is a **server**, a **client**, or **both**, and which **protocols** are involved (raw TCP, HTTP/1.1, HTTP/2, gRPC, WebSocket, QUIC/HTTP/3, UDP). The playbook branches hard on this — server-timeout advice is irrelevant to a pure client, transport-selection advice is irrelevant to code that's stuck with one protocol.
5. **Measure before tuning.** "Optimization without a baseline is just guesswork." If a claim is speculative, say so and recommend a load test / profile / `GODEBUG` run rather than asserting a number. Localhost-benchmark numbers are not production numbers — RTT changes everything; flag the difference.

## The playbook — 19 patterns

### A. Measure first

1. **Load-test baseline + profile under load.** Before any change, establish a baseline with a load generator and capture profiles while it runs.
    - Generators: `vegeta` (constant rate, precise latency percentiles, good for CI — `vegeta attack -rate=100 -duration=30s -targets=targets.txt | vegeta report`), `wrk` (raw throughput / upper-bound capacity — `wrk -t4 -c100 -d30s http://host/ep`), `k6` (scripted realistic user flows, ramp-up/down, thresholds), `ghz` (gRPC — `ghz --insecure --proto svc.proto --call pkg.Svc/Method -d '{}' -c 50 -n 100000 host:50051`).
    - Watch: latency **p50/p95/p99(/p999)**, throughput (RPS), success ratio, and **GC pressure** (allocs/op, GC frequency). Reference vegeta run: 100 RPS sustained, p95 ≈ 794 µs, 100 % success — note how p99 degrades first under load.
    - Profile _while loaded_, on a dedicated port: `import _ "net/http/pprof"`, then `go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30` (CPU), `…/heap` (retained — the leak hunt), `…/allocs` (allocation hot spots), `…/goroutine` (goroutine leaks on long-lived conns). In one reference profile, ~58 % CPU sat in `http.(*conn).serve` and ~20 % in GC (`runtime.gcDrain`, `runtime.scanobject`) — a synthetic `/gc` handler retained 649 MB of heap, 99.46 % traceable to that one function. Heap profiles show _what is still held_, not just what was allocated — that's the difference between "we allocate a lot" and "we leak."

### B. The `net` / `net/http` package — everyday correctness

2. **Drain and close every response body.** Go's HTTP client will **not** reuse a connection unless the body is fully consumed. After every request: `io.Copy(io.Discard, resp.Body)` then `resp.Body.Close()` (in that order). A `defer resp.Body.Close()` with no read is the single most common cause of connection-pool starvation, ephemeral-port exhaustion, and mysterious latency under load.

3. **Tune the HTTP client & `http.Transport`; never ship a timeout-less client.** Go's defaults favor convenience over throughput. Use a configured transport — `MaxIdleConns`, `MaxIdleConnsPerHost`, `MaxConnsPerHost` (caps in-flight conns to a downstream so a traffic spike can't exhaust it), `IdleConnTimeout`, `ExpectContinueTimeout` — and **always set `http.Client.Timeout`** (a tight 2 s beats a lazy 30 s — short timeouts shed goroutine pressure under load). Keep a **separate `http.Client` per upstream service** so a slow dependency can't poison the pool of a fast one. Avoid `http.Get` / `http.DefaultClient` in production paths (no timeout, shared pool). For dynamic environments (K8s service IPs that rotate), a custom `DialContext` that forces fresh DNS per dial may be warranted.

4. **Buffer raw connection I/O.** Wrap `net.Conn` in `bufio.NewReaderSize` / `bufio.NewWriterSize` (4–8 KB is a sane default for HTTP-shaped traffic) and **flush before close**. In the reference 10k-connection benchmark (c5.2xlarge, 8 vCPU), switching from unbuffered to buffered writes lifted aggregate downstream throughput from ~29.4 Mbps to ~232 Mbps — **>10×** — purely from balanced I/O. Direct `conn.Write` / `conn.Read` in a loop is the smell.

5. **Pool per-connection buffers — but copy bytes out before they escape.** Recycle `bufio.Reader`/`bufio.Writer` and scratch `[]byte` via `sync.Pool` so thousands of concurrent connections don't each churn the heap (fewer allocs → GC runs less often). **Critical caveat:** a slice handed to async code (stored in a channel, map, cache, struct) retains its _entire backing array_ — a 4 KB read buffer stays alive even if you only needed 5 bytes. Before queueing/caching: `b = append([]byte(nil), b[:n]...)`. Skip pooling when reuse is rare or teardown is complex.

### C. Server lifecycle & concurrency

6. **Deadlines on every connection; every `http.Server` timeout field set.** Raw conns: `conn.SetReadDeadline` / `SetWriteDeadline` before each blocking op (and reset them on activity for long-lived conns). `http.Server`: set `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` — a zero-valued `http.Server{}` is a slow-loris and goroutine-leak waiting to happen. For work a handler kicks off downstream, bound it with `context.WithTimeout` and pass `ctx` first. Missing deadlines = goroutines and connections that never come back.

7. **Bound the accept loop / handler concurrency.** `for { c, _ := ln.Accept(); go handle(c) }` with no bound is fine at 100 connections and a memory + scheduler problem at 100k. Gate with a semaphore (buffered channel) or a fixed worker pool reading accepted conns off a channel — cap concurrent in-flight handlers, and shed (close immediately, or 503) when the gate is full rather than queueing without limit.

### D. Scaling to 10k+ connections

8. **OS & kernel limits — present as a checklist, not a magic number.** "The application will hit OS-level ceilings long before Go becomes the bottleneck." Things to verify for a high-connection server (Linux): `ulimit -n` raised (e.g. `200000`); `net.core.somaxconn` (pending-accept queue, default 128–4096 — often too low for bursts); `net.ipv4.ip_local_port_range` (ephemeral ports for _outbound_ fan-out); `net.ipv4.tcp_tw_reuse=1` and a shorter `net.ipv4.tcp_fin_timeout` to reclaim `TIME_WAIT`/`FIN_WAIT2` faster. In containers, the FD ceiling may be fixed (AWS Fargate caps `nofile` at 65 535) — that bounds single-process concurrency regardless of Go. Surface these as environment-dependent recommendations to validate under real load; do not assert one value fits.

9. **GC tuning for connection churn.** Lots of short-lived per-connection allocation → frequent GC. Lowering `GOGC` (default 100; `GOGC=50` or `debug.SetGCPercent(50)`) trades more-but-shorter collections for lower peak heap; `GOMEMLIMIT` (Go 1.19+) is a soft ceiling worth setting in containers so the GC reacts before the OOM killer does. **Profile first** — confirm GC is actually the cost (it was ~20 % CPU in the reference profile) before turning knobs.

### E. Low-level tuning

10. **Socket options at listener/dialer creation, via `Control`.** Use `net.ListenConfig{Control: ...}` / `net.Dialer{Control: ...}` with `syscall.RawConn` to set options _before bind_ (right place, proper error handling): `TCP_NODELAY` (disable Nagle — essential for small-message latency-sensitive traffic like gaming/trading); `SO_REUSEPORT` (multiple sockets on one port → kernel spreads connections across independent accept queues → better multicore scaling, no single-queue contention); `SO_RCVBUF` / `SO_SNDBUF` (size by the bandwidth-delay product — RTT × link bandwidth — not a copied-from-a-blog constant; Linux defaults ~128–256 KB); TCP keepalive params tuned operationally (≈30–60 s idle, 10–15 s interval, 3–5 probes — not the 2-hour default) to detect dead peers; `somaxconn`/backlog for burst tolerance. "Tuning sockets is always a trade-off — without monitoring you're guessing."

11. **GOMAXPROCS / scheduler / netpoller tuning by measurement only.** `GOMAXPROCS` caps OS threads running Go code; raising it past the optimal point _degrades_ throughput (context-switch + cache contention). Diagnose before changing: `GODEBUG=schedtrace=1000,scheddetail=1` (idleprocs, spinningthreads, runqueue depths, P–M–G balance — when an M blocks on a syscall it detaches its P), `GODEBUG=netpoll=1` (poller activity), `GODEBUG=netpollWaitLatency=N` (poller handoff interval — one cited case: raising GOMAXPROCS with `netpollWaitLatency=100` cut p99 read latency >15 %). Avoid `runtime.LockOSThread()` / `taskset` / CPU affinity unless you have a real-time, dedicated-CPU, or NUMA workload **and** benchmarks proving it helps — otherwise it starves other work and adds subtle regressions. (As of Go 1.25, `GOMAXPROCS` is cgroup-CPU-quota-aware by default — still measure rather than hand-set.)

### F. Resilience under failure & overload

12. **Circuit breakers on flaky dependencies.** Wrap calls to a dependency that can fail or slow down in a breaker: **Closed** (normal) → **Open** (fail fast, don't even try) → **Half-Open** (limited trial traffic). Trip on error-rate / latency over a sliding window of time buckets; stay open for a reset timeout; recover via a few probes. Fail-fast beats piling goroutines onto a sick downstream and turning its outage into yours.

13. **Load shedding & backpressure.** "Handling overload is not a one-off feature but an architectural mindset." Passive shed: bounded channels — when the buffer is full, drop the incoming request immediately (deterministic memory bound). Active shed: watch CPU/latency health signals and flip a "shedding" flag to reject _before_ queues overflow. At the HTTP edge: return **`503 Service Unavailable` with `Retry-After`** (clear signal, prevents retry storms), and **degrade** — drop analytics/personalization/other non-critical work under load while keeping the core path alive. Context deadlines (50 ms favors fairness, 200 ms favors throughput) bound how long anything waits in a queue.

14. **Retries done right — never amplify an overload.** Retry **only idempotent** operations; **cap** the attempts; **exponential backoff with jitter** between them; **honor `Retry-After`** when the server sent one; **pair with the circuit breaker** so retries stop when it's open. An uncapped, un-jittered, un-gated retry loop is how a blip becomes an outage.

### G. Long-lived connections

15. **Long-lived connection hygiene (WebSocket / streaming / persistent TCP).** Memory leaks here are easier to prevent than to detect. (a) **Goroutines:** a handler that blocks indefinitely on I/O or spawns unbounded child goroutines leaks them past the connection's useful life — bound child goroutines, and use `context` cancellation to fan out shutdown across the hierarchy. (b) **Buffers:** copy bytes out of the read buffer before storing them anywhere async (see Pattern 5 — the 4 KB-retained-for-5-bytes trap). (c) **Channels:** bound any buffered channel feeding the connection. (d) **Liveness:** ping/pong heartbeats to detect dead peers, and **reset the read deadline on each pong** so a healthy-but-quiet connection isn't killed and a dead one is.

### H. Transport selection

16. **Pick the right transport for the job.** Reuse connections — never dial per request/RPC.
    - **Raw TCP + length-prefix framing** — lowest achievable latency, negligible protocol overhead, but _you_ build framing, flow control, error handling. For trading/real-time where every µs counts and you can afford the engineering.
    - **HTTP/2 (`net/http`)** — stream multiplexing over one connection + HPACK header compression; latency slightly above raw TCP but stable; holds up well under concurrent load. Solid default for public APIs and web services without pulling in an RPC stack. Note: HTTP/2 still rides one TCP connection, so a single lost segment head-of-line-blocks _all_ streams.
    - **gRPC** — HTTP/2 + Protocol Buffers; typed contracts, great DX, inherits HTTP/2 multiplexing; pays extra CPU/allocations for protobuf encode/decode. The sweet spot for internal microservices.
    - **QUIC / HTTP/3 (`quic-go`)** — independent streams over UDP with per-stream flow control → **no TCP-layer head-of-line blocking** (a lost packet stalls only its stream); **0-RTT** on repeat connections (only for _idempotent_ data — early data is replayable); connection survives network changes via connection IDs (caveat: `quic-go` ~v0.52 supports _passive_ NAT rebinding, not active interface switching like Wi-Fi→LTE); faster loss recovery (frame-scoped retransmits). Costs more CPU than TCP+TLS and wants UDP socket buffer tuning (`SO_RCVBUF`/`SO_SNDBUF`). Best for mobile-first, lossy networks, real-time (gaming/VoIP).

### I. DNS

17. **DNS resolver choice & caching.** Go ships two resolvers: **pure-Go** (`GODEBUG=netdns=go` — reads `/etc/resolv.conf`, talks to nameservers directly, keeps the binary fully static/self-contained — best for minimal containers) and **cgo** (`GODEBUG=netdns=cgo` — delegates to libc, handles exotic setups like LDAP/mDNS, but needs dynamic linking to `libc.so.6` + the loader, and adds overhead). Pick deliberately for your deployment. Go has **no built-in DNS cache** — every `Dial`/`LookupHost` is ≥1 network round-trip — so at scale either **pre-resolve hostnames at startup** (`net.LookupIP` once, dial the IP) or wrap a TTL cache (e.g. a small `go-cache` layer) around resolution, accepting the stale-record risk if you cache aggressively. A custom `net.Resolver` / `DialContext` lets you force a nameserver, cache, or bypass DNS entirely. Debug with `GODEBUG=netdns=2` (traces each request end to end); track lookup latency, especially p95/p99.

### J. TLS

18. **Cut TLS handshake cost.** Session resumption is the biggest single win — it removes a round trip _and_ the expensive asymmetric ops. Server: set a **persistent 32-byte `SessionTicketKey`** (don't let it regenerate on restart, and share it across the fleet so a client resuming hits any instance) — `SessionTicketsDisabled: false`. Client: provide a `ClientSessionCache` (`tls.NewLRUClientSessionCache(n)`). Prefer **TLS 1.3** (1-RTT handshake, 0-RTT resumption — idempotent requests only). Prefer **ECDHE** key exchange (forward secrecy + fast) with **AES-GCM** suites (AES-NI hardware acceleration) — e.g. `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`, `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256` — over CBC. Prefer **ECDSA certificates** (smaller signatures, cheaper verify) over RSA on hot paths; cache verification by fingerprint to skip repeat work. Set `MinVersion: tls.VersionTLS12`, `CurvePreferences: {X25519, CurveP256}`, and pin `NextProtos` (e.g. `{"h2", "http/1.1"}`) to avoid ALPN downgrade surprises.

### K. Observability

19. **Connection-lifecycle observability.** You can't tune what you can't see. Instrument the five phases separately: **DNS resolve** (query→answer duration, resolved IPs, transient vs permanent failure — wrap `net.Resolver`), **dial** (establishment failures = resource exhaustion / routing — wrap `net.Dialer`), **TLS handshake** (measure it apart from network latency — slow cert validation / OCSP / CRL is the usual culprit), **application I/O** (read/write timings reveal backpressure / TCP window exhaustion — wrap `net.Conn`), **teardown** (`Close()` duration/errors, lingering `TIME_WAIT`). Hooks: `httptrace.ClientTrace` (outbound HTTP), `http.Server.ConnState` (inbound conn state machine), per-connection `net.Conn` wrapper types (raw). Export **Prometheus histograms/counters** (`connection_dns_duration_seconds`, `connection_dial_errors_total`, handshake duration, …) rather than burying durations in free-text logs; stitch phases with **OpenTelemetry spans** for end-to-end traces; **sample** logs (e.g. zap's sampler) so granular logging doesn't become its own bottleneck — runtime-adjustable levels, aggregate metrics over per-event logs.

## Review methodology

### Step 1 — Establish scope & workload

- Confirm targets are `*.go`. If the user gave a directory, `Glob` for `**/*.go`.
- Skip vendored code (`vendor/`), generated files (`// Code generated`), and `*_test.go` unless the user asks for tests.
- **State the workload assumption up front**: server / client / both; protocol(s) in play; whether it's an edge service, an internal RPC service, a proxy, a streaming/WebSocket service, or a library. If you genuinely can't tell, ask one question before proceeding.
- Identify the hot surfaces first — `Accept` loops, request handlers, the outbound-call layer, marshalling, the read/write inner loop, connection setup/teardown. Networking wins concentrate at a handful of choke points; don't spread scrutiny evenly.
- Build a `TodoWrite` list (one item per playbook section A–K that's relevant to this workload) so progress is visible.

### Step 2 — Pattern scan (greppable tells)

| #   | Pattern                          | Tells                                                                                                                                                                                                                        |
| --- | -------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Measure first                    | no benchmarks, no `net/http/pprof` import, no load-test config in the repo; perf claims with no numbers                                                                                                                      |
| 2   | Drain response body              | `resp.Body` with `defer resp.Body.Close()` but no `io.Copy(io.Discard, resp.Body)`; `json.NewDecoder(resp.Body)` then early `return` on error without draining                                                               |
| 3   | HTTP client/Transport tuning     | `http.Get(`, `http.Post(`, `http.DefaultClient`, `&http.Client{}` with no `Timeout`; no `&http.Transport{}`; one client shared across many hosts                                                                             |
| 4   | Buffered conn I/O                | `conn.Write(` / `conn.Read(` directly in a loop; no `bufio.NewReaderSize`/`NewWriterSize` around a `net.Conn`; missing `Flush()` before `Close()`                                                                            |
| 5   | Conn buffer pooling              | `make([]byte, N)` or `bufio.NewReader(conn)` per accepted connection; a read-buffer slice stored in a map/chan/struct without an `append([]byte(nil), b...)` copy                                                            |
| 6   | Server deadlines                 | `conn.Read`/`Write` with no preceding `SetReadDeadline`/`SetWriteDeadline`; `http.Server{ Handler: ... }` with `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout` all zero; `http.ListenAndServe` directly       |
| 7   | Bound accept/handler concurrency | `go handle(conn)` / `go h(c)` inside `for { ln.Accept() }` with no semaphore channel or worker pool                                                                                                                          |
| 8   | OS / kernel limits               | high-conn server with no `ulimit`/sysctl note; Dockerfile or k8s manifest with no `nofile`/`somaxconn`; lots of short-lived outbound conns (`TIME_WAIT` risk)                                                                |
| 9   | GC tuning                        | very large long-lived caches/maps of conn state; no `GOMEMLIMIT` in a container build; `GOGC` untouched on an alloc-heavy path                                                                                               |
| 10  | Socket options                   | `net.Listen("tcp", ...)` / `net.Dial("tcp", ...)` where latency or multicore accept matters and there's no `net.ListenConfig{Control:}` / `net.Dialer{Control:}`; no `SetNoDelay`/keepalive config                           |
| 11  | GOMAXPROCS / scheduler           | `runtime.GOMAXPROCS(n)` or `runtime.LockOSThread()` with no benchmark or comment justifying it; CPU-bound work inside a connection goroutine                                                                                 |
| 12  | Circuit breaker                  | outbound calls to a flaky/3rd-party dependency with no breaker; retries with no breaker gate                                                                                                                                 |
| 13  | Load shedding / backpressure     | unbounded request queue or channel; no `503`+`Retry-After` path; no health-signal shedding; no feature-degradation switch                                                                                                    |
| 14  | Retries                          | retry loop with no attempt cap, no backoff, or no jitter; retrying non-idempotent requests; ignoring `Retry-After`                                                                                                           |
| 15  | Long-lived conn hygiene          | WebSocket/stream handler spawning unbounded child goroutines; no ping/pong; no `SetReadDeadline` reset on activity; read buffer escaping into a queue/cache                                                                  |
| 16  | Transport choice                 | hand-rolled TCP framing where HTTP/2 or gRPC would do; gRPC pulled in for a single public one-shot endpoint; a new connection/`grpc.Dial` per call                                                                           |
| 17  | DNS                              | `net.LookupHost`/`net.Dial` of the same hostname on every request with no cache or pre-resolve; no deliberate `GODEBUG=netdns` choice for the build target                                                                   |
| 18  | TLS                              | client `tls.Config{}` with no `ClientSessionCache`; server with no persistent `SessionTicketKey` (regenerated each start); `MinVersion` unset; RSA certs on a hot path; CBC cipher suites pinned; `InsecureSkipVerify: true` |
| 19  | Observability                    | no `httptrace`, no `http.Server.ConnState`, no `net.Conn` wrapper; connection durations only in free-text logs; no per-phase metrics; unsampled per-event logging at scale                                                   |

### Step 3 — Verify with the toolchain (when buildable / runnable)

Use `Bash` only when the module builds in the user's sandbox; never try to fix build errors yourself.

```bash
go vet ./...
go build ./...
# escape analysis on the read/write inner loop and dialers:
go build -gcflags='-m=2' ./... 2>&1 | head -100
# if the user can run the server locally, point them at these (don't assume you can):
GODEBUG=gctrace=1 ./server                              # GC pause + heap growth under load
GODEBUG=schedtrace=1000,scheddetail=1 ./server          # P–M–G balance, runqueue depth
GODEBUG=netpoll=1 ./server                              # netpoller activity
GODEBUG=netdns=2 ./server                               # DNS resolver path per lookup
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30   # CPU under load
go tool pprof http://localhost:6060/debug/pprof/heap                 # retained heap (leak hunt)
go tool pprof http://localhost:6060/debug/pprof/allocs               # allocation hot spots
go tool pprof http://localhost:6060/debug/pprof/goroutine            # goroutine leaks (long-lived conns)
# load generators the skill recommends (run them only if the user asks):
vegeta attack -rate=100 -duration=30s -targets=targets.txt | vegeta report
wrk -t4 -c100 -d30s http://localhost:8080/endpoint
ghz --insecure --proto svc.proto --call pkg.Svc/Method -d '{}' -c 50 -n 100000 localhost:50051
```

If the project doesn't build cleanly, skip these and rely on static reading.

### Step 4 — Produce the report

Always output a single Markdown report in this shape:

```markdown
# golang-network review — <file or path>

## Summary

- Workload assumed: <server | client | both>; protocols: <...>
- Files reviewed: N
- Findings: H high / M medium / L low
- Top opportunity: <one-line>

## Findings

### [HIGH] <Pattern name> — `path/to/file.go:LN`

**Smell:**
\`\`\`go
// 3-10 lines of the existing code
\`\`\`
**Why it matters:** 1-2 sentences citing the goperf.dev rationale (and a benchmark number when one applies — e.g. ">10× throughput from buffered writes", "−15% p99 from netpollWaitLatency tuning").
**Suggested change:**
\`\`\`go
// the proposed code
\`\`\`
**Expected impact:** e.g., "enables connection reuse — kills the ephemeral-port exhaustion under load", "bounds in-flight handlers — flat memory instead of OOM at 30k conns".
**Caveats:** trade-offs / risks / what to measure to confirm.

### [MED] ...

### [LOW] ...

## OS / environment checklist (if applicable)

- [ ] `ulimit -n` raised for the deployment (current: ?, suggested: ?)
- [ ] `net.core.somaxconn`, `ip_local_port_range`, `tcp_tw_reuse`, `tcp_fin_timeout` reviewed
- [ ] container FD cap known (e.g. Fargate 65 535) and capacity-planned around it
- [ ] `GOMEMLIMIT` set for the container
- [ ] `GODEBUG=netdns=go|cgo` chosen deliberately for the build target
      (only include the lines that are relevant; mark each as recommendation-to-validate, not assertion)

## Patterns considered & cleared

Brief list of playbook patterns checked and found correct or not applicable to this workload, so the user knows the scan was complete.

## Suggested validation

- Load test: <generator + command tailored to this service>
- Profiles to capture: <which pprof endpoints, what to look for>
- GODEBUG runs: <which ones, what signal>
- Metrics to add: <which per-phase histograms/counters>
```

### Severity rubric

- **HIGH** — leaks (goroutine / connection / memory), missing timeouts or deadlines on a production path, undrained response bodies causing connection-pool/port exhaustion, unbounded accept loop or request queue, no overload protection on a public surface, `InsecureSkipVerify` in prod. Things that fall over under load or in an incident.
- **MED** — sub-optimal transport/buffer configuration, missing socket options where latency genuinely matters, no DNS cache/pre-resolve on a hot client path, no circuit breaker on a known-flaky dependency, no session resumption on a TLS-heavy service.
- **LOW** — observability gaps, micro-tuning (GOMAXPROCS, cipher order, buffer sizes), speculative wins, stylistic.

## Operating rules

- **Measure-first mindset.** When a claim is speculative, say so and recommend a load test / `pprof` / `GODEBUG` run rather than asserting. Never present a localhost number as a production number.
- **Anchor every recommendation in one of the 19 patterns.** No folklore.
- **Branch on the workload.** Don't give server-timeout advice to a pure client, or transport-selection advice to code locked into one protocol. State your assumption; ask if unsure.
- **OS/sysctl/ulimit advice is a checklist of things to validate**, never a single asserted value — it depends on RTT, link bandwidth, message sizes, and the deployment platform.
- **Be specific.** Always cite `file:line`. Never say "this codebase" — point at the exact spot.
- **Quote benchmarks from the playbook** when they apply (">10× from buffered writes", "~58% CPU in `http.(*conn).serve`", "−15% p99 from `netpollWaitLatency`"); they're persuasive and grounded.
- **No emoji** unless the user asks.
- **No file edits, ever.** This skill is read-only by design.
- If the user invoked this skill without specifying targets, ask: "Which file(s) or directory should I review — and is this a server, a client, or both?"

## Refusal template for non-Go input

> This skill only reviews Go (`*.go`) source. The input you provided looks like `<lang/extension>`. Please point me at Go files — a path, a directory, or a `**/*.go` glob — and tell me whether it's a server, a client, or both, and I'll run the networking review. (I can also look at a `Dockerfile` / k8s manifest / sysctl config _alongside_ Go code when checking OS-limit findings — but the review subject has to be Go.)

# references

> [!NOTE]
> Human Read Only

- <https://goperf.dev/02-networking/>
