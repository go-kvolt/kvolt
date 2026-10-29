# Session Authentication

KVolt provides a flexible session management system powered by its caching layer (`pkg/cache`).

## Overview

Unlike stateless JWTs, sessions are stateful. The server generates a unique Session ID, stores the user data in the cache (memory or Redis), and sends the ID to the client via a cookie or header.

## Usage

### 1. Initialize Manager

```go
import (
    "time"

    "github.com/go-kvolt/kvolt/v2/pkg/cache"
    cacheredis "github.com/go-kvolt/kvolt/v2/pkg/cache/redis"
    "github.com/go-kvolt/kvolt/v2/pkg/session"
    "github.com/redis/go-redis/v9"
)

// Memory
store := cache.NewMemoryStore(10 * time.Minute)

// Or Redis (optional package — JSON values; use a key prefix)
rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
store = cacheredis.New(rdb, cacheredis.WithPrefix("myapp:sess:"))

sessManager := session.New(store, 24*time.Hour)
```

### 2. Login Handler (Create Session)

```go
app.POST("/login", func(c *context.Context) error {
    userData := map[string]string{"id": "123", "role": "admin"}
    token, err := sessManager.Create(userData)
    if err != nil {
        return c.Status(500).String(500, "Session Error")
    }

    c.SetCookie("session_id", token, int((24 * time.Hour).Seconds()), "/", "", true, true)
    return c.String(200, "Logged In")
})
```

### 3. Protect Routes (Middleware)

```go
import "github.com/go-kvolt/kvolt/v2/middleware"

authMiddleware := middleware.Session(middleware.SessionConfig{
    Manager: sessManager,
    Lookup:  "cookie:session_id",
})

app.GET("/profile", authMiddleware, func(c *context.Context) error {
    data := c.MustGet("session")
    return c.JSON(200, data)
})
```

### 4. Logout

```go
app.POST("/logout", authMiddleware, func(c *context.Context) error {
    cookie, _ := c.Request.Cookie("session_id")
    sessManager.Destroy(cookie.Value)
    c.ClearCookie("session_id", "/", "")
    return c.String(200, "Logged Out")
})
```

### Cookie helpers

```go
c.SetCookie(name, value, maxAge, path, domain, secure, httpOnly)
c.SetCookieWith(name, value, context.CookieOptions{SameSite: http.SameSiteStrictMode, ...})
c.ClearCookie(name, path, domain)
```

### CSRF (browser forms)

For cookie-authenticated browser apps, also enable double-submit CSRF:

```go
app.Use(middleware.CSRF())
// Client must send header X-CSRF-Token matching the csrf_token cookie on POST/PUT/PATCH/DELETE.
```
