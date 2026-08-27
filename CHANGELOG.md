# Changelog

All notable changes to KVolt will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

_No changes yet._

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

First stable release. Production-ready with full CLI, docs, and quality tooling.

### Added

- **Session middleware** (`middleware.Session`) and **pkg/session** for stateful session management (cookie/header/query lookup, TTL, sliding window).
- Session documentation (`docs/session.md`) and verification tests in the test project.
- **Max body size middleware** (`middleware.MaxBodySize`, `middleware.MaxBodySizeBytes`) to limit request body size and prevent large-payload attacks. Uses `http.MaxBytesReader`; handlers can respond with 413 when reads exceed the limit.
- **Production server timeouts** on `Run()` and `RunTLS()`: `ReadHeaderTimeout` (10s), `ReadTimeout` (30s), `WriteTimeout` (30s), `IdleTimeout` (120s). Exported constants: `DefaultReadHeaderTimeout`, `DefaultReadTimeout`, `DefaultWriteTimeout`, `DefaultIdleTimeout`.
- **Handler error handling** in `context.Next()`: when a handler returns an error and no response has been written, the framework now logs the error, sends 500 with JSON `{"error":"Internal Server Error"}`, and stops the chain.
- **`Context.HeaderWritten()`** method so middleware (e.g. Recovery) can check if the response has already been written.
- **RecoveryWithConfig** (`middleware.RecoveryConfig` with `LogStackTrace bool`) to optionally disable stack trace logging in production.
- **CLI**: `kvolt build` (with `-o`, `-e`), `kvolt test` (with `-cover`), `kvolt fmt`, `kvolt key`, `kvolt generate handler <name>`, `kvolt generate middleware <name>`, `kvolt docker` (generate Dockerfile). Global `-h`/`--help` and `-v`/`--version`. `kvolt run -e` for custom entry point; watch excludes `.git`, `vendor`, `node_modules`.
- **Docs**: Developer workflow, Go Report Card locally, testing (`pkg/test` + `pkg/testkit`), CLI reference. **CI**: `go fmt` and `go vet` in workflow. **Makefile**: `make report` for local quality checks.

### Changed

- API docs UI: README and swagger doc now state that the default UI is **Scalar** (OpenAPI spec).
- Reduced cyclomatic complexity in `middleware/jwt.go` (extracted `buildJWTExtractor`), `router/tree.go` (extracted `getValueParam`, `getValueCatchAll`), and `cmd/kvolt/main.go` (extracted `runWatcherLoop`, `shouldRestartOnEvent`) for better tooling scores.
- **Recovery middleware**: fixed typo "Painc" → "Panic"; only writes 500 when headers not yet sent; configurable stack trace logging via `RecoveryWithConfig`.

### Fixed

- **Router**: Param routes such as `GET /auth/:provider` and `GET /auth/:provider/callback` now correctly match URLs like `/auth/twitter` and `/auth/twitter/callback` (404 issue fixed in stable v1).
- **ineffassign** in `pkg/swagger/swagger.go`: removed ineffectual assignment to `openAPIPath`.
- Test project: **TestSwaggerUI** now expects `doc.json` in the response (Scalar UI) instead of `swagger-ui`.
- Handler-returned errors were previously ignored in `context.Next()`; they are now logged and result in a 500 response when no response has been written.

---

## [v0.1.3]

- Last release before the changes below. (No changelog entries before this version.)
