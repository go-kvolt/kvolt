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
```go
app.GET("/users/:id", func(c *context.Context) error {
    id := c.Params.Get("id")
    return c.String(200, "User ID: " + id)
})
```

## Real-World Pattern: RESTful API

Here is how you might structure a typical API resource.

```go
// Define Resource Handler
type UserHandler struct{}

func (h *UserHandler) Create(c *context.Context) error {
    return c.String(201, "Create User")
}
func (h *UserHandler) Get(c *context.Context) error {
    id := c.Param("id")
    return c.String(200, "Get User "+id)
}
func (h *UserHandler) Update(c *context.Context) error {
    id := c.Param("id")
    return c.String(200, "Update User "+id)
}
func (h *UserHandler) Delete(c *context.Context) error {
    id := c.Param("id")
    return c.String(200, "Delete User "+id)
}

// Register Routes
func RegisterUserRoutes(g *kvolt.RouterGroup) {
    h := &UserHandler{}
    g.POST("/", h.Create)
    g.GET("/:id", h.Get)
    g.PUT("/:id", h.Update)
    g.DELETE("/:id", h.Delete)
}

// Main
app := kvolt.New()
users := app.Group("/users")
RegisterUserRoutes(users)
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

## Static Files

Serve static files from a directory (e.g., images, scripts).

```go
// Endpoint: /assets/style.css -> ./public/style.css
app.Static("/assets", "./public")
```

