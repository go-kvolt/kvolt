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

For production you can disable stack trace logging with `RecoveryWithConfig`:

```go
app.Use(middleware.RecoveryWithConfig(middleware.RecoveryConfig{
    LogStackTrace: false, // set true in development only
}))
```

### 3. Gzip Compression
Compresses **large non-JSON** responses when the client sends `Accept-Encoding: gzip`.
JSON APIs and bodies under 1KB are skipped (CPU cost for tiny invoice/product JSON).

```go
app.Use(middleware.Gzip())
app.Use(middleware.GzipWithConfig(middleware.GzipConfig{
    MinSize:          2048,
    SkipContentTypes: nil, // compress JSON too
}))
```


### 4. CORS
```go
app.Use(middleware.CORS()) // Allow-Origin * (no credentials)

app.Use(middleware.CORSWithConfig(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowCredentials: true,
    MaxAge:           600,
}))
```

Never combine `AllowOrigins: ["*"]` with `AllowCredentials: true` (browsers reject it; KVolt disables credentials in that case).

### 5. Rate Limiter
In-process token bucket (default key: `ClientIP()` — set `Engine.TrustedProxies` behind a load balancer).
`MaxKeys` (default 10000) bounds memory if your `KeyFunc` is high-cardinality.
When the map is full, the **oldest** bucket is evicted (new clients are not 429'd just for being new):

```go
app.Use(middleware.Limiter(100, 200))

app.Use(middleware.LimiterWithConfig(middleware.LimiterConfig{
    RPS: 50, Burst: 100, MaxKeys: 10000,
    KeyFunc: func(c *context.Context) string {
        if k := c.Request.Header.Get("X-API-Key"); k != "" {
            return "key:" + k
        }
        return c.ClientIP()
    },
}))
```

Multi-instance Redis limiter (optional package):

```go
import "github.com/go-kvolt/kvolt/v2/middleware/redislimit"

app.Use(redislimit.New(rdb, 100, time.Second))
```

### 5b. Metrics
In-process request counters (optional `OnRequest` hook for OTel):

```go
store := &middleware.MetricsStore{}
app.Use(middleware.Metrics(store))
app.GET("/metrics.json", func(c *context.Context) error {
    return c.JSON(200, store.Snapshot())
})
```

### 5c. Prometheus (optional package)
Does not pull `client_golang` unless imported:

```go
import kvprom "github.com/go-kvolt/kvolt/v2/middleware/prometheus"

app.Use(kvprom.Middleware())
app.GET("/metrics", kvprom.Handler())
```

### 5d. Tenant (multi-tenant helper)
Resolves a tenant id into `c.Keys["tenant_id"]` (header / route param / JWT claim / custom extractor):

```go
app.Use(middleware.Tenant(middleware.TenantConfig{
    Header:   "X-Tenant-ID",
    Required: true,
}))
// later: middleware.TenantID(c)
```

### 5e. RBAC helpers
Generic permission gates — you supply the checker (DB, JWT claims, etc.):

```go
check := func(c *context.Context, perm string) (bool, error) {
    // look up tenant + user permissions
    return perm == "inv.read", nil
}
app.GET("/stock", middleware.RequirePermission("inv.read", check), handler)
app.POST("/stock", middleware.RequireAnyPermission(check, "inv.write", "inv.admin"), handler)
```

### 5f. Idempotency-Key
Replays prior responses for `Idempotency-Key` on POST/PUT/PATCH (memory store included; plug in Redis for multi-instance):

```go
store := middleware.NewMemoryIdempotencyStore()
app.Use(middleware.Idempotency(middleware.IdempotencyConfig{Store: store, Required: true}))
```

### 5g. Audit hook
Emits a structured event after each request (you persist/ship it):

```go
app.Use(middleware.Audit(middleware.AuditConfig{
    OnAudit: func(e middleware.AuditEvent) {
        log.Printf("audit method=%s path=%s tenant=%s status=%d actor=%s", e.Method, e.Path, e.TenantID, e.Status, e.Actor)
    },
    ActorFunc: func(c *context.Context) string { return "user-from-jwt" },
}))
```

### 5h. CSRF
Double-submit cookie protection for browser cookie sessions:

```go
app.Use(middleware.CSRF())
// or CSRFWithConfig(CSRFConfig{SkipPaths: []string{"/webhooks"}, InsecureCookie: true}) // local HTTP only
```

Safe methods set a `csrf_token` cookie when missing. Mutating methods require header `X-CSRF-Token` to match.
The cookie `Secure` flag is set automatically on HTTPS (TLS or trusted `X-Forwarded-Proto`); use `InsecureCookie` for plain local HTTP.

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

HSTS is set only for TLS requests, or when `X-Forwarded-Proto: https` comes from a peer in `Engine.TrustedProxies` (spoofed headers from clients are ignored).

### 7. Max Body Size
Limit request body size to prevent large-payload attacks. Reads beyond the limit fail (e.g. in `Bind()`); handle the error and respond with 413 if desired.

```go
app.Use(middleware.MaxBodySize(1 << 20)) // 1MB
// or: middleware.MaxBodySizeBytes(10 * 1024 * 1024)
```

`kvolt.Default()` already includes a 1MB limit.

### 8. Request ID
Sets `X-Request-ID` on every response. Reuses a sanitized incoming header when present (alphanumeric / `.` / `_` / `-`, max 128); otherwise generates a UUID.

```go
app.Use(middleware.RequestID())
id, _ := c.Get(middleware.RequestIDKey)
```

### 9. Timeout
Puts a deadline on `c.Request.Context()`. Slow invoice/DB handlers should honor cancel.

```go
app.Use(middleware.Timeout(15 * time.Second))
```

### 10. JWT Authentication
```go
app.Use(middleware.JWT(middleware.JWTConfig{
    SigningKey:  os.Getenv("JWT_SECRET"), // required
    TokenLookup: "header:Authorization",
}))
```

### 11. Basic Auth / API Key / Health
```go
app.GET("/admin", middleware.BasicAuth(map[string]string{"a": "b"}), h)
app.GET("/api", middleware.APIKey("secret"), h)
app.GET("/livez", middleware.Live())
app.GET("/readyz", middleware.Ready(&readyFlag))
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


