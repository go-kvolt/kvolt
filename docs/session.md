# Session Authentication 🍪

KVolt provides a flexible session management system powered by its high-performance caching layer (`pkg/cache`).

## Overview

Unlike stateless JWTs, sessions are stateful. The server generates a unique Session ID, stores the user data in the cache (In-Memory by default), and sends the ID to the client via a Cookie or Header.

## Usage

### 1. Initialize Manager

First, creating a `session.Manager` using a cache store.

```go
import (
    "time"
    "github.com/go-kvolt/kvolt/pkg/cache"
    "github.com/go-kvolt/kvolt/pkg/session"
)

// Create a cache store (Memory, Redis, etc.)
store := cache.NewMemoryStore(10 * time.Minute)

// Create Session Manager with default TTL
sessManager := session.New(store, 24 * time.Hour)
```

### 2. Login Handler (Create Session)

When a user logs in, create a session and set the cookie.

```go
app.POST("/login", func(c *context.Context) error {
    // ... validate credentials ...
    
    // Create Session
    userData := map[string]string{"id": "123", "role": "admin"}
    token, err := sessManager.Create(userData)
    if err != nil {
        return c.Status(500).String(500, "Session Error")
    }

    // Set Cookie
    c.SetCookie(&http.Cookie{
        Name:     "session_id",
        Value:    token,
        Path:     "/",
        HttpOnly: true,
        Secure:   true, // Set to true in production
        Expires:  time.Now().Add(24 * time.Hour),
    })

    return c.String(200, "Logged In")
})
```

### 3. Protect Routes (Middleware)

Use the `middleware.Session` to protect routes.

```go
import "github.com/go-kvolt/kvolt/middleware"

authMiddleware := middleware.Session(middleware.SessionConfig{
    Manager: sessManager,
    Lookup:  "cookie:session_id", // or "header:X-Session-ID"
})

app.GET("/profile", authMiddleware, func(c *context.Context) error {
    // Retrieve data from context
    data := c.MustGet("session").(map[string]string)
    return c.JSON(200, data)
})
```

### 4. Logout (Destroy Session)

```go
app.POST("/logout", authMiddleware, func(c *context.Context) error {
    // Get token from cookie
    cookie, _ := c.Request.Cookie("session_id")
    
    // Destroy from server
    sessManager.Destroy(cookie.Value)
    
    // Clear client cookie
    c.SetCookie(&http.Cookie{
        Name:   "session_id",
        MaxAge: -1,
    })
    
    return c.String(200, "Logged Out")
})
```
