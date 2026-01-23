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
	val, _, found := e.router.Find(r.Method, r.URL.Path)
	if found {
		if handlers, ok := val.([]context.HandlerFunc); ok {
			c.Handlers = handlers
		} else {
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
