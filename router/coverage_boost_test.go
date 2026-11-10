package router

import "testing"

func TestFindEx_TSRAndDocsWalkAllowed(t *testing.T) {
	r := New()
	var called bool
	r.AddRoute("GET", "/users", func() { called = true })
	r.AddRoute("POST", "/users", func() {})
	r.AddRoute("GET", "/items/:id", func() {})

	res := r.FindEx("GET", "/users/", nil)
	if !res.TSR {
		t.Fatalf("want TSR for trailing slash, got %+v", res)
	}
	res2 := r.FindEx("GET", "/users", nil)
	if !res2.Found || res2.Handler == nil {
		t.Fatal("want found")
	}
	res2.Handler.(func())()
	if !called {
		t.Fatal("handler not called")
	}
	res3 := r.FindEx("PUT", "/nope", nil)
	if res3.Found || res3.TSR {
		t.Fatalf("want miss %+v", res3)
	}
	_ = alternateSlash("")
	_ = alternateSlash("/")
	_ = alternateSlash("/a/")
	_ = alternateSlash("/a")

	methods := r.AllowedMethods("/users")
	if len(methods) < 2 {
		t.Fatalf("allowed=%v", methods)
	}
	_ = r.AllowedMethods("/users/")
	_ = r.AllowedMethods("/missing")

	r.SetDocumentation("GET", "/users", "list users")
	r.UpdateDoc("GET", "/users", func(d *RouteDoc) {
		d.Tags = []string{"u"}
		d.Security = []string{"bearer"}
	})
	var walked int
	r.Walk(func(method, path string, doc RouteDoc) {
		walked++
		if method == "GET" && path == "/users" && doc.Summary != "list users" {
			t.Fatalf("doc=%+v", doc)
		}
	})
	if walked < 2 {
		t.Fatalf("walked=%d", walked)
	}

	r2 := New()
	r2.AddRoute("GET", "/x/:id", func() {})
	r2.AddRoute("GET", "/x/static", func() {})
	if h, _, ok := r2.Find("GET", "/x/static"); !ok || h == nil {
		t.Fatal("static after param")
	}

	// Hit incrementChildPrio by registering many siblings then re-entering a popular prefix.
	r3 := New()
	r3.AddRoute("GET", "/a", func() {})
	r3.AddRoute("GET", "/b", func() {})
	r3.AddRoute("GET", "/c", func() {})
	r3.AddRoute("GET", "/a/1", func() {})
	r3.AddRoute("GET", "/a/2", func() {})
	r3.AddRoute("GET", "/a/3", func() {})
	r3.AddRoute("GET", "/b/1", func() {})
	if h, _, ok := r3.Find("GET", "/a/2"); !ok || h == nil {
		t.Fatal("prio routes")
	}

	r4 := New()
	r4.AddRoute("GET", "/files/*filepath", func() {})
	r4.AddRoute("GET", "/dir/", func() {})
	resCatch := r4.FindEx("GET", "/files/a/b/c", nil)
	if !resCatch.Found {
		t.Fatal("catch-all")
	}
	resTSR := r4.FindEx("GET", "/dir", nil)
	if !resTSR.TSR {
		t.Fatalf("want TSR for /dir -> /dir/, got %+v", resTSR)
	}
	ps := Params{{Key: "id", Value: "1"}}
	if ps.Get("id") != "1" || ps.Get("no") != "" {
		t.Fatal("Params.Get")
	}

	r5 := New()
	r5.AddRoute("GET", "/p/:id/x", func() {})
	r5.AddRoute("GET", "/p/:id/y", func() {})
	r5.AddRoute("GET", "/p/:id/x/z", func() {})
	if _, _, ok := r5.Find("GET", "/p/1/x"); !ok {
		t.Fatal("param continue")
	}
	if _, _, ok := r5.Find("GET", "/p/1/missing"); ok {
		t.Fatal("want miss after param")
	}
	// TSR: register slash-only child
	r5.AddRoute("GET", "/slash", func() {})
	r5.AddRoute("GET", "/slash/", func() {})
	resSlash := r5.FindEx("GET", "/slash", nil)
	_ = resSlash
	miss := r5.FindEx("GET", "/no/such/path", nil)
	if miss.Found {
		t.Fatal("miss")
	}
}
