package context

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/router"
)

func TestContext_New(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)
	if c.Writer != w || c.Request != r {
		t.Error("New: Writer or Request not set")
	}
	if c.index != -1 {
		t.Errorf("New: index want -1, got %d", c.index)
	}
}

func TestContext_SetGet(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)

	c.Set("foo", "bar")
	val, ok := c.Get("foo")
	if !ok || val != "bar" {
		t.Errorf("Get: want bar true, got %v %v", val, ok)
	}

	_, ok = c.Get("missing")
	if ok {
		t.Error("Get missing key should return false")
	}
}

func TestContext_MustGet(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)
	c.Set("k", "v")
	if c.MustGet("k") != "v" {
		t.Error("MustGet: want v")
	}
	defer func() {
		if recover() == nil {
			t.Error("MustGet missing key should panic")
		}
	}()
	c.MustGet("nonexistent")
}

func TestContext_Reset(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)
	c.Set("x", 1)
	c.Handlers = []HandlerFunc{nil}
	c.Params = router.Params{{Key: "id", Value: "1"}}

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("POST", "/other", nil)
	c.Reset(w2, r2)

	if c.Keys != nil || c.Handlers != nil || c.Params != nil {
		t.Error("Reset: Keys, Handlers, Params should be nil")
	}
	if c.index != -1 || c.Writer != w2 || c.Request != r2 {
		t.Error("Reset: index or Writer/Request not reset")
	}
}

func TestContext_Param(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/user/42", nil)
	c := New(w, r)
	c.Params = router.Params{{Key: "id", Value: "42"}}
	if c.Param("id") != "42" {
		t.Errorf("Param(id) want 42, got %s", c.Param("id"))
	}
	if c.Param("missing") != "" {
		t.Error("Param(missing) should return empty")
	}
}
