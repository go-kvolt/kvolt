# Changelog

All notable changes to KVolt will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

_No changes yet._

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

- **ineffassign** in `pkg/swagger/swagger.go`: removed ineffectual assignment to `openAPIPath`.
- Test project: **TestSwaggerUI** now expects `doc.json` in the response (Scalar UI) instead of `swagger-ui`.
- Handler-returned errors were previously ignored in `context.Next()`; they are now logged and result in a 500 response when no response has been written.

---

## [v0.1.3]

- Last release before the changes below. (No changelog entries before this version.)
