# Changelog

All notable changes to KVolt will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v2.5.0] - 2026-11-17

Production-readiness release: a full security and reliability review plus load testing with the race detector (about 17 million requests, 2 hours). Fixes about 25 bugs, including routes returning 404 for valid paths, an open redirect, a data race in `Timeout`, and lost jobs in the Redis queue. Adds `SetLogFunc`, `BindError`, and a CI coverage gate. Contains behaviour changes listed under "Changed (breaking)".

### Changed (breaking)

- **`pkg/validator`**: `required` now rejects the zero value of numbers, booleans, structs (`time.Time`) and arrays, not only empty strings, slices and maps. An omitted JSON field decodes to the zero value and used to pass. A non-nil pointer counts as present, so use `*int` / `*bool` where `0` or `false` is a valid value that must still be sent.
- **Router**: An empty path segment no longer matches a parameter. `/users//posts` and `/items/` no longer reach `/users/:id/posts` and `/items/:id` with `id == ""`; they answer 404.
- **`HEAD` requests**: A `GET` route now answers `HEAD` (same handler, body discarded by `net/http`) unless a `HEAD` route is registered for the path. These previously answered 405, which broke load-balancer health checks that use `HEAD`. The `Allow` header lists `HEAD` wherever `GET` exists.
- **`pkg/swagger`**: The docs page loads a pinned Scalar release (`@scalar/api-reference@1.78.1`) with a Subresource Integrity hash instead of the latest version from the CDN, so a change on the CDN cannot alter the page. `Config.ScriptURL` / `Config.ScriptIntegrity` override it.
- **`pkg/test`**: `Request.Do` fails the test when the `Tester` has no engine. It used to return `nil`, which turned every following `Expect*` into a no-op and let the test pass without sending a request.

### Security

- **`context.Context.Redirect`**: Rejects every ASCII control character in the location. Browsers strip tab / CR / LF while parsing a URL, so `/<tab>/evil.example` was followed as the protocol-relative `//evil.example` (open redirect). The location is now trimmed before validation and the trimmed value is what is sent. `RedirectURL` rejects control characters too.

### Added

- **`context.BindError`**: Marks an error caused by request content. `Bind`, `BindJSON`, `BindQuery`, `BindForm`, `BindURI` and `BindHeader` return it for bad input. `Error()` returns the original text and the original error stays reachable through `errors.As` / `errors.Is`; a direct type assertion on the returned error (`err.(*json.SyntaxError)`) no longer matches and should use `errors.As`.
- **`context.Context.Fork` / `Join`**: For middleware that runs the rest of the chain on another goroutine and may stop waiting for it. `Fork` returns an independent, never-pooled copy of the Context; `Join` copies its outcome back once it has finished.
- **`kvolt.SetLogFunc`**: Sends the framework's own status messages (server started / shutting down, gRPC server and interceptors, queue workers, API docs notice) to a function of your choice instead of standard output, so they can go through `slog`, `zap` or any structured logger. Default output is unchanged.
- **`middleware.LoggerConfig.Output`**: Writes the access log to any `io.Writer` instead of standard output.
- **`redislimit.Config.FailOpen`**: Lets requests through when Redis cannot be reached. The default still answers 500.
- **`swagger.Config.ScriptURL` / `ScriptIntegrity`**: Override the Scalar script loaded by the docs page, to use another version or a self-hosted copy.
- **CI coverage gate**: The Linux test job fails when library coverage (examples, benchmarks and the CLI excluded) drops below 90%. It is 93.3% at the time of writing.
- **`pkg/test`, `pkg/testkit`**: Assertion failures are reported at the calling test's line (`t.Helper`), and `testkit` fails the test when a request body cannot be encoded.
- **`.gitattributes`**: Go sources are checked out with LF on every platform, so `gofmt` and `golangci-lint` no longer flag every file on Windows checkouts with `core.autocrlf=true`. Existing Windows checkouts pick this up on a fresh clone or `git add --renormalize .`.
- **`middleware.IdempotencyConfig.MaxResponseBytes`**: Caps the response body kept for replay (default 1MB, negative for no limit). Larger responses are sent normally but not stored.

### Fixed

Found by running a real server under concurrent load with the race detector (about 350,000 requests per run):

