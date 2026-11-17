package context

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/router"
)

func TestFork_IsIndependentCopy(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/7", nil)
	c := New(rec, req)
	c.Params = append(make(router.Params, 0, 4), router.Param{Key: "id", Value: "7"})
	c.Set("user", "alice")
	c.RoutePath = "/users/:id"
	c.TrustedProxies = []string{"10.0.0.0/8"}
	ran := []string{}
	c.Handlers = []HandlerFunc{
		func(c *Context) error { ran = append(ran, "first"); return nil },
		func(c *Context) error { ran = append(ran, "second:"+c.Param("id")); return nil },
	}
	c.index = 0 // as if the first handler is running and about to fork

	forkRec := httptest.NewRecorder()
	forkReq := httptest.NewRequest(http.MethodGet, "/users/7", nil)
	f := c.Fork(forkRec, forkReq)

	if f == c || f.Writer != forkRec || f.Request != forkReq {
		t.Fatalf("fork must use the given writer and request")
	}
	if f.Param("id") != "7" || f.RoutePath != "/users/:id" || len(f.TrustedProxies) != 1 {
		t.Fatalf("fork lost request data: %+v", f)
	}
	if v, _ := f.Get("user"); v != "alice" {
		t.Fatalf("fork lost Keys")
	}

	// The fork continues the chain from the same position.
	f.Next()
	if len(ran) != 1 || ran[0] != "second:7" {
		t.Fatalf("fork ran %v, want only the remaining handler", ran)
	}

	// Nothing mutable is shared: the Engine reuses the original for the next request.
	c.Reset(rec, req)
	c.Params = append(c.Params, router.Param{Key: "id", Value: "999"})
	c.Set("user", "mallory")
	if f.Param("id") != "7" {
		t.Fatalf("fork params changed with the original: %q", f.Param("id"))
	}
	if v, _ := f.Get("user"); v != "alice" {
		t.Fatalf("fork keys changed with the original: %v", v)
	}
	f.Set("only-fork", 1)
	if _, ok := c.Get("only-fork"); ok {
		t.Fatalf("fork writes leaked into the original")
	}
}

func TestFork_WithoutKeysAndNilReceiver(t *testing.T) {
	c := New(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	f := c.Fork(c.Writer, c.Request)
	if f.Keys != nil {
		t.Fatalf("fork allocated Keys for nothing")
	}
	f.Set("a", 1) // must not panic on the nil map

	var nilCtx *Context
	w := httptest.NewRecorder()
	if got := nilCtx.Fork(w, nil); got == nil || got.Writer != w {
		t.Fatalf("Fork on a nil context must return a usable context")
	}
}

func TestJoin_CarriesOutcomeBack(t *testing.T) {
	rec := httptest.NewRecorder()
	c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Set("before", true)

	f := c.Fork(rec, c.Request)
	f.Set("user", "alice")
	_ = f.String(http.StatusCreated, "made")
	f.Abort()
	f.SetDetached()

	if c.HeaderWritten() || c.StatusCode() != 0 || c.IsAborted() {
		t.Fatalf("original changed before Join")
	}
	c.Join(f)

	if !c.HeaderWritten() || c.StatusCode() != http.StatusCreated {
		t.Fatalf("Join lost the response state: written=%v status=%d", c.HeaderWritten(), c.StatusCode())
	}
	if !c.IsAborted() || !c.IsDetached() {
		t.Fatalf("Join lost abort / detached flags")
	}
	if v, _ := c.Get("user"); v != "alice" {
		t.Fatalf("Join lost values set by the forked handlers")
	}
	if v, _ := c.Get("before"); v != true {
		t.Fatalf("Join lost values set before the fork")
	}

	// A fork that never used Keys leaves the original's Keys alone.
	c2 := New(rec, c.Request)
	c2.Set("keep", 1)
	bare := &Context{}
	c2.Join(bare)
	if v, _ := c2.Get("keep"); v != 1 {
		t.Fatalf("Join with a nil Keys map dropped the original Keys")
	}

	// nil safety
	var nilCtx *Context
	nilCtx.Join(f)
	c.Join(nil)
}
