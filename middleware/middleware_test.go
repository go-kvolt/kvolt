package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/context"
)

func TestSecure(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "OK") }}

	Secure()(c)

	h := w.Header()
	if h.Get("X-Frame-Options") != "DENY" {
		t.Errorf("X-Frame-Options want DENY, got %s", h.Get("X-Frame-Options"))
	}
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options want nosniff, got %s", h.Get("X-Content-Type-Options"))
	}
	if h.Get("X-XSS-Protection") != "1; mode=block" {
		t.Errorf("X-XSS-Protection want 1; mode=block, got %s", h.Get("X-XSS-Protection"))
	}
}

func TestSecureWithConfig(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "OK") }}

	SecureWithConfig(SecureConfig{
		XFrameOptions:      "SAMEORIGIN",
		ContentTypeNosniff:  "nosniff",
		XSSProtection:      "1; mode=block",
	})(c)

	if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options want SAMEORIGIN, got %s", w.Header().Get("X-Frame-Options"))
	}
}

func TestRecovery(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	panicMsg := "test panic"
	c.Handlers = []context.HandlerFunc{
		func(c *context.Context) error {
			panic(panicMsg)
		},
	}

	Recovery()(c)

	if w.Code != 500 {
		t.Errorf("Recovery: want status 500, got %d", w.Code)
	}
	if w.Body.String() != "Internal Server Error" {
		t.Errorf("Recovery: body want Internal Server Error, got %s", w.Body.String())
	}
}