- **`middleware.Timeout`**: A handler that ignored the deadline kept sharing the request `Context` with the middleware unwinding on the request goroutine. That was a data race on the Context, and with any writer-wrapping middleware outside `Timeout` (`Gzip`, `Idempotency`) the late handler wrote to the real `ResponseWriter` after the request had been answered. The rest of the chain now runs on a private copy of the Context (`Context.Fork`) with a writer that discards late output; the outcome is copied back (`Context.Join`) only when the handler finishes in time. The request Context is no longer marked detached on timeout, since nothing else holds it.
- **`middleware.Timeout`**: Headers set by a handler that wrote no body (`c.Status(204)`) are no longer lost, and upstream middleware sees the request's own context after `Timeout` returns rather than the cancelled timeout context.
- **`middleware.Gzip`**: A panic was answered `200` instead of `500` whenever `Gzip` was in the chain: it committed the headers while the panic unwound, before `Recovery` could respond. `Gzip` no longer commits anything when the handler wrote nothing.
- **`middleware.Gzip`**: A streamed response (SSE) was corrupted when a chunk larger than the gzip threshold followed an earlier flush: the stream switched to gzip after the headers had gone out uncompressed. A flush before the threshold now keeps the whole response uncompressed. A pending `c.Status()` is also no longer written twice.
- **`middleware.Idempotency`**: Under concurrent requests with the same key the handler could run twice. A request that missed the store, then took the gate just after the first request had stored its result and released it, ran the handler again. The gate holder now checks the store once more before running.

Found by review:

- **Router**: A static route no longer hides a param or catch-all route that shares a prefix with the request. With `/user/new` and `/user/:id` registered, `/user/next`, `/user/n` and `/user/new/posts` returned 404; so did `/a/b/c` with `/a/b/d` and `/a/:x/c`. Lookup now falls back to the wildcard when the static branch does not lead to a handler. Static routes still win when both match.
- **`context.Bind` and the other `Bind*` methods**: A handler that returns the bind error now answers `400 invalid_request` for malformed JSON, an empty body, a wrong value type or an unparseable parameter, and `422 validation_failed` (with `field` / `rule` details) for a failed `validate` rule. These previously answered `500` and were logged as handler errors. An oversized body is still `413`.
- **`pkg/session`**: `MaxAge` is now enforced across restarts and across instances sharing a store. The creation time was kept only in process memory, so after a restart (or on another instance) a session lived for as long as it stayed active. It is now stored beside the session under `<token>:created`, with a TTL of `MaxAge` that activity does not extend. Sessions that have no marker — created before this change with `MaxAge` set, or while `MaxAge` was 0 — are rejected once `MaxAge` is on, so those users sign in again once after upgrading.
- **`middleware/redislimit`**: The default key groups IPv6 clients by /64, matching `middleware.Limiter`. Limiting per IPv6 address let a client get a fresh bucket for every address it rotated through.
- **`kvolt new` / `kvolt docker`**: The scaffolded `main.go` exits 0 after a graceful shutdown; `log.Fatal(app.Run(addr))` exited 1 on every SIGTERM because `Run` returns `nil`. The scaffold now also writes a `.gitignore` for `.env` and serves API docs only when `DEBUG=true`. The generated Dockerfile runs as a non-root user, copies `go.sum` before downloading modules, and uses `alpine:3.22`.
- **`pkg/cache/redis`**: `Touch` with no TTL on a key that already has no expiry returned `ErrKeyNotFound`, so a session created with a zero TTL could never be read back from Redis.
- **`middleware/prometheus`**: The `method` label is limited to the standard HTTP methods; anything else is recorded as `OTHER`. A client could previously create an unlimited number of time series by sending made-up methods.
- **`middleware.Idempotency`**: A replay no longer duplicates headers set by middleware that runs before it (`CORS`, `RequestID`, `Secure`). Only headers the handler added or changed are stored, and they replace rather than append on replay. Previously a replayed response carried two `Access-Control-Allow-Origin` values, which browsers reject.
- **`middleware.Idempotency`**: `408`, `425` and `429` responses are no longer cached, so a retry with the same key runs the handler again.
- **`pkg/queue/redis`**: Promoting delayed jobs, reclaiming expired jobs and reclaiming orphans now run as atomic Lua scripts, and acknowledgement runs in one transaction. A crash between the individual commands could previously lose a retrying job or redeliver a finished one. Backlogs are moved in batches of 100.
- **`middleware.Logger`**: The `json` format now emits valid JSON for any request path. Control bytes and invalid UTF-8 were written with Go escapes (`\x01`) that JSON parsers reject.
- **`middleware.Timeout`**: A handler panic after the deadline is now logged. It cannot reach `Recovery` once the 504 has been sent and was previously dropped silently.
- **`middleware.CORS`**: `Vary: Origin` is set on every response when an allow-list is configured, including responses to refused origins, so shared caches do not serve one origin's response to another.
- **`middleware.Gzip`**: Responses that already carry a `Content-Encoding` are no longer compressed a second time. `Accept-Encoding` parsing now reads `q` values numerically (`gzip;q=0.05` is accepted), matches the `gzip` coding exactly, and lets an explicit `gzip;q=0` override `*`.

