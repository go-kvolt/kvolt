# Getting Started with KVolt ⚡

KVolt is a high-performance, developer-friendly Go web framework designed for building fast and scalable web applications.

## Installation

### Prerequisites

-   **Go**: Version 1.21 or higher.

### Option 1: Using the KVolt CLI (Recommended)

The KVolt CLI is the easiest way to start a new project. It scaffolds a production-ready directory structure for you.

1.  **Install the CLI:**

    ```bash
    go install github.com/go-kvolt/kvolt/cmd/kvolt@latest
    ```

2.  **Verify Installation:**

    ```bash
    kvolt --help
    ```

3.  **Create a New Project:**

    ```bash
    kvolt new my-app
    ```

    This will create a new directory `my-app` with the following structure:
    -   `cmd/api/`: Entry point of your application
    -   `internal/`: Business logic
    -   `pkg/`: shared packages
    -   `config.yaml`: Configuration file

4.  **Run the Project:**

    ```bash
    cd my-app
    go mod tidy
    kvolt run
    ```

    The `kvolt run` command starts your application in development mode with **hot-reload** enabled.

### Option 2: Standard Go Module

If you prefer to start from scratch or integrate KVolt into an existing project:

1.  **Initialize a Go Module:**

    ```bash
    mkdir my-app && cd my-app
    go mod init my-app
    ```

2.  **Install KVolt:**

    ```bash
    go get github.com/go-kvolt/kvolt@latest
    ```

3.  **Create `main.go`:**

    ```go
    package main

    import (
        "github.com/go-kvolt/kvolt"
        "github.com/go-kvolt/kvolt/context"
    )

    func main() {
        app := kvolt.New()

        app.GET("/", func(c *context.Context) error {
            return c.String(200, "Hello, KVolt!")
        })

        app.Run(":8080")
    }
    ```

4.  **Run:**

    ```bash

    go run main.go
    ```

## HTTPS & HTTP/2 🔒

KVolt supports HTTPS and HTTP/2 out of the box with `RunTLS`.

```go
// RunTLS(addr, certFile, keyFile)
app.RunTLS(":443", "cert.pem", "key.pem")
```

## Next Steps

-   [Routing Guide](router.md)
-   [Middleware Guide](middleware.md)
-   [CLI Usage](cli.md)

