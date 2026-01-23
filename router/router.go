package router

// Router registers routes to be matched and dispatches a handler.
type Router struct {
	trees map[string]*Node
}

// New creates a new Router.
func New() *Router {
	return &Router{
		trees: make(map[string]*Node),
	}
}

// AddRoute registers a new request handler with the given path and method.
func (r *Router) AddRoute(method, path string, handle Handler) {
	root := r.trees[method]
	if root == nil {
		root = &Node{path: "/"}
		r.trees[method] = root
	}
	root.insert(path, handle)
}

// Find lookup a handler given a method and path.
func (r *Router) Find(method, path string) (Handler, Params, bool) {
	root := r.trees[method]
	if root == nil {
		return nil, nil, false
	}
	return root.getValue(path)
}
