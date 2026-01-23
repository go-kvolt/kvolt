# Context API 📦

The `context.Context` object is the heart of every request in KVolt. It wraps the `http.Responsewriter` and `http.Request` and offers helper methods.

## JSON Response

```go
c.JSON(200, map[string]string{"msg": "Success"})
```

## Plain Text

```go
c.String(200, "Hello World")
```

## Parameters

```go
// Path Params: /users/:id
id := c.Params.Get("id")

// Query Params: /search?q=foo
q := c.Query("q")
```

## Status Codes

```go
c.Status(404).String(404, "Not Found")
```

## Zero-Allocation

The `Context` object is pooled using `sync.Pool`. This means:
1.  **Do NOT** pass the Context pointer to a background goroutine.
2.  If you need data in a goroutine, copy the values out first.
