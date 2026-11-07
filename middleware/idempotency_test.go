package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/golang-jwt/jwt/v5"
)

func TestIdempotency_Replays(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	mw := Idempotency(IdempotencyConfig{Store: store, TTL: time.Hour})
	hits := 0
	handler := func(c *context.Context) error {
		hits++
		return c.JSON(201, map[string]string{"id": "1"})
	}

	run := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/orders", nil)
		r.Header.Set(IdempotencyKeyHeader, "key-abc")
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{handler}
		_ = mw(c)
		return w
	}

	w1 := run()
	if w1.Code != 201 || hits != 1 {
		t.Fatalf("first: code=%d hits=%d", w1.Code, hits)
	}
	w2 := run()
	if w2.Code != 201 || hits != 1 {
		t.Fatalf("replay should not re-run handler: code=%d hits=%d", w2.Code, hits)
	}
	if w2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatal("missing Idempotent-Replayed header")
	}
	if w1.Body.String() != w2.Body.String() {
		t.Fatalf("body mismatch %q vs %q", w1.Body.String(), w2.Body.String())
	}
}

func TestIdempotency_Required(t *testing.T) {
	mw := Idempotency(IdempotencyConfig{
		Store:    NewMemoryIdempotencyStore(),
		Required: true,
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/x", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	_ = mw(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", w.Code)
	}
}

func TestIdempotency_SkipsSafeMethods(t *testing.T) {
	mw := Idempotency(IdempotencyConfig{Store: NewMemoryIdempotencyStore(), Required: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/x", nil)
	c := context.New(w, r)
	hit := false
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error {
		hit = true
		return c.String(200, "ok")
	}}
	_ = mw(c)
	if !hit || w.Code != 200 {
		t.Fatalf("GET should pass without key, hit=%v code=%d", hit, w.Code)
	}
}

func TestIdempotency_DoesNotCache5xx(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	mw := Idempotency(IdempotencyConfig{Store: store, TTL: time.Hour})
	hits := 0
	handler := func(c *context.Context) error {
		hits++
		return c.String(503, "busy")
	}
	run := func() {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/orders", nil)
		r.Header.Set(IdempotencyKeyHeader, "key-5xx")
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{handler}
		_ = mw(c)
	}
	run()
	run()
	if hits != 2 {
		t.Fatalf("5xx must not be replayed; hits=%d want 2", hits)
	}
}

func TestIdempotency_SeparateUsersSameKey(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	mw := Idempotency(IdempotencyConfig{Store: store, TTL: time.Hour})
	hits := 0
	handler := func(c *context.Context) error {
		hits++
		raw, _ := c.Get("user")
		sub := raw.(jwt.MapClaims)["sub"].(string)
		return c.JSON(200, map[string]string{"who": sub})
	}
	runAs := func(sub string) string {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/orders", nil)
		r.Header.Set(IdempotencyKeyHeader, "shared-key")
		c := context.New(w, r)
		c.Set("user", jwt.MapClaims{"sub": sub})
		c.Handlers = []context.HandlerFunc{handler}
		_ = mw(c)
		return w.Body.String()
	}
	b1 := runAs("alice")
	b2 := runAs("bob")
	if hits != 2 {
		t.Fatalf("different users must not share replay; hits=%d", hits)
	}
	if b1 == b2 {
		t.Fatalf("bodies should differ by user: %q", b1)
	}
}

func TestIdempotency_ConcurrentSingleFlight(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	mw := Idempotency(IdempotencyConfig{Store: store, TTL: time.Hour})
	var hits atomic.Int32
	handler := func(c *context.Context) error {
		time.Sleep(20 * time.Millisecond)
		hits.Add(1)
		return c.String(200, "ok")
	}
	const n = 8
	done := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func() {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/orders", nil)
			r.Header.Set(IdempotencyKeyHeader, "race-key")
			c := context.New(w, r)
			c.Handlers = []context.HandlerFunc{handler}
			_ = mw(c)
			done <- struct{}{}
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
	if hits.Load() != 1 {
		t.Fatalf("single-flight want 1 hit got %d", hits.Load())
	}
}

func TestIdempotency_StripsSetCookieOnReplay(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	mw := Idempotency(IdempotencyConfig{Store: store, TTL: time.Hour})
	handler := func(c *context.Context) error {
		c.Writer.Header().Set("Set-Cookie", "session=abc; Path=/")
		return c.String(200, "ok")
	}
	run := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/login", nil)
		r.Header.Set(IdempotencyKeyHeader, "cookie-key")
		c := context.New(w, r)
		c.Handlers = []context.HandlerFunc{handler}
		_ = mw(c)
		return w
	}
	_ = run()
	w2 := run()
	if w2.Header().Get("Set-Cookie") != "" {
		t.Fatalf("replay must strip Set-Cookie, got %q", w2.Header().Get("Set-Cookie"))
	}
	if w2.Body.String() != "ok" {
		t.Fatalf("body=%q", w2.Body.String())
	}
}
