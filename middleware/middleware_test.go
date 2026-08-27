package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
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
		ContentTypeNosniff: "nosniff",
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

func TestMaxBodySize(t *testing.T) {
	// Request with no body: middleware should not affect behavior
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "OK") }}
	MaxBodySize(1024)(c)
	if w.Code != 200 {
		t.Errorf("MaxBodySize: want 200, got %d", w.Code)
	}
}

func TestMaxBodySizeBytes(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "OK") }}
	MaxBodySizeBytes(1024)(c)
	if w.Code != 200 {
		t.Errorf("MaxBodySizeBytes: want 200, got %d", w.Code)
	}
}

func TestRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "abc-123")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "OK") }}
	RequestID()(c)
	if w.Header().Get("X-Request-ID") != "abc-123" {
		t.Errorf("RequestID: want abc-123, got %s", w.Header().Get("X-Request-ID"))
	}
}

func TestLimiter_SameHostDifferentPorts(t *testing.T) {
	lim := Limiter(1, 1)
	hit429 := false
	for _, addr := range []string{"1.2.3.4:1111", "1.2.3.4:2222"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = addr
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "OK") }}
		lim(c)
		if w.Code == 429 {
			hit429 = true
		}
	}
	if !hit429 {
		t.Fatal("Limiter should key on IP host, not port")
	}
}

func TestGzip_SkipsJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		Gzip(),
		func(c *context.Context) error {
			return c.JSON(200, map[string]string{"ok": "yes"})
		},
	}
	c.Next()
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("Gzip should skip application/json")
	}
}
