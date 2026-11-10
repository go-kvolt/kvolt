package grpc

import (
	stdContext "context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kvolt/kvolt/v2"
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
		return shutdownRun(s, sig)
	case <-testRunQuit:
		fmt.Println("\nReceived test quit, shutting down gRPC server...")
		s.GracefulStop()
		fmt.Println("gRPC server exited")
		return nil
	case err := <-errCh:
		return err
	}
}

func shutdownRun(s *Server, sig os.Signal) error {
	fmt.Printf("\nReceived %v, shutting down gRPC server...\n", sig)
	s.GracefulStop()
	fmt.Println("gRPC server exited")
	return nil
}

// RunDual starts HTTP (via Engine) and gRPC concurrently with unified shutdown.
func RunDual(e *kvolt.Engine, httpAddr, grpcAddr string) error {
	s, ok := FromEngine(e)
	if !ok || s == nil {
		return fmt.Errorf("grpc: server not attached; call grpc.Attach(engine) first")
	}

	httpSrv := e.HTTPServer(httpAddr)

	fmt.Println("⚡ KVolt HTTP  server on " + httpAddr)
	fmt.Println("⚡ KVolt gRPC  server on " + grpcAddr)
	fmt.Println("Press Ctrl+C to stop")

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

	select {
	case sig := <-quit:
		fmt.Printf("\nReceived %v, shutting down...\n", sig)
	case <-testRunQuit:
		fmt.Println("\nReceived test quit, shutting down...")
	case err := <-errCh:
		fmt.Printf("Server error: %v — shutting down...\n", err)
	}

	ctx, cancel := stdContext.WithTimeout(stdContext.Background(), e.ShutdownTimeout())
	defer cancel()

	var shutdownErr error
	if err := httpSrv.Shutdown(ctx); err != nil {
		shutdownErr = fmt.Errorf("http shutdown: %w", err)
	}
	s.GracefulStop()
	fmt.Println("All servers exited")
	return shutdownErr
}
