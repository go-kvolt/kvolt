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

## Raw body (webhooks)

Read bytes for signature checks without breaking later `Bind`/`BindJSON`:

```go
raw, err := c.RawBody()
// verify HMAC(raw, secret) …
var evt Event
if err := c.BindJSON(&evt); err != nil { … }
```

## Uploads

```go
fh, err := c.FormFileAllowed("image", 8<<20, ".jpg", ".png", ".webp")
if err != nil { return err }
// Always basename client filenames (blocks path traversal):
_ = c.SaveUploadedFile(fh, context.SafeUploadPath("./uploads", fh.Filename))
```

## Structured errors

See [errors.md](errors.md). Prefer `return httperr.BadRequest(...)` from handlers.

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

## Sending Files

```go
// Static path you control:
c.File("./public/document.pdf")

// User-supplied name under a root (blocks traversal):
_ = c.FileFrom("./public", name)
_ = c.AttachmentFrom("./exports", name, "report.pdf")
```

## Cookies

```go
c.SetCookie("session_id", token, 86400, "/", "", true, true)
c.SetCookieWith("csrf_token", tok, context.CookieOptions{
    Path: "/", MaxAge: 86400, HTTPOnly: false, SameSite: http.SameSiteLaxMode,
})
c.ClearCookie("session_id", "/", "")
```

## Server-Sent Events

See [sse.md](sse.md).

```go
_ = c.SSE()
_ = c.Event("tick", "1")
```

## WebSockets

```go
conn, err := c.Upgrade()
```

## Zero-Allocation

The `Context` object is pooled using `sync.Pool`. This means:
1.  **Do NOT** pass the Context pointer to a background goroutine.
2.  If you need data in a goroutine, copy the values out first.
