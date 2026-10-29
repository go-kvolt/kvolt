# Structured Errors (`pkg/httperr`)

Return typed API errors from handlers. KVolt maps them to JSON automatically.

```go
import "github.com/go-kvolt/kvolt/v2/pkg/httperr"

app.POST("/checkout", func(c *context.Context) error {
    if bad {
        return httperr.BadRequest("invalid_cart", "Cart is empty")
    }
    return c.JSON(200, order)
})
```

Response:

```json
{"error":"invalid_cart","message":"Cart is empty"}
```

## Helpers

| Helper | Status |
|--------|--------|
| `BadRequest` | 400 |
| `Unauthorized` | 401 |
| `Forbidden` | 403 |
| `NotFound` | 404 |
| `TooManyRequests` | 429 |
| `Internal` | 500 |
| `New(status, code, message)` | custom |
| `.WithDetails(any)` | adds `details` field |

```go
return httperr.BadRequest("validation_failed", "Invalid input").
    WithDetails(map[string]string{"email": "required"})
```

## Manual abort

```go
return c.AbortWithError(httperr.Unauthorized("login_required", "Sign in first"))
```
