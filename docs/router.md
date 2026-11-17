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

A parameter always holds a non-empty segment: `/users//posts` does not match
`/users/:id/posts`.

## Matching rules

- **Static beats parameter.** With `/users/new` and `/users/:id` registered, `/users/new`
  goes to the static route and everything else (`/users/next`, `/users/42`) to `:id`.
- **`HEAD` uses the `GET` route.** A `GET` route also answers `HEAD` with the same headers
  and no body, unless you register a `HEAD` route for that path.
- **405 and `Allow`.** A path registered for other methods answers 405 with an `Allow`
  header listing them.

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

### Nested groups and middleware isolation

`Group()` copies the parent middleware slice. Sibling groups do **not** share the same backing array, so `Use()` on one child does not leak onto another:

```go
api := app.Group("/api")
api.Use(authRequired)

exec := api.Group("/executive")
exec.Use(roleRequired("EXECUTIVE"))

designer := api.Group("/designer")
designer.Use(roleRequired("DESIGNER")) // only [auth] + designer role — not executive
```

## Multiple static segments after a parameter

Routes that share a param prefix but differ in the next segment are supported:

```go
app.GET("/orders/:orderId/assets", listAssets)
app.GET("/orders/:orderId/take", takeOrder)
app.POST("/orders/:orderId/decision", decide)
app.GET("/orders/:orderId/files/:assetId", downloadFile)
```

Registration order does not matter. Use `c.Param("orderId")` (or `c.Params.Get`) for captured values.

## Patterns to avoid

- **Param before static in one segment** — e.g. `GET /items/:id/read` may not populate params reliably. Prefer `GET /items/read/:id`.
- **Duplicate path + method** — only one handler per exact path and HTTP method.

## Static Files

Serve static files from a directory (e.g., images, scripts).

```go
// Endpoint: /assets/style.css -> ./public/style.css
app.Static("/assets", "./public")
```

