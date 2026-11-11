package context

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindForm_ParseErrors(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader("x"))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
	c := New(httptest.NewRecorder(), r)
	if err := c.BindForm(&struct {
		A string `form:"a"`
	}{}); err == nil {
		t.Fatal("want multipart parse error")
	}
}

func TestStatusCode_HeaderWrittenDefault(t *testing.T) {
	w := httptest.NewRecorder()
	c := New(w, httptest.NewRequest("GET", "/", nil))
	c.writeHeader(0) // defaults to 200
	if c.StatusCode() != 200 {
		t.Fatalf("got %d", c.StatusCode())
	}
	c.writeHeader(201) // already written — no-op
}

func TestBind_JSONError(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader("{"))
	r.Header.Set("Content-Type", "application/json")
	c := New(httptest.NewRecorder(), r)
	var m map[string]any
	if err := c.Bind(&m); err == nil {
		t.Fatal("want bind json error")
	}
}

func TestNext_Aborted(t *testing.T) {
	c := New(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	c.Abort()
	c.Handlers = []HandlerFunc{func(c *Context) error { t.Fatal("should not run"); return nil }}
	c.Next()
}

func TestRedirectURL_AndSafeRelative(t *testing.T) {
	w := httptest.NewRecorder()
	c := New(w, httptest.NewRequest("GET", "http://ex.com/", nil))
	if err := c.RedirectURL(302, "https://ex.com/x"); err != nil {
		t.Fatal(err)
	}
	w2 := httptest.NewRecorder()
	c2 := New(w2, httptest.NewRequest("GET", "/", nil))
	if err := c2.Redirect(302, "//evil.com"); err == nil {
		t.Fatal("want unsafe")
	}
	if err := c2.Redirect(302, "/ok"); err != nil {
		t.Fatal(err)
	}
}

func TestSSEvent_EmptyFields(t *testing.T) {
	w := httptest.NewRecorder()
	c := New(w, httptest.NewRequest("GET", "/", nil))
	if err := c.SSEvent(SSEvent{Data: "x", Event: "e\nf", ID: "1\n2", Retry: 1000}); err != nil {
		t.Fatal(err)
	}
}
