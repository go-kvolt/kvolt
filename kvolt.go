package kvolt

import (
	stdContext "context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
	"github.com/go-kvolt/kvolt/v2/router"
)

// Engine is the main framework instance.
type Engine struct {
	*RouterGroup    // Engine is the root group
	router          *router.Router
	pool            sync.Pool
	htmlTemplates   *template.Template
	attachments     sync.Map // optional features (e.g. gRPC) without hard imports
	noRouteMu       sync.Mutex
	noRoute         []context.HandlerFunc
	notFoundHandler context.HandlerFunc
	noMethodHandler context.HandlerFunc
	noRouteDirty    bool

	// RedirectTrailingSlash redirects when a trailing slash would match a route.
	RedirectTrailingSlash bool
	// HandleMethodNotAllowed responds 405 when the path exists for other methods.
	HandleMethodNotAllowed bool
	// TrustedProxies are CIDRs/IPs trusted to set X-Forwarded-For / X-Real-IP.
	TrustedProxies []string
	// Server holds timeouts applied by Run / RunTLS / RunWithServer.
	Server ServerConfig
}

// ServerConfig configures http.Server timeouts used by Run helpers.
type ServerConfig struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration // 0 disables write deadline (SSE / WebSockets)
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
}

// New creates a new kvolt Engine with no middleware.
func New() *Engine {
	engine := &Engine{
		router: router.New(),
		notFoundHandler: func(c *context.Context) error {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Not Found"})
		},
		noMethodHandler: func(c *context.Context) error {
			return c.JSON(http.StatusMethodNotAllowed, map[string]string{"error": "Method Not Allowed"})
		},
		RedirectTrailingSlash:  true,
		HandleMethodNotAllowed: true,
		Server: ServerConfig{
			ReadHeaderTimeout: DefaultReadHeaderTimeout,
			ReadTimeout:       DefaultReadTimeout,
			WriteTimeout:      DefaultWriteTimeout,
			IdleTimeout:       DefaultIdleTimeout,
			MaxHeaderBytes:    1 << 20,
			ShutdownTimeout:   5 * time.Second,
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

// NoMethod sets the handler used when the path matches another method (405).
func (e *Engine) NoMethod(h context.HandlerFunc) {
	if h != nil {
		e.noMethodHandler = h
	}
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
	e.noRouteMu.Lock()
	defer e.noRouteMu.Unlock()
	if h != nil {
		e.notFoundHandler = h
	}
	e.noRouteDirty = true
}

func (e *Engine) handlers404() []context.HandlerFunc {
	e.noRouteMu.Lock()
	defer e.noRouteMu.Unlock()
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
	c.TrustedProxies = e.TrustedProxies
	if e.htmlTemplates != nil {
		c.Templates = e.htmlTemplates
	}

	res := e.router.FindEx(r.Method, r.URL.Path, c.Params)
	c.Params = res.Params
	switch {
	case res.Found:
		if handlers, ok := res.Handler.([]context.HandlerFunc); ok {
			c.Handlers = handlers
		} else {
			c.Handlers = e.handlers404()
		}
	case e.RedirectTrailingSlash && res.TSR && r.Method != http.MethodConnect:
		code := http.StatusMovedPermanently
		if r.Method != http.MethodGet {
			code = http.StatusPermanentRedirect
		}
		c.Handlers = []context.HandlerFunc{func(c *context.Context) error {
			return c.Redirect(code, alternatePath(r.URL.Path))
		}}
	case e.HandleMethodNotAllowed:
		if allowed := e.router.AllowedMethods(r.URL.Path); len(allowed) > 0 {
			if !slices.Contains(allowed, http.MethodOptions) {
				allowed = append(allowed, http.MethodOptions)
				slices.Sort(allowed)
			}
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			c.Handlers = append(append([]context.HandlerFunc{}, e.RouterGroup.middleware...), e.noMethodHandler)
		} else {
			c.Handlers = e.handlers404()
		}
	default:
		c.Handlers = e.handlers404()
	}

	// Metrics label: registered pattern (/users/:id), not raw URL (avoids cardinality explosion).
	if res.Found && res.Pattern != "" {
		c.RoutePath = res.Pattern
	} else if res.Found {
		c.RoutePath = r.URL.Path
	} else {
		c.RoutePath = "unmatched"
	}

	if len(c.Handlers) == 1 {
		if err := c.Handlers[0](c); err != nil {
			c.WriteHandlerError(err)
		}
	} else {
		c.Next()
	}
	if !c.HeaderWritten() {
		c.FlushHeaders()
	}
	if !c.IsDetached() {
		e.pool.Put(c)
	}
}

func alternatePath(path string) string {
	if path == "" || path == "/" {
		return path
	}
	if path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path + "/"
}

// Attach stores an optional feature object (e.g. gRPC server) without a hard dependency.
func (e *Engine) Attach(key string, v any) {
	e.attachments.Store(key, v)
}

// Attachment returns a value previously stored with Attach.
func (e *Engine) Attachment(key string) (any, bool) {
	return e.attachments.Load(key)
}

// HTTPServer builds an http.Server with Engine.Server timeouts (used by Run and grpc.RunDual).
func (e *Engine) HTTPServer(addr string) *http.Server {
	return e.httpServer(addr)
}

// ShutdownTimeout returns the configured graceful shutdown timeout.
func (e *Engine) ShutdownTimeout() time.Duration {
	return e.shutdownTimeout()
}

func (e *Engine) httpServer(addr string) *http.Server {
	cfg := e.Server
	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = DefaultReadTimeout
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	// WriteTimeout 0 is intentional (SSE/WS). Negative is invalid — treat as default.
	write := cfg.WriteTimeout
	if write < 0 {
		write = DefaultWriteTimeout
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      write,
		IdleTimeout:       cfg.IdleTimeout,
	}
	if cfg.MaxHeaderBytes > 0 {
		srv.MaxHeaderBytes = cfg.MaxHeaderBytes
	}
	return srv
}

func (e *Engine) shutdownTimeout() time.Duration {
	if e.Server.ShutdownTimeout > 0 {
		return e.Server.ShutdownTimeout
	}
	return 5 * time.Second
}

// IsTrustedProxy reports whether remoteAddr's host is in TrustedProxies.
func (e *Engine) IsTrustedProxy(remoteAddr string) bool {
	return isTrustedProxy(remoteAddr, e.TrustedProxies)
}

// Default server timeouts for production (slowloris protection and connection hygiene).
const (
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
)

// Run starts the HTTP server with Graceful Shutdown and production timeouts.
// It returns immediately if the address cannot be bound (e.g. port in use).
func (e *Engine) Run(addr string) error {
	return e.serveAndWait(e.httpServer(addr), "", "")
}

// RunWithServer runs a preconfigured http.Server (Handler is set to the Engine).
// Binds srv.Addr before printing "running"; returns bind errors to the caller.
func (e *Engine) RunWithServer(srv *http.Server) error {
	if srv == nil {
		return fmt.Errorf("kvolt: nil server")
	}
	srv.Handler = e
	return e.serveAndWait(srv, "", "")
}

// testServeQuit, when closed/sent, triggers graceful shutdown (tests only; nil in production).
var testServeQuit <-chan struct{}

// serveAndWait binds first, then serves. certFile/keyFile non-empty enables TLS via ServeTLS.
func (e *Engine) serveAndWait(srv *http.Server, certFile, keyFile string) error {
	if srv.Addr == "" {
		srv.Addr = ":http"
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", srv.Addr, err)
	}

	scheme := "http"
	if certFile != "" && keyFile != "" {
		scheme = "https"
	}
	fmt.Println("⚡ KVolt is running on " + formatListenURL(scheme, srv.Addr))
	fmt.Println("Press Ctrl+C to stop")

	errCh := make(chan error, 1)
	go func() {
		var serveErr error
		if certFile != "" && keyFile != "" {
			serveErr = srv.ServeTLS(ln, certFile, keyFile)
		} else {
			serveErr = srv.Serve(ln)
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
			return
		}
		errCh <- nil
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case serveErr := <-errCh:
		if serveErr != nil {
			return serveErr
		}
		return nil
	case <-quit:
	case <-testServeQuit:
	}

	fmt.Println("\nShutting down server...")
	ctx, cancel := stdContext.WithTimeout(stdContext.Background(), e.shutdownTimeout())
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println("Server Shutdown Error:", err)
		return err
	}
	fmt.Println("Server exiting")
	return nil
}

// testListenHook, when set, receives the bound listener (tests close it to unblock Serve).
var testListenHook func(net.Listener)

// ListenAndServe starts HTTP with net/http defaults (no extra timeouts).
// Use Run() in production for slowloris timeouts and graceful shutdown.
func (e *Engine) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	if testListenHook != nil {
		testListenHook(ln)
	}
	fmt.Println("⚡ KVolt is running on " + formatListenURL("http", addr))
	// #nosec G114 -- ListenAndServe provides bare net/http behavior; use Run() for production timeouts
	return http.Serve(ln, e)
}

// LoadHTMLGlob loads HTML templates from a directory pattern.
func (e *Engine) LoadHTMLGlob(pattern string) {
	e.htmlTemplates = template.Must(template.ParseGlob(pattern))
}

// RunTLS starts the HTTPS server (enabling HTTP/2 by default) with production timeouts.
func (e *Engine) RunTLS(addr, certFile, keyFile string) error {
	return e.serveAndWait(e.httpServer(addr), certFile, keyFile)
}

// RouteInfo represents a route metadata.
type RouteInfo struct {
	Method    string
	Path      string
	Summary   string
	Tags      []string
	Request   interface{}
	Responses map[int]interface{}
	Security  []string
}

// Routes returns a list of registered routes.
func (e *Engine) Routes() []RouteInfo {
	var routes []RouteInfo
	e.router.Walk(func(method, path string, doc router.RouteDoc) {
		routes = append(routes, RouteInfo{
			Method:    method,
			Path:      path,
			Summary:   doc.Summary,
			Tags:      doc.Tags,
			Request:   doc.Request,
			Responses: doc.Responses,
			Security:  doc.Security,
		})
	})
	return routes
}

func formatListenURL(scheme, addr string) string {
	if strings.HasPrefix(addr, ":") {
		return scheme + "://localhost" + addr
	}
	return scheme + "://" + addr
}
