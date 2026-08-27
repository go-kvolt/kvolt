package kvolt

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestNew(t *testing.T) {
	app := New()
	if app == nil {
		t.Fatal("New: want non-nil Engine")
	}
	if app.router == nil {
		t.Fatal("New: router should be initialized")
	}
}

func TestEngine_ServeHTTP(t *testing.T) {
	app := New()
	app.GET("/ok", func(c *context.Context) error {
		return c.String(200, "OK")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ok", nil)
	app.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Errorf("ServeHTTP: want 200, got %d", w.Code)
	}
	if w.Body.String() != "OK" {
		t.Errorf("ServeHTTP: body want OK, got %s", w.Body.String())
	}
}

func TestEngine_ServeHTTP_404(t *testing.T) {
	app := New()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/notfound", nil)
	app.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("ServeHTTP 404: want 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Not Found") {
		t.Errorf("ServeHTTP 404 body: %s", w.Body.String())
	}
}

func TestDefault_RequestID(t *testing.T) {
	app := Default()
	app.GET("/ok", func(c *context.Context) error {
		return c.String(200, "OK")
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ok", nil)
	app.ServeHTTP(w, r)
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("Default: missing X-Request-ID")
	}
}

func TestEngine_Routes(t *testing.T) {
	app := New()
	app.GET("/a", func(c *context.Context) error { return nil })
	app.POST("/b", func(c *context.Context) error { return nil })
	routes := app.Routes()
	if len(routes) < 2 {
		t.Errorf("Routes: want at least 2, got %d", len(routes))
	}
}
