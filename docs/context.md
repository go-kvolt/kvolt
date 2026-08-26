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
c.JSON(404, map[string]string{"error": "Not Found"})
```

## Request Binding

Bind the request body to a struct (supports JSON).

```go
type User struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

var u User
// Fast path (ERP invoice, checkout): decode only
if err := c.BindJSON(&u); err != nil {
    return c.JSON(400, map[string]string{"error": err.Error()})
}
// Bind() parses JSON AND runs validation (`validate` tags)
if err := c.Bind(&u); err != nil {
    return c.JSON(400, map[string]string{"error": err.Error()})
}
// Use 'u' safely here...
```

## Data Sharing (Keys)

Share data between middleware and handlers.

```go
c.Set("user_id", 123)
id, exists := c.Get("user_id")
```

## HTML & Templates

```go
// Render template
c.RenderHTML(200, "index.html", data)

// Raw HTML
c.HTML(200, "<h1>Hello</h1>")
```

## File Uploads

```go
file, _ := c.FormFile("profile_pic")
c.SaveUploadedFile(file, "./uploads/"+file.Filename)
```

## Sending Files

```go
c.File("./public/document.pdf")
```

## WebSockets

```go
conn, err := c.Upgrade()
```

## Zero-Allocation

The `Context` object is pooled using `sync.Pool`. This means:
1.  **Do NOT** pass the Context pointer to a background goroutine.
2.  If you need data in a goroutine, copy the values out first.
