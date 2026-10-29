# gRPC

gRPC is an **optional** package. Importing `kvolt` / `kvolt.Default()` does **not** pull `google.golang.org/grpc`. Attach a server explicitly:

```go
import (
    "github.com/go-kvolt/kvolt/v2"
    kvctx "github.com/go-kvolt/kvolt/v2/context"
    kvgrpc "github.com/go-kvolt/kvolt/v2/grpc"
)

func main() {
    app := kvolt.Default()

    app.GET("/health", func(c *kvctx.Context) error {
        return c.JSON(200, map[string]string{"status": "ok"})
    })

    grpcSrv := kvgrpc.Attach(app,
        kvgrpc.WithReflection(true),
        kvgrpc.WithHealthCheck(true),
        kvgrpc.WithUnaryInterceptors(
            kvgrpc.LoggingInterceptor(),
            kvgrpc.RecoveryInterceptor(),
        ),
    )

    // pb.RegisterMyServiceServer(grpcSrv.Raw(), &myImpl{})

    // HTTP + gRPC together:
    _ = kvgrpc.RunDual(app, ":8080", ":9090")

    // Or gRPC only:
    // _ = kvgrpc.Run(app, ":9090")
}
```

## Options

```go
kvgrpc.WithReflection(true)
kvgrpc.WithHealthCheck(true)
kvgrpc.WithUnaryInterceptors(kvgrpc.LoggingInterceptor(), kvgrpc.RecoveryInterceptor())
kvgrpc.WithStreamInterceptors(kvgrpc.StreamLoggingInterceptor(), kvgrpc.StreamRecoveryInterceptor())
kvgrpc.AuthInterceptor(func(ctx context.Context, token string) error { ... })
```

## Standalone server

```go
srv := kvgrpc.NewServer(kvgrpc.WithReflection(true))
_ = srv.ListenAndServe(":9090")
```
