# Background Jobs (Queue) 📨

KVolt includes a "blazing fast" in-memory job queue system. It allows you to offload heavy tasks (email sending, image processing) to background workers.

## Features

-   **High Performance**: Uses buffered Go channels for nanosecond latency.
-   **Concurrency**: Built-in worker pool (default: concurrent processing).
-   **Zero-Dependency**: No Redis required (by default).

## Usage

### 1. Setup

Initialize the queue globally in your `main.go`.

```go
package main

import (
    "fmt"
    "github.com/go-kvolt/kvolt/pkg/queue"
)

func main() {
    // Buffer: 1000 jobs
    // Workers: 10 concurrent processors
    q := queue.NewMemoryQueue(1000, 10)
    
    // 1. Register Handlers
    q.Register("send_email", func(job queue.Job) error {
        email := job.Payload.(string)
        fmt.Printf("📧 Sending email to %s...\n", email)
        // Simulate work
        // time.Sleep(2 * time.Second) 
        return nil
    })
    
    // 2. Start Workers (Non-blocking)
    q.Start()
    defer q.Stop()
    
    // 3. Dispatch Job (Instant)
    q.Push("send_email", "user@example.com")
    
    // Keep main thread alive
    select {}
}
```

## API

### `NewMemoryQueue(buffer, workers)`
Creates a new queue.
-   `buffer`: Max jobs in queue before `Push()` blocks.
-   `workers`: Number of concurrent goroutines processing jobs.

### `Push(name, payload)`
Adds a job to the queue. Returns error if queue is full.

### `Register(name, handler)`
Registers a function to handle a specific job name.

### `Start()` / `Stop()`
Manages the lifecycle of the worker pool.
