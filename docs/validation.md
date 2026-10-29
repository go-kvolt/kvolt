# Validation

`Context.Bind` decodes JSON then validates with **[go-playground/validator/v10](https://github.com/go-playground/validator)** (`validate` struct tags).

`pkg/validator` is a small optional helper for manual checks; it is **not** what `Bind` uses.

## Usage with Bind

```go
type RegisterRequest struct {
    Username string `json:"username" validate:"required,min=3"`
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Age      int    `json:"age"      validate:"min=18"`
}

app.POST("/register", func(c *context.Context) error {
    var req RegisterRequest
    if err := c.Bind(&req); err != nil {
        return httperr.BadRequest("validation_failed", err.Error())
    }
    return c.JSON(200, req)
})
```

- `BindJSON` — decode only (no validation)
- `Bind` — decode + playground validation
- `BindQuery` / `BindForm` / `BindURI` — bind from query/form/path (`form` / `query` / `uri` tags); validate separately with `validator/v10` if needed

## Common tags

`required`, `email`, `min`, `max`, `len`, `oneof`, `uuid`, and many more — see playground docs.
