# Server-Sent Events (SSE)

KVolt can stream events to the browser with `Context` helpers.

## Usage

```go
app.GET("/events", func(c *context.Context) error {
    if err := c.SSE(); err != nil {
        return err
    }
    for i := 0; i < 5; i++ {
        if err := c.Event("tick", fmt.Sprintf("%d", i)); err != nil {
            return err
        }
        time.Sleep(time.Second)
    }
    return c.SSEvent(context.SSEvent{
        Event: "done",
        Data:  "bye",
        ID:    "5",
    })
})
```

## Helpers

| Method | Description |
| :--- | :--- |
| `SSE()` | Sets `text/event-stream` headers and flushes |
| `Event(event, data)` | Writes a simple event frame |
| `SSEvent(SSEvent)` | Full frame (`id`, `event`, `retry`, `data`) |
| `Flush()` | Flushes when `ResponseWriter` implements `http.Flusher` |

Requires a real server response writer (or a test double that implements `http.Flusher`).
