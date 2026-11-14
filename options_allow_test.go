package kvolt

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestOptions_Returns204WithAllowHeader(t *testing.T) {
	app := New()
	app.HandleMethodNotAllowed = true

	app.GET("/users", func(c *context.Context) error {
		return c.String(http.StatusOK, "get users")
	})
	app.POST("/users", func(c *context.Context) error {
		return c.String(http.StatusCreated, "post users")
	})

	req := httptest.NewRequest(http.MethodOptions, "/users", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for OPTIONS, got %d", rec.Code)
	}

	allow := rec.Header().Get("Allow")
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "POST") || !strings.Contains(allow, "OPTIONS") {
		t.Fatalf("expected Allow header to contain GET, POST, OPTIONS; got %q", allow)
	}
}