### Docs

- **README**: Replaced the "100% test coverage gates" claim with what CI actually runs, including the new 90% gate.
- **README and examples**: The quick start and the `full_app` / `tenant_app` examples no longer wrap `app.Run` in `log.Fatal`, which exited 1 after a graceful shutdown.
- **SECURITY.md**: Supported versions updated to 2.4.x / 2.3.x; added hardening notes for redirects and session `MaxAge`.

## [v2.4.0] - 2026-11-15

Enterprise hardening and reliability release: adds typed header binding, full HTTP verb testkit suites, RFC-compliant content negotiation, atomic queue retries, and defensive resilience across all 30 packages.

### Added

- **`context.Context.BindHeader`**: Added typed request header binding into structs (`header` tags) with case-insensitive and canonical header key fallback.
- **`pkg/testkit`**: Added `Put()`, `Delete()`, and `Patch()` methods on `TestServer`, and added `(*Response).Unmarshal(v interface{}) error` for direct response JSON decoding.
- **`pkg/test`**: Added `(*Response).Unmarshal(v interface{}) error` to decode response JSON into target structs directly.
- **`pkg/validator`**: Added support for multi-byte UTF-8 character length checks via `utf8.RuneCountInString` on string `min=` / `max=` rules; added automatic whitespace trimming on multi-rule struct tags (e.g. `validate:"required, min=5"`).

### Fixed

- **`middleware.Compress`**: Standardized Gzip content negotiation via `acceptsGzip` parsing quality parameters; clients sending `Accept-Encoding: gzip;q=0` or `q=0.0` explicitly refuse gzip without receiving compressed bytes; added support for wildcard `*` and fractional quality values.
- **`pkg/queue/redis`**: In `processAndAck`, reordered failed job retries to ensure `enqueueDelayed` executes *before* acknowledging the previous processing slot, guaranteeing at-least-once message durability without lost tasks on worker process crashes.
- **`pkg/cache`**: In `GetInto`, raw `[]byte` and `string` payloads are unmarshaled directly into destinations without double-marshaling through `sonic.Marshal`, preventing base64 string corruption.
- **`pkg/config`**: In `bindViperTags`, allocated initially-nil embedded pointer structs via `fieldVal.Set(reflect.New(...))` so Viper tags, environment variables, and defaults are properly discovered and mapped.
- **`pkg/swagger`**: Normalized trailing slashes in documentation endpoints (`/swagger/doc.json/` -> `doc.json`), and trimmed spaces around struct tag options (`json:", omitempty"`).
- **`context.ClientIP` & `middleware.Secure`**: Normalized IPv6 bracketed hosts (`[::1]`) when inspecting trusted proxies and peer addresses without ports.
- **`Engine.ServeHTTP`**: Trailing slash redirects (`RedirectTrailingSlash`) now issue `301 Moved Permanently` on safe `HEAD` requests matching `GET` behavior, rather than `308 Permanent Redirect`.
- **`pkg/di`**: Added `!val.Elem().CanSet()` reflection guard in `Invoke` to safely reject non-settable pointer targets without panicking.
- **`pkg/scheduler`**: Added `strings.TrimSpace(spec)` on cron schedules to prevent syntax errors caused by leading or trailing whitespace.

## [v2.3.1] - 2026-11-14

Hardening patch release for production enterprise deployment: resolves concurrency traps, background worker resilience, dual server error handling, and nil-pointer edge cases.

### Fixed

