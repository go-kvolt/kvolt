package router

// Handler is the request handler
type Handler any

// Param is a single URL parameter, consisting of a key and a value.
type Param struct {
	Key   string
	Value string
}

// Params is a Param-slice, as returned by the router.
// The slice is ordered, the first URL parameter is also the first slice value.
type Params []Param

// Get returns the value of the first Param which key matches the given name.
// If no matching Param is found, an empty string is returned.
func (ps Params) Get(name string) string {
	for _, entry := range ps {
		if entry.Key == name {
			return entry.Value
		}
	}
	return ""
}

type nodeType uint8

const (
	static nodeType = iota // default
	root
	param
	catchAll
)

type Node struct {
	path      string
	wildChild bool
	nType     nodeType
	indices   string
	children  []*Node
	handle    Handler
	priority  uint32
}

// incrementPriority increases the priority of the node and reorders children
func (n *Node) incrementPriority(pos int) {
	n.children[pos].priority++
	prio := n.children[pos].priority

	// Adjust position (move to front)
	newPos := pos
	for newPos > 0 && n.children[newPos-1].priority < prio {
		// swap
		n.children[newPos-1], n.children[newPos] = n.children[newPos], n.children[newPos-1]
		newPos--
	}

	// Rebuild indices
	if newPos != pos {
		n.indices = n.indices[:newPos] + // Start
			n.indices[pos:pos+1] + // Moved char
			n.indices[newPos:pos] + // Shifted chars
			n.indices[pos+1:] // End
	}
}

// insert adds a route to the tree
func (n *Node) insert(path string, handle Handler) {
	fullPath := path
	n.priority++

	// Empty tree
	if n.path == "" && len(n.children) == 0 {
		n.insertChild(path, fullPath, handle)
		n.nType = root
		return
	}

walk:
	for {
		// Find longest common prefix
		// This also implies that the common prefix is a substring of "path"
		i := longestCommonPrefix(path, n.path)

		// Split edge
		if i < len(n.path) {
			child := Node{
				path:      n.path[i:],
				wildChild: n.wildChild,
				nType:     static,
				indices:   n.indices,
				children:  n.children,
				handle:    n.handle,
				priority:  n.priority - 1,
			}

			n.children = []*Node{&child}
			// []byte for proper unicode handling could be better, but keeping simple
			n.indices = string([]byte{n.path[i]})
			n.path = path[:i]
			n.handle = nil
			n.wildChild = false
		}

		// Make new node a child of this node
		if i < len(path) {
			path = path[i:]
			c := path[0]

			// '/' after param
			if n.nType == param && c == '/' && len(n.children) == 1 {
				n = n.children[0]
				n.priority++
				continue walk
			}

			// Check if a child with the next path byte exists
			for i, max := 0, len(n.indices); i < max; i++ {
				if n.indices[i] == c {
					i = n.incrementChildPrio(i)
					n = n.children[i]
					continue walk
				}
			}

			// Otherwise insert it
			if c != ':' && c != '*' {
				// []byte for proper unicode handling could be better
				n.indices += string([]byte{c})
				child := &Node{}
				n.children = append(n.children, child)
				n.incrementPriority(len(n.indices) - 1)
				n = child
			}
			n.insertChild(path, fullPath, handle)
			return
		}

		// Otherwise add handle to current node
		if n.handle != nil {
			panic("handlers are already registered for path '" + fullPath + "'")
		}
		n.handle = handle
		return
	}
}

func (n *Node) insertChild(path, fullPath string, handle Handler) {
	for {
		// Find prefix until first wildcard
		wildcard, i, valid := findWildcard(path)
		if i < 0 { // No wildcard
			break
		}

		// The wildcard name must not contain ':' and '*'
		if !valid {
			panic("only one wildcard per path segment is allowed, has: '" +
				wildcard + "' in path '" + fullPath + "'")
		}

		// Check if the wildcard has an existing node
		if len(n.children) > 0 {
			// If we are here, we exist, and since we are a wildcard,
			// we must have only one child.
			if len(n.children) != 1 {
				// Logic error or conflict
			}
			// Validate if the existing wildcard matches the new one?
			// For simplicity, assuming no conflicting wildcard names at same level for now.
		}

		if wildcard[0] == ':' { // param
			if i > 0 {
				n.path = path[:i]
				path = path[i:]
			}

			child := &Node{
				nType: param,
				path:  wildcard,
			}
			n.children = []*Node{child}
			n.wildChild = true
			n = child
			n.priority++

			// If the path doesn't end with the wildcard, then there
			// will be another non-wildcard sub-path starting with '/'
			if len(wildcard) < len(path) {
				path = path[len(wildcard):]
				child := &Node{
					priority: 1,
				}
				n.children = []*Node{child}
				n = child
				continue
			}
			// Otherwise we're done. Insert the handle in the new leaf
			n.handle = handle
			return

		} else { // catchAll
			if i+len(wildcard) != len(path) {
				panic("catch-all routes are only allowed at the end of the path in path '" + fullPath + "'")
			}
			// if len(n.path) > 0 && n.path[len(n.path)-1] == '/' {
			// 	panic("catch-all conflicts with existing handle for the path segment root in path '" + fullPath + "'")
			// }
			// Currently fixed prefix required currently (TODO: fix)
			if i > 0 {
				n.path = path[:i]
				path = path[i:]
			}

			child := &Node{
				nType: catchAll,
				path:  wildcard,
			}
			n.children = []*Node{child}
			n.wildChild = true
			n = child
			n.priority++
			n.handle = handle
			return
		}
	}

	// If no wildcard, simple insertion
	n.path = path
	n.handle = handle
}

