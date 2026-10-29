# Background Jobs (Queue)

In-memory queue lives in `pkg/queue`. Redis is an **optional** subpackage (`pkg/queue/redis`) so memory-only apps do not pull `go-redis`.

## Memory queue

```go
import "github.com/go-kvolt/kvolt/v2/pkg/queue"

q := queue.NewMemoryQueue(1000, 10)
q.Register("send_email", func(job queue.Job) error {
    email := job.Payload.(string)
    return send(email)
})
q.Start()
defer q.Stop()
_ = q.Push("send_email", "user@example.com") // errors if buffer full
```

### Retries + dead letter (memory)

```go
q := queue.NewMemoryQueue(1000, 10,
    queue.WithDeadLetter(func(job queue.Job, err error) { /* ... */ }),
    queue.WithRetryWait(200*time.Millisecond),
)
q.PushWithOptions("send_email", email, queue.PushOptions{MaxRetries: 3})
```

`MaxRetries` is retries **after** the first failure (3 → up to 4 attempts).

## Redis queue (optional, at-least-once)

Uses `BRPOPLPUSH` ready→processing, an inflight ZSET with **visibility timeout**, and a reaper that reclaims expired jobs if a worker crashes.

```go
import (
    "github.com/go-kvolt/kvolt/v2/pkg/queue"
    kvqredis "github.com/go-kvolt/kvolt/v2/pkg/queue/redis"
    "github.com/redis/go-redis/v9"
)

rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
q := kvqredis.New(rdb, 10,
    kvqredis.WithKeyPrefix("myapp:jobs:"),
    kvqredis.WithVisibilityTimeout(30*time.Second),
    kvqredis.WithDeadLetter(func(job queue.Job, err error) { /* ... */ }),
    kvqredis.WithRetryWait(200*time.Millisecond),
)

q.Register("send_email", func(job queue.Job) error {
    email, _ := job.Payload.(string)
    return send(email)
})
q.Start()
defer q.Stop()
q.PushWithOptions("send_email", "user@example.com", queue.PushOptions{MaxRetries: 3})
```

Handlers should be **idempotent**: a reclaimed job may run again after a crash.

## API

| Package | Constructor | Notes |
| :--- | :--- | :--- |
| `pkg/queue` | `NewMemoryQueue` | In-process; `Push` errors when full |
| `pkg/queue/redis` | `redis.New` | At-least-once + visibility timeout |
