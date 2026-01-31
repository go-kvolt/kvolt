# Testing 🧪

KVolt is designed with testability in mind. The `pkg/testkit` package provides utilities to easily test your handlers and routes.

## Using `testkit`

The `testkit` allows you to spin up a test instance of your app and make requests against it without spawning a real network server.

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

-   **TestServer**: Wraps your KVolt app.
-   **Fluent assertions**: `AssertStatus`, `AssertBody`.
-   **Methods**: `Get`, `Post` (with auto-JSON marshalling).

## Integration Testing

For end-to-end integration tests, you can also run your server in a separate goroutine and use standard `net/http` clients, but `testkit` is faster and simpler for handler verification.
