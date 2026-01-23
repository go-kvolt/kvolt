# KVolt ⚡ 

**KVolt** is a high-performance, developer-friendly Go web framework built for speed and ease of use. It combines the raw power of `net/http` with a modern API, zero-allocation routing, and a suite of "Batteries Included" utilities.

## Features 🚀

*   **Lightning Fast**: Optimized for high throughput (>150k RPS) with zero-allocation `sync.Pool` usage.
*   **Radix Tree Router**: Smart routing with support for named parameters (`/users/:id`) and wildcards.
*   **Group Routing**: Organize routes like `v1.Group("/api")` with group-specific middleware.
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

## Installation

```bash
go get github.com/go-kvolt/kvolt
```

## CLI Tool 🛠️

KVolt comes with a powerful CLI to scaffold projects and speed up development.

### Install CLI
```bash
go install github.com/go-kvolt/kvolt/cmd/kvolt@latest
```

### Usage
*   **Create Project**: Generates a production-ready directory structure.
    ```bash
    kvolt new my-app
    ```
*   **Run Dev Server**: Starts the server with hot-reload (future support).
    ```bash
    cd my-app
    kvolt run
    ```

## Quick Start

```go
package main

import (
	"github.com/go-kvolt/kvolt"
	"github.com/go-kvolt/kvolt/context"
	"github.com/go-kvolt/kvolt/middleware"
)

func main() {
    // 1. Initialize Engine
	app := kvolt.New()

    // 2. Register Global Middleware
	app.Use(middleware.Logger())
	app.Use(middleware.Recovery())

    // 3. Define Routes
	app.GET("/", func(c *context.Context) error {
		return c.JSON(200, map[string]string{
			"message": "Welcome to KVolt ⚡",
		})
	})
    
    // 4. Route Grouping
    api := app.Group("/api")
    {
        api.GET("/ping", func(c *context.Context) error {
            return c.String(200, "pong")
        })
    }

    // 5. Start Server
	app.Run(":8080")
}
```

## Benchmarks 📊

KVolt is optimized for raw speed. By utilizing `sync.Pool` for Context recycling and a custom Radix Tree for routing, it achieves minimal memory footprint.

*   **150,000+ Req/Sec** on standard hardware.
*   **Zero-Allocation** parsing for hot paths.
*   **Asynchronous Logging** to prevent I/O bottlenecks.

## License

MIT
