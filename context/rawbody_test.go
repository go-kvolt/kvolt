package context

import (
	"bytes"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

func TestRawBody_AllowsRebind(t *testing.T) {
	body := `{"name":"kvolt"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	c := New(w, r)

	raw, err := c.RawBody()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Fatalf("raw want %s got %s", body, raw)
	}
	raw2, err := c.RawBody()
	if err != nil || !bytes.Equal(raw, raw2) {
		t.Fatalf("second RawBody mismatch: %v %s", err, raw2)
	}

	var obj struct {
		Name string `json:"name"`
	}
	if err := c.BindJSON(&obj); err != nil {
		t.Fatal(err)
	}
	if obj.Name != "kvolt" {
		t.Fatalf("bind after RawBody: %+v", obj)
	}
}

func TestWriteHandlerError_HTTPErr(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)
	c.Handlers = []HandlerFunc{
		func(c *Context) error {
			return httperr.BadRequest("bad_request", "nope")
		},
	}
	c.Next()
	if w.Code != 400 {
		t.Fatalf("status want 400 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRawBody_Empty(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Body = io.NopCloser(bytes.NewReader(nil))
	c := New(w, r)
	b, err := c.RawBody()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Fatalf("want empty, got %q", b)
	}
}