- **`middleware.Compress`**: Fixed data loss in `flushPlain()` where trailing buffered data below `MinSize` was dropped when headers were already committed; fixed double-close on gzip streams.
- **`grpc.RunDual`**: Captured startup errors on both HTTP and gRPC listener channels and returned them to caller/supervisor instead of silently dropping the error on exit.
- **`pkg/queue`**: Memory and Redis queue workers now safely recover from handler panics without terminating worker goroutines or leaking unacknowledged jobs.
- **`pkg/swagger`**: OpenAPI schema generator now correctly isolates cycle detection to active ancestor stacks, preserving sibling struct reuse across request/response models.
- **`middleware.Prometheus` & `middleware.Metrics`**: Deferred gauge and in-flight decrements so downstream panics do not permanently leak in-flight request counts.
- **`context.Context`**: Added nil-pointer protection across `SaveUploadedFile`, `FormFile`, `RawBody`, `File`, `Attachment`, `Redirect`, `Upgrade`, `JSON`, and `SSE`.
- **`context.BindQuery` / `BindForm`**: Added multi-value repeated parameter and comma-separated slice binding for integers, floats, booleans, and unsigned integers.
- **`pkg/di`**: Added interface resolution in `Invoke` and `ProvideFunc` (`sType.Implements(elemType)`).
- **`pkg/config`**: Guarded against reflection panics on non-struct pointer targets.
- **`pkg/scheduler`**: Added panic recovery in cron job execution and added `Remove(id int)` for dynamic job cancellation.
- **`pkg/cache`**: Added `Close()` with stop channel to gracefully terminate memory cache background janitor goroutine.
- **`middleware.Health`**: Wrapped readiness checker callbacks with panic recovery to prevent probe failures from taking down the server.

## [v2.3.0] - 2026-11-13

Production-grade release: 100% test coverage, comprehensive race condition fixes, automated `gosec` static analysis in CI, SemVer 2.0.0 stability guarantee, and security hardening across all core modules.

### Security

- **Static Analysis**: Integrated `gosec` into CI workflow with zero security issues.
- **Router**: prefer static siblings over `:param` / `*catchAll` (e.g. `/user/new` wins over `/user/:id`).
- **`Tenant`**: when `Claim` is set, ignore header as tenant source; reject header≠claim with 403.
- **`CSRF`**: HMAC-signed tokens (process secret or `CSRFConfig.Secret`) so planted cookies without the secret fail.
- **`JWT` / `JWTOptional`**: optional `KeyFunc` / `SigningMethod` (e.g. RS256), `Issuer` / `Audience`, `RequireExp` (default true), case-insensitive `Bearer`, `SkipOnInvalid` on optional.
- **`session.Manager`**: sliding TTL via `Touch` (no Set-resurrect race); optional `MaxAge` absolute cap.
- **Toolchain**: Updated Go toolchain to 1.27.4 to fix GO-2026-6859.
- **Repo**: `SECURITY.md`, Dependabot (gomod + Actions); ignore `/bin/`.

### Fixed

- **Data Races**: Fixed all data races in coverage tests across server, middleware, gRPC, and CLI watcher loops.
- **`Engine.Run` / `RunTLS`**: bind (`Listen`) first and return the error; print “running” only after the listen succeeds.
- **`middleware.Idempotency`**: no 5xx cache; strip `Set-Cookie`; actor + body hash in default key; reject oversized keys; bound memory store; single-flight; preserve `Flusher`/`Hijacker`; Redis body size cap.
- **`Static`**: block directory listings (no index) and dotfile path segments (e.g. `.env`).
- **`BindQuery` / `BindForm` / `BindURI`**: reject integer overflow; surface invalid JSON for struct/slice/map fields.
- **`Logger`**: sanitize/quote paths; sync fallback when buffer full; lazy worker (no silent drop).
- **`Limiter`**: no background goroutine; O(1) eviction when full.
- **`Audit`**: `defer` (panic-safe), sets `Err` on panic, async `OnAudit` (use `Sync` in tests).
- **`Recovery`**: re-raise `http.ErrAbortHandler`; larger stack dump (64KiB).
- **`CORS`**: only short-circuit preflight `OPTIONS` (with `Access-Control-Request-Method`); flush 204 headers; do not shadow app OPTIONS routes.
- **`ClientIP`**: do not treat a TrustedProxies address from `X-Real-IP` as the client.
- **`pkg/auth.ParseToken`**: require `exp` (aligned with JWT middleware).
- **`Gzip`**: expose `Flusher` / `Hijacker` / `Unwrap`.
- **`queue/redis`**: reclaim processing orphans missing an inflight deadline (crash between BRPOPLPUSH and ZADD).
- **`Timeout`**: bound response wait (504 + drop late writes); still cancel via request context.
- **Router**: panic on conflicting `:param` / catch-all names at the same level.
- **`session.Manager`**: prune expired MaxAge entries to bound memory.
- **BasicAuth** / **gRPC Auth**: case-insensitive `Basic` / `Bearer` scheme.

