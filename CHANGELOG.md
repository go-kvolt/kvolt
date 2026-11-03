# Changelog

All notable changes to KVolt will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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
