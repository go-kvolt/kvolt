# Routing Guide 🛣️

KVolt used a high-performance **Radix Tree** (Trie) based router. This allows for constant-time route matching regardless of how many routes you define.

## Basic Routing

```go
app.GET("/hello", handler)
app.POST("/users", createHandler)
```

## Named Parameters

You can use `:name` to capture path segments.

```go
app.GET("/users/:id", func(c *context.Context) error {
    id := c.Params.Get("id")
    return c.String(200, "User ID: " + id)
})
```

## Wildcards

Use `*name` to catch everything after a specific path.

```go
app.GET("/files/*filepath", func(c *context.Context) error {
    path := c.Params.Get("filepath")
    return c.String(200, "File: " + path)
})
```

## Route Groups

Grouping allows you to apply middleware to a specific set of routes.

```go
v1 := app.Group("/v1")
v1.Use(AuthMiddleware)

v1.GET("/profile", profileHandler) // /v1/profile (Protected)
```
