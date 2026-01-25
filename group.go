package kvolt

import (
	"net/http"

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

// Route represents a registered route.
type Route struct {
	Method string
	Path   string
	engine *Engine
}

// Desc adds a description/summary to the route for documentation.
func (r *Route) Desc(summary string) *Route {
	r.engine.router.SetDocumentation(r.Method, r.Path, summary)
	return r
}

// GET adds a GET route to the group.
func (group *RouterGroup) GET(path string, handler context.HandlerFunc) *Route {
	return group.addRoute("GET", path, handler)
}

// POST adds a POST route to the group.
func (group *RouterGroup) POST(path string, handler context.HandlerFunc) *Route {
	return group.addRoute("POST", path, handler)
}

// PUT adds a PUT route to the group.
func (group *RouterGroup) PUT(path string, handler context.HandlerFunc) *Route {
	return group.addRoute("PUT", path, handler)
}

// DELETE adds a DELETE route to the group.
func (group *RouterGroup) DELETE(path string, handler context.HandlerFunc) *Route {
	return group.addRoute("DELETE", path, handler)
}

// Static registers a route to serve static files from the provided root directory.
// relativePath: The path pattern (e.g. "/assets")
// root: The file system root (e.g. "./public")
func (group *RouterGroup) Static(relativePath, root string) *Route {
	// Construct the full prefix path to strip (e.g. /v1/assets)
	absolutePrefix := group.prefix + relativePath

	// Create the file server handler
	fs := http.StripPrefix(absolutePrefix, http.FileServer(http.Dir(root)))

	handler := func(c *context.Context) error {
		fs.ServeHTTP(c.Writer, c.Request)
		return nil
	}

	// Register the route with wildcard suffix
	// e.g. /assets/*filepath
	urlPattern := relativePath + "/*filepath"
	// Also register the exact path (e.g. /assets) to handle root requests
	group.GET(relativePath, handler)
	// Register the trailing slash path if it's different from relativePath
	// e.g. /assets/
	if relativePath != "/" && len(relativePath) > 0 {
		group.GET(relativePath+"/", handler)
	}

	return group.GET(urlPattern, handler)
}

func (group *RouterGroup) addRoute(method, path string, handler context.HandlerFunc) *Route {
	fullPath := group.prefix + path

	// Combine middleware: Group Middleware + Route Handler
	handlers := make([]context.HandlerFunc, 0, len(group.middleware)+1)
	handlers = append(handlers, group.middleware...)
	handlers = append(handlers, handler)

	group.engine.router.AddRoute(method, fullPath, handlers)

	return &Route{
		Method: method,
		Path:   fullPath,
		engine: group.engine,
	}
}
