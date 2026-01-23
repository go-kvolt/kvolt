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

## Creating Custom Middleware

```go
func MyMiddleware() func(c *context.Context) error {
    return func(c *context.Context) error {
        fmt.Println("Before Request")
        
        c.Next() // Continue to next handler
        
        fmt.Println("After Request")
        return nil
    }
}
```
