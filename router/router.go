package router

import "slices"

// RouteDoc holds OpenAPI-oriented metadata for a registered route.
type RouteDoc struct {
	Summary   string
	Tags      []string
	Request   interface{}         // sample request body for schema generation
	Responses map[int]interface{} // status -> sample response body
	Security  []string
}

// Router registers routes to be matched and dispatches a handler.
type Router struct {
	trees map[string]*Node
	docs  map[string]RouteDoc // Key: "METHOD /path"
}

// New creates a new Router.
func New() *Router {
	return &Router{
		trees: make(map[string]*Node),
		docs:  make(map[string]RouteDoc),
	}
}

// AddRoute registers a new request handler with the given path and method.
func (r *Router) AddRoute(method, path string, handle Handler) {
	root := r.trees[method]
	if root == nil {
		root = &Node{}
		r.trees[method] = root
	}
	root.insert(path, handle)
}

// Find lookup a handler given a method and path.
func (r *Router) Find(method, path string) (Handler, Params, bool) {
	return r.FindInto(method, path, nil)
}

// FindInto is Find with a reusable Params buffer (pass c.Params[:0] from the context pool).
func (r *Router) FindInto(method, path string, p Params) (Handler, Params, bool) {
	root := r.trees[method]
	if root == nil {
		return nil, p[:0], false
	}
	handle, ps, tsr, _ := root.getValue(path, p[:0])
	if handle != nil {
		return handle, ps, true
	}
	_ = tsr
	return nil, ps, false
}

// FindResult is an extended lookup result.
type FindResult struct {
	Handler Handler
	Params  Params
	Found   bool
	TSR     bool   // trailing-slash redirect suggested
	Pattern string // registered route pattern (e.g. /users/:id), for metrics
}

// FindEx looks up a route and reports trailing-slash redirect hints.
func (r *Router) FindEx(method, path string, p Params) FindResult {
	root := r.trees[method]
	if root == nil {
		return FindResult{Params: p[:0]}
	}
	handle, ps, tsr, pattern := root.getValue(path, p[:0])
	if handle != nil {
		return FindResult{Handler: handle, Params: ps, Found: true, Pattern: pattern}
	}
	if tsr {
		return FindResult{Params: ps, TSR: true}
	}
	// Suggest redirect if the alternate slash form exists.
	alt := alternateSlash(path)
	if alt != path {
		if h, _, _, _ := root.getValue(alt, nil); h != nil {
			return FindResult{Params: ps, TSR: true}
		}
	}
	return FindResult{Params: ps}
}

func alternateSlash(path string) string {
	if path == "" || path == "/" {
		return path
	}
	if path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path + "/"
}

// AllowedMethods returns HTTP methods registered for path (for 405 Allow header).
func (r *Router) AllowedMethods(path string) []string {
	var methods []string
	for method, root := range r.trees {
		if h, _, _, _ := root.getValue(path, nil); h != nil {
			methods = append(methods, method)
		} else if alt := alternateSlash(path); alt != path {
			if h, _, _, _ := root.getValue(alt, nil); h != nil {
				methods = append(methods, method)
			}
		}
	}
	slices.Sort(methods)
	return methods
}

// SetDocumentation adds a description for a registered route (backward compatible).
func (r *Router) SetDocumentation(method, path, desc string) {
	key := method + " " + path
	doc := r.docs[key]
	doc.Summary = desc
	r.docs[key] = doc
}

// UpdateDoc merges fields into the route documentation.
func (r *Router) UpdateDoc(method, path string, mut func(*RouteDoc)) {
	key := method + " " + path
	doc := r.docs[key]
	mut(&doc)
	r.docs[key] = doc
}

// Walk iterates over all registered routes.
func (r *Router) Walk(walkFunc func(method, path string, doc RouteDoc)) {
	for method, root := range r.trees {
		root.walk(pathStub, func(path string) {
			key := method + " " + path
			walkFunc(method, path, r.docs[key])
		})
	}
}

const pathStub = ""

func (n *Node) walk(path string, walkFunc func(path string)) {
	fullPath := path + n.path
	if n.handle != nil {
		if n.fullPath != "" {
			walkFunc(n.fullPath)
		} else {
			walkFunc(fullPath)
		}
	}
	for _, child := range n.children {
		child.walk(fullPath, walkFunc)
	}
}
