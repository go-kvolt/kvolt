package router

// Router registers routes to be matched and dispatches a handler.
type Router struct {
	trees map[string]*Node
	docs  map[string]string // Key: "METHOD /path", Value: "Description"
}

// New creates a new Router.
func New() *Router {
	return &Router{
		trees: make(map[string]*Node),
		docs:  make(map[string]string),
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
	root := r.trees[method]
	if root == nil {
		return nil, nil, false
	}
	handle, ps, _ := root.getValue(path)
	if handle != nil {
		return handle, ps, true
	}
	return nil, nil, false
}

// SetDocumentation adds a description for a registered route.
func (r *Router) SetDocumentation(method, path, desc string) {
	key := method + " " + path
	r.docs[key] = desc
}

// Walk iterates over all registered routes.
// The callback function is called for each route with the method, full path, and description.
func (r *Router) Walk(walkFunc func(method, path, desc string)) {
	for method, root := range r.trees {
		root.walk(pathStub, func(path string) {
			key := method + " " + path
			desc := r.docs[key]
			walkFunc(method, path, desc)
		})
	}
}

const pathStub = ""

func (n *Node) walk(path string, walkFunc func(path string)) {
	fullPath := path + n.path
	if n.handle != nil {
		walkFunc(fullPath)
	}
	for _, child := range n.children {
		child.walk(fullPath, walkFunc)
	}
}