## [v2.2.0] - 2026-11-07

Pre-production platform release: multi-tenant helpers, idempotency, audit, optional OpenTelemetry, and dependency security bumps. Tested (fmt/vet/govulncheck/race/lint) before tag.

### Added

- **`middleware.Tenant` / `TenantID`**: multi-tenant id from header, route param, JWT claim, or custom extractor.
- **`middleware.RequirePermission` / `RequireAnyPermission` / `RequireAllPermissions`**: generic RBAC gates via `PermissionChecker`.
- **`middleware.Idempotency`**: `Idempotency-Key` replay for POST/PUT/PATCH (`MemoryIdempotencyStore` + pluggable store).
- **`middleware/redisidem`**: Redis-backed `IdempotencyStore` for multi-instance deployments.
- **`middleware.Audit`**: post-request audit hook (`AuditEvent` with tenant, actor, status, duration).
- **`middleware/otel`**: optional OpenTelemetry HTTP tracing middleware (explicit import).
- **`example/tenant_app`**: Tenant + JWTOptional + RBAC + Idempotency + Audit demo.
- **CLI `kvolt new`**: scaffold includes `Default()`, Logger, Live/Ready, OpenAPI/Swagger, optional tenant stub (`TENANT_REQUIRED` / `X-Tenant-ID`).
- **CI**: `golangci-lint` job (`.golangci.yml`).

### Security

- Bump **`google.golang.org/grpc`** to **v1.83.2**, **`golang.org/x/net`** to **v0.62.0**, **`golang.org/x/crypto`** past openpgp advisory (govulncheck clean).
- **`Limiter` `MaxKeys`**: when full, **evict oldest** bucket instead of 429'ing brand-new keys (availability DoS under high-cardinality `KeyFunc`).

### Fixed

- **`example/full_app`**: no hardcoded JWT secret; limiter uses `ClientIP`; CSRF not mis-skipped on API routes.
- **`redislimit`**: unit tests (allow/block + fail-closed).
- Docs: remove sunset Go Report Card badge; maturity wording pre-production.

## [v2.1.1] - 2026-11-04

Patch on top of v2.1.0 (Go module tags are immutable once proxied — do not retag v2.1.0).

### Fixed

- **CLI**: default `Version` string is **2.1.1** (v2.1.0 tag still reported `2.0.1` when installed without `-ldflags`).
- **gofmt**: align `Engine` struct fields so CI `go fmt ./...` passes on Ubuntu.

### Added

- **Live HTTP harness** (`benchmarks/live`): bombardier `/ping` vs Gin/Echo/Chi/stdlib/Fiber; sample medians + laptop/CPU in [`docs/benchmarks.md`](docs/benchmarks.md).

## [v2.1.0] - 2026-11-04

Feature and security release: optional Redis/Prometheus/gRPC splits, CSRF/SSE, Go 1.27, and reproducible benchmarks.

### Changed

- **Go toolchain**: module requires **Go 1.27** (`toolchain go1.27.3`) for runtime/compiler improvements (e.g. size-specialized malloc) and current security fixes.
- **sonic** bumped to **v1.15.5** (Go 1.27-compatible; avoids encoding/json fallback).
- **Benchmarks**: reproducible microbenches (`bench_test.go`, `router`/`context` benches, `scripts/bench.ps1`); README marketing Req/Sec removed — see [`docs/benchmarks.md`](docs/benchmarks.md).

### Security

