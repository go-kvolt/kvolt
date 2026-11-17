package grpc

import (
	stdContext "context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/internal/logx"
)

const engineAttachKey = "kvolt.grpc.server"

// Attach creates (or returns) a gRPC server bound to the Engine without importing
// gRPC from the core kvolt package.
func Attach(e *kvolt.Engine, opts ...Option) *Server {
	if e == nil {
		panic("grpc: nil engine")
	}
	if existing, ok := FromEngine(e); ok {
		return existing
	}
	s := NewServer(opts...)
	e.Attach(engineAttachKey, s)
	return s
}

// FromEngine returns the gRPC server previously attached to e.
func FromEngine(e *kvolt.Engine) (*Server, bool) {
	if e == nil {
		return nil, false
	}
	v, ok := e.Attachment(engineAttachKey)
	if !ok {
		return nil, false
	}
	s, ok := v.(*Server)
	return s, ok
}

// testRunQuit triggers graceful shutdown from tests (nil in production).
var testRunQuit <-chan struct{}

// signalNotify is signal.Notify; tests may override to capture the quit channel.
var signalNotify = signal.Notify

// Run starts the attached gRPC server with graceful shutdown.
func Run(e *kvolt.Engine, addr string) error {
	if e == nil {
		return fmt.Errorf("grpc: nil engine")
	}
	s, ok := FromEngine(e)
	if !ok || s == nil {
		return fmt.Errorf("grpc: server not attached; call grpc.Attach(engine) first")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe(addr)
	}()

	quit := make(chan os.Signal, 1)
	signalNotify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case sig := <-quit:
		return shutdownRun(e, s, sig)
	case <-testRunQuit:
		logx.Println("\nReceived test quit, shutting down gRPC server...")
		gracefulStopWithTimeout(s, e.ShutdownTimeout())
		logx.Println("gRPC server exited")
		return nil
	case err := <-errCh:
		return err
	}
}

func shutdownRun(e *kvolt.Engine, s *Server, sig os.Signal) error {
	logx.Printf("\nReceived %v, shutting down gRPC server...\n", sig)
	timeout := 5 * time.Second
	if e != nil {
		timeout = e.ShutdownTimeout()
	}
	gracefulStopWithTimeout(s, timeout)
	logx.Println("gRPC server exited")
	return nil
}

func gracefulStopWithTimeout(s *Server, timeout time.Duration) {
	if s == nil {
		return
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-stopped:
	case <-timer.C:
		s.Stop()
	}
}

// RunDual starts HTTP (via Engine) and gRPC concurrently with unified shutdown.
func RunDual(e *kvolt.Engine, httpAddr, grpcAddr string) error {
	if e == nil {
		return fmt.Errorf("grpc: nil engine")
	}
	s, ok := FromEngine(e)
	if !ok || s == nil {
		return fmt.Errorf("grpc: server not attached; call grpc.Attach(engine) first")
	}

	httpSrv := e.HTTPServer(httpAddr)

	logx.Println("⚡ KVolt HTTP  server on " + httpAddr)
	logx.Println("⚡ KVolt gRPC  server on " + grpcAddr)
	logx.Println("Press Ctrl+C to stop")

	errCh := make(chan error, 2)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http: %w", err)
		}
	}()
	go func() {
		if err := s.ListenAndServe(grpcAddr); err != nil {
			errCh <- fmt.Errorf("grpc: %w", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signalNotify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	var startErr error
	select {
	case sig := <-quit:
		logx.Printf("\nReceived %v, shutting down...\n", sig)
	case <-testRunQuit:
		logx.Println("\nReceived test quit, shutting down...")
	case err := <-errCh:
		startErr = err
		logx.Printf("Server error: %v — shutting down...\n", err)
	}

	ctx, cancel := stdContext.WithTimeout(stdContext.Background(), e.ShutdownTimeout())
	defer cancel()

	var shutdownErr error
	if err := httpSrv.Shutdown(ctx); err != nil {
		shutdownErr = fmt.Errorf("http shutdown: %w", err)
	}
	gracefulStopWithTimeout(s, e.ShutdownTimeout())
	logx.Println("All servers exited")
	if startErr != nil {
		return startErr
	}
	return shutdownErr
}
