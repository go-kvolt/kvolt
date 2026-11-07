package router

import "testing"

func TestFindInto_EmptyMethod(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/x", "ok")
	buf := Params{{Key: "k", Value: "v"}}
	h, ps, ok := r.FindInto("POST", "/x", buf)
	if ok || h != nil {
		t.Fatalf("want miss for empty method tree, got h=%v ok=%v", h, ok)
	}
	if len(ps) != 0 {
		t.Fatalf("want empty params, got %v", ps)
	}
}

func TestIncrementChildPrio_ReordersIndices(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/a", "a")
	r.AddRoute("GET", "/b", "b")
	r.AddRoute("GET", "/c", "c")
	// Bump /c above /a and /b so incrementChildPrio swaps indices.
	r.AddRoute("GET", "/c/1", "c1")
	r.AddRoute("GET", "/c/2", "c2")
	r.AddRoute("GET", "/c/3", "c3")
	h, _, ok := r.Find("GET", "/c/2")
	if !ok || h != "c2" {
		t.Fatalf("want c2, got ok=%v h=%v", ok, h)
	}
	root := r.trees["GET"]
	if root.indices[0] != 'c' {
		t.Fatalf("want 'c' first after priority bumps, indices=%q", root.indices)
	}
}

func TestIncrementPriority_DirectReorder(t *testing.T) {
	n := &Node{
		indices:  "ab",
		children: []*Node{{path: "a", priority: 0}, {path: "b", priority: 0}},
	}
	n.incrementPriority(1)
	if n.indices != "ba" {
		t.Fatalf("want indices ba after reorder, got %q", n.indices)
	}
	if n.children[0].path != "b" || n.children[0].priority != 1 {
		t.Fatalf("want b@1 first, got %+v", n.children[0])
	}
}

func TestInsert_AddHandleAfterSplitAndDuplicatePanic(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/foo/bar", "bar")
	r.AddRoute("GET", "/foo", "foo") // exhausts path on split parent → insert handle branch
	h, _, ok := r.Find("GET", "/foo")
	if !ok || h != "foo" {
		t.Fatalf("want foo after split-handle, got ok=%v h=%v", ok, h)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected duplicate handler panic")
		}
	}()
	r.AddRoute("GET", "/foo", "again")
}

func TestInsert_WildcardConflictTruncatesPathSeg(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/api/:x", "x")
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected wildcard conflict panic")
		}
		msg, _ := rec.(string)
		if msg == "" {
			t.Fatalf("want string panic, got %T %v", rec, rec)
		}
	}()
	// pathSeg contains '/' so stringsIndexByte hit truncates before panic text.
	r.AddRoute("GET", "/api/:y/z", "y")
}

func TestInsertChild_InvalidWildcardAndConflicts(t *testing.T) {
	t.Run("invalid wildcard chars", func(t *testing.T) {
		n := &Node{}
		defer func() {
			if recover() == nil {
				t.Fatal("expected invalid wildcard panic")
			}
		}()
		n.insertChild("/user/:id:name", "/user/:id:name", "h")
	})

	t.Run("param name conflict via insertChild", func(t *testing.T) {
		n := &Node{
			wildChild: true,
			children:  []*Node{{nType: param, path: ":id"}},
		}
		defer func() {
			if recover() == nil {
				t.Fatal("expected param name conflict panic")
			}
		}()
		n.insertChild("/:name", "/:name", "h")
	})

	t.Run("catch-all not at end", func(t *testing.T) {
		n := &Node{}
		defer func() {
			if recover() == nil {
				t.Fatal("expected catch-all end panic")
			}
		}()
		n.insertChild("/files/*path/extra", "/files/*path/extra", "h")
	})

	t.Run("catch-all name conflict via insertChild", func(t *testing.T) {
		n := &Node{
			children: []*Node{{nType: catchAll, path: "*a"}},
		}
		defer func() {
			if recover() == nil {
				t.Fatal("expected catch-all conflict panic")
			}
		}()
		n.insertChild("/*b", "/*b", "h")
	})
}

