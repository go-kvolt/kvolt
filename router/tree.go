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
				fullPath:  n.fullPath,
				priority:  n.priority - 1,
			}

			n.children = []*Node{&child}
			// []byte for proper unicode handling could be better, but keeping simple
			n.indices = string([]byte{n.path[i]})
			n.path = path[:i]
			n.handle = nil
			n.fullPath = ""
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
// tsr reports that the same path with a trailing slash added is registered.
func (n *Node) getValue(path string, p Params) (handle Handler, outP Params, tsr bool, pattern string) {
	handle, outP, tsr, pattern = n.lookup(path, p)
	if handle == nil {
		// Hand back the caller's buffer, not a partial match's params.
		return nil, p, tsr, ""
	}
	return handle, outP, false, pattern
}

// lookup matches path against the subtree rooted at n.
//
// Static children are tried before the wildcard child, so /user/new wins over
// /user/:id. When the static branch does not lead to a handler the wildcard is
// tried next: with both routes registered, /user/next shares the "n" prefix with
// /user/new but must still match /user/:id.
//
// On a miss the returned Params are p unchanged. Params appended by a failed
// branch are overwritten by the next one, so no allocation is needed to back out.
func (n *Node) lookup(path string, p Params) (handle Handler, outP Params, tsr bool, pattern string) {
	prefix := n.path
	if len(path) < len(prefix) || path[:len(prefix)] != prefix {
		// Request is this node's path minus its trailing slash.
		if len(path)+1 == len(prefix) && prefix[len(path)] == '/' && path == prefix[:len(path)] {
			if n.handle != nil || n.catchAllChild() != nil {
				return nil, p, true, ""
			}
		}
		return nil, p, false, ""
	}

	rest := path[len(prefix):]
	if rest == "" {
		if n.handle != nil {
			return n.handle, p, false, n.fullPath
		}
		// Path ends exactly where a catch-all starts (e.g. /files/ for /files/*filepath).
		if child := n.catchAllChild(); child != nil {
			return child.handle, append(p, Param{Key: child.path[1:], Value: ""}), false, child.fullPath
		}
		// Node without a handle: a "/" child means the trailing-slash form is registered.
		if path != "/" && len(path) > 0 && path[len(path)-1] != '/' {
			for _, child := range n.children {
				if child.path == "/" && child.handle != nil {
					return nil, p, true, ""
				}
			}
		}
		return nil, p, false, ""
	}

	// Static children first. Index bytes are unique, so at most one can match.
	c := rest[0]
	for i := 0; i < len(n.indices); i++ {
		if n.indices[i] != c {
			continue
		}
		var staticTSR bool
		handle, outP, staticTSR, pattern = n.children[i].lookup(rest, p)
		if handle != nil {
			return handle, outP, false, pattern
		}
		tsr = staticTSR
		break
	}

	if !n.wildChild || len(n.children) == 0 {
		return nil, p, tsr, ""
	}

	// The wildcard child is always last.
	wild := n.children[len(n.children)-1]
	switch wild.nType {
	case param:
		end := 0
		for end < len(rest) && rest[end] != '/' {
			end++
		}
		if end == 0 {
			// An empty segment ("/users//posts") is not a parameter value: a handler
			// reading :id must never see "".
			return nil, p, tsr, ""
		}
		withParam := append(p, Param{Key: wild.path[1:], Value: rest[:end]})
		if end == len(rest) {
			if wild.handle != nil {
				return wild.handle, withParam, false, wild.fullPath
			}
			return nil, p, tsr, ""
		}
		// A param node has at most one child: the static continuation after it.
		if len(wild.children) > 0 {
			var contTSR bool
			handle, outP, contTSR, pattern = wild.children[0].lookup(rest[end:], withParam)
			if handle != nil {
				return handle, outP, false, pattern
			}
			tsr = tsr || contTSR
		}
		return nil, p, tsr, ""
	case catchAll:
		if wild.handle == nil {
			return nil, p, tsr, ""
		}
		return wild.handle, append(p, Param{Key: wild.path[1:], Value: rest}), false, wild.fullPath
	default:
		panic("invalid node type")
	}
}

// catchAllChild returns n's catch-all child when it has a handle, else nil.
func (n *Node) catchAllChild() *Node {
	if !n.wildChild || len(n.children) == 0 {
		return nil
	}
	child := n.children[len(n.children)-1]
	if child.nType == catchAll && child.handle != nil {
		return child
	}
	return nil
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
