# Task Scheduler (Cron) ⏰

KVolt includes a robust task scheduler for running recurring jobs (e.g., database backups, email reports, cache cleanup).

## Usage

### 1. Setup

Initialize the scheduler and add jobs.

```go
package main

import (
    "fmt"
    "github.com/go-kvolt/kvolt/pkg/scheduler"
)

func main() {
    s := scheduler.New()

    // Run every 5 seconds
    s.Add("@every 5s", func() {
        fmt.Println("Running cleanup task...")
    })

    // Run every day at Midnight
    s.Add("0 0 * * *", func() {
        fmt.Println("Daily Report...")
    })

    // Start background thread
    s.Start()
    
    // Keep app running
    select {} 
}
```

## Syntax

The scheduler supports standard Cron syntax and shorthand descriptors.

| Entry | Description |
| :--- | :--- |
| `@every <duration>` | Runs at a fixed interval (e.g., `@every 1h30m`). |
| `* * * * *` | Run every minute. |
| `0 0 * * *` | Run daily at midnight. |
| `0 0 1 * *` | Run monthly on the 1st. |
