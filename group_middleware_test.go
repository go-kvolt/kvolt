package kvolt

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestRouterGroup_MiddlewareIsolatedBetweenSiblings(t *testing.T) {
	app := New()

	api := app.Group("/api")
	api.Use(func(c *context.Context) error {
		c.Set("auth", true)
		c.Next()
		return nil
	})

	exec := api.Group("/executive")
	exec.Use(func(c *context.Context) error {
		c.Set("role", "EXECUTIVE")
		c.Next()
		return nil
	})
	exec.GET("/ping", func(c *context.Context) error {
		if _, ok := c.Get("auth"); !ok {
			t.Fatal("executive route missing auth middleware")
		}
		if c.MustGet("role") != "EXECUTIVE" {
			t.Fatal("executive route missing role middleware")
		}
		return c.String(200, "exec")
	})

	designer := api.Group("/designer")
	designer.Use(func(c *context.Context) error {
		c.Set("role", "DESIGNER")
		c.Next()
		return nil
	})
	designer.GET("/ping", func(c *context.Context) error {
		if _, ok := c.Get("auth"); !ok {
			t.Fatal("designer route missing auth middleware")
		}
		if role, _ := c.Get("role"); role == "EXECUTIVE" {
			t.Fatal("designer route inherited executive role middleware")
		}
		if c.MustGet("role") != "DESIGNER" {
			t.Fatal("designer route missing designer role middleware")
		}
		return c.String(200, "designer")
	})

	// Mutating executive middleware after designer registration must not affect designer.
	exec.Use(func(c *context.Context) error {
		c.Set("extra", "exec-only")
		c.Next()
		return nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/designer/ping", nil)
	app.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("designer ping: status %d body %s", w.Code, w.Body.String())
	}
}
