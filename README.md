# KVolt ⚡ 

<p align="center">
  <img src="assets/logo.png" alt="KVolt Logo" width="200">
</p>


**KVolt** is a high-performance, developer-friendly Go web framework built for speed and ease of use. It combines the raw power of `net/http` with a modern API, zero-allocation routing, and a suite of "Batteries Included" utilities.

## Features 🚀

*   **Extreme Performance**: **250,000+ Req/Sec** using `bytedance/sonic` for JSON serialization and `sync.Pool`.
*   **Radix Tree Router**: Smart routing with support for named parameters (`/users/:id`), wildcards, and regex.
*   **Static Assets**: Built-in support for serving static files (`app.Static()`) with correct prefix handling.
*   **Protocol Ready**: Native support for **HTTP/2** (`RunTLS`) and **WebSockets** (`c.Upgrade()`).
*   **Auto-Documentation**:
    *   Built-in **Swagger UI** integration.
    *   Automatic route discovery and documentation generation (`app.Routes()`).
*   **Middleware Ecosystem**:
    *   **Logger**: Asynchronous, non-blocking console logging.
    *   **Recovery**: Panic recovery to keep your server alive.
    *   **Gzip**: Automatic response compression.
    *   **CORS**: Configurable Cross-Origin Resource Sharing.
    *   **Rate Limiter**: Token-bucket strategy for API protection.
*   **Batteries Included**:
    *   **Dependency Injection** (`pkg/di`)
    *   **Configuration Loader** (`pkg/config`)
    *   **Structured Logging** (`pkg/logger`)
    *   **Input Validation** (`pkg/validator`)
    *   **Authentication** (`pkg/auth` - JWT & Bcrypt)
*   **Graceful Shutdown**: Native support for OS signals (SIGINT/SIGTERM).

## Getting Started 🛠️

You can start a KVolt project in two ways: using our powerful CLI or the standard Go method.

### Option 1: The KVolt Way (Recommended) ⚡
The CLI scaffolds a production-ready directory structure (`cmd`, `internal`, `pkg`, etc.) for you.

1. **Install the CLI**
   ```bash
   go install github.com/go-kvolt/kvolt/cmd/kvolt@latest
   ```

2. **Create & Run a New Project**
   ```bash
   # Create a new project
   kvolt new my-app
   
   # Run it
   cd my-app
   go mod tidy      # Autodetects latest kvolt version
   go run cmd/api/main.go
   ```

### Option 2: The Standard Go Way 📦
If you prefer starting from scratch or adding KVolt to an existing project.

1. **Initialize Module**
   ```bash
   mkdir my-app && cd my-app
   go mod init my-app
   ```

2. **Install Framework**
   ```bash
   go get github.com/go-kvolt/kvolt@latest
   ```

3. **Create `main.go`**
   (See the Quick Start example below)

## Quick Start

```go
package main

import (
	"github.com/go-kvolt/kvolt"
	"github.com/go-kvolt/kvolt/context"
	"github.com/go-kvolt/kvolt/middleware"
	"github.com/go-kvolt/kvolt/pkg/swagger"
)

func main() {
    // 1. Initialize Engine
	app := kvolt.New()

    // 2. Register Global Middleware
	app.Use(middleware.Logger())
	app.Use(middleware.Recovery())

	// 3. Define Routes (with Binding!)
	app.POST("/users", func(c *context.Context) error {
        type User struct {
            Name string `validate:"required"`
        }
        var u User
        if err := c.Bind(&u); err != nil {
            return c.Status(400).String(400, err.Error())
        }
		return c.JSON(201, u)
	})
    
    // 4. Serve Static Files
    app.Static("/assets", "./public")
    
    // 5. Auto-Documentation (Scalar UI)
    // Visit http://localhost:8080/swagger/index.html
    doc, _ := swagger.ReadDoc()
    app.GET("/swagger/*any", swagger.Handler(swagger.Config{
          SpecJSON:       doc,
          RoutesProvider: swagger.Adapter(app),
    }))

	// 6. Start Server
	app.Run(":8080")
}
```

## Benchmarks 📊

KVolt is optimized for raw speed. By utilizing `sync.Pool` for Context recycling and the **Sonic** JSON engine, it achieves massive throughput.

*   **250,000+ Req/Sec** on standard hardware (v0.2).
*   **~8µs Latency** per JSON request.
*   **Zero-Allocation** hot paths.
*   **Asynchronous Logging** to prevent I/O bottlenecks.

## License

MIT
