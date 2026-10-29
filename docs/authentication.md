# Authentication

KVolt uses **HS256 JWT** via `github.com/golang-jwt/jwt/v5` for both:

- `pkg/auth` — `GenerateToken` / `ParseToken` (set secret first)
- `middleware.JWT` / `middleware.JWTOptional` — request validation

There is **no default secret**. Misconfiguration fails closed.

## Generate tokens

```go
import (
    "time"
    "github.com/go-kvolt/kvolt/v2/pkg/auth"
)

auth.SetSecret(os.Getenv("JWT_SECRET")) // or hard-fail at startup if empty

token, err := auth.GenerateToken(auth.Claims{
    "sub":  "user-123",
    "role": "admin",
}, 24*time.Hour)
```

You can also set `JWT_SECRET` in the environment before process start.

## Protect routes

```go
import "github.com/go-kvolt/kvolt/v2/middleware"

app.Use(middleware.JWT(middleware.JWTConfig{
    SigningKey: os.Getenv("JWT_SECRET"), // required — empty panics
}))

// Optional bearer (anonymous if missing; invalid still 401)
app.Use(middleware.JWTOptional(middleware.JWTConfig{
    SigningKey: os.Getenv("JWT_SECRET"),
}))
```

| Option | Description | Default |
| :--- | :--- | :--- |
| `SigningKey` | HMAC secret (**required**) | — |
| `TokenLookup` | `header`, `query`, or `cookie` | `header:Authorization` |
| `ContextKey` | Where claims are stored | `"user"` |

Prefer **header** tokens. Query tokens can leak via logs and Referer.

## Read claims

```go
claims, ok := middleware.Claims(c)
role := middleware.ClaimString(c, "role")
```

## Basic Auth / API keys

```go
app.GET("/admin", middleware.BasicAuth(map[string]string{"admin": "secret"}), handler)
app.GET("/api", middleware.APIKey("my-api-key"), handler)
```

## Sessions

See [session.md](session.md) for cookie sessions + CSRF.
