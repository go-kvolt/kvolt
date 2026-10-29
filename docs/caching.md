# Caching system

KVolt includes a sharded in-memory cache. Redis is an **optional** subpackage so memory-only apps do not pull `go-redis`.

## Memory store

```go
import (
    "time"
    "github.com/go-kvolt/kvolt/v2/pkg/cache"
)

c := cache.NewMemoryStore(5 * time.Minute)
c.Set("user_123", map[string]string{"name": "Admin"}, time.Hour)
val, err := c.Get("user_123")
```

## Redis store (optional)

```go
import (
    "time"
    "github.com/go-kvolt/kvolt/v2/pkg/cache"
    cacheredis "github.com/go-kvolt/kvolt/v2/pkg/cache/redis"
    "github.com/redis/go-redis/v9"
)

rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
store := cacheredis.New(rdb, cacheredis.WithPrefix("myapp:cache:"))

store.Set("user_123", map[string]string{"name": "Admin"}, time.Hour)
val, err := store.Get("user_123")

// Typed decode after Redis JSON round-trip:
var user map[string]string
_ = cache.GetInto(store, "user_123", &user)
```

`Flush` requires a key prefix so it cannot wipe an entire Redis database.

## Cache-aside pattern

```go
app.GET("/users/:id", func(c *context.Context) error {
    id := c.Param("id")
    cacheKey := "user:" + id
    if val, err := myCache.Get(cacheKey); err == nil {
        return c.JSON(200, val)
    }
    user := db.Find(id)
    myCache.Set(cacheKey, user, 10*time.Minute)
    return c.JSON(200, user)
})
```
