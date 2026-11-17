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

`required` rejects the zero value: an empty string, `0`, `false`, a zero `time.Time`, an
empty slice or map. A field left out of the JSON body decodes to the zero value, so this
is what makes "required" mean "the client sent it". When `0` or `false` is a real value
the client may send, make the field a pointer (`*int`, `*bool`): `nil` is missing, a
pointer to `0` is present. The built-in `pkg/validator` follows the same rule.
