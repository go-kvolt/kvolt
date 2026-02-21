# Changelog

All notable changes to KVolt will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **Session middleware** (`middleware.Session`) and **pkg/session** for stateful session management (cookie/header/query lookup, TTL, sliding window).
- Session documentation (`docs/session.md`) and verification tests in the test project.

### Changed

- API docs UI: README and swagger doc now state that the default UI is **Scalar** (OpenAPI spec).
- Reduced cyclomatic complexity in `middleware/jwt.go` (extracted `buildJWTExtractor`), `router/tree.go` (extracted `getValueParam`, `getValueCatchAll`), and `cmd/kvolt/main.go` (extracted `runWatcherLoop`, `shouldRestartOnEvent`) for better tooling scores.

### Fixed

- **ineffassign** in `pkg/swagger/swagger.go`: removed ineffectual assignment to `openAPIPath`.
- Test project: **TestSwaggerUI** now expects `doc.json` in the response (Scalar UI) instead of `swagger-ui`.

---

## [v0.1.3]

- Last release before the changes below. (No changelog entries before this version.)