func TestGetValue_TSRBothDirectionsAndEdges(t *testing.T) {
	// Request missing trailing slash that registered path has (getValue TSR).
	r := New()
	r.AddRoute("GET", "/dir/", "d")
	res := r.FindEx("GET", "/dir", nil)
	if !res.TSR {
		t.Fatalf("want TSR /dir -> /dir/, got %+v", res)
	}

	// Extra trailing slash: getValue misses; FindEx TSR via alternateSlash.
	rLeaf := New()
	rLeaf.AddRoute("GET", "/leaf", "L")
	resLeaf := rLeaf.FindEx("GET", "/leaf/", nil)
	if !resLeaf.TSR {
		t.Fatalf("want TSR /leaf/ -> /leaf, got %+v", resLeaf)
	}

	// Exact match on handle-less node with "/" child → TSR.
	parent := &Node{
		path: "/mid",
		children: []*Node{
			{path: "/", handle: "slash"},
		},
	}
	_, _, tsr2, _ := parent.getValue("/mid", nil)
	if !tsr2 {
		t.Fatal("want TSR via slash child")
	}

	// Param with continuation leftover but no child.
	r2 := New()
	r2.AddRoute("GET", "/user/:id", "u")
	if h, _, ok := r2.Find("GET", "/user/1/extra"); ok || h != nil {
		t.Fatal("want miss for leftover after param")
	}

	// Param segment consumed but param node has no handle.
	r3 := New()
	r3.AddRoute("GET", "/p/:id/x", "x")
	if h, _, ok := r3.Find("GET", "/p/1"); ok || h != nil {
		t.Fatal("want miss for param without handle")
	}

	// Catch-all lookup.
	r4 := New()
	r4.AddRoute("GET", "/static/*filepath", "f")
	h, ps, ok := r4.Find("GET", "/static/a/b")
	if !ok || h != "f" || ps.Get("filepath") != "a/b" {
		t.Fatalf("catch-all: ok=%v h=%v fp=%q", ok, h, ps.Get("filepath"))
	}
}

func TestGetValue_InvalidWildcardTypePanics(t *testing.T) {
	n := &Node{
		path:      "/w",
		wildChild: true,
		children:  []*Node{{nType: static, path: "bad"}},
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expected invalid node type panic")
		}
	}()
	n.getValue("/wX", nil)
}

func TestFindWildcard_AndStringsIndexByte(t *testing.T) {
	w, i, valid := findWildcard("/ok/path")
	if i != -1 || w != "" || valid {
		t.Fatalf("want no wildcard, got %q %d %v", w, i, valid)
	}
	w, i, valid = findWildcard("/x/:a*b")
	if i < 0 || valid {
		t.Fatalf("want invalid wildcard, got %q %d valid=%v", w, i, valid)
	}
	w, i, valid = findWildcard("/x/:id/y")
	if w != ":id" || i != 3 || !valid {
		t.Fatalf("want :id, got %q %d %v", w, i, valid)
	}
	if stringsIndexByte("ab/cd", '/') != 2 {
		t.Fatal("stringsIndexByte hit")
	}
	if stringsIndexByte("abcd", '/') != -1 {
		t.Fatal("stringsIndexByte miss")
	}
}

func TestInsertChild_CatchAllAndParamContinuation(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/assets/*filepath", "all")
	r.AddRoute("GET", "/assets/logo.png", "logo") // static sibling beside catch-all
	h, _, ok := r.Find("GET", "/assets/logo.png")
	if !ok || h != "logo" {
		t.Fatalf("static beside catch-all: ok=%v h=%v", ok, h)
	}
	h2, ps, ok2 := r.Find("GET", "/assets/css/app.css")
	if !ok2 || h2 != "all" || ps.Get("filepath") != "css/app.css" {
		t.Fatalf("catch-all: ok=%v h=%v fp=%q", ok2, h2, ps.Get("filepath"))
	}

	r2 := New()
	r2.AddRoute("GET", "/docs/:section/page", "page")
	r2.AddRoute("GET", "/docs/:section/meta", "meta")
	if h, _, ok := r2.Find("GET", "/docs/intro/page"); !ok || h != "page" {
		t.Fatalf("param continuation page: ok=%v h=%v", ok, h)
	}
	if h, _, ok := r2.Find("GET", "/docs/intro/meta"); !ok || h != "meta" {
		t.Fatalf("param continuation meta: ok=%v h=%v", ok, h)
	}
}
