# Authentication 🔐

KVolt provides utilities for securing your application, primarily through JSON Web Tokens (JWT).

## JWT Middleware

The `middleware.JWT` middleware validates tokens on incoming requests.

```go
import "github.com/go-kvolt/kvolt/middleware"

// ...

app.Use(middleware.JWT(middleware.JWTConfig{
    SigningKey: "your-secret-key",
}))
```

### Configuration Options

| Option | Description | Default |
| :--- | :--- | :--- |
| `SigningKey` | Secret key used to sign tokens (Required). | - |
| `ContextKey` | Key to store claims in `c.Context`. | `"user"` |
| `TokenLookup` | Source of the token (`header`, `query`, `cookie`). | `"header:Authorization"` |
| `AuthScheme` | Prefix for the header value (e.g. Bearer). | `"Bearer"` |


## Real-World Example: Login & Protection

Here is a complete example showing how to implement a Login endpoint (generating tokens) and a Protected endpoint (validating tokens).

```go
package main

import (
    "time"
    "github.com/go-kvolt/kvolt"
    "github.com/go-kvolt/kvolt/context"
    "github.com/go-kvolt/kvolt/middleware"
    "github.com/go-kvolt/kvolt/pkg/auth"
    "github.com/golang-jwt/jwt/v5"
)

func main() {
    app := kvolt.New()
    
    // Shared Secret
    secret := "super-secret-key"
    
    // Update pkg/auth secret so generated tokens match the middleware's expectation
    auth.Secret = []byte(secret)

    // 1. Login Endpoint (Public)
    app.POST("/login", func(c *context.Context) error {
        var creds struct {
            Username string `json:"username"`
            Password string `json:"password"`
        }
        if err := c.Bind(&creds); err != nil {
            return c.Status(400).String(400, "Invalid Request")
        }

        // Mock User Check (Replace with DB check)
        if creds.Username != "admin" || creds.Password != "password" {
            return c.Status(401).String(401, "Invalid Credentials")
        }

        // Generate Token using pkg/auth helper
        token, err := auth.GenerateToken(auth.Claims{
            "sub":  "123",
            "name": "Admin User",
            "role": "admin",
        }, 30*time.Minute)
        
        if err != nil {
            return c.Status(500).String(500, "Could not generate token")
        }

        return c.JSON(200, map[string]string{
            "token": token,
        })
    })

    // 2. Setup JWT Middleware for Protected Routes
    authMiddleware := middleware.JWT(middleware.JWTConfig{
        SigningKey: secret,
    })

    // 3. Protected Routes
    protected := app.Group("/api")
    protected.Use(authMiddleware)
    {
        protected.GET("/profile", func(c *context.Context) error {
            // Retrieve claims set by the middleware
            user := c.MustGet("user").(jwt.MapClaims)
            
            return c.JSON(200, map[string]interface{}{
                "message": "Welcome back!",
                "user_id": user["sub"],
                "role":    user["role"],
            })
        })
    }

    app.Run(":8080")
}
```

## Token Extraction Strategy

KVolt supports multiple ways to extract tokens:

1.  **Header**: `TokenLookup: "header:Authorization"` (e.g., `Authorization: Bearer <token>`)
2.  **Query**: `TokenLookup: "query:token"` (e.g., `?token=<token>`)
3.  **Cookie**: `TokenLookup: "cookie:auth_token"`

## Helper Package (`pkg/auth`)

The `pkg/auth` package (if available) may contain helpers for password hashing (Bcrypt) and token generation.

*(Check your local `pkg/auth` for specific helper functions)*
