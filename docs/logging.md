# Structured Logging 📝

KVolt provides a fast, structured JSON logger in `pkg/logger`. It is designed for production environments where logs need to be parsed by tools like Datadog, ELK, or CloudWatch.

## Usage

```go
package main

import (
    "os"
    "github.com/go-kvolt/kvolt/pkg/logger"
)

func main() {
    // 1. Create Logger
    log := logger.New(os.Stdout, logger.INFO)
    
    // 2. Log Info
    log.Info("Server started", map[string]interface{}{
        "port": 8080,
        "env": "production",
    })
    // Output: {"level":"INFO","time":"...","message":"Server started","fields":{"port":8080,"env":"production"}}

    // 3. Log Error
    // log.Error("Database connection failed", err)
}
```

## Log Levels

-   `DEBUG`
-   `INFO` (Default)
-   `WARN`
-   `ERROR`

## Middleware

The default KVolt `middleware.Logger()` uses an optimized console logger for development, but for production, you might want to wrap this structured logger.
