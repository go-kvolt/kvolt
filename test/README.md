# KVolt test package

This folder holds **verification tests** that run as part of the framework repo. CI runs them with:

```bash
go test ./...
```

from the kvolt root (so `./test/...` is included).

## Test files in the repo

| Package | File | Tests |
| :--- | :--- | :--- |
| **kvolt** | `kvolt_test.go` | New, ServeHTTP (route + 404), Routes |
| `context` | `context/context_test.go` | New, Set/Get, MustGet, Reset, Param |
| `middleware` | `middleware/middleware_test.go` | Secure, SecureWithConfig, Recovery |
| `pkg/cache` | `pkg/cache/cache_test.go` | Cache get/set/delete/expiry |
| `pkg/session` | `pkg/session/session_test.go` | Manager Create/Get/Destroy, invalid token |
| `pkg/auth` | `pkg/auth/auth_test.go` | GenerateToken, ParseToken, invalid/expired/signature |
| `pkg/config` | `pkg/config/config_test.go` | Load with valid struct |
| `pkg/di` | `pkg/di/di_test.go` | Provide, Invoke, missing service, nil target |
| `pkg/logger` | `pkg/logger/logger_test.go` | New, Default, Info, Level filter, Level.String |
| `pkg/queue` | `pkg/queue/queue_test.go` | MemoryQueue Push, Push full, Register and process |
| `pkg/scheduler` | `pkg/scheduler/scheduler_test.go` | New, Add, Start, Stop, job interval |
| `pkg/swagger` | `pkg/swagger/swagger_test.go` | Handler Disabled, doc.json, index.html |
| `pkg/test` | `pkg/test/tester_test.go` | Tester GET, POST WithJSON Do ExpectStatus/Body |
| `pkg/testkit` | `pkg/testkit/testkit_test.go` | TestServer Get/Post, Response AssertStatus/Body |
| `pkg/validator` | `pkg/validator/validator_test.go` | Validate required, email, min, no tags |
| `router` | `router/router_test.go` | Router AddRoute, Find |
| `test` | `test/verification_test.go` | Static/param/wildcard/group routes, concurrency |

## This folder (`test/`)

| File | Tests |
| :--- | :--- |
| `verification_test.go` | Static routes, param routes, wildcard routes, group routes, concurrency |

These tests build an engine, register routes, and assert HTTP responses so that framework changes don’t break routing or the context API.
