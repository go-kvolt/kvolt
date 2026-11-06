# OpenTelemetry (optional)

Package `middleware/otel` adds HTTP server spans. It is **not** imported by core middleware — pull it only when you configure an OTel SDK exporter in your app.

```go
import (
    kvotel "github.com/go-kvolt/kvolt/v2/middleware/otel"
    "go.opentelemetry.io/otel"
    // set up TracerProvider / propagator in your main
)

app.Use(kvotel.Middleware(kvotel.Config{
    TracerProvider: otel.GetTracerProvider(),
}))
```

Spans use the route pattern when `RoutePath` is set (KVolt engine does this), and record HTTP status codes.
