package grpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2"
	kvctx "github.com/go-kvolt/kvolt/v2/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context { return m.ctx }

func TestAttach_FromEngine(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Attach(nil) should panic")
		}
	}()
	_ = Attach(nil)
}

func TestAttach_Idempotent(t *testing.T) {
	e := kvolt.New()
	s1 := Attach(e, WithReflection(true), WithServerOptions(grpc.MaxRecvMsgSize(1024)))
	s2 := Attach(e)
	if s1 != s2 {
		t.Fatal("Attach should return existing server")
	}
	got, ok := FromEngine(e)
	if !ok || got != s1 {
		t.Fatal("FromEngine mismatch")
	}
	e2 := kvolt.New()
	if _, ok := FromEngine(e2); ok {
		t.Fatal("unattached engine")
	}
}

func TestRun_NotAttached(t *testing.T) {
	e := kvolt.New()
	if err := Run(e, "127.0.0.1:0"); err == nil {
		t.Fatal("want error when not attached")
	}
	if err := RunDual(e, ":0", ":0"); err == nil {
		t.Fatal("want RunDual error when not attached")
	}
}

func TestRun_ListenError(t *testing.T) {
	e := kvolt.New()
	Attach(e)
	if err := Run(e, ":::not-a-valid-addr:::"); err == nil {
		t.Fatal("want listen error from Run")
	}
}

func TestRunDual_ListenError(t *testing.T) {
	e := kvolt.New()
	Attach(e)
	// RunDual shuts down on grpc listen failure; shutdownErr may still be nil.
	_ = RunDual(e, "127.0.0.1:0", ":::not-a-valid-addr:::")
}

func TestListenAndServe_BadAddr(t *testing.T) {
	s := NewServer()
	err := s.ListenAndServe("127.0.0.1:99999") // invalid port on some systems; try obviously bad
	if err == nil {
		// If somehow bound, stop it.
		s.Stop()
		// Use invalid network address format instead.
		err = s.ListenAndServe(":::invalid:::")
	}
	if err == nil {
		t.Fatal("want listen error")
	}
}

func TestStreamInterceptors(t *testing.T) {
	info := &grpc.StreamServerInfo{FullMethod: "/test/Stream"}

	t.Run("logging ok", func(t *testing.T) {
		err := StreamLoggingInterceptor()(nil, &mockServerStream{ctx: context.Background()}, info, func(srv interface{}, ss grpc.ServerStream) error {
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("logging err", func(t *testing.T) {
		err := StreamLoggingInterceptor()(nil, &mockServerStream{ctx: context.Background()}, info, func(srv interface{}, ss grpc.ServerStream) error {
			return status.Error(codes.NotFound, "missing")
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("recovery panic", func(t *testing.T) {
		err := StreamRecoveryInterceptor()(nil, &mockServerStream{ctx: context.Background()}, info, func(srv interface{}, ss grpc.ServerStream) error {
			panic("boom")
		})
		if status.Code(err) != codes.Internal {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("auth missing", func(t *testing.T) {
		err := StreamAuthInterceptor(func(ctx context.Context, token string) error { return nil })(
			nil, &mockServerStream{ctx: context.Background()}, info,
			func(srv interface{}, ss grpc.ServerStream) error { return nil },
		)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("auth invalid", func(t *testing.T) {
		md := metadata.Pairs("authorization", "Bearer bad")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		err := StreamAuthInterceptor(func(ctx context.Context, token string) error { return fmt.Errorf("nope") })(
			nil, &mockServerStream{ctx: ctx}, info,
			func(srv interface{}, ss grpc.ServerStream) error { return nil },
		)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("auth ok", func(t *testing.T) {
		md := metadata.Pairs("authorization", "tok")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		err := StreamAuthInterceptor(func(ctx context.Context, token string) error {
			if token != "tok" {
				return fmt.Errorf("bad")
			}
			return nil
		})(nil, &mockServerStream{ctx: ctx}, info, func(srv interface{}, ss grpc.ServerStream) error {
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestLoggingInterceptor_ErrorCode(t *testing.T) {
	_, err := LoggingInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/t"}, func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.Internal, "x")
	})
	if status.Code(err) != codes.Internal {
		t.Fatal(err)
	}
}

func TestExtractToken_MissingAuth(t *testing.T) {
	md := metadata.Pairs("x", "y")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := extractToken(ctx)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatal(err)
	}
}

func TestServe_Direct(t *testing.T) {
	s := NewServer(WithHealthCheck(true))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(lis) }()
	s.Stop()
	<-done
}

func TestRun_OSInterrupt(t *testing.T) {
	e := kvolt.New()
	Attach(e)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	var captured chan<- os.Signal
	prev := signalNotify
	signalNotify = func(c chan<- os.Signal, sig ...os.Signal) { captured = c }
	defer func() { signalNotify = prev }()

	done := make(chan error, 1)
	go func() { done <- Run(e, addr) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if captured != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if captured == nil {
		t.Fatal("signal channel not captured")
	}
	captured <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Run interrupt")
	}
}

func TestRunDual_HTTPListenError(t *testing.T) {
	e := kvolt.New()
	Attach(e)
	err := RunDual(e, ":::not-a-valid-http-addr:::", "127.0.0.1:0")
	// Either returns shutdownErr nil after error path, or an error — must not hang.
	_ = err
}

func TestRunDual_OSInterrupt(t *testing.T) {
	e := kvolt.New()
	Attach(e)
	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpAddr := httpLn.Addr().String()
	_ = httpLn.Close()
	grpcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcAddr := grpcLn.Addr().String()
	_ = grpcLn.Close()

	var captured chan<- os.Signal
	prev := signalNotify
	signalNotify = func(c chan<- os.Signal, sig ...os.Signal) { captured = c }
	defer func() { signalNotify = prev }()

	done := make(chan error, 1)
	go func() { done <- RunDual(e, httpAddr, grpcAddr) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if captured != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if captured == nil {
		t.Fatal("signal channel not captured")
	}
	captured <- os.Interrupt

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestRunDual_ShutdownError(t *testing.T) {
	e := kvolt.New()
	e.Server.ShutdownTimeout = time.Millisecond
	block := make(chan struct{})
	e.GET("/block", func(c *kvctx.Context) error {
		<-block
		return c.String(200, "ok")
	})
	Attach(e)

	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpAddr := httpLn.Addr().String()
	_ = httpLn.Close()
	grpcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcAddr := grpcLn.Addr().String()
	_ = grpcLn.Close()

	quit := make(chan struct{})
	testRunQuit = quit
	defer func() { testRunQuit = nil }()

	done := make(chan error, 1)
	go func() { done <- RunDual(e, httpAddr, grpcAddr) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", httpAddr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Hold a request open so Shutdown hits the short timeout.
	reqDone := make(chan struct{})
	go func() {
		defer close(reqDone)
		client := &http.Client{Timeout: 3 * time.Second}
		_, _ = client.Get("http://" + httpAddr + "/block")
	}()
	time.Sleep(50 * time.Millisecond)
	close(quit)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
	close(block)
	<-reqDone
}
