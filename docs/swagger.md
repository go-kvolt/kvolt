# API Documentation (Swagger) 📜

KVolt has built-in support for generating OpenAPI 3.0 specifications and serving them via a beautiful UI (Scalar).

## Setup

To enable the documentation endpoint, you need to register the swagger handler.

```go
package main

import (
    "github.com/go-kvolt/kvolt"
    "github.com/go-kvolt/kvolt/context"
    "github.com/go-kvolt/kvolt/pkg/swagger"
)

func main() {
    app := kvolt.New()

    // Define Routes
    app.GET("/users", func(c *context.Context) error {
        return c.String(200, "Users")
    }).Desc("Get all users") // Add description for docs

    // 1. Read Generated Spec (optional, if using swaggo)
    // doc, _ := swagger.ReadDoc()

    // 2. serve Swagger UI
    // Visits http://localhost:8080/swagger/index.html
    app.GET("/swagger/*any", swagger.Handler(swagger.Config{
          Title:          "My API Docs",
          RoutesProvider: swagger.Adapter(app), // Auto-discover routes
    }))

    app.Run(":8080")
}
```

## Features

-   **Auto-Discovery**: `swagger.Adapter(app)` automatically scans your registered routes.
-   **Descriptions**: Use `.Desc("Summary")` on your route definitions to add documentation.
-   **Scalar UI**: Uses the modern Scalar UI for rendering.
-   **Parameter Parsing**: Automatically detects `:id` and `*wildcard` parameters and adds them to the spec.
