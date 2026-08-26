package kvolt

import (
	stdContext "context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-kvolt/kvolt/context"
	kvgrpc "github.com/go-kvolt/kvolt/grpc"
	"github.com/go-kvolt/kvolt/middleware"
	"github.com/go-kvolt/kvolt/router"
)

// Engine is the main framework instance.
type Engine struct {
	*RouterGroup    // Engine is the root group
	router          *router.Router
	pool            sync.Pool
	htmlTemplates   *template.Template
	grpcServer      *kvgrpc.Server
	noRoute         []context.HandlerFunc
	notFoundHandler context.HandlerFunc
	noRouteDirty    bool
}

// New creates a new kvolt Engine with no middleware.
func New() *Engine {
	engine := &Engine{
		router: router.New(),
		notFoundHandler: func(c *context.Context) error {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Not Found"})
		},
		noRouteDirty: true,
	}
	engine.RouterGroup = &RouterGroup{
		engine:     engine,
		middleware: make([]context.HandlerFunc, 0),
	}
	engine.pool.New = func() interface{} {
		return context.New(nil, nil)
	}
	return engine
}

// Default returns an Engine with production middleware: Recovery, RequestID, MaxBodySize (1MB).
// Logger and Gzip are opt-in — they cost CPU on every request.
func Default() *Engine {
	e := New()
	e.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.MaxBodySize(middleware.DefaultMaxBodyBytes),
	)
	return e
}

// NoRoute sets the handler used when no route matches.
func (e *Engine) NoRoute(h context.HandlerFunc) {
	if h != nil {
		e.notFoundHandler = h
	}
	e.noRouteDirty = true
}

func (e *Engine) handlers404() []context.HandlerFunc {
	if e.noRoute == nil || e.noRouteDirty {
		h := make([]context.HandlerFunc, 0, len(e.RouterGroup.middleware)+1)
		h = append(h, e.RouterGroup.middleware...)
		h = append(h, e.notFoundHandler)
		e.noRoute = h
		e.noRouteDirty = false
	}
	return e.noRoute
}

// ServeHTTP implements the http.Handler interface.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := e.pool.Get().(*context.Context)
	c.Reset(w, r)
	if e.htmlTemplates != nil {
		c.Templates = e.htmlTemplates
	}

	val, params, found := e.router.FindInto(r.Method, r.URL.Path, c.Params)
	c.Params = params
	if found {
		if handlers, ok := val.([]context.HandlerFunc); ok {
			c.Handlers = handlers
		} else {
			c.Handlers = e.handlers404()
		}
	} else {
		c.Handlers = e.handlers404()
	}

	if len(c.Handlers) == 1 {
		if err := c.Handlers[0](c); err != nil {
			c.InternalError()
		}
	} else {
		c.Next()
	}
	if !c.HeaderWritten() {
		c.FlushHeaders()
	}
	e.pool.Put(c)
}

// Default server timeouts for production (slowloris protection and connection hygiene).
const (
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
)

// Run starts the HTTP server with Graceful Shutdown and production timeouts.
func (e *Engine) Run(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		ReadTimeout:       DefaultReadTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}

	fmt.Println("⚡ KVolt is running on " + formatListenURL("http", addr))
	fmt.Println("Press Ctrl+C to stop")

	// Non-blocking start
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Listen: %s\n", err)
		}
	}()

	// Wait for signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\nShutting down server...")

	// Context with timeout
	ctx, cancel := stdContext.WithTimeout(stdContext.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println("Server Shutdown Error:", err)
		return err
	}

	fmt.Println("Server exiting")
	return nil
}

// ListenAndServe starts HTTP with net/http defaults (no extra timeouts).
// Use Run() in production for slowloris timeouts and graceful shutdown.
func (e *Engine) ListenAndServe(addr string) error {
	fmt.Println("⚡ KVolt is running on " + formatListenURL("http", addr))
	return http.ListenAndServe(addr, e)
}

// LoadHTMLGlob loads HTML templates from a directory pattern.
func (e *Engine) LoadHTMLGlob(pattern string) {
	e.htmlTemplates = template.Must(template.ParseGlob(pattern))
}

