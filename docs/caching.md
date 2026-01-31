# Caching system ⚡

KVolt includes a "blazing fast" sharded in-memory cache system. It allows you to store and retrieve data with nanosecond-level latency, significantly reducing database load.

## Features

-   **Sharded Map**: Uses 64 shards to minimize lock contention, allowing high concurrency.
-   **TTL Support**: Automatic expiration of keys.
-   **Janitor Cleanups**: Background goroutine handles clearing expired items.
-   **Simple Interface**: Clean API for `Get`, `Set`, `Delete`, and `Flush`.

## Usage

### 1. Simple Setup

Initialize the cache and use it globally.

```go
package main

import (
    "time"
    "fmt"
    "github.com/go-kvolt/kvolt/pkg/cache"
)

func main() {
    // 1. Create a Memory Store (Cleanup every 5 minutes)
    c := cache.NewMemoryStore(5 * time.Minute)

    // 2. Set a value with 1 hour expiration
    c.Set("user_123", map[string]string{"name": "Admin"}, 1 * time.Hour)

    // 3. Get the value
    val, err := c.Get("user_123")
    if err == nil {
        user := val.(map[string]string)
        fmt.Println("User Name:", user["name"])
    }
}
```

### 2. Cache-Aside Pattern (Real World)

This is the most common way to use caching in an API.

```go
app.GET("/users/:id", func(c *context.Context) error {
    id := c.Params.Get("id")
    cacheKey := "user:" + id

    // 1. Try to get from cache
    if val, err := myCache.Get(cacheKey); err == nil {
        return c.JSON(200, val)
    }

    // 2. If not in cache, get from Database
    user := db.Find(id) 

    // 3. Store in cache for 10 minutes
    myCache.Set(cacheKey, user, 10 * time.Minute)

    return c.JSON(200, user)
})
```

## Performance

KVolt's sharded cache is designed for extreme throughput. By splitting the map into 64 shards, multiple CPU cores can access different parts of the cache at the same time without waiting for a single lock.

| Metric | Result |
| :--- | :--- |
| **Set Latency** | ~40-60 nanoseconds |
| **Get Latency** | ~30-50 nanoseconds |
| **Throughput** | 10,000,000+ ops/sec |
