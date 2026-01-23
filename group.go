package kvolt

import (
	"github.com/go-kvolt/kvolt/context"
)

// RouterGroup is a wrapper to group routes with a common prefix and middleware.
type RouterGroup struct {
	prefix     string
	middleware []context.HandlerFunc
	engine     *Engine // Recursive ref to register final route
}

// Group creates a new child group.
func (group *RouterGroup) Group(prefix string) *RouterGroup {
	return &RouterGroup{
		prefix:     group.prefix + prefix,
		middleware: group.middleware, // Inherit parent middleware
		engine:     group.engine,
	}
}

// Use adds middleware to the group.
func (group *RouterGroup) Use(h ...context.HandlerFunc) {
	group.middleware = append(group.middleware, h...)
}

// GET adds a GET route to the group.
func (group *RouterGroup) GET(path string, handler context.HandlerFunc) {
	group.addRoute("GET", path, handler)
}

// POST adds a POST route to the group.
func (group *RouterGroup) POST(path string, handler context.HandlerFunc) {
	group.addRoute("POST", path, handler)
}

func (group *RouterGroup) addRoute(method, path string, handler context.HandlerFunc) {
	fullPath := group.prefix + path

	// Combine middleware: Group Middleware + Route Handler
	// We do NOT modify the handler itself, we register a closure that runs chain?
	// OR we register the middleware in the router?
	// Efficient way: Native middleware support in Engine's ServeHTTP loop logic?
	// Previous Engine logic copied GLOBAL middleware.
	// Now we need Group logic.
	//
	// Optimization: Merge middlewares into a single slice for this specific route at registration time?
	// Yes. (group.middleware + handler)

	// Create a chain for this route
	handlers := make([]context.HandlerFunc, 0, len(group.middleware)+1)
	handlers = append(handlers, group.middleware...)
	handlers = append(handlers, handler)

	// We need to tell the Engine's Router to store THIS chain.
	// BUT the Router currently stores `Handler` (interface/Handle).
	// If we store []HandlerFunc in the router, Engine.ServeHTTP needs to handle that.

	group.engine.router.AddRoute(method, fullPath, handlers)
}
