package kvolt

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestPATCHAndMethodNotAllowed(t *testing.T) {
	app := New()
	app.PATCH("/items/:id", func(c *context.Context) error {
		return c.String(200, "patched:"+c.Param("id"))
	})

	req := httptest.NewRequest(http.MethodPatch, "/items/1", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "patched:1" {
		t.Fatalf("PATCH: %d %q", rec.Code, rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/items/1", nil)
	rec2 := httptest.NewRecorder()
	app.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d allow=%q body=%q", rec2.Code, rec2.Header().Get("Allow"), rec2.Body.String())
	}
	if allow := rec2.Header().Get("Allow"); allow == "" {
		t.Fatal("expected Allow header")
	}
}
