package context

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

	if len(c.Keys) != 0 || c.Handlers != nil || len(c.Params) != 0 {
		t.Error("Reset: Keys should be empty, Handlers nil, Params empty")
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

func TestContext_Next_HandlerError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)
	c.Handlers = []HandlerFunc{
		func(c *Context) error {
			return fmt.Errorf("something went wrong")
		},
	}
	c.Next()
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Next handler error: want status %d, got %d", http.StatusInternalServerError, w.Code)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Next handler error: want application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestContext_Query(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/search?q=foo", nil)
	c := New(w, r)
	if c.Query("q") != "foo" {
		t.Errorf("Query(q) want foo, got %s", c.Query("q"))
	}
}

func TestContext_StatusThenJSON_SetsContentType(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)
	_ = c.Status(404).JSON(404, map[string]string{"error": "Not Found"})
	if w.Code != 404 {
		t.Errorf("status want 404, got %d", w.Code)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Content-Type missing json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestContext_StringSprintf(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)
	_ = c.String(200, "hi %s", "meera")
	if w.Body.String() != "hi meera" {
		t.Errorf("String sprintf: got %q", w.Body.String())
	}
}

func TestContext_BindJSON_SkipsValidation(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"party":"Meera","amount_paise":1}`))
	r.Header.Set("Content-Type", "application/json")
	c := New(w, r)
	var req struct {
		Party       string `json:"party"`
		AmountPaise int64  `json:"amount_paise"`
	}
	if err := c.BindJSON(&req); err != nil {
		t.Fatal(err)
	}
	if req.Party != "Meera" || req.AmountPaise != 1 {
		t.Errorf("BindJSON: %+v", req)
	}
}
