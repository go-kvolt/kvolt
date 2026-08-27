// Package main demonstrates a KVolt application with both HTTP and gRPC servers.
//
// This example shows how to:
//   - Create a kvolt HTTP app with routes
//   - Initialize a gRPC server with interceptors
//   - Register a gRPC service
//   - Run both servers concurrently with RunAll()
//
// To generate the proto Go code:
//
//	protoc --go_out=. --go-grpc_out=. proto/hello.proto
//
// To run:
//
//	go run main.go
package main

import (
	"context"
	"fmt"

	"github.com/go-kvolt/kvolt/v2"
	kvctx "github.com/go-kvolt/kvolt/v2/context"
	kvgrpc "github.com/go-kvolt/kvolt/v2/grpc"
	"github.com/go-kvolt/kvolt/v2/middleware"

	"google.golang.org/grpc"
)

// ── Greeter Service (In-line, no proto codegen needed for this demo). ──

// greeterServer implements a simple Greeter service.
type greeterServer struct{}

type helloRequest struct {
	Name string
}

type helloReply struct {
	Message string
}

// SayHello is the RPC handler.
func (s *greeterServer) SayHello(ctx context.Context, req *helloRequest) (*helloReply, error) {
	return &helloReply{Message: "Hello, " + req.Name + "!"}, nil
}

// greeterServiceDesc is a minimal gRPC service descriptor for the demo.
// In real usage you'd use the protoc-generated RegisterXxxServer function.
var greeterServiceDesc = grpc.ServiceDesc{
	ServiceName: "hello.Greeter",
	HandlerType: (*greeterServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SayHello",
			Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
				in := new(helloRequest)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return srv.(*greeterServer).SayHello(ctx, in)
				}
				info := &grpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: "/hello.Greeter/SayHello",
				}
				handler := func(ctx context.Context, req interface{}) (interface{}, error) {
					return srv.(*greeterServer).SayHello(ctx, req.(*helloRequest))
				}
				return interceptor(ctx, in, info, handler)
			},
		},
	},
	Streams: []grpc.StreamDesc{},
}

func main() {
	app := kvolt.New()

	// ── HTTP middleware ──
	app.Use(middleware.Logger())
	app.Use(middleware.Recovery())

	// ── HTTP routes ──
	app.GET("/", func(c *kvctx.Context) error {
		return c.JSON(200, map[string]string{
			"message": "Hello from KVolt HTTP!",
		})
	})

	app.GET("/health", func(c *kvctx.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// ── gRPC server ──
	grpcSrv := app.GRPCServer(
		kvgrpc.WithReflection(true),
		kvgrpc.WithHealthCheck(true),
		kvgrpc.WithUnaryInterceptors(
			kvgrpc.LoggingInterceptor(),
			kvgrpc.RecoveryInterceptor(),
		),
		kvgrpc.WithStreamInterceptors(
			kvgrpc.StreamLoggingInterceptor(),
			kvgrpc.StreamRecoveryInterceptor(),
		),
	)

	// Register gRPC service.
	// In real usage: pb.RegisterGreeterServer(grpcSrv.Raw(), &greeterServer{})
	grpcSrv.Raw().RegisterService(&greeterServiceDesc, &greeterServer{})

	// ── Start both HTTP (:8080) and gRPC (:9090) ──
	fmt.Println("Starting KVolt with HTTP + gRPC...")
	if err := app.RunAll(":8080", ":9090"); err != nil {
		fmt.Printf("Fatal: %v\n", err)
	}
}
