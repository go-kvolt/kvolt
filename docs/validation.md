# Validation ✅

KVolt includes a struct validator package `pkg/validator` to ensure data integrity for your request inputs.

## Usage

Tag your struct fields with regex-like validation rules using the `validate` tag.

```go
type RegisterRequest struct {
    Username string `json:"username" validate:"required,min=3"`
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Age      int    `json:"age"      validate:"min=18"`
}

app.POST("/register", func(c *context.Context) error {
    var req RegisterRequest
    
    // 1. Bind JSON (Parses body)
    if err := c.Bind(&req); err != nil {
        return c.Status(400).JSON(400, map[string]string{
            "error": "Invalid JSON format",
        })
    }

    // 2. Validate (Checks struct tags)
    // Note: c.Bind() typically calls Validate() internally if your framework supports it, 
    // but here we show explicit validation usage if needed separate from binding.
    // *In KVolt, c.Bind() calls validator.Validate() automatically!*
    // So if you reach here, validation has already passed. 
    // If you want to handle validation errors specifically:
    
    /* 
    err := c.Bind(&req)
    if err != nil {
       // Check if it's a validation error
       return c.JSON(400, map[string]string{"error": err.Error()})
    } 
    */

    return c.JSON(201, map[string]string{
        "message": "User registered successfully",
        "username": req.Username,
    })
})
```


## Supported Tags

| Tag | Description | Example |
| :--- | :--- | :--- |
| `required` | Field cannot be empty, nil, or zero. | `validate:"required"` |
| `email` | Field must be a valid email format. | `validate:"email"` |
| `min=n` | String must be at least `n` characters long. | `validate:"min=5"` |

> Note: To chain multiple rules, separate them with a comma (e.g., `required,email`).
