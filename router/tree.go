package router

import "fmt"

// Wrapper for the handler
type Handler any

type Param struct {
	Key   string
	Value string
}

type Params []Param

func (ps Params) Get(name string) string {
	return ""
}

// Node (Naive Map impl)
type Node struct {
	routes map[string]Handler
	path   string // Required by router.go AddRoute
}

func (n *Node) insert(path string, handle Handler) {
	if n.routes == nil {
		n.routes = make(map[string]Handler)
	}
	fmt.Println("MockRouter: Registered", path)
	n.routes[path] = handle
}

func (n *Node) getValue(path string) (Handler, Params, bool) {
	// fmt.Println("MockRouter: Lookup", path)
	if n.routes == nil {
		return nil, nil, false
	}
	if h, ok := n.routes[path]; ok {
		return h, nil, true
	}
	return nil, nil, false
}
