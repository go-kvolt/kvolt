package swagger

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/router"
)

func TestHandler_Disabled(t *testing.T) {
	handler := Handler(Config{Disabled: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/swagger/", nil)
	c := context.New(w, r)
	c.Params = router.Params{{Key: "any", Value: "index.html"}}
	err := handler(c)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if w.Code != 404 {
		t.Errorf("Disabled: want 404, got %d", w.Code)
	}
}

func TestHandler_DocJSON(t *testing.T) {
	handler := Handler(Config{SpecJSON: `{"openapi":"3.0.0"}`, Title: "Test"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/swagger/doc.json", nil)
	c := context.New(w, r)
	c.Params = router.Params{{Key: "any", Value: "doc.json"}}
	err := handler(c)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if w.Code != 200 {
		t.Errorf("doc.json: want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "openapi") {
		t.Errorf("doc.json: body should contain openapi, got %s", w.Body.String())
	}

	// Doc JSON with trailing slash
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/swagger/doc.json/", nil)
	c2 := context.New(w2, r2)
	c2.Params = router.Params{{Key: "any", Value: "doc.json/"}}
	if err := handler(c2); err != nil {
		t.Fatalf("handler trailing slash: %v", err)
	}
	if w2.Code != 200 {
		t.Errorf("doc.json/: want 200, got %d", w2.Code)
	}
}

func TestHandler_IndexHTML(t *testing.T) {
	handler := Handler(Config{Title: "My API"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/swagger/index.html", nil)
	c := context.New(w, r)
	c.Params = router.Params{{Key: "any", Value: "index.html"}}
	err := handler(c)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if w.Code != 200 {
		t.Errorf("index.html: want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "My API") {
		t.Errorf("index.html: body should contain title")
	}
}
