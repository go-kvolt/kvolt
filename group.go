package kvolt

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/router"
)

// onlyFilesFS wraps http.FileSystem to disable directory listings and block dotfiles.
type onlyFilesFS struct {
	fs http.FileSystem
}

func (o onlyFilesFS) Open(name string) (http.File, error) {
	cleaned := path.Clean("/" + name)
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == "" || seg == "." {
			continue
		}
		if strings.HasPrefix(seg, ".") {
			return nil, os.ErrNotExist
		}
	}
	f, err := o.fs.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if st.IsDir() {
		// Allow only if index.html exists; otherwise no directory listing.
		if _, err := o.fs.Open(strings.TrimSuffix(name, "/") + "/index.html"); err != nil {
			_ = f.Close()
			return nil, fs.ErrNotExist
		}
	}
	return f, nil
}

// RouterGroup is a wrapper to group routes with a common prefix and middleware.
type RouterGroup struct {
	prefix     string
	middleware []context.HandlerFunc
	engine     *Engine // Recursive ref to register final route
}

// Group creates a new child group.
func (group *RouterGroup) Group(prefix string) *RouterGroup {
	md := append([]context.HandlerFunc(nil), group.middleware...)
	return &RouterGroup{
		prefix:     group.prefix + prefix,
		middleware: md,
		engine:     group.engine,
	}
}

// Use adds middleware to the group.
func (group *RouterGroup) Use(h ...context.HandlerFunc) {
	group.middleware = append(group.middleware, h...)
	if group.engine != nil && group.engine.RouterGroup == group {
		group.engine.noRouteDirty = true
	}
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

// Tags sets OpenAPI tags for the route.
func (r *Route) Tags(tags ...string) *Route {
	r.engine.router.UpdateDoc(r.Method, r.Path, func(doc *router.RouteDoc) {
		doc.Tags = append([]string(nil), tags...)
	})
	return r
}

// Request sets a sample request body used to generate an OpenAPI schema.
func (r *Route) Request(sample interface{}) *Route {
	r.engine.router.UpdateDoc(r.Method, r.Path, func(doc *router.RouteDoc) {
		doc.Request = sample
	})
	return r
}

// Response sets a sample response body for the given status code.
func (r *Route) Response(code int, sample interface{}) *Route {
	r.engine.router.UpdateDoc(r.Method, r.Path, func(doc *router.RouteDoc) {
		if doc.Responses == nil {
			doc.Responses = make(map[int]interface{})
		}
		doc.Responses[code] = sample
	})
	return r
}

// Security sets OpenAPI security requirement names (e.g. "bearer").
func (r *Route) Security(schemes ...string) *Route {
	r.engine.router.UpdateDoc(r.Method, r.Path, func(doc *router.RouteDoc) {
		doc.Security = append([]string(nil), schemes...)
	})
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

// PATCH adds a PATCH route to the group.
func (group *RouterGroup) PATCH(path string, handler context.HandlerFunc) *Route {
	return group.addRoute("PATCH", path, handler)
}

// HEAD adds a HEAD route to the group.
func (group *RouterGroup) HEAD(path string, handler context.HandlerFunc) *Route {
	return group.addRoute("HEAD", path, handler)
}

// OPTIONS adds an OPTIONS route to the group.
func (group *RouterGroup) OPTIONS(path string, handler context.HandlerFunc) *Route {
	return group.addRoute("OPTIONS", path, handler)
}

// Handle registers a handler for an arbitrary HTTP method.
func (group *RouterGroup) Handle(method, path string, handler context.HandlerFunc) *Route {
	return group.addRoute(strings.ToUpper(method), path, handler)
}

// Any registers the handler for GET, POST, PUT, PATCH, DELETE, HEAD, and OPTIONS.
func (group *RouterGroup) Any(path string, handler context.HandlerFunc) []*Route {
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	out := make([]*Route, 0, len(methods))
	for _, m := range methods {
		out = append(out, group.addRoute(m, path, handler))
	}
	return out
}

// Static registers a route to serve static files from the provided root directory.
// Directory listings and dotfile paths (e.g. .env) are blocked.
// relativePath: The path pattern (e.g. "/assets")
// root: The file system root (e.g. "./public")
func (group *RouterGroup) Static(relativePath, root string) *Route {
	// Construct the full prefix path to strip (e.g. /v1/assets)
	absolutePrefix := group.prefix + relativePath

	fs := http.StripPrefix(absolutePrefix, http.FileServer(onlyFilesFS{http.Dir(root)}))

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
