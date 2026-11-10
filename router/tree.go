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
	fullPath  string // registered pattern when handle is set (stable metrics label)
	wildChild bool
	nType     nodeType
	indices   string
	children  []*Node
	handle    Handler
	priority  uint32
}

// addChild appends a child, keeping any wildcard child at the end so static
// indices stay aligned with children[0:len(indices)].
func (n *Node) addChild(child *Node) {
	if n.wildChild && len(n.children) > 0 {
		wildcard := n.children[len(n.children)-1]
		n.children = append(n.children[:len(n.children)-1], child, wildcard)
		return
	}
	n.children = append(n.children, child)
}

// incrementPriority increases the priority of the node and reorders children
func (n *Node) incrementPriority(pos int) {
	n.children[pos].priority++
	prio := n.children[pos].priority

	// Adjust position (move to front) among static children only.
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

			// Continue static suffix after a param via its single continuation node.
			if n.nType == param && c == '/' && len(n.children) == 1 {
				n = n.children[0]
				n.priority++
				continue walk
			}

			// Check if a static child with the next path byte exists
			for i, max := 0, len(n.indices); i < max; i++ {
				if n.indices[i] == c {
					i = n.incrementChildPrio(i)
					n = n.children[i]
					continue walk
				}
			}

			// Insert a new static child (keep wildcard at end via addChild).
			if c != ':' && c != '*' && n.nType != catchAll {
				n.indices += string([]byte{c})
				child := &Node{}
				n.addChild(child)
				n.incrementPriority(len(n.indices) - 1)
				n = child
			} else if n.wildChild {
				// Inserting another wildcard — must match the existing one.
				n = n.children[len(n.children)-1]
				n.priority++
				if len(path) >= len(n.path) && n.path == path[:len(n.path)] &&
					n.nType != catchAll &&
					(len(n.path) >= len(path) || path[len(n.path)] == '/') {
					continue walk
				}
				pathSeg := path
				if n.nType != catchAll {
					if j := stringsIndexByte(pathSeg, '/'); j >= 0 {
						pathSeg = pathSeg[:j]
					}
				}
				panic("'" + pathSeg + "' in new path '" + fullPath +
					"' conflicts with existing wildcard '" + n.path + "'")
			}
			n.insertChild(path, fullPath, handle)
			return
		}

		// Otherwise add handle to current node
		if n.handle != nil {
			panic("handlers are already registered for path '" + fullPath + "'")
		}
		n.handle = handle
		n.fullPath = fullPath
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

		// Conflicting wildcard names at the same level are not allowed.
		if n.wildChild && len(n.children) > 0 {
			existing := n.children[len(n.children)-1]
			if (existing.nType == param || existing.nType == catchAll) && existing.path != wildcard {
				panic("'" + wildcard + "' in new path '" + fullPath +
					"' conflicts with existing wildcard '" + existing.path + "'")
			}
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
			n.addChild(child)
			n.wildChild = true
			n = child
			n.priority++

			// Static continuation after the param (e.g. /assets, /take).
			if len(wildcard) < len(path) {
				path = path[len(wildcard):]
				child := &Node{priority: 1}
				n.addChild(child)
				n = child
				continue
			}
			// Otherwise we're done. Insert the handle in the new leaf
			n.handle = handle
			n.fullPath = fullPath
			return

		} else { // catchAll
			if i+len(wildcard) != len(path) {
				panic("catch-all routes are only allowed at the end of the path in path '" + fullPath + "'")
			}
			if len(n.children) > 0 {
				for _, ch := range n.children {
					if ch.nType == catchAll && ch.path != wildcard {
						panic("catch-all conflict: '" + ch.path + "' vs '" + wildcard + "' in path '" + fullPath + "'")
					}
				}
			}
			if i > 0 {
				n.path = path[:i]
			}

			child := &Node{
				nType: catchAll,
				path:  wildcard,
			}
			n.addChild(child)
			n.wildChild = true
			n = child
			n.priority++
			n.handle = handle
			n.fullPath = fullPath
			return
		}
	}

	// If no wildcard, simple insertion
	n.path = path
	n.handle = handle
	n.fullPath = fullPath
}

// getValue returns the handle registered with the given path (key). The values of
// wildcards are saved to a slice. pattern is the registered route path when found.
func (n *Node) getValue(path string, p Params) (handle Handler, outP Params, tsr bool, pattern string) {
walk: // Outer loop for walking the tree
	for {
		prefix := n.path
		if len(path) > len(prefix) {
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):]

				// Prefer static children (indices) over wildcard so
				// /user/new wins over /user/:id.
				idxc := path[0]
				for i := 0; i < len(n.indices); i++ {
					if n.indices[i] == idxc {
						n = n.children[i]
						continue walk
					}
				}
				if !n.wildChild {
					return nil, p, false, ""
				}

				// Wildcard child is always last.
				n = n.children[len(n.children)-1]
				switch n.nType {
				case param:
					var pathLeft string
					var nextN *Node
					var returnNow bool
					handle, p, pathLeft, nextN, returnNow = n.getValueParam(path, p)
					if returnNow {
						if handle != nil {
							return handle, p, false, n.fullPath
						}
						return nil, p, false, ""
					}
					path, n = pathLeft, nextN
					continue walk
				case catchAll:
					handle, p = n.getValueCatchAll(path, p)
					return handle, p, false, n.fullPath
				default:
					panic("invalid node type")
				}
			}
		} else if path == prefix {
			if handle = n.handle; handle != nil {
				return handle, p, false, n.fullPath
			}
			// Path matches a node without a handle — trailing slash redirect?
			if path != "/" && len(path) > 0 && path[len(path)-1] != '/' {
				for _, child := range n.children {
					if child.path == "/" && child.handle != nil {
						return nil, p, true, ""
					}
				}
			}
		} else if len(path)+1 == len(prefix) && prefix[len(path)] == '/' && path == prefix[:len(path)] {
			// Request missing trailing slash that node has
			if n.handle != nil {
				return nil, p, true, ""
			}
		}
		return nil, p, false, ""
	}
}

// getValueParam handles a param node: extracts segment, appends to p, and returns either
// (handle, p, "", nil, true) to return, or ("", p, pathRest, nextNode, false) to continue.
func (n *Node) getValueParam(path string, p Params) (handle Handler, outP Params, pathRest string, nextNode *Node, returnNow bool) {
	end := 0
	for end < len(path) && path[end] != '/' {
		end++
	}
	p = append(p, Param{Key: n.path[1:], Value: path[:end]})

	if end < len(path) {
		if len(n.children) > 0 {
			return nil, p, path[end:], n.children[0], false
		}
		return nil, nil, "", nil, true // no child to go to
	}
	if n.handle != nil {
		return n.handle, p, "", nil, true
	}
	return nil, nil, "", nil, true // path consumed but no handle
}

// getValueCatchAll handles a catchAll node: the rest of path is the param value.
func (n *Node) getValueCatchAll(path string, p Params) (handle Handler, outP Params) {
	p = append(p, Param{Key: n.path[1:], Value: path})
	return n.handle, p
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

func stringsIndexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
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
