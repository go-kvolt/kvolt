# Testing 🧪

KVolt is designed with testability in mind. You can use **`pkg/test`** (fluent request/response API) or **`pkg/testkit`** (simple Get/Post + assertions) to test handlers and routes without a real server.

## Using `pkg/test` (recommended for most tests)

Import as `kvtest "github.com/go-kvolt/kvolt/pkg/test"`. Build a request, call `Do()`, then chain assertions.

```go
test := kvtest.New(t, app)
test.GET("/health").Do().
    ExpectStatus(200).
    ExpectBodyContains("ok")
test.POST("/echo").WithJSON(map[string]string{"msg": "hi"}).Do().
    ExpectStatus(201).
    ExpectJSON(map[string]string{"echo": "received"})
```

Available: `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `WithHeader`, `WithJSON`, `WithBody`, `Do`, `ExpectStatus`, `ExpectBody`, `ExpectBodyContains`, `ExpectHeader`, `ExpectJSON`.

## Using `testkit`

The `pkg/testkit` package lets you spin up a test instance of your app and make requests without a real network server.

### Example

## Real-World Example: Table-Driven Tests

Table-driven tests are the idiomatic Go way to test multiple scenarios.

```go
func TestUserValidation(t *testing.T) {
    app := kvolt.New()
    app.POST("/users", CreateUserHandler) // Assume this handler binds & validates

    ts := testkit.New(app)

    tests := []struct {
        name       string
        body       map[string]interface{}
        wantStatus int
        wantBody   string
    }{
        {
            name:       "Valid User",
            body:       map[string]interface{}{"username": "alice", "email": "alice@example.com"},
            wantStatus: 201,
            wantBody:   "",
        },
        {
            name:       "Missing Email",
            body:       map[string]interface{}{"username": "bob"},
            wantStatus: 400,
            wantBody:   "Field validation for 'Email' failed",
        },
        {
            name:       "Short Username",
            body:       map[string]interface{}{"username": "bo", "email": "bob@example.com"},
            wantStatus: 400,
            wantBody:   "min",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            res := ts.Post(t, "/users", tt.body)
            
            res.AssertStatus(t, tt.wantStatus)
            if tt.wantBody != "" {
                res.AssertBody(t, tt.wantBody)
            }
        })
    }
}
```


## Features

| Package | Features |
|--------|----------|
| **pkg/test** | Fluent API: `GET/POST/...`, `WithJSON`, `Do()`, `ExpectStatus`, `ExpectBody`, `ExpectBodyContains`, `ExpectHeader`, `ExpectJSON`. |
| **pkg/testkit** | `TestServer`, `Get(t, path)`, `Post(t, path, body)`, `AssertStatus`, `AssertBody`. |

## Integration Testing

For end-to-end integration tests, you can run your server in a separate goroutine and use standard `net/http` clients; for handler-level checks, `pkg/test` or `testkit` is faster and simpler.
