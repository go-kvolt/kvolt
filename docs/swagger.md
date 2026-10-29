# API Documentation (Swagger)

KVolt generates OpenAPI 3.0 specs and serves them with Scalar.

## Setup

```go
package main

import (
    "github.com/go-kvolt/kvolt/v2"
    "github.com/go-kvolt/kvolt/v2/context"
    "github.com/go-kvolt/kvolt/v2/pkg/swagger"
)

type CreateUser struct {
    Email string `json:"email"`
    Name  string `json:"name,omitempty"`
}

type User struct {
    ID    string `json:"id"`
    Email string `json:"email"`
}

func main() {
    app := kvolt.Default()

    app.POST("/users", func(c *context.Context) error {
        var in CreateUser
        if err := c.BindJSON(&in); err != nil {
            return err
        }
        return c.JSON(201, User{ID: "1", Email: in.Email})
    }).
        Desc("Create user").
        Tags("users").
        Request(CreateUser{}).
        Response(201, User{}).
        Security("bearer")

    app.GET("/swagger/*any", swagger.Handler(swagger.Config{
        Title:          "My API Docs",
        RoutesProvider: swagger.Adapter(app),
        Host:           "localhost:8080",
    }))

    app.Run(":8080")
}
```

Visit `http://localhost:8080/swagger/index.html`.

## Features

-   **Auto-discovery** via `swagger.Adapter(app)`
-   **`.Desc` / `.Tags` / `.Request` / `.Response` / `.Security`** on routes
-   **Schema reflection** from sample structs (`json` tags; `omitempty` → not required)
-   **Path params** from `:id` / `*wildcard`
-   **Scalar UI**

Limits: exotic Go types become generic `object`; prefer JSON-friendly samples.
