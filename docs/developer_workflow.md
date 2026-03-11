# Developer Workflow: Lint, Test & Run

Quick reference for linting, testing, and running your KVolt app.

---

## Lint (code quality)

Use standard Go tools in your app directory:

| Command | Purpose |
|--------|--------|
| `go fmt ./...` | Format code |
| `go vet ./...` | Static analysis |

**One-liner before commit:**

```bash
go fmt ./... && go vet ./...
```

Optional: copy the [Makefile](../Makefile) into your project and run `make report` for fmt + vet + test + optional linters.

---

## Test (handler & integration)

KVolt provides two test helpers for your app.

### 1. `pkg/test` (fluent API)

Import as `kvtest "github.com/go-kvolt/kvolt/pkg/test"`. Build requests, run them, assert on response.

```go
test := kvtest.New(t, app)
test.GET("/health").Do().
    ExpectStatus(200).
    ExpectBodyContains("ok").
    ExpectJSON(map[string]string{"status": "ok"})

test.POST("/users").WithJSON(user).Do().
    ExpectStatus(201).
    ExpectHeader("Content-Type", "application/json")
```

Methods: `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `WithHeader`, `WithJSON`, `WithBody`, `Do`, `ExpectStatus`, `ExpectBody`, `ExpectBodyContains`, `ExpectHeader`, `ExpectJSON`.

### 2. `pkg/testkit` (simple API)

Import `"github.com/go-kvolt/kvolt/pkg/testkit"`. Good for table-driven tests.

```go
ts := testkit.New(app)
ts.Get(t, "/ping")
ts.Post(t, "/users", body)
// Response: AssertStatus(t, code), AssertBody(t, contains)
```

See **[Testing Guide](testing.md)** for more examples.

---

## Run (local dev)

| Command | Purpose |
|--------|--------|
| `go run ./cmd/your-app` | Run the app |
| `kvolt run` | Run with hot reload (if using CLI) |

See **[CLI Guide](cli.md)** for `kvolt new`, `kvolt run`, and project layout.

---

**Summary:** Lint with `go fmt` + `go vet`, test with `pkg/test` or `pkg/testkit`, run with `go run` or `kvolt run`.
