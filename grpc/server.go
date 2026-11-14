package grpc

import (
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Server wraps google.golang.org/grpc.Server with KVolt-style configuration.
type Server struct {
	srv        *grpc.Server
	mu         sync.Mutex
	listener   net.Listener
	reflection bool
	health     bool
	healthSrv  *health.Server
}

// NewServer creates a new gRPC server with functional options.
//
//	grpcSrv := grpc.NewServer(
//	    grpc.WithReflection(true),
//	    grpc.WithUnaryInterceptors(grpc.LoggingInterceptor(), grpc.RecoveryInterceptor()),
//	)
func NewServer(opts ...Option) *Server {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}

	// Build native gRPC server options: chain interceptors then append user options.
	var serverOpts []grpc.ServerOption
	if len(cfg.unaryInterceptors) > 0 {
		serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(cfg.unaryInterceptors...))
	}
	if len(cfg.streamInterceptors) > 0 {
		serverOpts = append(serverOpts, grpc.ChainStreamInterceptor(cfg.streamInterceptors...))
	}
	serverOpts = append(serverOpts, cfg.serverOptions...)

	s := &Server{
		srv:        grpc.NewServer(serverOpts...),
		reflection: cfg.reflection,
		health:     cfg.health,
	}

	// Register reflection service.
	if s.reflection {
		reflection.Register(s.srv)
	}

	// Register health service.
	if s.health {
		s.healthSrv = health.NewServer()
		healthpb.RegisterHealthServer(s.srv, s.healthSrv)
	}

	return s
}

// Raw returns the underlying *grpc.Server for direct service registration
// using protoc-generated RegisterXxxServer functions.
//
//	pb.RegisterGreeterServer(grpcSrv.Raw(), &myService{})
func (s *Server) Raw() *grpc.Server {
	return s.srv
}

// HealthServer returns the health server instance (nil if health check is disabled).
// Use this to update serving status of individual services.
//
//	grpcSrv.HealthServer().SetServingStatus("myService", healthpb.HealthCheckResponse_SERVING)
func (s *Server) HealthServer() *health.Server {
	return s.healthSrv
}

// Serve starts the gRPC server on the given listener.
// This is a blocking call.
func (s *Server) Serve(lis net.Listener) error {
	s.mu.Lock()
	s.listener = lis
	s.mu.Unlock()

	fmt.Printf("⚡ KVolt gRPC server listening on %s\n", lis.Addr().String())
	return s.srv.Serve(lis)
}

// ListenAndServe creates a TCP listener on addr and starts the gRPC server.
// This is a blocking call.
func (s *Server) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpc: failed to listen on %s: %w", addr, err)
	}
	return s.Serve(lis)
}

// GracefulStop gracefully stops the gRPC server.
// It stops accepting new connections and RPCs, and blocks until all pending RPCs finish.
func (s *Server) GracefulStop() {
	if s == nil || s.srv == nil {
		return
	}
	s.srv.GracefulStop()
}

// Stop immediately stops the gRPC server.
func (s *Server) Stop() {
	if s == nil || s.srv == nil {
		return
	}
	s.srv.Stop()
}
