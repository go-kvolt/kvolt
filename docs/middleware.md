# Middleware Guide 🛡️

Middleware function with the signature `func(c *context.Context) error`.

## Built-in Middleware

KVolt comes with a standard library of middleware ready to use.

### 1. Logger
Asynchronous, zero-blocking console logger.

```go
app.Use(middleware.Logger())
```

### 2. Recovery
Catches panics in your handlers and returns a 500 error instead of crashing the server.

```go
app.Use(middleware.Recovery())
```

### 3. Gzip Compression
Compresses responses using Gzip if the client supports it.

```go
app.Use(middleware.Gzip())
```


### 4. CORS
Configures Cross-Origin Resource Sharing.

```go
app.Use(middleware.CORS())
```

### 5. Rate Limiter
Protect your API from abuse using the Token Bucket rate limiter.

```go
// Allow 100 requests per second with a burst of 200
app.Use(middleware.Limiter(100, 200))
```

### 6. Secure Headers
Protects your application from common web vulnerabilities by setting standard HTTP headers (HSTS, X-Frame-Options, CSP, etc.).

```go
app.Use(middleware.Secure())
```

#### Configuration
You can customize the security headers using `SecureWithConfig`:

```go
app.Use(middleware.SecureWithConfig(middleware.SecureConfig{
    XFrameOptions: "SAMEORIGIN",
    HSTSMaxAge:    31536000,
}))
```

### 6. JWT Authentication
Secure your routes with JSON Web Tokens.

```go
app.Use(middleware.JWT(middleware.JWTConfig{
    SigningKey:  "secret-key",
    TokenLookup: "header:Authorization", // or "query:token", "cookie:auth"
}))
```

## Creating Custom Middleware

```go
// RequestTimer measures the duration of each request
func RequestTimer() func(c *kvolt.Context) error {
    return func(c *kvolt.Context) error {
        start := time.Now()
        
        // Process request
        c.Next()
        
        // Calculate duration and log
        latency := time.Since(start)
        log.Printf("[%s] %s | %v", c.Request.Method, c.Request.URL.Path, latency)
        
        // Optional: Add to response header
        c.Writer.Header().Set("X-Response-Time", latency.String())
        return nil
    }
}
```