- **`Context.Redirect`**: same-origin path only (`/...`); blocks open redirects / `@` / schemes; use **`RedirectURL`** for trusted absolutes.
- **`Attachment`**: sanitizes download filenames (strips CR/LF/quotes/path segments).
- **`FileFrom` / `AttachmentFrom`**: serve under a root without path traversal (incl. symlink escape via `EvalSymlinks`); prefer over raw `File` with user input.
- **`Ready`**: readiness failures return generic `not_ready` (checker errors logged only).
- **gRPC `AuthInterceptor`**: returns generic `unauthenticated` (validator details logged only).
- **SSE**: strips CR/LF from `event`/`id` fields (blocks frame injection).
- **`SaveUploadedFile`**: rejects non-local / escaping destinations (`filepath.IsLocal`); prefer **`SafeUploadPath`**.
- **`RequestID`**: sanitizes incoming `X-Request-ID` (no header injection).
- **`Secure` HSTS**: only for TLS or trusted-proxy `X-Forwarded-Proto: https`.
- **`CSRF`**: auto `Secure` cookie on HTTPS; empty `SkipPaths` entries ignored; `InsecureCookie` / `ManualSecure` overrides.
- **`BasicAuth`**: constant-time hashed compare + dummy work for unknown users; realm sanitized.
- **`APIKey`**: SHA-256 + constant-time OR-compare across all keys (no early exit).
- **`ClientIP`**: right-to-left XFF (skip trusted hops) so spoofed left-most values are ignored when proxies append.
- **`Limiter`**: `MaxKeys` cap (default 10k) to bound memory under high-cardinality keys.
- **Metrics / Prometheus**: `RoutePath` uses registered pattern (`/users/:id`), not raw URL.

### Added

- **`pkg/httperr`**: structured API errors; handlers can `return httperr.…` and get JSON responses.
- **`Context.RawBody()`**: webhook-safe body read; restores body for later `Bind`/`BindJSON`.
- **`Context.FormFileMax` / `FormFileAllowed`**: multipart helpers (size + extension allowlist).
- **`Context.SetCookie` / `SetCookieWith` / `ClearCookie`**: cookie helpers (incl. SameSite).
- **`Context.SSE` / `Event` / `SSEvent` / `Flush`**: Server-Sent Events helpers.
- **`middleware.JWTOptional`**, **`Claims`**, **`ClaimString`**: optional JWT + claim helpers.
- **`middleware.LimiterWithConfig`**: rate limit by custom key (tenant / API key / IP).
- **`middleware.Metrics` / `MetricsStore`**: in-process counters + optional `OnRequest` hook.
- **`middleware/prometheus`**: optional Prometheus metrics (not pulled by core middleware).
- **`middleware/redislimit`**: optional Redis rate limiter (not pulled by core middleware).
- **`middleware.CSRF`**: double-submit cookie CSRF (`X-CSRF-Token`).
- **`pkg/cache/redis`**: optional Redis `cache.Store` (JSON via sonic, key prefix + safe Flush).
- **`pkg/queue/redis`**: optional at-least-once Redis queue (`BRPOPLPUSH` + visibility timeout + reaper).
- **`grpc.Attach` / `Run` / `RunDual`**: gRPC no longer imported by core `kvolt` package.
- **Queue retries**: `PushWithOptions`, `WithDeadLetter`, `WithRetryWait` on `MemoryQueue`.
- **OpenAPI schemas**: route `.Tags` / `.Request` / `.Response` / `.Security`; swagger schema reflection.
- **`di.ProvideFunc` / `MustProvideFunc`**: constructor injection.
- **Example**: [`example/full_app`](example/full_app) demo of the new APIs.
- Docs: [`docs/errors.md`](docs/errors.md), [`docs/sse.md`](docs/sse.md).
- **HTTP methods**: `PATCH` / `HEAD` / `OPTIONS` / `Handle` / `Any`; optional **405** + trailing-slash redirect.
- **CORSWithConfig**: origins list, credentials, Max-Age, PATCH.
- **ClientIP** + `Engine.TrustedProxies`; limiters use ClientIP.
- **`middleware/redislimit`** for multi-instance Redis rate limits.
- **BasicAuth** / **APIKey** middleware; **Live** / **Ready** health helpers.
- **Context**: `Redirect`, `Attachment`, `Abort`, `BindQuery`/`BindForm`/`BindURI`, `SafeUploadPath`, `NoContent`.
- **ServerConfig**: configurable timeouts (WriteTimeout `0` for SSE/WS), MaxHeaderBytes, shutdown timeout, `RunWithServer`.
- **Recovery** JSON + `RecoverHandler`; **httperr** unwrap via `errors.As`; 413/409/422/503 helpers.
- **JWT**: empty SigningKey panics; `pkg/auth` uses golang-jwt; no default insecure secret.
- **Secure** headers: Referrer-Policy, HSTS behind `X-Forwarded-Proto`.
- **CSRF** accepts form `_csrf` / `csrf_token`; **LoggerWithConfig** JSON + skipper.
- **cache.GetInto**; queue durability semantics documented (at-most-once).