// RunTLS starts the HTTPS server (enabling HTTP/2 by default) with production timeouts.
func (e *Engine) RunTLS(addr, certFile, keyFile string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		ReadTimeout:       DefaultReadTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}

	fmt.Println("⚡ KVolt (HTTPS) is running on " + formatListenURL("https", addr))
	fmt.Println("Press Ctrl+C to stop")

	// Non-blocking start
	go func() {
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Listen: %s\n", err)
		}
	}()

	// Wait for signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\nShutting down server...")

	// Context with timeout
	ctx, cancel := stdContext.WithTimeout(stdContext.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println("Server Shutdown Error:", err)
		return err
	}

	fmt.Println("Server exiting")
	return nil
}

// RouteInfo represents a route metadata.
type RouteInfo struct {
	Method  string
	Path    string
	Summary string
}

// Routes returns a list of registered routes.
func (e *Engine) Routes() []RouteInfo {
	var routes []RouteInfo
	e.router.Walk(func(method, path, desc string) {
		routes = append(routes, RouteInfo{
			Method:  method,
			Path:    path,
			Summary: desc,
		})
	})
	return routes
}

// ─── gRPC ─────────────────────────────────────────────────────────

// GRPCServer returns the Engine's gRPC server, creating it on first call.
// Pass options to configure interceptors, reflection, health checks, etc.
//
//	grpcSrv := app.GRPCServer(
//	    grpc.WithReflection(true),
//	    grpc.WithUnaryInterceptors(grpc.LoggingInterceptor(), grpc.RecoveryInterceptor()),
//	)
//	pb.RegisterMyServiceServer(grpcSrv.Raw(), &myImpl{})
func (e *Engine) GRPCServer(opts ...kvgrpc.Option) *kvgrpc.Server {
	if e.grpcServer == nil {
		e.grpcServer = kvgrpc.NewServer(opts...)
	}
	return e.grpcServer
}

// RunGRPC starts a standalone gRPC server with graceful shutdown.
func (e *Engine) RunGRPC(addr string) error {
	if e.grpcServer == nil {
		return fmt.Errorf("kvolt: gRPC server not initialized; call GRPCServer() first")
	}

	// Start gRPC in background.
	errCh := make(chan error, 1)
	go func() {
		errCh <- e.grpcServer.ListenAndServe(addr)
	}()

	// Wait for signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		fmt.Printf("\nReceived %v, shutting down gRPC server...\n", sig)
		e.grpcServer.GracefulStop()
		fmt.Println("gRPC server exited")
		return nil
	case err := <-errCh:
		return err
	}
}

// RunAll starts both the HTTP server and gRPC server concurrently.
// Both servers share a unified graceful shutdown triggered by SIGINT/SIGTERM.
func (e *Engine) RunAll(httpAddr, grpcAddr string) error {
	if e.grpcServer == nil {
		return fmt.Errorf("kvolt: gRPC server not initialized; call GRPCServer() first")
	}

	httpSrv := &http.Server{
		Addr:              httpAddr,
		Handler:           e,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		ReadTimeout:       DefaultReadTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}

	fmt.Println("⚡ KVolt HTTP  server on " + formatListenURL("http", httpAddr))
	fmt.Println("⚡ KVolt gRPC  server on " + grpcAddr)
	fmt.Println("Press Ctrl+C to stop")

	errCh := make(chan error, 2)

	// Start HTTP.
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http: %w", err)
		}
	}()

	// Start gRPC.
	go func() {
		if err := e.grpcServer.ListenAndServe(grpcAddr); err != nil {
			errCh <- fmt.Errorf("grpc: %w", err)
		}
	}()

	// Wait for signal or fatal error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		fmt.Printf("\nReceived %v, shutting down...\n", sig)
	case err := <-errCh:
		fmt.Printf("Server error: %v — shutting down...\n", err)
	}

	// Graceful shutdown for both.
	ctx, cancel := stdContext.WithTimeout(stdContext.Background(), 5*time.Second)
	defer cancel()

	var shutdownErr error
	if err := httpSrv.Shutdown(ctx); err != nil {
		shutdownErr = fmt.Errorf("http shutdown: %w", err)
	}
	e.grpcServer.GracefulStop()

	fmt.Println("All servers exited")
	return shutdownErr
}

func formatListenURL(scheme, addr string) string {
	if strings.HasPrefix(addr, ":") {
		return scheme + "://localhost" + addr
	}
	return scheme + "://" + addr
}
