package kvolt

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func serve(app *Engine, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

// A GET route answers HEAD, so health checks and link checkers that use HEAD work
// without registering every route twice.
func TestHEAD_AnsweredByGETRoute(t *testing.T) {
	app := New()
	app.GET("/health", func(c *context.Context) error {
		c.Writer.Header().Set("X-Checked", "yes")
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	app.GET("/users/:id", func(c *context.Context) error {
		return c.String(http.StatusOK, "user "+c.Param("id"))
	})
	app.POST("/orders", func(c *context.Context) error { return c.NoContent() })
	app.GET("/dir/", func(c *context.Context) error { return c.String(http.StatusOK, "dir") })

	rec := serve(app, http.MethodHead, "/health")
	if rec.Code != http.StatusOK || rec.Header().Get("X-Checked") != "yes" {
		t.Fatalf("HEAD /health = %d, headers %v; want 200 with the GET handler's headers", rec.Code, rec.Header())
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("HEAD response lost Content-Type")
	}

	// Params are filled from the GET route.
	rec = serve(app, http.MethodHead, "/users/42")
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD /users/42 = %d, want 200", rec.Code)
	}

	// A route with no GET does not answer HEAD.
	rec = serve(app, http.MethodHead, "/orders")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("HEAD /orders = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "OPTIONS, POST" {
		t.Fatalf("Allow = %q, want OPTIONS, POST", allow)
	}

	// Allow advertises HEAD wherever GET exists.
	rec = serve(app, http.MethodPost, "/health")
	if allow := rec.Header().Get("Allow"); rec.Code != http.StatusMethodNotAllowed || allow != "GET, HEAD, OPTIONS" {
		t.Fatalf("POST /health = %d Allow %q, want 405 with GET, HEAD, OPTIONS", rec.Code, allow)
	}

	// Trailing-slash redirect follows the GET route too.
	rec = serve(app, http.MethodHead, "/dir")
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/dir/" {
		t.Fatalf("HEAD /dir = %d Location %q, want 301 to /dir/", rec.Code, rec.Header().Get("Location"))
	}

	if rec := serve(app, http.MethodHead, "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("HEAD /nope = %d, want 404", rec.Code)
	}
}

// An explicitly registered HEAD route still wins over the GET route.
func TestHEAD_ExplicitRouteWins(t *testing.T) {
	app := New()
	app.GET("/file", func(c *context.Context) error { return c.String(http.StatusOK, "get") })
	app.HEAD("/file", func(c *context.Context) error {
		c.Writer.Header().Set("X-From", "head")
		return c.NoContent()
	})
	rec := serve(app, http.MethodHead, "/file")
	if rec.Code != http.StatusNoContent || rec.Header().Get("X-From") != "head" {
		t.Fatalf("HEAD /file = %d X-From %q, want the HEAD handler", rec.Code, rec.Header().Get("X-From"))
	}
}

// An empty path segment is not a parameter value: handlers never see :id == "".
func TestParam_EmptySegmentDoesNotMatch(t *testing.T) {
	app := New()
	seen := "unset"
	app.GET("/users/:id/posts", func(c *context.Context) error {
		seen = c.Param("id")
		return c.String(http.StatusOK, "posts")
	})
	app.GET("/items/:id", func(c *context.Context) error {
		seen = c.Param("id")
		return c.String(http.StatusOK, "item")
	})

	if rec := serve(app, http.MethodGet, "/users//posts"); rec.Code != http.StatusNotFound {
		t.Fatalf("GET /users//posts = %d, want 404", rec.Code)
	}
	if rec := serve(app, http.MethodGet, "/items/"); rec.Code != http.StatusNotFound {
		t.Fatalf("GET /items/ = %d, want 404", rec.Code)
	}
	if seen != "unset" {
		t.Fatalf("handler ran with id = %q", seen)
	}
	if rec := serve(app, http.MethodGet, "/users/7/posts"); rec.Code != http.StatusOK || seen != "7" {
		t.Fatalf("GET /users/7/posts = %d id %q, want 200 with id 7", rec.Code, seen)
	}
}
