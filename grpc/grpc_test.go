package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestNewServer(t *testing.T) {
	srv := NewServer()
	if srv == nil {
		t.Fatal("NewServer: want non-nil")
	}
	if srv.Raw() == nil {
		t.Fatal("NewServer: Raw() want non-nil *grpc.Server")
	}
}

func TestNewServerWithOptions(t *testing.T) {
	srv := NewServer(
		WithReflection(true),
		WithHealthCheck(true),
		WithUnaryInterceptors(LoggingInterceptor(), RecoveryInterceptor()),
		WithStreamInterceptors(StreamLoggingInterceptor(), StreamRecoveryInterceptor()),
	)
	if srv == nil {
		t.Fatal("NewServer with options: want non-nil")
	}
	if srv.HealthServer() == nil {
		t.Fatal("HealthServer should be non-nil when health check enabled")
	}
}

func TestNewServerHealthDisabled(t *testing.T) {
	srv := NewServer()
	if srv.HealthServer() != nil {
		t.Fatal("HealthServer should be nil when health check not enabled")
	}
}

func TestRecoveryInterceptor(t *testing.T) {
	interceptor := RecoveryInterceptor()

	// Handler that panics.
	panicHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("test panic")
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test/Panic"}
	resp, err := interceptor(context.Background(), nil, info, panicHandler)
	if resp != nil {
		t.Errorf("RecoveryInterceptor: want nil resp, got %v", resp)
	}
	if err == nil {
		t.Fatal("RecoveryInterceptor: want error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("RecoveryInterceptor: want gRPC status error, got %v", err)
	}
	if st.Code() != codes.Internal {
		t.Errorf("RecoveryInterceptor: want codes.Internal, got %v", st.Code())
	}
}

func TestLoggingInterceptor(t *testing.T) {
	interceptor := LoggingInterceptor()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test/Log"}
	resp, err := interceptor(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("LoggingInterceptor: unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("LoggingInterceptor: want 'ok', got %v", resp)
	}
}

func TestAuthInterceptor_MissingMetadata(t *testing.T) {
	interceptor := AuthInterceptor(func(ctx context.Context, token string) error {
		return nil
	})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test/Auth"}
	_, err := interceptor(context.Background(), nil, info, handler)
	if err == nil {
		t.Fatal("AuthInterceptor: want error for missing metadata")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("AuthInterceptor: want gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("AuthInterceptor: want codes.Unauthenticated, got %v", st.Code())
	}
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	interceptor := AuthInterceptor(func(ctx context.Context, token string) error {
		if token != "valid-token" {
			return fmt.Errorf("bad token")
		}
		return nil
	})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "authenticated", nil
	}

	// Add metadata with authorization.
	md := metadata.Pairs("authorization", "Bearer valid-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	info := &grpc.UnaryServerInfo{FullMethod: "/test/Auth"}
	resp, err := interceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("AuthInterceptor: unexpected error: %v", err)
	}
	if resp != "authenticated" {
		t.Errorf("AuthInterceptor: want 'authenticated', got %v", resp)
	}
}

func TestAuthInterceptor_InvalidToken(t *testing.T) {
	interceptor := AuthInterceptor(func(ctx context.Context, token string) error {
		return fmt.Errorf("invalid")
	})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	md := metadata.Pairs("authorization", "Bearer bad-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	info := &grpc.UnaryServerInfo{FullMethod: "/test/Auth"}
	_, err := interceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("AuthInterceptor: want error for invalid token")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("AuthInterceptor: want codes.Unauthenticated, got %v", st.Code())
	}
	if st.Message() != "unauthenticated" {
		t.Fatalf("want generic message (no validator leak), got %q", st.Message())
	}
}

func TestServerListenAndServe(t *testing.T) {
	srv := NewServer(
		WithUnaryInterceptors(LoggingInterceptor()),
	)

	// Find a free port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := lis.Addr().String()
	lis.Close()

	// Start server in background.
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(addr)
	}()

	// Give it time to start.
	time.Sleep(100 * time.Millisecond)

	// Try to connect.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	conn.Close()

	// Stop server.
	srv.Stop()

	select {
	case err := <-errCh:
		// grpc.Server.Serve returns nil on Stop.
		if err != nil {
			t.Logf("serve returned: %v (acceptable)", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop in time")
	}
}

func TestGracefulStop(t *testing.T) {
	srv := NewServer()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	go func() {
		_ = srv.Serve(lis)
	}()

	time.Sleep(100 * time.Millisecond)

	// GracefulStop should not hang.
	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		// Success.
	case <-time.After(5 * time.Second):
		t.Fatal("GracefulStop did not complete in time")
	}
}

func TestRun_NilEngine(t *testing.T) {
	if err := Run(nil, ":0"); err == nil {
		t.Fatal("Run(nil) want error, got nil")
	}
	if err := RunDual(nil, ":0", ":0"); err == nil {
		t.Fatal("RunDual(nil) want error, got nil")
	}
}

func TestInterceptors_NilSafety(t *testing.T) {
	logInt := LoggingInterceptor()
	_, _ = logInt(context.Background(), "req", nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})

	recInt := RecoveryInterceptor()
	_, _ = recInt(context.Background(), "req", nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("boom")
	})

	authNil := AuthInterceptor(nil)
	resp, _ := authNil(context.Background(), "req", nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})
	if resp != "ok" {
		t.Fatalf("auth nil validate want ok, got %v", resp)
	}
}

func TestAuthInterceptor_Panic(t *testing.T) {
	interceptor := AuthInterceptor(func(ctx context.Context, token string) error {
		panic("auth validate panic")
	})

	md := metadata.Pairs("authorization", "Bearer some-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/test/PanicAuth"}

	resp, err := interceptor(ctx, nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "should not be reached", nil
	})
	if resp != nil {
		t.Errorf("want nil resp on auth panic, got %v", resp)
	}
	if err == nil {
		t.Fatal("want error on auth panic, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Errorf("want codes.Unauthenticated, got %v", err)
	}
}

func TestStreamAuthInterceptor_Panic(t *testing.T) {
	interceptor := StreamAuthInterceptor(func(ctx context.Context, token string) error {
		panic("stream auth panic")
	})

	md := metadata.Pairs("authorization", "Bearer some-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.StreamServerInfo{FullMethod: "/test/PanicStreamAuth"}

	err := interceptor(nil, &mockServerStream{ctx: ctx}, info, func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	})
	if err == nil {
		t.Fatal("want error on stream auth panic, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Errorf("want codes.Unauthenticated, got %v", err)
	}
}

func TestServer_NilGuards(t *testing.T) {
	var s *Server
	if s.Raw() != nil {
		t.Fatal("expected nil Raw on nil server")
	}
	if s.HealthServer() != nil {
		t.Fatal("expected nil HealthServer on nil server")
	}
	if err := s.Serve(nil); err == nil {
		t.Fatal("expected error on nil server Serve")
	}
	if err := s.ListenAndServe(":0"); err == nil {
		t.Fatal("expected error on nil server ListenAndServe")
	}
	s.Stop()
	s.GracefulStop()

	// Server with nil listener
	validS := NewServer()
	if err := validS.Serve(nil); err == nil {
		t.Fatal("expected error on valid server with nil listener")
	}
}