// getValue returns the handle registered with the given path (key). The values of
// wildcards are saved to a slice.
func (n *Node) getValue(path string) (handle Handler, p Params, tsr bool) {
walk: // Outer loop for walking the tree
	for {
		prefix := n.path
		if len(path) > len(prefix) {
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):]

				// If this node does not have a wildcard (param or catchAll)
				// child,  we must look up the next child node
				if !n.wildChild {
					idxc := path[0]
					for i, c := range []byte(n.indices) {
						if c == idxc {
							n = n.children[i]
							continue walk
						}
					}
					// Nothing found.
					// TSR: If the path is equal to prefix + '/', return TSR=true?
					// For now, no.
					return nil, nil, false
				}

				// Handle wildcard child
				n = n.children[0]
				switch n.nType {
				case param:
					// Find param end (either '/' or end of path)
					end := 0
					for end < len(path) && path[end] != '/' {
						end++
					}

					// Add param value
					if p == nil {
						// Lazy allocation
						p = make(Params, 0, 4)
					}
					i := len(p)
					p = p[:i+1] // expand slice
					p[i].Key = n.path[1:]
					p[i].Value = path[:end]

					// We need to go deeper!
					if end < len(path) {
						if len(n.children) > 0 {
							path = path[end:]
							n = n.children[0]
							continue walk
						}
						// ... but we can't
						return nil, nil, false
					}

					if handle = n.handle; handle != nil {
						return handle, p, false
					}

				case catchAll:
					// Variable is everything left
					if p == nil {
						p = make(Params, 0, 4)
					}
					i := len(p)
					p = p[:i+1]
					p[i].Key = n.path[1:] // remove "*"
					// Verify this... normally catch all is "*name" or "/*name" ?
					// n.path is whatever we stored during insert.
					p[i].Value = path
					// assuming catch all is *name
					// Actually let's assume wildcard syntax is *name

					handle = n.handle
					return handle, p, false

				default:
					panic("invalid node type")
				}
			}
		} else if path == prefix {
			// specific handler
			if handle = n.handle; handle != nil {
				return handle, p, false
			}
			// If not found, and path == prefix, and we have wildChild...
			// e.g. /users matches /users/:id ? No.
		}
		return nil, nil, false
	}
}

// Helpers

func (n *Node) incrementChildPrio(pos int) int {
	n.children[pos].priority++
	prio := n.children[pos].priority

	// Adjust
	newPos := pos
	for newPos > 0 && n.children[newPos-1].priority < prio {
		// swap
		n.children[newPos-1], n.children[newPos] = n.children[newPos], n.children[newPos-1]
		newPos--
	}

	// Rebuild indices
	if newPos != pos {
		n.indices = n.indices[:newPos] + // Start
			n.indices[pos:pos+1] + // Moved char
			n.indices[newPos:pos] + // Shifted chars
			n.indices[pos+1:] // End
	}

	return newPos
}

func longestCommonPrefix(a, b string) int {
	i := 0
	max := len(a)
	if len(b) < max {
		max = len(b)
	}
	for i < max && a[i] == b[i] {
		i++
	}
	return i
}

// findWildcard searches for a wildcard segment
func findWildcard(path string) (wildcard string, i int, valid bool) {
	// Find start
	for start, c := range []byte(path) {
		// A wildcard starts with ':' (param) or '*' (catch-all)
		if c != ':' && c != '*' {
			continue
		}

		// Find end and check for invalid characters
		valid = true
		for end, c := range []byte(path[start+1:]) {
			switch c {
			case '/':
				return path[start : start+1+end], start, valid
			case ':', '*':
				valid = false
			}
		}
		return path[start:], start, valid
	}
	return "", -1, false
}