---

## [v2.0.1] - 2026-08-27

Local production bar: valid Go v2 module path, regression tests, fuzz, soak harness.

### Added

- Module path **`github.com/go-kvolt/kvolt/v2`** so `go get github.com/go-kvolt/kvolt/v2@v2.0.1` works after this tag. `v2.0.0` is not a valid Go module.
- Production regression tests: concurrent `BindJSON`/`JSON`, logger real status, gzip skip tiny bodies, max-body 413, five-param routes, concurrent ping/invoice.
- Fuzz tests: `BindJSON`, `Query`, router `Find`.
- Soak: `KVOLT_SOAK=1 KVOLT_SOAK_SEC=120 go test -count=1 -timeout 5m -run TestSoak .`
- `scripts/local10.ps1` — gofmt, vet, **`-race`**, 2-minute soak, fuzz, install CLI. Requires gcc (WinLibs MinGW on this machine).

### Fixed

- Logger log emit is injectable so tests can assert the status line.

---

## [v2.0.0] - 2026-08-27

Production release. Safe JSON under load, a production `Default()` engine, and middleware/cache fixes from real API benches.

### Added

- **`kvolt.Default()`**: Engine with Recovery, RequestID, and MaxBodySize (1MB). Logger and Gzip stay opt-in.
- **`BindJSON`**: JSON decode without validation. `Bind` still decodes then validates.
- **`Engine.ListenAndServe`**: stdlib listen (no extra timeouts). `Run()` still uses production timeouts and graceful shutdown.
- **`Context.Query`**, **`Context.StatusCode`**, **`Context.FlushHeaders`**, **`Engine.NoRoute`**.
- **`middleware.RequestID`** (`X-Request-ID`) and **`middleware.Timeout`**.
- **`GzipWithConfig`**: min size (1KB) and skip `application/json` by default; gzip writers are pooled.
- **`cache.NewMemoryStoreSized`**: LRU cap (`DefaultMaxKeys` = 50_000). `Len()`.
- **`SetWebsocketCheckOrigin`**: WebSocket origin check defaults to same-origin.

### Fixed

- **`Status()`** no longer calls `WriteHeader` immediately, so `Status().JSON()` can still set `Content-Type`.
- **Listen banner**: `Run("127.0.0.1:8080")` no longer prints `http://localhost127.0.0.1:8080`.
- **404**: reused handler chain (no per-miss allocation).
- **Router params**: more than 4 path params no longer panics; params buffer is reused from the context pool.
- **Rate limiter**: keys on IP host (not `ip:ephemeral-port`) and does not hold the mutex during `Next()`.
- **Logger**: logs the real status code (was always 200).
- **`String`**: applies `fmt.Sprintf` when extra args are passed.
- **Memory cache**: expired keys are deleted on Get; cache cannot grow without bound.

### Changed

- JSON / text / HTML responses set `charset=utf-8`.
- Gzip no longer compresses JSON or tiny bodies (use `GzipWithConfig` to override).
- JSON encode uses sonic fastest options with a pooled buffer. `BindJSON` streams the body (no pooled request buffer — that aliased strings under load).
- Single-handler routes skip the middleware `Next()` loop.

---

## [v1.1.0] - 2026-05-17

### Added

- **gRPC server** (`grpc` package): `Engine.GRPCServer()`, unified `RunDual()` / `RunDualTLS()` for HTTP + gRPC, built-in logging/recovery/auth interceptors, reflection and health checks. See `docs/grpc.md`.

### Fixed

- **CLI hot reload** (`kvolt run`): Stops the full dev server process tree before restart (build temp binary + process group kill) so the listen port is released instead of leaving an orphaned `go run` child.
- **Router groups**: `Group()` now copies the parent middleware slice so sibling groups (`/api/executive`, `/api/designer`, …) no longer leak each other's `Use()` middleware.
- **Router**: Multiple static suffixes after the same `:param` (e.g. `/orders/:id/assets` and `/orders/:id/take`) register and match correctly regardless of registration order. Consecutive params (e.g. `/files/:orderId/:assetId`) continue to work.

---

## [v1.0.0] - 2025-03-03

Initial stable release.
