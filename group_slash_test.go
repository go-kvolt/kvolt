package kvolt

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestGroup_SlashNormalization(t *testing.T) {
	app := New()

	// 1. Root route without leading slash
	app.GET("hello", func(c *context.Context) error {
		return c.String(http.StatusOK, "world")
	})

	// 2. Group without leading slash, route without leading slash
	v1 := app.Group("v1")
	v1.GET("items", func(c *context.Context) error {
		return c.String(http.StatusOK, "v1 items")
	})

	// 3. Group with trailing slash, route with leading slash
	v2 := app.Group("/v2/")
	v2.GET("/users", func(c *context.Context) error {
		return c.String(http.StatusOK, "v2 users")
	})

	// Test GET /hello
	rec1 := httptest.NewRecorder()
	app.ServeHTTP(rec1, httptest.NewRequest("GET", "/hello", nil))
	if rec1.Code != 200 || rec1.Body.String() != "world" {
		t.Fatalf("expected /hello to match, got %d %s", rec1.Code, rec1.Body.String())
	}

	// Test GET /v1/items
	rec2 := httptest.NewRecorder()
	app.ServeHTTP(rec2, httptest.NewRequest("GET", "/v1/items", nil))
	if rec2.Code != 200 || rec2.Body.String() != "v1 items" {
		t.Fatalf("expected /v1/items to match, got %d %s", rec2.Code, rec2.Body.String())
	}

	// Test GET /v2/users
	rec3 := httptest.NewRecorder()
	app.ServeHTTP(rec3, httptest.NewRequest("GET", "/v2/users", nil))
	if rec3.Code != 200 || rec3.Body.String() != "v2 users" {
		t.Fatalf("expected /v2/users to match, got %d %s", rec3.Code, rec3.Body.String())
	}
}
