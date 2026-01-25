package kvolt

import (
	stdContext "context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-kvolt/kvolt/context"
	"github.com/go-kvolt/kvolt/router"
)

// Engine is the main framework instance.
type Engine struct {
	*RouterGroup // Engine is the root group
	router       *router.Router
	pool         sync.Pool
}

// New creates a new kvolt Engine.
func New() *Engine {
	engine := &Engine{
		router: router.New(),
	}
	engine.RouterGroup = &RouterGroup{
		engine:     engine,
		middleware: make([]context.HandlerFunc, 0),
	}
	// Initialize Sync.Pool
	engine.pool.New = func() interface{} {
		return context.New(nil, nil)
	}
	return engine
}

// ServeHTTP implements the http.Handler interface.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get context from pool
	c := e.pool.Get().(*context.Context)
	c.Reset(w, r)

	// Route matching
	val, params, found := e.router.Find(r.Method, r.URL.Path)
	if found {
		if handlers, ok := val.([]context.HandlerFunc); ok {
			c.Handlers = handlers
			c.Params = params
		} else {
			// This path should ideally not be reached if AddRoute type checks or strict typing is used
			// But for now keeping compatible with current structure where handle is interface{}
			// If handle is NOT []HandlerFunc (e.g. single handler), we might need to wrap it?
			// The current AddRoute in router.go takes `Handler any`.
			// In kvolt.go, we seem to treat it as []context.HandlerFunc?
			// Let's check AddRoute usage in RouterGroup (not visible here but inferred).
			// If matching logic passes, we assume it's correct type.
			c.Handlers = []context.HandlerFunc{}
		}
	} else {
		// 404 Handler - Append to global middleware
		c.Handlers = append(e.RouterGroup.middleware, func(c *context.Context) error {
			return c.Status(404).String(404, "Not Found")
		})
	}

	// Start the chain
	c.Next()

	// Put context back to pool
	e.pool.Put(c)
}

// Run starts the HTTP server with Graceful Shutdown.
func (e *Engine) Run(addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: e,
	}

	fmt.Println("⚡ KVolt is running on http://localhost" + addr)
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

// RunTLS starts the HTTPS server (enabling HTTP/2 by default).
func (e *Engine) RunTLS(addr, certFile, keyFile string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: e,
	}

	fmt.Println("⚡ KVolt (HTTPS) is running on https://localhost" + addr)
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
